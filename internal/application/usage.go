package application

import (
	"context"
	"fmt"
	"sync"

	"github.com/trezutil/trezutil/internal/domain"
)

// UsageService implements domain.UsageService for usage and credits management.
type UsageService struct {
	planRepo     domain.PlanRepository
	accountRepo  domain.CreditAccountRepository
	ledgerRepo   domain.UsageLedgerRepository
	policyRepo   domain.UsagePolicyRepository
	clock        domain.Clock
	reservations map[string]*domain.UsageReservation
	mu           sync.RWMutex
}

// NewUsageService creates a new UsageService.
func NewUsageService(
	planRepo domain.PlanRepository,
	accountRepo domain.CreditAccountRepository,
	ledgerRepo domain.UsageLedgerRepository,
	policyRepo domain.UsagePolicyRepository,
	clock domain.Clock,
) *UsageService {
	return &UsageService{
		planRepo:     planRepo,
		accountRepo:  accountRepo,
		ledgerRepo:   ledgerRepo,
		policyRepo:   policyRepo,
		clock:        clock,
		reservations: make(map[string]*domain.UsageReservation),
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

// Reserve credits for an action.
func (s *UsageService) Reserve(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64) (*domain.UsageReservation, error) {
	if accountID == "" {
		return nil, fmt.Errorf("accountID is required")
	}
	if cost <= 0 {
		return nil, fmt.Errorf("cost must be positive")
	}

	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return nil, domain.ErrUsageAccountNotFound
	}

	if !account.CanAfford(cost) {
		return nil, domain.ErrInsufficientCredits
	}

	now := s.clock.Now()
	reservationID := fmt.Sprintf("res-%s-%d", accountID, now)

	reservation := &domain.UsageReservation{
		ReservationID: reservationID,
		AccountID:     accountID,
		ActionType:    actionType,
		Amount:        cost,
		ReservedAt:    now,
		Status:        domain.UsageReservationActive,
	}

	account.Reserved += cost
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	event := domain.NewUsageEvent(
		reservationID,
		accountID,
		actionType,
		account.OwnerID,
		account.TenantID,
		domain.UsageReserve,
		cost,
		account.Remaining()+cost,
		account.Remaining(),
		account.PeriodID,
		now,
	)
	if err := s.ledgerRepo.Record(ctx, &event); err != nil {
		return nil, fmt.Errorf("failed to record usage event: %w", err)
	}

	s.mu.Lock()
	s.reservations[reservationID] = reservation
	s.mu.Unlock()

	return reservation, nil
}

// Consume a reservation, permanently using the reserved credits.
func (s *UsageService) Consume(ctx context.Context, reservationID string) error {
	reservation, err := s.getReservation(reservationID)
	if err != nil {
		return domain.ErrUsageReservationNotFound
	}

	if reservation.Status != domain.UsageReservationActive {
		if reservation.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageAlreadyConsumed
		}
		if reservation.Status == domain.UsageReservationReleased {
			return domain.ErrUsageAlreadyReleased
		}
		return domain.ErrUsageReservationConflict
	}

	account, err := s.accountRepo.FindByID(ctx, reservation.AccountID)
	if err != nil {
		return domain.ErrUsageAccountNotFound
	}

	now := s.clock.Now()
	account.Reserved -= reservation.Amount
	account.Consumed += reservation.Amount
	if account.Granted < account.Reserved+account.Consumed {
		account.Granted = account.Reserved + account.Consumed
	}
	account.UpdatedAt = now

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	event := domain.NewUsageEvent(
		reservationID,
		reservation.AccountID,
		reservation.ActionType,
		account.OwnerID,
		account.TenantID,
		domain.UsageConsume,
		reservation.Amount,
		account.Remaining()+reservation.Amount,
		account.Remaining(),
		account.PeriodID,
		now,
	)
	if err := s.ledgerRepo.Record(ctx, &event); err != nil {
		return fmt.Errorf("failed to record usage event: %w", err)
	}

	reservation.Status = domain.UsageReservationConsumed
	return nil
}

// Release a reservation, returning the reserved credits to the account.
func (s *UsageService) Release(ctx context.Context, reservationID string) error {
	reservation, err := s.getReservation(reservationID)
	if err != nil {
		return domain.ErrUsageReservationNotFound
	}

	if reservation.Status != domain.UsageReservationActive {
		if reservation.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageAlreadyConsumed
		}
		if reservation.Status == domain.UsageReservationReleased {
			return domain.ErrUsageAlreadyReleased
		}
		return domain.ErrUsageReservationConflict
	}

	account, err := s.accountRepo.FindByID(ctx, reservation.AccountID)
	if err != nil {
		return domain.ErrUsageAccountNotFound
	}

	now := s.clock.Now()
	account.Reserved -= reservation.Amount
	account.UpdatedAt = now

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	event := domain.NewUsageEvent(
		reservationID,
		reservation.AccountID,
		reservation.ActionType,
		account.OwnerID,
		account.TenantID,
		domain.UsageRelease,
		reservation.Amount,
		account.Remaining()+reservation.Amount,
		account.Remaining(),
		account.PeriodID,
		now,
	)
	if err := s.ledgerRepo.Record(ctx, &event); err != nil {
		return fmt.Errorf("failed to record usage event: %w", err)
	}

	reservation.Status = domain.UsageReservationReleased
	return nil
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

func (s *UsageService) getReservation(reservationID string) (*domain.UsageReservation, error) {
	s.mu.RLock()
	r, ok := s.reservations[reservationID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("reservation not found")
	}
	return r, nil
}
