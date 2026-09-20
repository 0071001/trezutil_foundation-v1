package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestDefaultConfirmationPolicy_NilAction(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	needs, reason := policy.RequiresConfirmation(context.Background(), nil)
	if needs {
		t.Error("expected no confirmation for nil action")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %s", reason)
	}
}

func TestDefaultConfirmationPolicy_ReadActions(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	readActions := []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionGetListing,
		domain.ActionCompareListings,
		domain.ActionGetOrder,
		domain.ActionGetPaymentStatus,
		domain.ActionGetDeliveryStatus,
		domain.ActionCheckInventory,
		domain.ActionGetSellerReport,
	}
	for _, at := range readActions {
		action := domain.NewCommerceAction(at, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
		needs, _ := policy.RequiresConfirmation(context.Background(), action)
		if needs {
			t.Errorf("expected no confirmation for %s", at)
		}
	}
}

func TestDefaultConfirmationPolicy_WriteActions(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	writeActions := []domain.CommerceActionType{
		domain.ActionCreateListing,
		domain.ActionUpdateListing,
		domain.ActionPublishListing,
		domain.ActionPauseListing,
		domain.ActionCreateOrder,
		domain.ActionCancelOrder,
		domain.ActionCreateCampaign,
	}
	for _, at := range writeActions {
		action := domain.NewCommerceAction(at, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
		needs, _ := policy.RequiresConfirmation(context.Background(), action)
		if !needs {
			t.Errorf("expected confirmation for %s", at)
		}
	}
}

func TestDefaultConfirmationPolicy_FinancialActions(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	action := domain.NewCommerceAction(domain.ActionInitiatePayment, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
	needs, reason := policy.RequiresConfirmation(context.Background(), action)
	if !needs {
		t.Error("expected confirmation for payment action")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDefaultConfirmationPolicy_ConfirmedActionSkips(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	action := domain.NewCommerceAction(domain.ActionCreateOrder, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
	action.ConfirmationState = domain.ConfirmationApproved
	_, reason := policy.RequiresConfirmation(context.Background(), action)
	if reason == "" {
		t.Error("expected non-empty reason for write action")
	}
}

func TestDefaultConfirmationPolicy_ActionTypeDeterminesConfirmation(t *testing.T) {
	policy := NewDefaultConfirmationPolicy()
	readAction := domain.NewCommerceAction(domain.ActionSearchListings, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
	writeAction := domain.NewCommerceAction(domain.ActionCreateOrder, domain.NewCommerceActor("u1", domain.ActorTypeCustomer), "c1")
	writeAction.ConfirmationState = domain.ConfirmationApproved

	readNeeds, _ := policy.RequiresConfirmation(context.Background(), readAction)
	if readNeeds {
		t.Error("expected no confirmation for read action")
	}
	writeNeeds, _ := policy.RequiresConfirmation(context.Background(), writeAction)
	if !writeNeeds {
		t.Error("expected confirmation for write action regardless of confirmation state")
	}
}
