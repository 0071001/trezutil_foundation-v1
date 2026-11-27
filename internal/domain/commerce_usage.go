package domain

import "context"

// PlanRepository defines the interface for plan persistence.
// FindActive/FindByID retain the original single-global-plan semantics for the
// legacy usage path; ListByTenant/FindByTenantAndSlug back the Sprint 20
// per-tenant monetization catalog.
type PlanRepository interface {
	FindActive(ctx context.Context) (*Plan, error)
	FindByID(ctx context.Context, id string) (*Plan, error)
	ListByTenant(ctx context.Context, tenantID string) ([]*Plan, error)
	FindByTenantAndSlug(ctx context.Context, tenantID, slug string) (*Plan, error)
}

// CreditAccountRepository defines the interface for credit account persistence.
type CreditAccountRepository interface {
	Create(ctx context.Context, account *CreditAccount) error
	FindByID(ctx context.Context, accountID string) (*CreditAccount, error)
	FindByOwner(ctx context.Context, ownerType ActorType, ownerID string, tenantID string) (*CreditAccount, error)
	FindByOwnerAndPeriod(ctx context.Context, ownerType ActorType, ownerID, tenantID, periodID string) (*CreditAccount, error)
	FindOrCreateByOwnerAndPeriod(ctx context.Context, ownerType ActorType, ownerID, tenantID, planID, periodID string) (*CreditAccount, error)
	Update(ctx context.Context, account *CreditAccount) error
}

// UsageLedgerRepository defines the interface for usage event persistence.
type UsageLedgerRepository interface {
	Record(ctx context.Context, event *UsageEvent) error
	FindByAccount(ctx context.Context, accountID string, periodID string) ([]UsageEvent, error)
	FindByAction(ctx context.Context, actionType CommerceActionType, periodID string) ([]UsageEvent, error)
}

// UsagePolicyRepository defines the interface for usage policy persistence.
type UsagePolicyRepository interface {
	FindByAction(ctx context.Context, actionType CommerceActionType) (*UsagePolicy, error)
	FindAll(ctx context.Context) ([]UsagePolicy, error)
}

// UsageService defines the application service for usage & credits operations.
type UsageService interface {
	GetBalance(ctx context.Context, accountID string) (*CreditAccount, error)
	ResolvePolicy(ctx context.Context, actionType CommerceActionType, channel Channel, actorType ActorType) (*UsagePolicy, error)
	Check(ctx context.Context, accountID string, cost int64) (bool, error)
	Reserve(ctx context.Context, accountID string, actionType CommerceActionType, cost int64, idempotencyKey, tenantID string) (*UsageReservation, error)
	Consume(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
	// MarkAction records what is known of the action an active reservation pays for.
	MarkAction(ctx context.Context, reservationID string, state UsageActionState) error
	Grant(ctx context.Context, accountID string, amount int64, reason string) error
	// FindOrCreateCreditAccountByOwner finds or creates a credit account for the given owner.
	FindOrCreateCreditAccountByOwner(ctx context.Context, ownerType ActorType, ownerID string, tenantID string, planID string, periodID string) (*CreditAccount, error)
}

// UsageReservation represents a credit reservation.
type UsageReservation struct {
	ReservationID  string
	AccountID      string
	ActionType     CommerceActionType
	Amount         int64
	ReservedAt     int64
	Status         UsageReservationStatus
	IdempotencyKey string
	TenantID       string
}

// UsageReservationStatus represents the status of a reservation.
type UsageReservationStatus string

const (
	UsageReservationActive   UsageReservationStatus = "active"
	UsageReservationConsumed UsageReservationStatus = "consumed"
	UsageReservationReleased UsageReservationStatus = "released"
)

// UsageReservationRepository defines persistence for usage reservations.
// Reservations must be durable (not in-process only) to survive process restarts.
type UsageReservationRepository interface {
	Record(ctx context.Context, reservation *UsageReservation) error
	FindByID(ctx context.Context, reservationID string) (*UsageReservation, error)
	FindByAccount(ctx context.Context, accountID string) ([]UsageReservation, error)
	UpdateStatus(ctx context.Context, reservationID string, status UsageReservationStatus) error
	// ReserveCreditsTx atomically reserves credits on the account and creates a
	// durable reservation row. The idempotency_key prevents duplicate reservations
	// for the same logical action (F2/F5). The balance-guard UPDATE prevents the
	// account from going negative under concurrent reservations (F4/F6).
	ReserveCreditsTx(ctx context.Context, accountID string, actionType CommerceActionType, cost int64, idempotencyKey, tenantID string, now int64) (*UsageReservation, error)
	// ConsumeReservation atomically marks a reservation consumed, moves reserved
	// credits to consumed, and appends a ledger event. Returns the domain error
	// directly (ErrUsageReservationNotFound, ErrUsageAlreadyConsumed, etc.).
	ConsumeReservation(ctx context.Context, reservationID string, now int64) error
	// ReleaseReservation atomically marks a reservation released, returns reserved
	// credits to the account, and appends a ledger event.
	ReleaseReservation(ctx context.Context, reservationID string, now int64) error
	// MarkActionState records the action state of an active reservation.
	MarkActionState(ctx context.Context, reservationID string, state UsageActionState) error
}

// CreditGranter is the narrow, idempotent contract that monetization services depend
// on to add credits to a CreditAccount. The single GrantCreditsTx method is the atomic
// primitive: it inserts a credit_grants row (ON CONFLICT idempotency key), bumps
// credit_accounts.granted, and appends a usage_ledger event in one Postgres transaction.
// Returning granted=false signals an idempotent replay (no mutation).
type CreditGranter interface {
	GrantCreditsTx(ctx context.Context, accountID, idempotencyKey string, source CreditGrantSource, amount int64, tenantID, periodID string, paymentID *string, reason string) (granted bool, newBalance int64, err error)
}

// PaymentStatusChecker is the narrow contract monetization needs from Payment Core.
// PaymentService satisfies it via GetPaymentStatus; no payment-provider SDK touches
// the monetization domain.
type PaymentStatusChecker interface {
	GetPaymentStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*Payment, error)
}

// CreditAccountByOwnerStore resolves the credit account for an owner within a billing
// period (tenant-scoped). Extends the legacy FindByOwner, which ignored period_id.
type CreditAccountByOwnerStore interface {
	FindByOwnerAndPeriod(ctx context.Context, ownerType ActorType, ownerID, tenantID, periodID string) (*CreditAccount, error)
}

// CreditAccountCreator find-or-creates a credit account for an owner within a billing
// period, generating a proper UUID server-side and using an ON CONFLICT upsert so the
// legacy string-ID/F1 bug and the check-then-act race (F4) do not occur on the
// monetization path.
type CreditAccountCreator interface {
	FindOrCreateByOwnerAndPeriod(ctx context.Context, ownerType ActorType, ownerID, tenantID, planID, periodID string) (*CreditAccount, error)
}
