package domain

// UsagePolicy represents the cost of a commerce action.
type UsagePolicy struct {
	ID         string
	ActionType CommerceActionType
	Channel    Channel
	ActorType  ActorType
	Cost       int64
	Version    int
	Active     bool
	CreatedAt  int64
	UpdatedAt  int64
}

// IsFree returns true if the action has no usage cost.
func (p UsagePolicy) IsFree() bool {
	return p.Cost == 0
}

// IsMetered returns true if the action has a usage cost.
func (p UsagePolicy) IsMetered() bool {
	return p.Cost > 0
}

// IsActive returns true if the policy is currently active.
func (p UsagePolicy) IsActive() bool {
	return p.Active
}
