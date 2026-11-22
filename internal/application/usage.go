package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// UsageService implements domain.UsageService for usage and credits management.
type UsageService struct {
	planRepo        domain.PlanRepository
	accountRepo     domain.CreditAccountRepository
	ledgerRepo      domain.UsageLedgerRepository
	policyRepo      domain.UsagePolicyRepository
	reservationRepo domain.UsageReservationRepository
	clock           domain.Clock
	personal        domain.PersonalSpaces
}

// WithPersonalSpaces lets a person without an organisation hold credits: their account lives in their personal
// ('individual') organisation, so the same accounts, ledger, grants and purchases apply (migration 000033).
func (s *UsageService) WithPersonalSpaces(personal domain.PersonalSpaces) *UsageService {
	s.personal = personal
	return s
}

// NewUsageService creates a new UsageService.
func NewUsageService(
	planRepo domain.PlanRepository,
	accountRepo domain.CreditAccountRepository,
	ledgerRepo domain.UsageLedgerRepository,
	policyRepo domain.UsagePolicyRepository,
	reservationRepo domain.UsageReservationRepository,
	clock domain.Clock,
) *UsageService {
	return &UsageService{
		planRepo:        planRepo,
		accountRepo:     accountRepo,
		ledgerRepo:      ledgerRepo,
		policyRepo:      policyRepo,
		reservationRepo: reservationRepo,
		clock:           clock,
	}
}

// GetBalance returns the current balance of a credit account.
func (s *UsageService) GetBalance(ctx context.Context, accountID string) (*domain.CreditAccount, error) {
	if accountID == "" {
		return nil, fmt.Errorf("accountID is required")
	}
	return s.accountRepo.FindByID(ctx, accountID)
}

// ResolvePolicy returns the usage policy for a given action, channel, and actor type.
func (s *UsageService) ResolvePolicy(ctx context.Context, actionType domain.CommerceActionType, channel domain.Channel, actorType domain.ActorType) (*domain.UsagePolicy, error) {
	policies, err := s.policyRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range policies {
		if p.ActionType == actionType && p.IsActive() {
			return &p, nil
		}
	}

	return nil, domain.ErrUsagePolicyNotFound
}

// Check returns true if the account has enough credits for the given cost.
func (s *UsageService) Check(ctx context.Context, accountID string, cost int64) (bool, error) {
	if cost <= 0 {
		return true, nil
	}
	account, err := s.GetBalance(ctx, accountID)
	if err != nil {
		return false, err
	}
	return account.CanAfford(cost), nil
}

// Reserve credits for an action. Delegates to the durable, atomic
// ReserveCreditsTx primitive (F2/F4/F5/F6): UUID reservation id generated
// server-side, idempotent via idempotency_key, and balance-guard UPDATE
// prevents negative credits under concurrent access.
func (s *UsageService) Reserve(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64, idempotencyKey string, tenantID string) (*domain.UsageReservation, error) {
	if accountID == "" {
		return nil, fmt.Errorf("accountID is required")
	}
	if cost <= 0 {
		return nil, fmt.Errorf("cost must be positive")
	}
	if s.reservationRepo == nil {
		return nil, domain.ErrNotImplemented
	}
	// The reservation and its ledger entry belong to the account's organisation, whatever the caller passed (the
	// runtime passes the user id for a personal account, which no organisation has).
	if s.accountRepo != nil {
		if account, err := s.accountRepo.FindByID(ctx, accountID); err == nil && account != nil && account.TenantID != "" {
			tenantID = account.TenantID
		}
	}
	return s.reservationRepo.ReserveCreditsTx(ctx, accountID, actionType, cost, idempotencyKey, tenantID, s.clock.Now())
}

// Consume a reservation, permanently using the reserved credits.
// Delegates to the durable, atomic ConsumeReservation primitive.
func (s *UsageService) Consume(ctx context.Context, reservationID string) error {
	if reservationID == "" {
		return fmt.Errorf("reservationID is required")
	}
	if s.reservationRepo == nil {
		return domain.ErrNotImplemented
	}
	return s.reservationRepo.ConsumeReservation(ctx, reservationID, s.clock.Now())
}

// Release a reservation, returning the reserved credits to the account.
// Delegates to the durable, atomic ReleaseReservation primitive.
func (s *UsageService) Release(ctx context.Context, reservationID string) error {
	if reservationID == "" {
		return fmt.Errorf("reservationID is required")
	}
	if s.reservationRepo == nil {
		return domain.ErrNotImplemented
	}
	return s.reservationRepo.ReleaseReservation(ctx, reservationID, s.clock.Now())
}

// Grant adds credits to an account.
func (s *UsageService) Grant(ctx context.Context, accountID string, amount int64, reason string) error {
	if accountID == "" {
		return fmt.Errorf("accountID is required")
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return domain.ErrUsageAccountNotFound
	}

	now := s.clock.Now()
	account.Granted += amount
	account.UpdatedAt = now

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	event := domain.NewUsageEvent(
		fmt.Sprintf("grant-%s-%d", accountID, now),
		accountID,
		"",
		account.OwnerID,
		account.TenantID,
		domain.UsageGrant,
		amount,
		account.Remaining()-amount,
		account.Remaining(),
		account.PeriodID,
		now,
	)
	event.Metadata["reason"] = reason
	if err := s.ledgerRepo.Record(ctx, &event); err != nil {
		return fmt.Errorf("failed to record usage event: %w", err)
	}

	return nil
}

// FindOrCreateCreditAccountByOwner finds or creates a credit account for the given owner.
// Delegates to the atomic FindOrCreateByOwnerAndPeriod to fix the check-then-act race
// and the legacy string-ID bug (F1/F4).
func (s *UsageService) FindOrCreateCreditAccountByOwner(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string, planID string, periodID string) (*domain.CreditAccount, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("ownerID is required")
	}
	// A customer acting for themself has no organisation: callers pass "" or their own id (the runtime's fallback),
	// which no organisation has. Their credits are in their personal space.
	if ownerType == domain.ActorTypeCustomer && (tenantID == "" || tenantID == ownerID) && s.personal != nil {
		personal, err := s.personal.Ensure(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("personal credit space: %w", err)
		}
		tenantID = personal
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}
	if periodID == "" {
		periodID = time.Now().Format("2006-01")
	}
	if s.accountRepo == nil {
		return nil, domain.ErrNotImplemented
	}
	return s.accountRepo.FindOrCreateByOwnerAndPeriod(ctx, ownerType, ownerID, tenantID, planID, periodID)
}
