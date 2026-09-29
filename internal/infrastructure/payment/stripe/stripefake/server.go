// Package stripefake is an in-process emulator of the subset of the Stripe
// REST API used by the Trezutil Stripe adapter (PaymentIntents, capture,
// cancel, refunds, idempotency keys). It exists for contract, integration and
// local E2E tests; it is not a Stripe replacement and must never be deployed.
//
// Test payment methods (mirroring Stripe test tokens):
//
//	pm_card_visa           -> succeeds (requires_capture when capture_method=manual)
//	pm_card_chargeDeclined -> 402 card_error / card_declined
//	pm_timeout             -> responds after Delay (to exercise client timeouts)
//	pm_5xx                 -> 500 api_error
//	(none)                 -> requires_payment_method + client_secret
package stripefake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Server is a running emulator.
type Server struct {
	*httptest.Server
	SecretKey string
	Delay     time.Duration

	mu      sync.Mutex
	seq     int
	intents map[string]*Intent
	refunds map[string]*Refund
	idem    map[string][]byte // idempotency key -> response body
	Calls   map[string]int    // "METHOD path-prefix" -> count

	pendingRefunds bool
	inflight       map[string]bool
}

// Intent is an emulated PaymentIntent.
type Intent struct {
	ID               string            `json:"id"`
	Object           string            `json:"object"`
	Status           string            `json:"status"`
	Amount           int64             `json:"amount"`
	AmountReceived   int64             `json:"amount_received"`
	AmountCapturable int64             `json:"amount_capturable"`
	AmountRefunded   int64             `json:"-"`
	Currency         string            `json:"currency"`
	CaptureMethod    string            `json:"capture_method"`
	ClientSecret     string            `json:"client_secret"`
	Metadata         map[string]string `json:"metadata"`
}

// Refund is an emulated Refund.
type Refund struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	PaymentIntent string `json:"payment_intent"`
}

// New starts an emulator requiring the given secret key.
func New(secretKey string) *Server {
	s := &Server{SecretKey: secretKey, intents: map[string]*Intent{}, refunds: map[string]*Refund{}, idem: map[string][]byte{}, Calls: map[string]int{}, inflight: map[string]bool{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// Handler returns the emulator's HTTP handler, to serve it on a fixed address.
func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.handle) }

// Intent returns a copy of an intent by ID.
func (s *Server) Intent(id string) (Intent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pi, ok := s.intents[id]
	if !ok {
		return Intent{}, false
	}
	return *pi, true
}

// Confirm simulates the payer confirming an intent in the browser (Stripe.js).
func (s *Server) Confirm(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pi, ok := s.intents[id]; ok && pi.Status == "requires_payment_method" {
		s.settle(pi)
	}
}

// PendingRefunds toggles asynchronous refund behaviour.
func (s *Server) PendingRefunds(on bool) {
	s.mu.Lock()
	s.pendingRefunds = on
	s.mu.Unlock()
}

// SettleRefund marks a pending refund succeeded (as Stripe would later).
func (s *Server) SettleRefund(id string) (Refund, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rf, ok := s.refunds[id]
	if !ok {
		return Refund{}, false
	}
	rf.Status = "succeeded"
	return *rf, true
}

// CallCount returns the number of calls whose "METHOD /path" starts with prefix.
func (s *Server) CallCount(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, v := range s.Calls {
		if strings.HasPrefix(k, prefix) {
			n += v
		}
	}
	return n
}

func (s *Server) settle(pi *Intent) {
	if pi.CaptureMethod == "manual" {
		pi.Status, pi.AmountCapturable = "requires_capture", pi.Amount
	} else {
		pi.Status, pi.AmountReceived = "succeeded", pi.Amount
	}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	user, _, ok := r.BasicAuth()
	if !ok || user != s.SecretKey {
		writeErr(w, http.StatusUnauthorized, "invalid_request_error", "", "", "Invalid API Key provided")
		return
	}
	_ = r.ParseForm()

	s.mu.Lock()
	s.Calls[r.Method+" "+r.URL.Path]++
	key := r.Header.Get("Idempotency-Key")
	if r.Method == http.MethodPost && key != "" {
		if cached, ok := s.idem[key]; ok {
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotent-Replayed", "true")
			_, _ = w.Write(cached)
			return
		}
		if s.inflight[key] {
			// Like Stripe: a concurrent request with an in-flight key is refused.
			s.mu.Unlock()
			writeErr(w, http.StatusConflict, "idempotency_error", "idempotency_key_in_use", "", "There is currently another in-progress request using this Idempotent Key")
			return
		}
		s.inflight[key] = true
		defer func() { s.mu.Lock(); delete(s.inflight, key); s.mu.Unlock() }()
	}
	s.mu.Unlock()

	if r.PostForm.Get("payment_method") == "pm_timeout" && s.Delay > 0 {
		time.Sleep(s.Delay)
	}
	if r.PostForm.Get("payment_method") == "pm_5xx" {
		writeErr(w, http.StatusInternalServerError, "api_error", "", "", "internal error")
		return
	}

	s.mu.Lock()
	status, body := s.route(r)
	if r.Method == http.MethodPost && key != "" && status < 300 {
		s.idem[key] = body
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) route(r *http.Request) (int, []byte) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/")
	parts := strings.Split(path, "/")
	switch {
	case r.Method == http.MethodPost && path == "payment_intents":
		return s.createIntent(r)
	case r.Method == http.MethodGet && len(parts) == 2 && parts[0] == "payment_intents":
		if pi, ok := s.intents[parts[1]]; ok {
			return ok200(pi)
		}
		return errBody(http.StatusNotFound, "invalid_request_error", "resource_missing", "", "No such payment_intent")
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "payment_intents" && parts[2] == "capture":
		pi, ok := s.intents[parts[1]]
		if !ok {
			return errBody(http.StatusNotFound, "invalid_request_error", "resource_missing", "", "No such payment_intent")
		}
		if pi.Status != "requires_capture" {
			return errBody(http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state", "", "This PaymentIntent could not be captured")
		}
		amount := pi.AmountCapturable
		if v := r.PostForm.Get("amount_to_capture"); v != "" {
			amount, _ = strconv.ParseInt(v, 10, 64)
		}
		pi.Status, pi.AmountReceived, pi.AmountCapturable = "succeeded", amount, 0
		return ok200(pi)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "payment_intents" && parts[2] == "cancel":
		pi, ok := s.intents[parts[1]]
		if !ok {
			return errBody(http.StatusNotFound, "invalid_request_error", "resource_missing", "", "No such payment_intent")
		}
		if pi.Status == "succeeded" || pi.Status == "canceled" {
			return errBody(http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state", "", "cannot cancel")
		}
		pi.Status = "canceled"
		return ok200(pi)
	case r.Method == http.MethodPost && path == "refunds":
		pi, ok := s.intents[r.PostForm.Get("payment_intent")]
		if !ok {
			return errBody(http.StatusNotFound, "invalid_request_error", "resource_missing", "", "No such payment_intent")
		}
		if pi.Status != "succeeded" {
			return errBody(http.StatusBadRequest, "invalid_request_error", "charge_not_captured", "", "not captured")
		}
		amount := pi.AmountReceived - pi.AmountRefunded
		if v := r.PostForm.Get("amount"); v != "" {
			amount, _ = strconv.ParseInt(v, 10, 64)
		}
		if amount <= 0 || pi.AmountRefunded+amount > pi.AmountReceived {
			return errBody(http.StatusBadRequest, "invalid_request_error", "amount_too_large", "", "Refund amount is greater than unrefunded amount")
		}
		pi.AmountRefunded += amount
		s.seq++
		status := "succeeded"
		if s.pendingRefunds {
			status = "pending"
		}
		rf := &Refund{ID: fmt.Sprintf("re_%d", s.seq), Object: "refund", Status: status, Amount: amount, Currency: pi.Currency, PaymentIntent: pi.ID}
		s.refunds[rf.ID] = rf
		return ok200(rf)
	}
	return errBody(http.StatusNotFound, "invalid_request_error", "", "", "Unrecognized request URL")
}

func (s *Server) createIntent(r *http.Request) (int, []byte) {
	amount, err := strconv.ParseInt(r.PostForm.Get("amount"), 10, 64)
	if err != nil || amount <= 0 {
		return errBody(http.StatusBadRequest, "invalid_request_error", "parameter_invalid_integer", "", "Invalid amount")
	}
	s.seq++
	pi := &Intent{
		ID:            fmt.Sprintf("pi_%d", s.seq),
		Object:        "payment_intent",
		Status:        "requires_payment_method",
		Amount:        amount,
		Currency:      r.PostForm.Get("currency"),
		CaptureMethod: r.PostForm.Get("capture_method"),
		Metadata:      map[string]string{},
	}
	pi.ClientSecret = pi.ID + "_secret_test"
	for k, v := range r.PostForm {
		if strings.HasPrefix(k, "metadata[") {
			pi.Metadata[strings.TrimSuffix(strings.TrimPrefix(k, "metadata["), "]")] = v[0]
		}
	}
	s.intents[pi.ID] = pi
	switch pm := r.PostForm.Get("payment_method"); {
	case pm == "pm_card_chargeDeclined":
		body, _ := json.Marshal(map[string]interface{}{"error": map[string]interface{}{
			"type": "card_error", "code": "card_declined", "decline_code": "generic_decline",
			"message": "Your card was declined.", "payment_intent": pi,
		}})
		return http.StatusPaymentRequired, body
	case pm != "" && r.PostForm.Get("confirm") == "true":
		s.settle(pi)
	}
	return ok200(pi)
}

func ok200(v interface{}) (int, []byte) {
	b, _ := json.Marshal(v)
	return http.StatusOK, b
}

func errBody(status int, typ, code, decline, message string) (int, []byte) {
	b, _ := json.Marshal(map[string]interface{}{"error": map[string]string{"type": typ, "code": code, "decline_code": decline, "message": message}})
	return status, b
}

func writeErr(w http.ResponseWriter, status int, typ, code, decline, message string) {
	_, b := errBody(status, typ, code, decline, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// Event builds a Stripe event JSON for the given type and object.
func Event(id, typ string, object interface{}) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"id": id, "object": "event", "type": typ, "created": time.Now().Unix(),
		"data": map[string]interface{}{"object": object},
	})
	return b
}
