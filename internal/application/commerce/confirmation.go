package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// ConfirmationPolicy defines rules for when a commerce action requires confirmation.
type ConfirmationPolicy interface {
	RequiresConfirmation(ctx context.Context, action *domain.CommerceAction) (bool, string)
}

// DefaultConfirmationPolicy implements confirmation rules by action category.
// Write and Financial actions require confirmation; Read actions do not.
type DefaultConfirmationPolicy struct{}

// NewDefaultConfirmationPolicy creates a new DefaultConfirmationPolicy.
func NewDefaultConfirmationPolicy() *DefaultConfirmationPolicy {
	return &DefaultConfirmationPolicy{}
}

// RequiresConfirmation returns true if the action requires user confirmation.
func (p *DefaultConfirmationPolicy) RequiresConfirmation(ctx context.Context, action *domain.CommerceAction) (bool, string) {
	if action == nil {
		return false, ""
	}

	switch action.ActionType {
	case domain.ActionCreateListing, domain.ActionUpdateListing, domain.ActionPublishListing, domain.ActionPauseListing,
		domain.ActionCreateOrder, domain.ActionCancelOrder, domain.ActionCreateCampaign:
		return true, "This action requires confirmation before execution"
	case domain.ActionInitiatePayment, domain.ActionCancelPayment, domain.ActionRefundPayment,
		domain.ActionAuthorizePayment, domain.ActionCapturePayment:
		return true, "Payment action requires confirmation before execution"
	default:
		return false, ""
	}
}
