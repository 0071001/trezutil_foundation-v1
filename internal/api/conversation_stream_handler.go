package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
)

// StreamBudget bounds one streamed answer: interpreting, running the action and writing the text.
const StreamBudget = 90 * time.Second

// streamHeartbeat keeps proxies (nginx: proxy_read_timeout) from closing a quiet stream.
const streamHeartbeat = 15 * time.Second

// HandleProcessMessageStream handles POST /api/v1/conversations/message/stream: the same request and the same
// processing as POST /api/v1/conversations/message, answered as Server-Sent Events while the work happens:
//
//	event: stage  data: {"stage":"interpreting"|"acting"|"writing","detail":"SEARCH_LISTINGS"}
//	event: delta  data: {"text":"…"}        a piece of the answer, as the model writes it
//	event: reset  data: {}                  the pieces sent so far are discarded; "done" holds the answer
//	event: done   data: <the response of POST /conversations/message>   always the last event on success
//	event: error  data: {"code":"…","message":"…"}
//
// "done" is the truth: its text is the stored answer. When nothing was written by the model (no AI, a confirmation,
// a question), no delta is sent and "done" carries the whole text at once.
func (h *ConversationHandler) HandleProcessMessageStream(w http.ResponseWriter, r *http.Request) {
	req, failure, status := h.parseMessageRequest(r)
	if failure != nil {
		writeJSON(w, failure, status)
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(StreamBudget + 10*time.Second)) // the server's own WriteTimeout is shorter

	header := w.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Accel-Buffering", "no") // nginx: pass each event on at once
	w.WriteHeader(http.StatusOK)

	out := &sseWriter{w: w, rc: rc}
	out.comment("open")

	ctx, cancel := context.WithTimeout(r.Context(), StreamBudget)
	defer cancel()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTicker(streamHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				out.comment("keep-alive")
			case <-stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	resp, err := h.useCases.ProcessMessage(conversation.WithStreamSink(ctx, out), req)
	switch {
	case errors.Is(err, domain.ErrConversationForbidden):
		out.event("error", map[string]string{"code": "FORBIDDEN", "message": "This conversation belongs to another user"})
	case err != nil:
		h.logger.Error().Err(err).Msg("Failed to process a streamed message")
		out.event("error", map[string]string{"code": "PROCESSING_FAILED", "message": "The assistant could not process this message"})
	default:
		out.event("done", resp)
	}
}

// parseMessageRequest reads and checks the body shared by both message routes.
func (h *ConversationHandler) parseMessageRequest(r *http.Request) (*conversation.ProcessConversationRequest, *ErrorResponse, int) {
	var httpReq conversationMessageRequest
	if err := readJSONBody(r, &httpReq); err != nil {
		return nil, &ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest
	}
	if httpReq.MessageText == "" {
		return nil, &ErrorResponse{Error: Error{Code: "MISSING_MESSAGE_TEXT", Message: "message_text is required"}}, http.StatusBadRequest
	}
	channel := domain.Channel(strings.ToLower(httpReq.Channel))
	if channel == "" {
		channel = domain.ChannelWeb
	}
	if !channel.IsValid() {
		return nil, &ErrorResponse{Error: Error{Code: "INVALID_CHANNEL", Message: "Invalid channel"}}, http.StatusBadRequest
	}
	if len(httpReq.MessageText) > maxMessageTextLength {
		return nil, &ErrorResponse{Error: Error{Code: "MESSAGE_TOO_LONG", Message: "message_text is too long"}}, http.StatusRequestEntityTooLarge
	}
	return &conversation.ProcessConversationRequest{
		Channel:               channel,
		SenderID:              httpReq.SenderID,
		SenderExternalID:      httpReq.SenderExternalID,
		MessageText:           httpReq.MessageText,
		MessageIdempotencyKey: httpReq.MessageIdempotencyKey,
		Locale:                httpReq.Locale,
		Actor:                 principalActor(r),
	}, nil, 0
}

// sseWriter writes Server-Sent Events and flushes each one; it is the conversation.StreamSink of one request. Writes
// come from the processing and from the heartbeat, so they are serialised. Once the client is gone, writes stop.
type sseWriter struct {
	mu     sync.Mutex
	w      http.ResponseWriter
	rc     *http.ResponseController
	broken bool
}

func (s *sseWriter) Stage(stage, detail string) {
	s.event("stage", map[string]string{"stage": stage, "detail": detail})
}
func (s *sseWriter) Delta(text string) { s.event("delta", map[string]string{"text": text}) }
func (s *sseWriter) Reset()            { s.event("reset", struct{}{}) }

func (s *sseWriter) event(name string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.write(fmt.Sprintf("event: %s\ndata: %s\n\n", name, data))
}

func (s *sseWriter) comment(text string) { s.write(": " + text + "\n\n") }

func (s *sseWriter) write(frame string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return
	}
	if _, err := s.w.Write([]byte(frame)); err != nil {
		s.broken = true
		return
	}
	if err := s.rc.Flush(); err != nil {
		s.broken = true
	}
}

var _ conversation.StreamSink = (*sseWriter)(nil)
