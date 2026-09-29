// Package anthropicfake emulates the subset of the Anthropic Messages API the
// Trezutil provider uses (POST /v1/messages with structured output). It is
// for contract tests and local E2E runs without credentials; it is not a
// model and must never be deployed.
package anthropicfake

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
)

// Reply is what a Responder returns for one request.
type Reply struct {
	Status  int               // default 200
	Body    interface{}       // default Message(...) built from Text
	Text    string            // assistant text for a 200 reply
	Stop    string            // stop_reason, default "end_turn"
	Headers map[string]string // extra response headers
}

// Responder decides the reply for a decoded request body.
type Responder func(req map[string]interface{}) Reply

// Handler serves the emulated API; mount it on any listener.
type Handler struct {
	APIKey  string
	Respond Responder

	mu       sync.Mutex
	requests []map[string]interface{}
	headers  []http.Header
}

// Server is a Handler on an httptest server.
type Server struct {
	*httptest.Server
	*Handler
}

// New starts an emulator requiring apiKey and answering with respond
// (Classifier when nil).
func New(apiKey string, respond Responder) *Server {
	h := &Handler{APIKey: apiKey, Respond: respond}
	return &Server{Server: httptest.NewServer(h), Handler: h}
}

// Requests returns the decoded request bodies received so far.
func (h *Handler) Requests() []map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]map[string]interface{}{}, h.requests...)
}

// Headers returns the request headers received so far.
func (h *Handler) Headers() []http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]http.Header{}, h.headers...)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
		writeJSON(w, http.StatusNotFound, apiError("not_found_error", "Not found"), nil)
		return
	}
	if r.Header.Get("x-api-key") != h.APIKey || r.Header.Get("anthropic-version") == "" {
		writeJSON(w, http.StatusUnauthorized, apiError("authentication_error", "invalid x-api-key"), nil)
		return
	}
	raw, _ := io.ReadAll(r.Body)
	var req map[string]interface{}
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError("invalid_request_error", "malformed JSON"), nil)
		return
	}
	h.mu.Lock()
	h.requests = append(h.requests, req)
	h.headers = append(h.headers, r.Header.Clone())
	h.mu.Unlock()

	respond := h.Respond
	if respond == nil {
		respond = Classifier
	}
	reply := respond(req)
	if reply.Status == 0 {
		reply.Status = http.StatusOK
	}
	body := reply.Body
	if body == nil && reply.Status == http.StatusOK {
		model, _ := req["model"].(string)
		body = Message(model, reply.Text, reply.Stop)
	}
	writeJSON(w, reply.Status, body, reply.Headers)
}

// Message builds a Messages API response.
func Message(model, text, stop string) map[string]interface{} {
	if stop == "" {
		stop = "end_turn"
	}
	return map[string]interface{}{
		"id": "msg_fake", "type": "message", "role": "assistant", "model": model,
		"content":       []map[string]interface{}{{"type": "text", "text": text}},
		"stop_reason":   stop,
		"stop_sequence": nil,
		"usage":         map[string]interface{}{"input_tokens": 120, "output_tokens": 30},
	}
}

// Error builds a reply carrying an API error.
func Error(status int, typ, message string) Reply {
	return Reply{Status: status, Body: apiError(typ, message)}
}

func apiError(typ, message string) map[string]interface{} {
	return map[string]interface{}{"type": "error", "error": map[string]string{"type": typ, "message": message}}
}

// LastUserText returns the text of the final user turn.
func LastUserText(req map[string]interface{}) string {
	msgs, _ := req["messages"].([]interface{})
	for i := len(msgs) - 1; i >= 0; i-- {
		m, _ := msgs[i].(map[string]interface{})
		if m["role"] != "user" {
			continue
		}
		switch c := m["content"].(type) {
		case string:
			return c
		case []interface{}:
			var parts []string
			for _, b := range c {
				if bm, ok := b.(map[string]interface{}); ok {
					if t, ok := bm["text"].(string); ok {
						parts = append(parts, t)
					}
				}
			}
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

var (
	searchWords = regexp.MustCompile(`(?i)\b(find|search|looking for|show me|apartment|house|room|bedroom|car|phone|hotel|studio)\b`)
	fillers     = regexp.MustCompile(`(?i)\b(i am|i'm|please|can you|could you|find me|find|search for|search|looking for|show me|i want|i need|a|an|in)\b`)
	orderRef    = regexp.MustCompile(`(?i)\border\s+([0-9a-f-]{8,})`)
)

// Classifier is a deterministic stand-in for the model: it recognises
// listing searches and order lookups and answers NONE otherwise, in the
// structured format the provider requests.
func Classifier(req map[string]interface{}) Reply {
	text := LastUserText(req)
	out := map[string]interface{}{"intent": "NONE", "confidence": 0.9, "parameters": map[string]interface{}{}, "explanation": "not a supported read request"}
	switch {
	case orderRef.MatchString(text):
		out["intent"] = "GET_ORDER"
		out["parameters"] = map[string]interface{}{"order_id": orderRef.FindStringSubmatch(text)[1]}
		out["explanation"] = "order lookup"
	case searchWords.MatchString(text):
		q := strings.Join(strings.Fields(fillers.ReplaceAllString(text, " ")), " ")
		out["intent"] = "SEARCH_LISTINGS"
		out["parameters"] = map[string]interface{}{"text": strings.Trim(q, " ?.!")}
		out["explanation"] = "listing search"
	}
	b, _ := json.Marshal(out)
	return Reply{Text: string(b)}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}, headers map[string]string) {
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
