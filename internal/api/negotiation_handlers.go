package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/negotiation"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// NegotiationHandler serves the negotiation engine (negotiation.Service). Identity comes only from the access token.
//
// Buyer:
//
//	POST /api/v1/negotiations                                   start: {listing_ids, quantity, target_price, max_price, deadline_hours, max_rounds, auto_counter}
//	GET  /api/v1/negotiations                                   the buyer's negotiations
//	GET  /api/v1/negotiations/{id}                              one, with its mandate, offers, journal and messages
//	POST /api/v1/negotiations/{id}/counter                      {participant_id, unit_price, note}
//	POST /api/v1/negotiations/{id}/accept                       {offer_id}: the order is created at the agreed price
//	POST /api/v1/negotiations/{id}/participants/{pid}/reject    turn one supplier's offer down
//	POST /api/v1/negotiations/{id}/cancel                       withdraw the request
//
// Supplier (the businesses the person answers for; the buyer's mandate and identity are never in these answers):
//
//	GET  /api/v1/supplier-requests                              the requests received
//	GET  /api/v1/supplier-requests/{pid}                        one request
//	POST /api/v1/supplier-requests/{pid}/offer                  {unit_price, delivery_days, valid_days, note}
//	POST /api/v1/supplier-requests/{pid}/accept-counter         answer the buyer's counter-offer at its price
//	POST /api/v1/supplier-requests/{pid}/decline                {reason}
type NegotiationHandler struct {
	svc    *negotiation.Service
	logger zerolog.Logger
}

// NewNegotiationHandler builds the handler.
func NewNegotiationHandler(svc *negotiation.Service, logger zerolog.Logger) *NegotiationHandler {
	return &NegotiationHandler{svc: svc, logger: logger}
}

// Register mounts the routes.
func (h *NegotiationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/negotiations", h.start)
	mux.HandleFunc("GET /api/v1/negotiations", h.list)
	mux.HandleFunc("GET /api/v1/negotiations/{id}", h.get)
	mux.HandleFunc("POST /api/v1/negotiations/{id}/counter", h.counter)
	mux.HandleFunc("POST /api/v1/negotiations/{id}/accept", h.accept)
	mux.HandleFunc("POST /api/v1/negotiations/{id}/participants/{pid}/reject", h.reject)
	mux.HandleFunc("POST /api/v1/negotiations/{id}/cancel", h.cancel)
	mux.HandleFunc("GET /api/v1/supplier-requests", h.inbox)
	mux.HandleFunc("GET /api/v1/supplier-requests/{pid}", h.request)
	mux.HandleFunc("POST /api/v1/supplier-requests/{pid}/offer", h.offer)
	mux.HandleFunc("POST /api/v1/supplier-requests/{pid}/accept-counter", h.acceptCounter)
	mux.HandleFunc("POST /api/v1/supplier-requests/{pid}/decline", h.decline)
}

// RunTick closes what reached its deadline and reminds suppliers (the periodic job).
func (h *NegotiationHandler) RunTick(ctx context.Context) {
	if n, err := h.svc.Tick(ctx); err != nil {
		h.logger.Warn().Err(err).Msg("negotiation deadlines and reminders failed")
	} else if n > 0 {
		h.logger.Info().Int("negotiations", n).Msg("negotiation deadlines and reminders handled")
	}
}

func (h *NegotiationHandler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	return auth.RequireUser(w, r)
}

func (h *NegotiationHandler) body(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := readJSONBody(r, v); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return false
	}
	return true
}

func (h *NegotiationHandler) fail(w http.ResponseWriter, err error) {
	var inv *negotiation.InvalidError
	var guard *negotiation.GuardError
	code, status, msg := "", 0, ""
	switch {
	case errors.As(err, &inv):
		writeJSON(w, map[string]interface{}{"error": map[string]string{"code": "INVALID_FIELD", "message": inv.Message, "field": inv.Field}}, http.StatusBadRequest)
		return
	case errors.As(err, &guard):
		code, status, msg = "MANDATE", http.StatusUnprocessableEntity, guard.Reason
	case errors.Is(err, negotiation.ErrNotFound):
		code, status, msg = "NOT_FOUND", http.StatusNotFound, "Negotiation not found"
	case errors.Is(err, negotiation.ErrForbidden):
		code, status, msg = "FORBIDDEN", http.StatusForbidden, "Not allowed"
	case errors.Is(err, negotiation.ErrConflict):
		code, status, msg = "CONFLICT", http.StatusConflict, "The negotiation changed meanwhile; try again"
	case errors.Is(err, negotiation.ErrClosed):
		code, status, msg = "CLOSED", http.StatusConflict, "The negotiation is closed"
	case errors.Is(err, negotiation.ErrDemoSupplier):
		code, status, msg = "DEMO_SUPPLIER", http.StatusUnprocessableEntity, "Demonstration suppliers do not negotiate"
	case errors.Is(err, negotiation.ErrOwnOffer):
		code, status, msg = "OWN_OFFER", http.StatusUnprocessableEntity, "You cannot negotiate with your own business"
	case errors.Is(err, negotiation.ErrMixedCurrency):
		code, status, msg = "MIXED_CURRENCY", http.StatusUnprocessableEntity, "Every offer must be in the same currency"
	case errors.Is(err, negotiation.ErrUnavailable), errors.Is(err, domain.ErrListingUnavailable):
		code, status, msg = "LISTING_UNAVAILABLE", http.StatusConflict, "The offer is no longer published"
	case errors.Is(err, negotiation.ErrOfferExpired):
		code, status, msg = "OFFER_EXPIRED", http.StatusConflict, "This offer is no longer valid"
	default:
		h.logger.Error().Err(err).Msg("negotiation request failed")
		code, status, msg = "NEGOTIATION_FAILED", http.StatusInternalServerError, "The negotiation could not be updated"
	}
	writeJSON(w, ErrorResponse{Error: Error{Code: code, Message: msg}}, status)
}

func (h *NegotiationHandler) reply(w http.ResponseWriter, snap *negotiation.Snapshot, err error, supplier bool, status int) {
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, negotiationJSON(snap, supplier), status)
}

// ---- buyer

func (h *NegotiationHandler) start(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	var req negotiation.StartRequest
	if !ok || !h.body(w, r, &req) {
		return
	}
	snap, err := h.svc.Start(r.Context(), userID, req)
	h.reply(w, snap, err, false, http.StatusCreated)
}

func (h *NegotiationHandler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListForBuyer(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		items = append(items, negotiationJSON(&list[i], false))
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

func (h *NegotiationHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	snap, err := h.svc.Get(r.Context(), userID, r.PathValue("id"))
	h.reply(w, snap, err, false, http.StatusOK)
}

func (h *NegotiationHandler) counter(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	var req negotiation.CounterRequest
	if !ok || !h.body(w, r, &req) {
		return
	}
	snap, err := h.svc.Counter(r.Context(), userID, r.PathValue("id"), req)
	h.reply(w, snap, err, false, http.StatusOK)
}

func (h *NegotiationHandler) accept(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	var req struct {
		OfferID string `json:"offer_id"`
	}
	if !ok || !h.body(w, r, &req) {
		return
	}
	snap, err := h.svc.Accept(r.Context(), userID, r.PathValue("id"), req.OfferID)
	h.reply(w, snap, err, false, http.StatusOK)
}

func (h *NegotiationHandler) reject(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	snap, err := h.svc.Reject(r.Context(), userID, r.PathValue("id"), r.PathValue("pid"))
	h.reply(w, snap, err, false, http.StatusOK)
}

func (h *NegotiationHandler) cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	snap, err := h.svc.Cancel(r.Context(), userID, r.PathValue("id"))
	h.reply(w, snap, err, false, http.StatusOK)
}

// ---- supplier

func (h *NegotiationHandler) inbox(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Inbox(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		items = append(items, negotiationJSON(&list[i], true))
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

func (h *NegotiationHandler) request(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	snap, err := h.svc.SupplierGet(r.Context(), userID, r.PathValue("pid"))
	h.reply(w, snap, err, true, http.StatusOK)
}

func (h *NegotiationHandler) offer(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	var req negotiation.OfferRequest
	if !ok || !h.body(w, r, &req) {
		return
	}
	snap, err := h.svc.SupplierOffer(r.Context(), userID, r.PathValue("pid"), req)
	h.reply(w, snap, err, true, http.StatusOK)
}

func (h *NegotiationHandler) acceptCounter(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	snap, err := h.svc.SupplierAcceptCounter(r.Context(), userID, r.PathValue("pid"))
	h.reply(w, snap, err, true, http.StatusOK)
}

func (h *NegotiationHandler) decline(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	var req struct {
		Reason string `json:"reason"`
	}
	if !ok || !h.body(w, r, &req) {
		return
	}
	snap, err := h.svc.SupplierDecline(r.Context(), userID, r.PathValue("pid"), req.Reason)
	h.reply(w, snap, err, true, http.StatusOK)
}

// negotiationJSON writes a negotiation. For a supplier, ForSupplier has already removed what is not theirs; the
// mandate's prices are left out here too.
func negotiationJSON(s *negotiation.Snapshot, supplier bool) map[string]interface{} {
	unix := func(t time.Time) int64 { return t.Unix() }
	n := s.Negotiation
	out := map[string]interface{}{
		"id": n.ID, "title": n.Title, "quantity": n.Quantity, "currency": n.Currency, "status": string(n.Status),
		"deadline": unix(n.Deadline), "order_id": n.OrderID, "created_at": unix(n.CreatedAt), "updated_at": unix(n.UpdatedAt),
	}
	if supplier {
		out["mandate"] = map[string]interface{}{"max_rounds": s.Mandate.MaxRounds}
	} else {
		out["conversation_id"] = n.ConversationID
		out["accepted_offer_id"] = n.AcceptedOfferID
		out["mandate"] = map[string]interface{}{
			"target_unit_minor": s.Mandate.TargetUnitMinor, "max_unit_minor": s.Mandate.MaxUnitMinor,
			"max_rounds": s.Mandate.MaxRounds, "auto_counter": s.Mandate.AutoCounter,
		}
	}
	parts := make([]map[string]interface{}, 0, len(s.Participants))
	for _, p := range s.Participants {
		parts = append(parts, map[string]interface{}{
			"id": p.ID, "organization_id": p.OrganizationID, "supplier": p.SupplierName, "listing_id": p.ListingID,
			"listing_title": p.ListingTitle, "list_unit_minor": p.ListUnitMinor, "status": string(p.Status), "round": p.Round,
			"reminded": p.RemindedAt != nil,
		})
	}
	out["participants"] = parts
	offers := make([]map[string]interface{}, 0, len(s.Offers))
	for _, o := range s.Offers {
		item := map[string]interface{}{
			"id": o.ID, "participant_id": o.ParticipantID, "round": o.Round, "author": o.Author, "source": o.Source,
			"unit_minor": o.UnitMinor, "quantity": o.Quantity, "note": o.Note, "status": o.Status, "created_at": unix(o.CreatedAt),
			"delivery_days": o.DeliveryDays, "valid_until": nil,
		}
		if o.ValidUntil != nil {
			item["valid_until"] = unix(*o.ValidUntil)
		}
		offers = append(offers, item)
	}
	out["offers"] = offers
	events := make([]map[string]interface{}, 0, len(s.Events))
	for _, e := range s.Events {
		events = append(events, map[string]interface{}{"kind": e.Kind, "actor": e.Actor, "participant_id": e.ParticipantID, "data": e.Data, "created_at": unix(e.CreatedAt)})
	}
	out["events"] = events
	msgs := make([]map[string]interface{}, 0, len(s.Messages))
	for _, m := range s.Messages {
		item := map[string]interface{}{"participant_id": m.ParticipantID, "channel": m.Channel, "body": m.Body, "created_at": unix(m.CreatedAt)}
		if !supplier {
			item["state"], item["last_error"] = m.State, m.LastError
		}
		msgs = append(msgs, item)
	}
	out["messages"] = msgs
	return out
}
