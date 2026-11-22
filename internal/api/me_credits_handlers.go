package api

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// PersonalCreditsReader reads credit data (the console's reader: one source for organisations and personal spaces).
type PersonalCreditsReader interface {
	Usage(ctx context.Context, organizationID string) (*domain.UsageSummary, error)
	UsagePolicies(ctx context.Context) ([]domain.UsagePolicyRow, error)
}

// MeCreditsHandler serves GET /api/v1/me/credits: the signed-in person's own credits, read from their personal space
// (migration 000033), never an organisation's. Organisation credits stay on GET /api/v1/sellers/usage, for the people
// who manage them. Reading never creates anything: a person whose credits were never used has a zero balance.
type MeCreditsHandler struct {
	spaces domain.PersonalSpaces
	reader PersonalCreditsReader
	logger zerolog.Logger
	now    func() time.Time
}

// NewMeCreditsHandler builds the handler.
func NewMeCreditsHandler(spaces domain.PersonalSpaces, reader PersonalCreditsReader, logger zerolog.Logger) *MeCreditsHandler {
	return &MeCreditsHandler{spaces: spaces, reader: reader, logger: logger, now: time.Now}
}

// ServeHTTP answers:
//
//	{"period_id":"2026-11",
//	 "personal":{"opened":true,"granted":100,"reserved":0,"consumed":12,"balance":88},
//	 "ledger":[{"id","action_type","kind","amount","balance_after","period_id","created_at"}],
//	 "metering":{"active":false,"costs":[{"action_type","channel","cost"}]}}
//
// balance = granted − reserved − consumed of the current month's account, as the runtime checks it. "metering.active"
// is false when no active policy charges a personal account: actions are then free, whatever the balance.
func (h *MeCreditsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, ErrorResponse{Error: Error{Code: "METHOD_NOT_ALLOWED", Message: "GET only"}}, http.StatusMethodNotAllowed)
		return
	}
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	period := h.now().Format("2006-01")
	personal := map[string]interface{}{"opened": false, "granted": int64(0), "reserved": int64(0), "consumed": int64(0), "balance": int64(0)}
	ledger := []map[string]interface{}{}

	orgID, err := h.spaces.Find(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if orgID != "" {
		summary, err := h.reader.Usage(r.Context(), orgID)
		if err != nil {
			h.fail(w, err)
			return
		}
		for _, a := range summary.Accounts {
			// The personal space holds this person's accounts only; the check keeps it true if that ever changes.
			if a.OwnerID != userID || a.PeriodID != period {
				continue
			}
			personal = map[string]interface{}{
				"opened": true, "granted": a.Granted, "reserved": a.Reserved, "consumed": a.Consumed,
				"balance": a.Granted - a.Reserved - a.Consumed, "updated_at": isoTime(a.UpdatedAt),
			}
			break
		}
		for i, e := range summary.Ledger {
			if i == 20 {
				break
			}
			ledger = append(ledger, map[string]interface{}{
				"id": e.ID, "action_type": e.ActionType, "kind": e.TransactionType, "amount": e.Amount,
				"balance_after": e.BalanceAfter, "period_id": e.PeriodID, "created_at": isoTime(e.CreatedAt),
			})
		}
	}

	policies, err := h.reader.UsagePolicies(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	costs := []map[string]interface{}{}
	for _, p := range policies {
		if p.Active && p.Cost > 0 && p.ActorType == string(domain.ActorTypeCustomer) {
			costs = append(costs, map[string]interface{}{"action_type": p.ActionType, "channel": p.Channel, "cost": p.Cost})
		}
	}
	writeJSON(w, map[string]interface{}{
		"period_id": period,
		"personal":  personal,
		"ledger":    ledger,
		"metering":  map[string]interface{}{"active": len(costs) > 0, "costs": costs},
	}, http.StatusOK)
}

func (h *MeCreditsHandler) fail(w http.ResponseWriter, err error) {
	h.logger.Error().Err(err).Msg("Personal credits could not be read")
	writeJSON(w, ErrorResponse{Error: Error{Code: "CREDITS_FAILED", Message: "The credits could not be read"}}, http.StatusInternalServerError)
}
