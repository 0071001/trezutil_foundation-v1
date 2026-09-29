package domain

import (
	"testing"
)

func TestNewCommerceActor(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)

	if actor.ID != "user-1" {
		t.Errorf("expected user-1, got %s", actor.ID)
	}
	if actor.Type != ActorTypeCustomer {
		t.Errorf("expected customer, got %s", actor.Type)
	}
	if actor.UserID != "" {
		t.Errorf("expected empty user ID, got %s", actor.UserID)
	}
	if actor.OrgID != "" {
		t.Errorf("expected empty org ID, got %s", actor.OrgID)
	}
}

func TestCommerceActorIsAuthenticated(t *testing.T) {
	tests := []struct {
		actorType ActorType
		expected  bool
	}{
		{ActorTypeCustomer, true},
		{ActorTypeSeller, true},
		{ActorTypeStoreOperator, true},
		{ActorTypeOrganizationAdmin, true},
		{ActorTypePlatformOperator, true},
		{ActorTypeSystem, false},
		{ActorTypeGuest, false},
	}

	for _, tt := range tests {
		actor := NewCommerceActor("user-1", tt.actorType)
		if actor.IsAuthenticated() != tt.expected {
			t.Errorf("IsAuthenticated() for %s: expected %v", tt.actorType, tt.expected)
		}
	}
}

func TestCommerceActorCanActAs(t *testing.T) {
	customer := NewCommerceActor("user-1", ActorTypeCustomer)
	platform := NewCommerceActor("admin-1", ActorTypePlatformOperator)

	if !customer.CanActAs(ActorTypeCustomer) {
		t.Error("customer should act as customer")
	}
	if customer.CanActAs(ActorTypeSeller) {
		t.Error("customer should not act as seller")
	}
	if !platform.CanActAs(ActorTypeCustomer) {
		t.Error("platform operator should act as customer")
	}
	if !platform.CanActAs(ActorTypeSeller) {
		t.Error("platform operator should act as seller")
	}
}

func TestCommerceActorHasOrganization(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	if actor.HasOrganization() {
		t.Error("expected no organization")
	}

	actor.OrgID = "org-1"
	if !actor.HasOrganization() {
		t.Error("expected organization")
	}
}
