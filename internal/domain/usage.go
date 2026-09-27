package domain

// BillingCycle represents the cadence of a paid plan's billing.
type BillingCycle string

const (
	BillingCycleMonthly BillingCycle = "monthly"
	BillingCycleYearly  BillingCycle = "yearly"
)

// PlanStatus represents the status of a plan.
type PlanStatus string

const (
	PlanStatusActive   PlanStatus = "active"
	PlanStatusInactive PlanStatus = "inactive"
	PlanStatusArchived PlanStatus = "archived"
)

// QuotaPeriod represents a billing/quota period.
type QuotaPeriod struct {
	StartTime int64
	EndTime   int64
}

// NewQuotaPeriod creates a new QuotaPeriod.
func NewQuotaPeriod(startTime, endTime int64) QuotaPeriod {
	return QuotaPeriod{
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// IsCurrent returns true if the period is currently active.
func (p QuotaPeriod) IsCurrent(now int64) bool {
	return now >= p.StartTime && now <= p.EndTime
}

// RemainingFraction returns the fraction of the period remaining (0.0 to 1.0).
func (p QuotaPeriod) RemainingFraction(now int64) float64 {
	total := p.EndTime - p.StartTime
	if total <= 0 {
		return 0
	}
	remaining := p.EndTime - now
	if remaining <= 0 {
		return 0
	}
	if remaining > total {
		return 1
	}
	return float64(remaining) / float64(total)
}

// Plan represents an entitlement plan.
// For Sprint 20 monetization, a Plan is a per-tenant template describing a pricing
// tier: a free tier grants QuotaAmount credits monthly at no charge; a paid plan
// charges PriceMinor (minor currency units) for a CreditPack of credits.
type Plan struct {
	ID              string
	Name            string
	IncludedCredits int64
	Period          QuotaPeriod
	Status          PlanStatus
	CarryOver       bool
	FairUse         bool
	CreatedAt       int64
	UpdatedAt       int64

	TenantID     string
	Slug         string
	PriceMinor   int64
	Currency     string
	BillingCycle BillingCycle
	QuotaAmount  int64
	CreditPack   int64
}

// AvailableCredits returns credits that haven't been used.
func (p Plan) AvailableCredits() int64 {
	return p.IncludedCredits
}

// PlanName returns the plan name.
func (p Plan) PlanName() string {
	return p.Name
}

// IsFree returns true if the plan has no charge (free tier / allocation).
func (p Plan) IsFree() bool {
	return p.PriceMinor <= 0
}

// IsPaid returns true if the plan charges a nonzero price.
func (p Plan) IsPaid() bool {
	return p.PriceMinor > 0
}

// IsArchived returns true if the plan can no longer be subscribed to.
func (p Plan) IsArchived() bool {
	return p.Status == PlanStatusArchived
}

// IsActive returns true if the plan can be subscribed to.
func (p Plan) IsActive() bool {
	return p.Status == PlanStatusActive
}
