package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSender_SendText(t *testing.T) {
	var got map[string]interface{}
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
	}))
	defer srv.Close()

	s := NewSender(SenderConfig{BaseURL: srv.URL + "/v20.0/", PhoneNumberID: "123", AccessToken: "tok"})
	if err := s.SendText(context.Background(), "+243812345678", "hello"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if auth != "Bearer tok" || path != "/v20.0/123/messages" {
		t.Fatalf("unexpected request auth=%q path=%q", auth, path)
	}
	if got["to"] != "243812345678" || got["type"] != "text" {
		t.Fatalf("unexpected payload: %v", got)
	}
}

func TestSender_ProviderErrorAndNotConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
	}))
	defer srv.Close()

	s := NewSender(SenderConfig{BaseURL: srv.URL, PhoneNumberID: "1", AccessToken: "secret-token"})
	err := s.SendText(context.Background(), "1", "x")
	if err == nil {
		t.Fatal("expected provider error")
	}
	if contains(err.Error(), "secret-token") {
		t.Fatal("error must not leak the access token")
	}

	if err := NewSender(SenderConfig{}).SendText(context.Background(), "1", "x"); !errors.Is(err, ErrSenderNotConfigured) {
		t.Fatalf("expected ErrSenderNotConfigured, got %v", err)
	}
}

func TestSender_OTPTemplate(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	s := NewSender(SenderConfig{BaseURL: srv.URL, PhoneNumberID: "1", AccessToken: "t", OTPTemplate: "trezutil_otp"})
	if err := s.Send(context.Background(), "+243812345678", "123456"); err != nil {
		t.Fatalf("send otp: %v", err)
	}
	if got["type"] != "template" {
		t.Fatalf("expected template message, got %v", got["type"])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
