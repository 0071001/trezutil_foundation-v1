// Package exchangerate keeps the CDF / USD rate that a Trezutil administrator enters by hand. Nothing fetches or
// guesses a rate: until an administrator sets one, there is none, and the interface shows prices only in the currency
// the supplier set. The rate only gives an indicative conversion; it never changes what is ordered or paid.
package exchangerate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// Plausible bounds for francs per US dollar: they catch a typo (an extra or missing zero, the rate entered the other
// way round) without pretending to know the market.
const (
	MinRate = 100.0
	MaxRate = 100000.0
)

var (
	// ErrForbidden: only an administrator may set the rate.
	ErrForbidden = errors.New("only an administrator can set the exchange rate")
	// ErrInvalidRate: the rate is missing, not a number, or outside the plausible bounds.
	ErrInvalidRate = fmt.Errorf("the rate must be between %.0f and %.0f francs for one US dollar", MinRate, MaxRate)
)

// Users finds the caller, to check their role.
type Users interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
}

// Service reads and sets the rate.
type Service struct {
	store domain.ExchangeRateStore
	users Users
	audit domain.AuditLogger
}

// NewService builds the service; audit may be nil.
func NewService(store domain.ExchangeRateStore, users Users, audit domain.AuditLogger) *Service {
	return &Service{store: store, users: users, audit: audit}
}

// Current returns the rate in force, or domain.ErrNoExchangeRate.
func (s *Service) Current(ctx context.Context) (*domain.ExchangeRate, error) {
	return s.store.Latest(ctx)
}

// Set records a new rate entered by an administrator (role admin only: operators read it but cannot change it).
func (s *Service) Set(ctx context.Context, actorID string, rate float64, note string) (*domain.ExchangeRate, error) {
	u, err := s.users.FindByID(ctx, actorID)
	if err != nil || u == nil || !u.HasRole(domain.RoleAdmin) {
		return nil, ErrForbidden
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < MinRate || rate > MaxRate {
		return nil, ErrInvalidRate
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 200 {
		note = string([]rune(note)[:200])
	}
	previous, _ := s.store.Latest(ctx)
	r := &domain.ExchangeRate{Base: "USD", Quote: "CDF", Rate: math.Round(rate*100) / 100, Note: note, SetBy: actorID}
	if err := s.store.Add(ctx, r); err != nil {
		return nil, err
	}
	if s.audit != nil {
		meta := map[string]interface{}{"rate": r.Rate, "note": r.Note}
		if previous != nil {
			meta["previous_rate"] = previous.Rate
		}
		_ = s.audit.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: "exchange_rate.set", TargetType: "exchange_rate", TargetID: r.ID, Metadata: meta})
	}
	return r, nil
}

// History lists the rates entered, most recent first (staff only: admin or operator).
func (s *Service) History(ctx context.Context, actorID string, limit int) ([]domain.ExchangeRate, error) {
	u, err := s.users.FindByID(ctx, actorID)
	if err != nil || u == nil || !(u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator)) {
		return nil, ErrForbidden
	}
	return s.store.History(ctx, limit)
}
