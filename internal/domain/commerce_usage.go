package domain

import "context"

// PlanRepository defines the interface for plan persistence.
type PlanRepository interface {
	FindActive(ctx context.Context) (*Plan, error)
	FindByID(ctx context.Context, id string) (*Plan, error)
}

// CreditAccountRepository defines the interface for credit account persistence.
type CreditAccountRepository interface {
	Create(ctx context.Context, account *CreditAccount) error
	FindByID(ctx context.Context, accountID string) (*CreditAccount, error)
	FindByOwner(ctx context.Context, ownerType ActorType, ownerID string, tenantID string) (*CreditAccount, error)
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
	Reserve(ctx context.Context, accountID string, actionType CommerceActionType, cost int64) (*UsageReservation, error)
	Consume(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
	Grant(ctx context.Context, accountID string, amount int64, reason string) error
}

// UsageReservation represents a credit reservation.
type UsageReservation struct {
	ReservationID string
	AccountID     string
	ActionType    CommerceActionType
	Amount        int64
	ReservedAt    int64
	Status        UsageReservationStatus
}

// UsageReservationStatus represents the status of a reservation.
type UsageReservationStatus string

const (
	UsageReservationActive   UsageReservationStatus = "active"
	UsageReservationConsumed UsageReservationStatus = "consumed"
	UsageReservationReleased UsageReservationStatus = "released"
)
