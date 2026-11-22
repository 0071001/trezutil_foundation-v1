package gateway

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// A streamed answer must cross the gateway event by event (not buffered until the end), and may last longer than
// the ordinary upstream timeout.
func TestStreamRouteIsPassedOnEventByEventAndOutlivesTheUpstreamTimeout(t *testing.T) {
	release := make(chan struct{})
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: stage\ndata: {\"stage\":\"interpreting\"}\n\n"))
		w.(http.Flusher).Flush()
		select {
		case <-release: // sent only once the client has read the first event through the gateway
		case <-time.After(5 * time.Second):
		}
		time.Sleep(400 * time.Millisecond) // longer than the upstream timeout below
		_, _ = w.Write([]byte("event: done\ndata: {}\n\n"))
	}))
	t.Cleanup(conv.Close)

	g := New(Config{
		WebRoot:         t.TempDir(),
		Upstreams:       map[string]*Upstream{"conversation": {Name: "conversation", URL: mustURL(t, conv.URL)}},
		UpstreamTimeout: 200 * time.Millisecond,
		RequestTimeout:  10 * time.Second,
	}, zerolog.Nop())
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/conversations/message/stream", strings.NewReader(`{"message_text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	select {
	case l := <-lines:
		if l != "event: stage" {
			t.Fatalf("first line %q", l)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the first event did not arrive before the end of the answer: the gateway buffers the stream")
	}
	close(release)
	var rest []string
	for l := range lines {
		rest = append(rest, l)
	}
	if !strings.Contains(strings.Join(rest, "\n"), "event: done") {
		t.Fatalf("the stream was cut before its end (upstream timeout applied?): %q", rest)
	}
}
