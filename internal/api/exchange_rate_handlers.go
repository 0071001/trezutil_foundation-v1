package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/exchangerate"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// ExchangeRateHandler serves the CDF / USD rate entered by an administrator (exchangerate.Service):
//
//	GET /api/v1/exchange-rate                     public: {"base":"USD","quote":"CDF","rate":2850,"set_at":"…"} or {"rate":null}
//	PUT /api/v1/admin/exchange-rate               admin only: {"rate":2850,"note":"…"}
//	GET /api/v1/admin/exchange-rate/history       staff: the rates entered, most recent first
type ExchangeRateHandler struct {
	svc    *exchangerate.Service
	logger zerolog.Logger
}

// NewExchangeRateHandler builds the handler.
func NewExchangeRateHandler(svc *exchangerate.Service, logger zerolog.Logger) *ExchangeRateHandler {
	return &ExchangeRateHandler{svc: svc, logger: logger}
}

// Register mounts the routes on a Go 1.22+ mux.
func (h *ExchangeRateHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exchange-rate", h.current)
	mux.HandleFunc("PUT /api/v1/admin/exchange-rate", h.set)
	mux.HandleFunc("GET /api/v1/admin/exchange-rate/history", h.history)
}

func rateJSON(r *domain.ExchangeRate) map[string]interface{} {
	return map[string]interface{}{"base": r.Base, "quote": r.Quote, "rate": r.Rate, "note": r.Note, "set_at": r.SetAt.UTC().Format(time.RFC3339)}
}

func (h *ExchangeRateHandler) current(w http.ResponseWriter, r *http.Request) {
	// Public and cacheable for a short while: a new rate shows within a minute.
	w.Header().Set("Cache-Control", "public, max-age=60")
	rate, err := h.svc.Current(r.Context())
	if errors.Is(err, domain.ErrNoExchangeRate) {
		writeJSON(w, map[string]interface{}{"base": "USD", "quote": "CDF", "rate": nil}, http.StatusOK)
		return
	}
	if err != nil {
		h.logger.Error().Err(err).Msg("Exchange rate read failed")
		writeJSON(w, ErrorResponse{Error: Error{Code: "EXCHANGE_RATE_FAILED", Message: "The exchange rate could not be read"}}, http.StatusInternalServerError)
		return
	}
	out := rateJSON(rate)
	delete(out, "note") // the note is for staff
	writeJSON(w, out, http.StatusOK)
}

func (h *ExchangeRateHandler) set(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	var body struct {
		Rate *float64 `json:"rate"`
		Note string   `json:"note"`
	}
	if err := readJSONBody(r, &body); err != nil || body.Rate == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "rate is required"}}, http.StatusBadRequest)
		return
	}
	rate, err := h.svc.Set(r.Context(), userID, *body.Rate, body.Note)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, rateJSON(rate), http.StatusOK)
}

func (h *ExchangeRateHandler) history(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	list, err := h.svc.History(r.Context(), userID, 20)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		items = append(items, rateJSON(&list[i]))
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

func (h *ExchangeRateHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exchangerate.ErrForbidden):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only an administrator can set the exchange rate"}}, http.StatusForbidden)
	case errors.Is(err, exchangerate.ErrInvalidRate):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_RATE", Message: err.Error()}}, http.StatusBadRequest)
	default:
		h.logger.Error().Err(err).Msg("Exchange rate update failed")
		writeJSON(w, ErrorResponse{Error: Error{Code: "EXCHANGE_RATE_FAILED", Message: "The request could not be completed"}}, http.StatusInternalServerError)
	}
}
