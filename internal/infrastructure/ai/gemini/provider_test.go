package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

const testKey = "test-gemini-key-0123456789"

// serve emulates the two Gemini endpoints the provider uses.
func serve(t *testing.T, models []string, answer func(body map[string]interface{}) (int, interface{})) (*httptest.Server, *[]map[string]interface{}) {
	t.Helper()
	var seen []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != testKey {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"API key not valid. Please pass a valid API key."}}`)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/models/")
		if r.Method == http.MethodGet {
			for _, m := range models {
				if m == path {
					_, _ = io.WriteString(w, `{"name":"models/`+m+`"}`)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen = append(seen, body)
		status, reply := answer(body)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func okText(text string) (int, interface{}) {
	return 200, map[string]interface{}{
		"candidates":    []interface{}{map[string]interface{}{"content": map[string]interface{}{"parts": []interface{}{map[string]interface{}{"text": text}}}, "finishReason": "STOP"}},
		"usageMetadata": map[string]interface{}{"totalTokenCount": 42},
		"modelVersion":  "gemini-3.5-flash-lite",
	}
}

func newProvider(t *testing.T, base string) *Provider {
	t.Helper()
	p, err := New(Config{APIKey: testKey, BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func intentRequest() domain.AIRequest {
	return domain.AIRequest{
		Task:             domain.AITaskIntentClassification,
		SystemPrompt:     "Classify the request.",
		Messages:         []domain.AIMessage{{Role: "user", Content: "Je cherche un hôtel à Gombe"}},
		StructuredOutput: true,
		OutputSchema: &domain.JSONSchema{
			Properties: map[string]domain.JSONSchemaProperty{
				"intent":     {Type: "string", Enum: []string{"SEARCH_LISTINGS", "NONE"}},
				"confidence": {Type: "number"},
				"parameters": {Type: "object", Properties: map[string]domain.JSONSchemaProperty{"location": {Type: "string"}}},
			},
			Required: []string{"intent"},
		},
	}
}

func TestGenerateSendsAStructuredRequestAndDecodesTheAnswer(t *testing.T) {
	srv, seen := serve(t, nil, func(map[string]interface{}) (int, interface{}) {
		return okText(`{"intent":"SEARCH_LISTINGS","confidence":0.9,"parameters":{"location":"Gombe"}}`)
	})
	resp, err := newProvider(t, srv.URL).Generate(context.Background(), intentRequest())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Structured["intent"] != "SEARCH_LISTINGS" || resp.TokensUsed != 42 || resp.ProviderID != ProviderID {
		t.Fatalf("resp = %+v", resp)
	}
	body := (*seen)[0]
	cfg := body["generationConfig"].(map[string]interface{})
	if cfg["responseMimeType"] != "application/json" {
		t.Fatalf("generationConfig = %v", cfg)
	}
	schema := cfg["responseSchema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})
	if schema["type"] != "OBJECT" || props["intent"].(map[string]interface{})["type"] != "STRING" || props["confidence"].(map[string]interface{})["type"] != "NUMBER" {
		t.Fatalf("schema = %v", schema)
	}
	if body["systemInstruction"] == nil {
		t.Fatal("the system prompt must go to systemInstruction")
	}
	contents := body["contents"].([]interface{})
	if len(contents) != 1 || contents[0].(map[string]interface{})["role"] != "user" {
		t.Fatalf("contents = %v", contents)
	}
}

func TestGenerateMapsFailuresToGatewayErrors(t *testing.T) {
	cases := map[int]domain.AIErrorCode{
		http.StatusTooManyRequests:    domain.AIErrProviderRateLimited,
		http.StatusServiceUnavailable: domain.AIErrProviderUnavailable,
		http.StatusNotFound:           domain.AIErrModelUnavailable,
		http.StatusForbidden:          domain.AIErrProviderAuthFailed,
		http.StatusBadRequest:         domain.AIErrInvalidResponse,
	}
	for status, want := range cases {
		srv, _ := serve(t, nil, func(map[string]interface{}) (int, interface{}) {
			return status, map[string]interface{}{"error": map[string]interface{}{"message": "secret detail " + testKey}}
		})
		_, err := newProvider(t, srv.URL).Generate(context.Background(), intentRequest())
		var ae domain.AIError
		if !errors.As(err, &ae) || ae.Code != want {
			t.Errorf("HTTP %d: err = %v, want %s", status, err, want)
		}
		if err != nil && strings.Contains(err.Error(), testKey) {
			t.Errorf("HTTP %d: the error leaks provider detail", status)
		}
	}
}

func TestGenerateRejectsBlockedTruncatedAndEmptyAnswers(t *testing.T) {
	for name, reply := range map[string]interface{}{
		"truncated": map[string]interface{}{"candidates": []interface{}{map[string]interface{}{"finishReason": "MAX_TOKENS", "content": map[string]interface{}{"parts": []interface{}{map[string]interface{}{"text": "{"}}}}}},
		"blocked":   map[string]interface{}{"promptFeedback": map[string]interface{}{"blockReason": "SAFETY"}},
		"empty":     map[string]interface{}{"candidates": []interface{}{}},
	} {
		srv, _ := serve(t, nil, func(map[string]interface{}) (int, interface{}) { return 200, reply })
		if _, err := newProvider(t, srv.URL).Generate(context.Background(), intentRequest()); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	srv, _ := serve(t, nil, func(map[string]interface{}) (int, interface{}) { return okText("not json") })
	_, err := newProvider(t, srv.URL).Generate(context.Background(), intentRequest())
	var ae domain.AIError
	if !errors.As(err, &ae) || ae.Code != domain.AIErrSchemaValidationFailed {
		t.Fatalf("invalid JSON: err = %v", err)
	}
}

func TestVerifyDistinguishesBadModelFromBadKey(t *testing.T) {
	srv, _ := serve(t, []string{DefaultModel}, nil)
	if err := newProvider(t, srv.URL).Verify(context.Background()); err != nil {
		t.Fatalf("a served model verifies: %v", err)
	}
	other, _ := New(Config{APIKey: testKey, Model: "gemini-9-ultra", BaseURL: srv.URL})
	if err := other.Verify(context.Background()); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("unknown model: %v", err)
	}
	bad, _ := New(Config{APIKey: "wrong-key-wrong-key", BaseURL: srv.URL})
	if err := bad.Verify(context.Background()); !errors.Is(err, ErrKeyRejected) {
		t.Fatalf("bad key: %v", err)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("a missing key must be refused")
	}
	if _, err := New(Config{APIKey: testKey, Model: "claude-opus-5"}); !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("a Claude id is not a Gemini model: %v", err)
	}
	p, err := New(Config{APIKey: testKey})
	if err != nil || p.Models()[0].ModelID != DefaultModel {
		t.Fatalf("default model: %v %v", err, p)
	}
}

func TestConversationHistoryAlternatesFromAUserTurn(t *testing.T) {
	_, contents := convertMessages(domain.AIRequest{Messages: []domain.AIMessage{
		{Role: "assistant", Content: "Bonjour"}, {Role: "user", Content: "un hôtel"}, {Role: "user", Content: "à Gombe"}, {Role: "assistant", Content: "ok"},
	}})
	if len(contents) != 1 || contents[0].Role != "user" || !strings.Contains(contents[0].Parts[0].Text, "à Gombe") {
		t.Fatalf("contents = %+v", contents)
	}
}

func TestGenerateRetriesOnceOnAHighDemandAnswer(t *testing.T) {
	calls := 0
	srv, _ := serve(t, nil, func(map[string]interface{}) (int, interface{}) {
		calls++
		if calls == 1 {
			return http.StatusServiceUnavailable, map[string]interface{}{"error": map[string]interface{}{"status": "UNAVAILABLE"}}
		}
		return okText(`{"intent":"SEARCH_LISTINGS"}`)
	})
	p := newProvider(t, srv.URL)
	p.retryDelay = time.Millisecond
	if _, err := p.Generate(context.Background(), intentRequest()); err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	calls = -10 // keeps failing: one retry only, then the error is returned for the failover
	srv2, _ := serve(t, nil, func(map[string]interface{}) (int, interface{}) {
		calls++
		return http.StatusServiceUnavailable, map[string]interface{}{}
	})
	p2 := newProvider(t, srv2.URL)
	p2.retryDelay = time.Millisecond
	calls = 0
	if _, err := p2.Generate(context.Background(), intentRequest()); err == nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
