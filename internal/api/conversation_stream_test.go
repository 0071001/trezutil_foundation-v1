package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
)

// streamUseCases plays a processing that reports stages and text through the stream sink.
type streamUseCases struct {
	err error
}

func (s *streamUseCases) ProcessMessage(ctx context.Context, req *conversation.ProcessConversationRequest) (*conversation.ProcessConversationResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	sink := conversation.SinkFrom(ctx)
	sink.Stage("interpreting", "")
	sink.Stage("acting", "SEARCH_LISTINGS")
	sink.Delta("Voici ")
	sink.Delta("2 offres.")
	resp := domain.NewConversationResponse("Voici 2 offres.", "fr")
	return &conversation.ProcessConversationResponse{ConversationID: "c-1", Response: resp}, nil
}
func (s *streamUseCases) ConfirmAction(context.Context, string, string, domain.CommerceActor) (*conversation.ProcessConversationResponse, error) {
	return nil, nil
}
func (s *streamUseCases) RejectAction(context.Context, string, string, domain.CommerceActor) (*conversation.ProcessConversationResponse, error) {
	return nil, nil
}

func streamCall(h *ConversationHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/message/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleConversationRoutes(h, rec, req)
	return rec
}

func TestStreamRouteSendsStagesTextThenTheWholeAnswer(t *testing.T) {
	rec := streamCall(NewConversationHandler(&streamUseCases{}, zerolog.Nop()), `{"message_text":"des ordinateurs","channel":"web"}`)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("nginx must not buffer the stream and nothing may cache it")
	}
	body := rec.Body.String()
	order := []string{
		`event: stage` + "\n" + `data: {"detail":"","stage":"interpreting"}`,
		`data: {"detail":"SEARCH_LISTINGS","stage":"acting"}`,
		`event: delta` + "\n" + `data: {"text":"Voici "}`,
		`data: {"text":"2 offres."}`,
		`event: done` + "\n" + `data: {"conversation_id":"c-1"`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(body[at:], want)
		if i < 0 {
			t.Fatalf("missing or out of order %q in:\n%s", want, body)
		}
		at += i + len(want)
	}
}

func TestStreamRouteRejectsABadRequestBeforeStreaming(t *testing.T) {
	rec := streamCall(NewConversationHandler(&streamUseCases{}, zerolog.Nop()), `{"message_text":""}`)
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Header().Get("Content-Type"), "event-stream") {
		t.Fatalf("want a plain 400, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestStreamRouteReportsAFailureAsAnErrorEvent(t *testing.T) {
	for err, code := range map[error]string{
		domain.ErrConversationForbidden: "FORBIDDEN",
		errors.New("boom"):              "PROCESSING_FAILED",
	} {
		rec := streamCall(NewConversationHandler(&streamUseCases{err: err}, zerolog.Nop()), `{"message_text":"x"}`)
		body := rec.Body.String()
		if !strings.Contains(body, "event: error") || !strings.Contains(body, code) || strings.Contains(body, "event: done") || strings.Contains(body, "boom") {
			t.Fatalf("%v: %s", err, body)
		}
	}
}
