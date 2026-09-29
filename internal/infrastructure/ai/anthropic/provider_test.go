package anthropic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/ai/anthropic/anthropicfake"
)

const testKey = "sk-ant-test"

func newProvider(t *testing.T, url string, mod func(*Config)) *Provider {
	t.Helper()
	cfg := Config{APIKey: testKey, BaseURL: url, MaxRetries: 0, RequestTimeout: 2 * time.Second}
	if mod != nil {
		mod(&cfg)
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func classifyRequest(text string) domain.AIRequest {
	return domain.AIRequest{
		Task:             domain.AITaskIntentClassification,
		SystemPrompt:     "classify",
		Messages:         []domain.AIMessage{{Role: "system", Content: "classify"}, {Role: "assistant", Content: "hello"}, {Role: "user", Content: "hi"}, {Role: "user", Content: text}},
		StructuredOutput: true,
		Temperature:      0.3,
		MaxTokens:        500,
		OutputSchema: &domain.JSONSchema{Type: "object", Required: []string{"intent"}, Properties: map[string]domain.JSONSchemaProperty{
			"intent":     {Type: "string", Enum: []string{"SEARCH_LISTINGS", "NONE"}},
			"parameters": {Type: "object", Properties: map[string]domain.JSONSchemaProperty{"text": {Type: "string"}}},
		}},
	}
}

func aiCode(err error) domain.AIErrorCode {
	var e domain.AIError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestNew_RequiresAPIKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("a provider without credentials must not be constructed")
	}
}

func TestGenerate_StructuredRequestAndResponse(t *testing.T) {
	srv := anthropicfake.New(testKey, nil)
	defer srv.Close()
	p := newProvider(t, srv.URL, func(c *Config) { c.Model = "claude-test" })

	resp, err := p.Generate(context.Background(), classifyRequest("2 bedroom apartment in Kinshasa"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Structured["intent"] != "SEARCH_LISTINGS" {
		t.Fatalf("structured = %v", resp.Structured)
	}
	params := resp.Structured["parameters"].(map[string]interface{})
	if !strings.HasSuffix(params["text"].(string), "2 bedroom apartment Kinshasa") {
		t.Fatalf("search text = %q", params["text"])
	}
	if resp.TokensUsed != 150 || resp.ProviderID != ProviderID || resp.ModelID != "claude-test" {
		t.Fatalf("usage/provider not reported: %+v", resp)
	}

	h := srv.Headers()[0]
	if h.Get("x-api-key") != testKey || h.Get("anthropic-version") == "" {
		t.Fatalf("auth headers missing: %v", h)
	}
	req := srv.Requests()[0]
	if req["model"] != "claude-test" {
		t.Fatalf("model = %v", req["model"])
	}
	if _, ok := req["temperature"]; ok {
		t.Fatal("temperature must not be sent")
	}
	if !strings.Contains(systemText(req), "classify") || strings.Count(systemText(req), "classify") != 1 {
		t.Fatalf("system prompt = %q", systemText(req))
	}
	msgs := req["messages"].([]interface{})
	if len(msgs) != 1 || msgs[0].(map[string]interface{})["role"] != "user" {
		t.Fatalf("leading assistant turn must be dropped and user turns merged: %v", msgs)
	}
	oc := req["output_config"].(map[string]interface{})
	if oc["effort"] != "low" {
		t.Fatalf("effort = %v", oc["effort"])
	}
	format := oc["format"].(map[string]interface{})
	schema := format["schema"].(map[string]interface{})
	if format["type"] != "json_schema" || schema["additionalProperties"] != false {
		t.Fatalf("schema must be strict: %v", format)
	}
	nested := schema["properties"].(map[string]interface{})["parameters"].(map[string]interface{})
	if nested["additionalProperties"] != false {
		t.Fatalf("nested objects must be closed: %v", nested)
	}
}

func systemText(req map[string]interface{}) string {
	switch s := req["system"].(type) {
	case string:
		return s
	case []interface{}:
		var parts []string
		for _, b := range s {
			parts = append(parts, b.(map[string]interface{})["text"].(string))
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func TestGenerate_MalformedAndRefusedOutput(t *testing.T) {
	cases := map[string]struct {
		reply anthropicfake.Reply
		code  domain.AIErrorCode
	}{
		"not json":  {anthropicfake.Reply{Text: "SEARCH_LISTINGS please"}, domain.AIErrSchemaValidationFailed},
		"empty":     {anthropicfake.Reply{Text: "  "}, domain.AIErrInvalidResponse},
		"refusal":   {anthropicfake.Reply{Text: "", Stop: "refusal"}, domain.AIErrInvalidResponse},
		"truncated": {anthropicfake.Reply{Text: `{"intent":`, Stop: "max_tokens"}, domain.AIErrInvalidResponse},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := anthropicfake.New(testKey, func(map[string]interface{}) anthropicfake.Reply { return tc.reply })
			defer srv.Close()
			_, err := newProvider(t, srv.URL, nil).Generate(context.Background(), classifyRequest("x"))
			if aiCode(err) != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
}

func TestGenerate_ErrorNormalizationAndRetry(t *testing.T) {
	cases := []struct {
		name    string
		reply   anthropicfake.Reply
		retries int
		code    domain.AIErrorCode
		calls   int
	}{
		{"auth", anthropicfake.Error(http.StatusUnauthorized, "authentication_error", "bad key"), 2, domain.AIErrProviderAuthFailed, 1},
		{"rate limited", withRetryAfter(anthropicfake.Error(http.StatusTooManyRequests, "rate_limit_error", "slow down")), 1, domain.AIErrProviderRateLimited, 2},
		{"overloaded", withRetryAfter(anthropicfake.Error(529, "overloaded_error", "overloaded")), 1, domain.AIErrProviderUnavailable, 2},
		{"unknown model", anthropicfake.Error(http.StatusNotFound, "not_found_error", "model"), 2, domain.AIErrModelUnavailable, 1},
		{"too long", anthropicfake.Error(http.StatusBadRequest, "invalid_request_error", "prompt is too long: 300000 tokens"), 2, domain.AIErrContextTooLarge, 1},
		{"bad request", anthropicfake.Error(http.StatusBadRequest, "invalid_request_error", "sk-ant-test leaked?"), 2, domain.AIErrInvalidResponse, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := anthropicfake.New(testKey, func(map[string]interface{}) anthropicfake.Reply { return tc.reply })
			defer srv.Close()
			_, err := newProvider(t, srv.URL, func(c *Config) { c.MaxRetries = tc.retries }).Generate(context.Background(), classifyRequest("x"))
			if aiCode(err) != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Fatal("provider error text must not be forwarded")
			}
			if n := len(srv.Requests()); n != tc.calls {
				t.Fatalf("calls = %d, want %d", n, tc.calls)
			}
		})
	}
}

func withRetryAfter(r anthropicfake.Reply) anthropicfake.Reply {
	r.Headers = map[string]string{"retry-after-ms": "10"}
	return r
}

func TestGenerate_RetryRecoversFromTransientError(t *testing.T) {
	var n int32
	srv := anthropicfake.New(testKey, func(req map[string]interface{}) anthropicfake.Reply {
		if atomic.AddInt32(&n, 1) == 1 {
			return withRetryAfter(anthropicfake.Error(http.StatusServiceUnavailable, "api_error", "try again"))
		}
		return anthropicfake.Classifier(req)
	})
	defer srv.Close()
	resp, err := newProvider(t, srv.URL, func(c *Config) { c.MaxRetries = 2 }).Generate(context.Background(), classifyRequest("find a car"))
	if err != nil || resp.Structured["intent"] != "SEARCH_LISTINGS" || n != 2 {
		t.Fatalf("retry did not recover: %v %v calls=%d", resp.Structured, err, n)
	}
}

func TestGenerate_Timeout(t *testing.T) {
	srv := anthropicfake.New(testKey, func(req map[string]interface{}) anthropicfake.Reply {
		time.Sleep(500 * time.Millisecond)
		return anthropicfake.Classifier(req)
	})
	defer srv.Close()
	p := newProvider(t, srv.URL, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Generate(ctx, classifyRequest("find a car"))
	if aiCode(err) != domain.AIErrProviderTimeout {
		t.Fatalf("got %v, want timeout", err)
	}
	if time.Since(start) > 400*time.Millisecond {
		t.Fatal("deadline was not honoured")
	}
	if !(domain.AIError{Code: domain.AIErrProviderTimeout}).IsRetryable() {
		t.Fatal("timeouts are retryable by callers")
	}
}
