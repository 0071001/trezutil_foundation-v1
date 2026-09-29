package application

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakePlanRepo struct {
	plan *domain.Plan
	err  error
}

func (f *fakePlanRepo) FindActive(ctx context.Context) (*domain.Plan, error) {
	return f.plan, f.err
}
func (f *fakePlanRepo) FindByID(ctx context.Context, id string) (*domain.Plan, error) {
	if f.plan != nil && f.plan.ID == id {
		return f.plan, nil
	}
	return nil, domain.ErrUsagePlanNotFound
}
func (f *fakePlanRepo) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Plan, error) {
	if f.plan != nil && f.plan.TenantID == tenantID {
		return []*domain.Plan{f.plan}, nil
	}
	return nil, nil
}
func (f *fakePlanRepo) FindByTenantAndSlug(ctx context.Context, tenantID, slug string) (*domain.Plan, error) {
	if f.plan != nil && f.plan.TenantID == tenantID && f.plan.Slug == slug {
		return f.plan, nil
	}
	return nil, domain.ErrUsagePlanNotFound
}

type fakeAccountRepo struct {
	account *domain.CreditAccount
	err     error
}

func (f *fakeAccountRepo) Create(ctx context.Context, account *domain.CreditAccount) error {
	f.account = account
	return nil
}
func (f *fakeAccountRepo) FindByID(ctx context.Context, accountID string) (*domain.CreditAccount, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.account != nil && f.account.AccountID == accountID {
		return f.account, nil
	}
	return nil, domain.ErrUsageAccountNotFound
}
func (f *fakeAccountRepo) FindByOwner(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string) (*domain.CreditAccount, error) {
	if f.account != nil && f.account.OwnerID == ownerID {
		return f.account, nil
	}
	return nil, domain.ErrUsageAccountNotFound
}

func (f *fakeAccountRepo) FindByOwnerAndPeriod(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string, periodID string) (*domain.CreditAccount, error) {
	if f.account != nil && f.account.OwnerID == ownerID && f.account.TenantID == tenantID && f.account.PeriodID == periodID {
		return f.account, nil
	}
	return nil, domain.ErrUsageAccountNotFound
}

func (f *fakeAccountRepo) FindOrCreateByOwnerAndPeriod(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string, planID string, periodID string) (*domain.CreditAccount, error) {
	if f.account != nil && f.account.OwnerID == ownerID && f.account.TenantID == tenantID && f.account.PeriodID == periodID {
		return f.account, nil
	}
	now := int64(1000000)
	if f.account != nil {
		now = f.account.CreatedAt
	}
	f.account = &domain.CreditAccount{
		AccountID: "acc-" + ownerID,
		OwnerType: ownerType,
		OwnerID:   ownerID,
		TenantID:  tenantID,
		PlanID:    planID,
		PeriodID:  periodID,
		Granted:   0,
		Reserved:  0,
		Consumed:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return f.account, nil
}

func (f *fakeAccountRepo) Update(ctx context.Context, account *domain.CreditAccount) error {
	f.account = account
	return nil
}

type fakeLedgerRepo struct {
	events []*domain.UsageEvent
	err    error
}

func (f *fakeLedgerRepo) Record(ctx context.Context, event *domain.UsageEvent) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}
func (f *fakeLedgerRepo) FindByAccount(ctx context.Context, accountID string, periodID string) ([]domain.UsageEvent, error) {
	return nil, nil
}
func (f *fakeLedgerRepo) FindByAction(ctx context.Context, actionType domain.CommerceActionType, periodID string) ([]domain.UsageEvent, error) {
	return nil, nil
}

type fakePolicyRepo struct {
	policies []domain.UsagePolicy
	err      error
}

func (f *fakePolicyRepo) FindByAction(ctx context.Context, actionType domain.CommerceActionType) (*domain.UsagePolicy, error) {
	for i := range f.policies {
		if f.policies[i].ActionType == actionType {
			return &f.policies[i], nil
		}
	}
	return nil, domain.ErrUsagePolicyNotFound
}
func (f *fakePolicyRepo) FindAll(ctx context.Context) ([]domain.UsagePolicy, error) {
	return f.policies, nil
}

type fakeClock struct{}

func (f *fakeClock) Now() int64 {
	return 1000000
}

func NewTestUsageService(
	planRepo *fakePlanRepo,
	accountRepo *fakeAccountRepo,
	ledgerRepo *fakeLedgerRepo,
	policyRepo *fakePolicyRepo,
) *UsageService {
	fakeResRepo := newFakeReservationRepo(accountRepo, ledgerRepo, &fakeClock{})
	return NewUsageService(planRepo, accountRepo, ledgerRepo, policyRepo, fakeResRepo, &fakeClock{})
}

type fakeReservationRepo struct {
	accountRepo  *fakeAccountRepo
	ledgerRepo   *fakeLedgerRepo
	clock        domain.Clock
	reservations map[string]*domain.UsageReservation
	mu           sync.RWMutex
}

func newFakeReservationRepo(accountRepo *fakeAccountRepo, ledgerRepo *fakeLedgerRepo, clock domain.Clock) *fakeReservationRepo {
	return &fakeReservationRepo{
		accountRepo:  accountRepo,
		ledgerRepo:   ledgerRepo,
		clock:        clock,
		reservations: make(map[string]*domain.UsageReservation),
	}
}

func (f *fakeReservationRepo) Record(ctx context.Context, reservation *domain.UsageReservation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reservations[reservation.ReservationID] = reservation
	return nil
}

func (f *fakeReservationRepo) FindByID(ctx context.Context, reservationID string) (*domain.UsageReservation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if r, ok := f.reservations[reservationID]; ok {
		return r, nil
	}
	return nil, domain.ErrUsageReservationNotFound
}

func (f *fakeReservationRepo) FindByAccount(ctx context.Context, accountID string) ([]domain.UsageReservation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []domain.UsageReservation
	for _, r := range f.reservations {
		if r.AccountID == accountID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeReservationRepo) UpdateStatus(ctx context.Context, reservationID string, status domain.UsageReservationStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.reservations[reservationID]; ok {
		r.Status = status
		return nil
	}
	return domain.ErrUsageReservationNotFound
}

func (f *fakeReservationRepo) ReserveCreditsTx(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64, idempotencyKey, tenantID string, now int64) (*domain.UsageReservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, r := range f.reservations {
		if r.IdempotencyKey == idempotencyKey && r.Status == domain.UsageReservationActive {
			return r, nil
		}
	}

	account, err := f.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return nil, domain.ErrUsageAccountNotFound
	}
	if !account.CanAfford(cost) {
		return nil, domain.ErrInsufficientCredits
	}

	resID := fmt.Sprintf("res-%s-%d", accountID, now)
	reservation := &domain.UsageReservation{
		ReservationID:  resID,
		AccountID:      accountID,
		ActionType:     actionType,
		Amount:         cost,
		ReservedAt:     now,
		Status:         domain.UsageReservationActive,
		IdempotencyKey: idempotencyKey,
		TenantID:       tenantID,
	}

	account.Reserved += cost
	_ = f.accountRepo.Update(ctx, account)

	event := domain.NewUsageEvent(
		resID, accountID, actionType,
		account.OwnerID, account.TenantID,
		domain.UsageReserve, cost,
		account.Remaining()+cost, account.Remaining(),
		account.PeriodID, now,
	)
	_ = f.ledgerRepo.Record(ctx, &event)

	f.reservations[resID] = reservation
	return reservation, nil
}

func (f *fakeReservationRepo) ConsumeReservation(ctx context.Context, reservationID string, now int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	res, ok := f.reservations[reservationID]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	if res.Status != domain.UsageReservationActive {
		if res.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageAlreadyConsumed
		}
		if res.Status == domain.UsageReservationReleased {
			return domain.ErrUsageReservationConflict
		}
		return domain.ErrUsageReservationConflict
	}

	account, err := f.accountRepo.FindByID(ctx, res.AccountID)
	if err != nil {
		return domain.ErrUsageAccountNotFound
	}

	account.Reserved -= res.Amount
	account.Consumed += res.Amount
	_ = f.accountRepo.Update(ctx, account)

	res.Status = domain.UsageReservationConsumed

	event := domain.NewUsageEvent(
		reservationID, res.AccountID, res.ActionType,
		account.OwnerID, account.TenantID,
		domain.UsageConsume, res.Amount,
		account.Remaining()+res.Amount, account.Remaining(),
		account.PeriodID, now,
	)
	_ = f.ledgerRepo.Record(ctx, &event)
	return nil
}

func (f *fakeReservationRepo) ReleaseReservation(ctx context.Context, reservationID string, now int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	res, ok := f.reservations[reservationID]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	if res.Status != domain.UsageReservationActive {
		if res.Status == domain.UsageReservationReleased {
			return domain.ErrUsageAlreadyReleased
		}
		if res.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageReservationConflict
		}
		return domain.ErrUsageReservationConflict
	}

	account, err := f.accountRepo.FindByID(ctx, res.AccountID)
	if err != nil {
		return domain.ErrUsageAccountNotFound
	}

	account.Reserved -= res.Amount
	_ = f.accountRepo.Update(ctx, account)

	res.Status = domain.UsageReservationReleased

	event := domain.NewUsageEvent(
		reservationID, res.AccountID, res.ActionType,
		account.OwnerID, account.TenantID,
		domain.UsageRelease, res.Amount,
		account.Remaining()+res.Amount, account.Remaining(),
		account.PeriodID, now,
	)
	_ = f.ledgerRepo.Record(ctx, &event)
	return nil
}

func TestUsageService_GetBalance_Success(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 10, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	result, err := svc.GetBalance(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Remaining() != 85 {
		t.Errorf("expected 85, got %d", result.Remaining())
	}
}

func TestUsageService_GetBalance_NotFound(t *testing.T) {
	repo := &fakeAccountRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	_, err := svc.GetBalance(context.Background(), "missing")
	if err != domain.ErrUsageAccountNotFound {
		t.Errorf("expected ErrUsageAccountNotFound, got %v", err)
	}
}

func TestUsageService_ResolvePolicy_Success(t *testing.T) {
	policies := []domain.UsagePolicy{
		{ActionType: domain.ActionSearchListings, Cost: 2, Active: true},
	}
	repo := &fakePolicyRepo{policies: policies}
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, repo)

	result, err := svc.ResolvePolicy(context.Background(), domain.ActionSearchListings, domain.ChannelWeb, domain.ActorTypeCustomer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Cost != 2 {
		t.Errorf("expected cost 2, got %d", result.Cost)
	}
}

func TestUsageService_ResolvePolicy_NotFound(t *testing.T) {
	repo := &fakePolicyRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, repo)

	_, err := svc.ResolvePolicy(context.Background(), domain.ActionCreateOrder, domain.ChannelWeb, domain.ActorTypeCustomer)
	if err != domain.ErrUsagePolicyNotFound {
		t.Errorf("expected ErrUsagePolicyNotFound, got %v", err)
	}
}

func TestUsageService_Check_Sufficient(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 10, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	ok, err := svc.Check(context.Background(), "acc-1", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected sufficient credits")
	}
}

func TestUsageService_Check_Insufficient(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 90, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	ok, err := svc.Check(context.Background(), "acc-1", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected insufficient credits")
	}
}

func TestUsageService_Check_ZeroCost(t *testing.T) {
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, &fakePolicyRepo{})
	ok, err := svc.Check(context.Background(), "acc-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected zero cost to always pass")
	}
}

func TestUsageService_Reserve_Success(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 10, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reservation == nil {
		t.Fatal("expected non-nil reservation")
	}
	if reservation.Amount != 20 {
		t.Errorf("expected amount 20, got %d", reservation.Amount)
	}
	if reservation.Status != domain.UsageReservationActive {
		t.Errorf("expected ACTIVE, got %s", reservation.Status)
	}
	if account.Reserved != 30 {
		t.Errorf("expected Reserved=30, got %d", account.Reserved)
	}
}

func TestUsageService_Reserve_InsufficientCredits(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 95, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	_, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 10, "test-key", "tenant-1")
	if err != domain.ErrInsufficientCredits {
		t.Errorf("expected ErrInsufficientCredits, got %v", err)
	}
}

func TestUsageService_Reserve_InvalidCost(t *testing.T) {
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, &fakePolicyRepo{})

	_, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 0, "test-key", "tenant-1")
	if err == nil {
		t.Error("expected error for zero cost")
	}
}

func TestUsageService_Consume_Success(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 0, Consumed: 0}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	if err := svc.Consume(context.Background(), reservation.ReservationID); err != nil {
		t.Fatalf("consume failed: %v", err)
	}
	if account.Reserved != 0 {
		t.Errorf("expected Reserved=0, got %d", account.Reserved)
	}
	if account.Consumed != 20 {
		t.Errorf("expected Consumed=20, got %d", account.Consumed)
	}
}

func TestUsageService_Consume_NotFound(t *testing.T) {
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, &fakePolicyRepo{})

	err := svc.Consume(context.Background(), "missing")
	if err != domain.ErrUsageReservationNotFound {
		t.Errorf("expected ErrUsageReservationNotFound, got %v", err)
	}
}

func TestUsageService_Consume_AlreadyConsumed(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 20, Consumed: 0}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	if err := svc.Consume(context.Background(), reservation.ReservationID); err != nil {
		t.Fatalf("first consume failed: %v", err)
	}

	err = svc.Consume(context.Background(), reservation.ReservationID)
	if err != domain.ErrUsageAlreadyConsumed {
		t.Errorf("expected ErrUsageAlreadyConsumed, got %v", err)
	}
}

func TestUsageService_Release_Success(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 0, Consumed: 0}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	if err := svc.Release(context.Background(), reservation.ReservationID); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if account.Reserved != 0 {
		t.Errorf("expected Reserved=0, got %d", account.Reserved)
	}
}

func TestUsageService_Release_AlreadyReleased(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 20, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	if err := svc.Release(context.Background(), reservation.ReservationID); err != nil {
		t.Fatalf("first release failed: %v", err)
	}

	err = svc.Release(context.Background(), reservation.ReservationID)
	if err != domain.ErrUsageAlreadyReleased {
		t.Errorf("expected ErrUsageAlreadyReleased, got %v", err)
	}
}

func TestUsageService_Grant_Success(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 10, Consumed: 5}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	if err := svc.Grant(context.Background(), "acc-1", 50, "promotion"); err != nil {
		t.Fatalf("grant failed: %v", err)
	}
	if account.Granted != 150 {
		t.Errorf("expected Granted=150, got %d", account.Granted)
	}
}

func TestUsageService_Grant_InvalidAmount(t *testing.T) {
	svc := NewTestUsageService(&fakePlanRepo{}, &fakeAccountRepo{}, &fakeLedgerRepo{}, &fakePolicyRepo{})

	err := svc.Grant(context.Background(), "acc-1", 0, "promotion")
	if err == nil {
		t.Error("expected error for zero amount")
	}
}

func TestUsageService_Reserve_Consume_Release_Workflow(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 100, Reserved: 0, Consumed: 0}
	repo := &fakeAccountRepo{account: account}
	ledger := &fakeLedgerRepo{}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, ledger, &fakePolicyRepo{})

	// Reserve
	reservation, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if account.Remaining() != 80 {
		t.Errorf("after reserve: expected Remaining=80, got %d", account.Remaining())
	}

	// Release (simulating action failure)
	if err := svc.Release(context.Background(), reservation.ReservationID); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if account.Remaining() != 100 {
		t.Errorf("after release: expected Remaining=100, got %d", account.Remaining())
	}

	// Reserve again
	reservation2, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != nil {
		t.Fatalf("second reserve failed: %v", err)
	}

	// Consume (simulating action success)
	if err := svc.Consume(context.Background(), reservation2.ReservationID); err != nil {
		t.Fatalf("consume failed: %v", err)
	}
	if account.Remaining() != 80 {
		t.Errorf("after consume: expected Remaining=80, got %d", account.Remaining())
	}
	if account.Consumed != 20 {
		t.Errorf("expected Consumed=20, got %d", account.Consumed)
	}
}

func TestUsageService_NegativeBalancePrevention(t *testing.T) {
	account := &domain.CreditAccount{AccountID: "acc-1", Granted: 10, Reserved: 0, Consumed: 0}
	repo := &fakeAccountRepo{account: account}
	svc := NewTestUsageService(&fakePlanRepo{}, repo, &fakeLedgerRepo{}, &fakePolicyRepo{})

	_, err := svc.Reserve(context.Background(), "acc-1", domain.ActionSearchListings, 20, "test-key", "tenant-1")
	if err != domain.ErrInsufficientCredits {
		t.Errorf("expected ErrInsufficientCredits, got %v", err)
	}
	if account.Remaining() != 10 {
		t.Errorf("balance should not change, got %d", account.Remaining())
	}
}
