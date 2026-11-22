package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// sseServer answers POST /v1/messages with a real Messages API event stream, one frame per piece, flushed.
func sseServer(t *testing.T, pieces []string, stop string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(event, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			w.(http.Flusher).Flush()
		}
		send("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"usage":{"input_tokens":12,"output_tokens":1}}}`)
		send("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		for _, p := range pieces {
			send("content_block_delta", fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, p))
		}
		send("content_block_stop", `{"type":"content_block_stop","index":0}`)
		send("message_delta", fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q},"usage":{"output_tokens":9}}`, stop))
		send("message_stop", `{"type":"message_stop"}`)
	}))
}

func streamProvider(t *testing.T, url string) *Provider {
	t.Helper()
	p, err := New(Config{APIKey: "test-key", Model: "claude-opus-5", BaseURL: url, MaxRetries: 0})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGenerateStream_DeliversEachPieceInOrder(t *testing.T) {
	srv := sseServer(t, []string{"J’ai trouvé ", "2 offres ", "à Gombe."}, "end_turn")
	defer srv.Close()
	var got []string
	resp, err := streamProvider(t, srv.URL).GenerateStream(context.Background(), domain.AIRequest{
		Messages: []domain.AIMessage{{Role: "user", Content: "hôtel à Gombe"}},
	}, func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "J’ai trouvé |2 offres |à Gombe." {
		t.Fatalf("pieces: %q", got)
	}
	if resp.Content != "J’ai trouvé 2 offres à Gombe." || resp.ProviderID != ProviderID {
		t.Fatalf("response: %+v", resp)
	}
}

func TestGenerateStream_TruncatedOrRefusedIsAnError(t *testing.T) {
	for _, stop := range []string{"max_tokens", "refusal"} {
		srv := sseServer(t, []string{"partiel"}, stop)
		_, err := streamProvider(t, srv.URL).GenerateStream(context.Background(), domain.AIRequest{
			Messages: []domain.AIMessage{{Role: "user", Content: "x"}},
		}, nil)
		srv.Close()
		if err == nil {
			t.Fatalf("%s: a truncated or refused answer must not be kept", stop)
		}
	}
}

func TestOutOfCreditIsProviderUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API."}}`))
	}))
	defer srv.Close()
	_, err := streamProvider(t, srv.URL).GenerateStream(context.Background(), domain.AIRequest{
		Messages: []domain.AIMessage{{Role: "user", Content: "x"}},
	}, nil)
	aiErr, ok := err.(domain.AIError)
	if !ok || aiErr.Code != domain.AIErrProviderUnavailable {
		t.Fatalf("an account without credit is unavailable (not a bad request), got %#v", err)
	}
}
