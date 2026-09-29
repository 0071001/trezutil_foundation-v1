package domain

import (
	"testing"
)

func TestNewCommerceAction(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	action := NewCommerceAction(ActionSearchListings, actor, "conv-1")

	if action.ActionType != ActionSearchListings {
		t.Errorf("expected SEARCH_LISTINGS, got %s", action.ActionType)
	}
	if action.Actor.ID != "user-1" {
		t.Errorf("expected user-1, got %s", action.Actor.ID)
	}
	if action.Actor.Type != ActorTypeCustomer {
		t.Errorf("expected customer, got %s", action.Actor.Type)
	}
	if action.ConversationID != "conv-1" {
		t.Errorf("expected conv-1, got %s", action.ConversationID)
	}
	if action.TenantID != "" {
		t.Errorf("expected empty tenant ID, got %s", action.TenantID)
	}
	if action.Source != "USER" {
		t.Errorf("expected USER, got %s", action.Source)
	}
	if action.ConfirmationState != ConfirmationNotRequired {
		t.Errorf("expected not_required, got %s", action.ConfirmationState)
	}
	if action.Parameters == nil {
		t.Error("expected parameters map to be initialized")
	}
	// Every action gets a unique ID up front: the runtime uses it as the
	// credit-reservation idempotency key, so an empty or shared ID would make
	// unrelated actions collapse onto one reservation.
	if action.ActionID == "" {
		t.Error("expected constructor to assign an action ID")
	}
	if other := NewCommerceAction(ActionSearchListings, actor, "conv-1"); other.ActionID == action.ActionID {
		t.Error("expected each action to receive a unique ID")
	}
	if action.RequestedAt == 0 {
		t.Error("expected requested_at to be set")
	}
}

func TestCommerceActionWithParameters(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	action := NewCommerceAction(ActionSearchListings, actor, "conv-1")
	action.WithParameters(map[string]interface{}{
		"query":     "Samsung",
		"max_price": 300,
	})

	if action.Parameters["query"] != "Samsung" {
		t.Errorf("expected Samsung, got %v", action.Parameters["query"])
	}
}

func TestCommerceActionWithIdempotencyKey(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	action := NewCommerceAction(ActionSearchListings, actor, "conv-1")
	action.WithIdempotencyKey("key-123")

	if action.IdempotencyKey != "key-123" {
		t.Errorf("expected key-123, got %s", action.IdempotencyKey)
	}
	if !action.IsIdempotent() {
		t.Error("expected action to be idempotent")
	}
}

func TestCommerceActionWithTenant(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	action := NewCommerceAction(ActionSearchListings, actor, "conv-1")
	action.WithTenant("org-1")

	if action.TenantID != "org-1" {
		t.Errorf("expected org-1, got %s", action.TenantID)
	}
}

func TestCommerceActionWithSource(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)
	action := NewCommerceAction(ActionSearchListings, actor, "conv-1")
	action.WithSource("AI")

	if action.Source != "AI" {
		t.Errorf("expected AI, got %s", action.Source)
	}
}

func TestCommerceActionRequiresConfirmation(t *testing.T) {
	tests := []struct {
		state    ConfirmationState
		expected bool
	}{
		{ConfirmationPending, true},
		{ConfirmationRequested, true},
		{ConfirmationApproved, false},
		{ConfirmationRejected, false},
		{ConfirmationNotRequired, false},
	}

	for _, tt := range tests {
		action := NewCommerceAction(ActionSearchListings, NewCommerceActor("user-1", ActorTypeCustomer), "conv-1")
		action.ConfirmationState = tt.state

		if action.RequiresConfirmation() != tt.expected {
			t.Errorf("RequiresConfirmation() for %s: expected %v", tt.state, tt.expected)
		}
	}
}

func TestCommerceActionIsConfirmed(t *testing.T) {
	action := NewCommerceAction(ActionSearchListings, NewCommerceActor("user-1", ActorTypeCustomer), "conv-1")

	if action.IsConfirmed() {
		t.Error("expected not confirmed for not_required")
	}

	action.ConfirmationState = ConfirmationApproved
	if !action.IsConfirmed() {
		t.Error("expected confirmed for approved")
	}
}

func TestCommerceActionValidate(t *testing.T) {
	actor := NewCommerceActor("user-1", ActorTypeCustomer)

	tests := []struct {
		name    string
		action  *CommerceAction
		wantErr bool
	}{
		{
			name:    "valid action",
			action:  NewCommerceAction(ActionSearchListings, actor, "conv-1"),
			wantErr: false,
		},
		{
			name: "missing action type",
			action: func() *CommerceAction {
				a := NewCommerceAction(ActionSearchListings, actor, "conv-1")
				a.ActionType = ""
				return a
			}(),
			wantErr: true,
		},
		{
			name: "missing actor ID",
			action: func() *CommerceAction {
				a := NewCommerceAction(ActionSearchListings, actor, "conv-1")
				a.Actor.ID = ""
				return a
			}(),
			wantErr: true,
		},
		{
			name: "missing actor type",
			action: func() *CommerceAction {
				a := NewCommerceAction(ActionSearchListings, actor, "conv-1")
				a.Actor.Type = ""
				return a
			}(),
			wantErr: true,
		},
		{
			name: "missing conversation ID",
			action: func() *CommerceAction {
				a := NewCommerceAction(ActionSearchListings, actor, "conv-1")
				a.ConversationID = ""
				return a
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(): expected error=%v, got error=%v", tt.wantErr, err != nil)
			}
		})
	}
}

func TestCommerceActionGetCategory(t *testing.T) {
	tests := []struct {
		actionType CommerceActionType
		expected   ActionCategory
	}{
		{ActionSearchListings, ActionCategoryRead},
		{ActionGetListing, ActionCategoryRead},
		{ActionCompareListings, ActionCategoryRead},
		{ActionCreateListing, ActionCategoryWrite},
		{ActionUpdateListing, ActionCategoryWrite},
		{ActionPublishListing, ActionCategoryWrite},
		{ActionPauseListing, ActionCategoryWrite},
		{ActionCreateOrder, ActionCategoryWrite},
		{ActionGetOrder, ActionCategoryRead},
		{ActionCancelOrder, ActionCategoryWrite},
		{ActionCheckInventory, ActionCategoryRead},
		{ActionInitiatePayment, ActionCategoryFinancial},
		{ActionGetPaymentStatus, ActionCategoryRead},
		{ActionGetDeliveryStatus, ActionCategoryRead},
		{ActionCreateCampaign, ActionCategoryWrite},
		{ActionGetSellerReport, ActionCategoryRead},
	}

	for _, tt := range tests {
		result := tt.actionType.GetCategory()
		if result != tt.expected {
			t.Errorf("GetCategory() for %s: expected %s, got %s", tt.actionType, tt.expected, result)
		}
	}
}
