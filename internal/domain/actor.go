package domain

// ActorType represents the type of an actor in a commerce action.
type ActorType string

const (
	ActorTypeCustomer          ActorType = "customer"
	ActorTypeSeller            ActorType = "seller"
	ActorTypeStoreOperator     ActorType = "store_operator"
	ActorTypeOrganizationAdmin ActorType = "organization_admin"
	ActorTypePlatformOperator  ActorType = "platform_operator"
	ActorTypeSystem            ActorType = "system"
)

// CommerceActor represents an actor in a commerce action.
// This actor is resolved from authenticated identity, NOT from AI/client input.
type CommerceActor struct {
	ID     string
	Type   ActorType
	UserID string
	OrgID  string
}

// NewCommerceActor creates a commerce actor from authenticated identity.
func NewCommerceActor(id string, actorType ActorType) CommerceActor {
	return CommerceActor{
		ID:   id,
		Type: actorType,
	}
}

// IsAuthenticated returns true if the actor is not a system actor.
func (a CommerceActor) IsAuthenticated() bool {
	return a.Type != ActorTypeSystem
}

// CanActAs returns true if this actor can act as the given type.
// Platform operators can act as any type.
func (a CommerceActor) CanActAs(target ActorType) bool {
	if a.Type == ActorTypePlatformOperator {
		return true
	}
	return a.Type == target
}

// HasOrganization returns true if the actor belongs to an organization.
func (a CommerceActor) HasOrganization() bool {
	return a.OrgID != ""
}
