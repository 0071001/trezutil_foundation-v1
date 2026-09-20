package domain

// CreditAccount represents the current usage allowance for an actor.
type CreditAccount struct {
	AccountID string
	OwnerType ActorType
	OwnerID   string
	TenantID  string
	PlanID    string
	PeriodID  string
	Granted   int64
	Reserved  int64
	Consumed  int64
	CreatedAt int64
	UpdatedAt int64
}

// Remaining returns available credits (Granted - Reserved - Consumed).
func (a CreditAccount) Remaining() int64 {
	return a.Granted - a.Reserved - a.Consumed
}

// CanAfford returns true if the account has enough unreserved credits for the given cost.
func (a CreditAccount) CanAfford(cost int64) bool {
	return a.Remaining() >= cost
}

// UsageTransactionType represents the type of usage transaction.
type UsageTransactionType string

const (
	UsageGrant    UsageTransactionType = "GRANT"
	UsageReserve  UsageTransactionType = "RESERVE"
	UsageConsume  UsageTransactionType = "CONSUME"
	UsageRelease  UsageTransactionType = "RELEASE"
	UsageRefund   UsageTransactionType = "REFUND"
	UsageExpire   UsageTransactionType = "EXPIRE"
	UsageAdjust   UsageTransactionType = "ADJUST"
	UsagePurchase UsageTransactionType = "PURCHASE"
)

// UsageEvent represents a usage transaction or event.
type UsageEvent struct {
	ID              string
	AccountID       string
	ActionType      CommerceActionType
	ActorID         string
	TenantID        string
	TransactionType UsageTransactionType
	Amount          int64
	BalanceBefore   int64
	BalanceAfter    int64
	PeriodID        string
	CreatedAt       int64
	Metadata        map[string]interface{}
}

// NewUsageEvent creates a new UsageEvent.
func NewUsageEvent(id, accountID string, actionType CommerceActionType, actorID, tenantID string, txnType UsageTransactionType, amount, balanceBefore, balanceAfter int64, periodID string, createdAt int64) UsageEvent {
	return UsageEvent{
		ID:              id,
		AccountID:       accountID,
		ActionType:      actionType,
		ActorID:         actorID,
		TenantID:        tenantID,
		TransactionType: txnType,
		Amount:          amount,
		BalanceBefore:   balanceBefore,
		BalanceAfter:    balanceAfter,
		PeriodID:        periodID,
		CreatedAt:       createdAt,
		Metadata:        make(map[string]interface{}),
	}
}
