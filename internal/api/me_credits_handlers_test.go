package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type fakeSpaces struct {
	byUser  map[string]string
	ensured int
}

func (f *fakeSpaces) Find(_ context.Context, userID string) (string, error) { return f.byUser[userID], nil }
func (f *fakeSpaces) Ensure(_ context.Context, userID string) (string, error) {
	f.ensured++
	return f.byUser[userID], nil
}

type fakeCreditsReader struct {
	summary  map[string]*domain.UsageSummary
	policies []domain.UsagePolicyRow
}

func (f *fakeCreditsReader) Usage(_ context.Context, orgID string) (*domain.UsageSummary, error) {
	if s := f.summary[orgID]; s != nil {
		return s, nil
	}
	return &domain.UsageSummary{}, nil
}
func (f *fakeCreditsReader) UsagePolicies(context.Context) ([]domain.UsagePolicyRow, error) {
	return f.policies, nil
}

func meCredits(h *MeCreditsHandler, user string) (*httptest.ResponseRecorder, map[string]interface{}) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/credits", nil)
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestMeCredits(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	spaces := &fakeSpaces{byUser: map[string]string{"u-1": "org-personal-1"}}
	reader := &fakeCreditsReader{
		summary: map[string]*domain.UsageSummary{"org-personal-1": {
			Accounts: []domain.CreditAccountRow{
				{OwnerID: "u-1", PeriodID: "2026-11", Granted: 100, Reserved: 5, Consumed: 12},
				{OwnerID: "u-1", PeriodID: "2026-10", Granted: 50, Consumed: 50},
			},
			Ledger: []domain.UsageEntry{{ID: "l-1", ActionType: "SEARCH_LISTINGS", TransactionType: "consume", Amount: -3}},
		}},
		policies: []domain.UsagePolicyRow{
			{ActionType: "SEARCH_LISTINGS", ActorType: "customer", Cost: 3, Active: true},
			{ActionType: "CREATE_ORDER", ActorType: "customer", Cost: 10, Active: false},
			{ActionType: "PUBLISH_LISTING", ActorType: "seller", Cost: 5, Active: true},
		},
	}
	h := NewMeCreditsHandler(spaces, reader, zerolog.Nop())
	h.now = func() time.Time { return now }

	if rec, _ := meCredits(h, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a guest: want 401, got %d", rec.Code)
	}

	rec, body := meCredits(h, "u-1")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d", rec.Code)
	}
	personal := body["personal"].(map[string]interface{})
	if personal["opened"] != true || personal["balance"].(float64) != 83 || personal["granted"].(float64) != 100 {
		t.Fatalf("the current month's balance is granted − reserved − consumed: %v", personal)
	}
	metering := body["metering"].(map[string]interface{})
	if metering["active"] != true || len(metering["costs"].([]interface{})) != 1 {
		t.Fatalf("only active customer policies with a cost count: %v", metering)
	}
	if len(body["ledger"].([]interface{})) != 1 {
		t.Fatalf("ledger: %v", body["ledger"])
	}

	// Somebody whose credits were never used: zero, and reading creates nothing.
	_, body = meCredits(h, "u-2")
	if p := body["personal"].(map[string]interface{}); p["opened"] != false || p["balance"].(float64) != 0 {
		t.Fatalf("no personal space yet: %v", p)
	}
	if spaces.ensured != 0 {
		t.Fatal("reading the balance must never create a personal space")
	}
}
