package conversation

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ActionExecutor executes validated and authorized commerce actions.
type ActionExecutor struct {
	authResolver AuthorizationResolver
}

// NewActionExecutor creates a new ActionExecutor.
func NewActionExecutor(authResolver AuthorizationResolver) *ActionExecutor {
	return &ActionExecutor{authResolver: authResolver}
}

// ExecuteResult represents the result of executing a commerce action.
type ExecuteResult struct {
	ActionID   string
	Status     domain.ActionResultStatus
	MessageKey string
	Data       map[string]interface{}
	Errors     []domain.ActionError
}

// Execute executes a commerce action with authorization and AI identity protection.
// The authenticatedActor is the real actor from JWT/session. The aiActor is the
// actor suggested by the AI (which MUST NOT override the authenticated actor).
func (e *ActionExecutor) Execute(
	ctx context.Context,
	action *domain.CommerceAction,
	authenticatedActor domain.CommerceActor,
) (*ExecuteResult, error) {
	if action == nil {
		return nil, fmt.Errorf("action is required")
	}

	// CRITICAL: AI identity protection.
	// Always use the authenticated actor, never the AI-supplied actor.
	action.Actor = authenticatedActor

	// Validate action.
	if err := action.Validate(); err != nil {
		return &ExecuteResult{
			ActionID:   action.ActionID,
			Status:     domain.ActionResultValidationFailed,
			MessageKey: "ACTION_VALIDATION_FAILED",
			Errors:     []domain.ActionError{domain.NewActionError("INVALID_ACTION_PARAMETER", err.Error(), "")},
		}, nil
	}

	// Check if the action type is supported.
	if !isSupportedAction(action.ActionType) {
		return &ExecuteResult{
			ActionID:   action.ActionID,
			Status:     domain.ActionResultNotImplemented,
			MessageKey: "ACTION_NOT_IMPLEMENTED",
			Errors:     []domain.ActionError{domain.NewActionError("ACTION_NOT_IMPLEMENTED", fmt.Sprintf("Action %s is not yet implemented", action.ActionType), "")},
		}, nil
	}

	// Resolve authorization.
	if err := e.authResolver.AuthorizeAction(ctx, authenticatedActor, action); err != nil {
		if err == domain.ErrInsufficientPermissions {
			return &ExecuteResult{
				ActionID:   action.ActionID,
				Status:     domain.ActionResultUnauthorized,
				MessageKey: "ACTION_UNAUTHORIZED",
				Errors:     []domain.ActionError{domain.NewActionError("ACTION_UNAUTHORIZED", "Insufficient permissions", "")},
			}, nil
		}
		return nil, err
	}

	// Check confirmation if required.
	if action.ConfirmationState == domain.ConfirmationPending {
		return &ExecuteResult{
			ActionID:   action.ActionID,
			Status:     domain.ActionResultRequiresConfirmation,
			MessageKey: "ACTION_REQUIRES_CONFIRMATION",
			Data: map[string]interface{}{
				"action_type": action.ActionType,
			},
		}, nil
	}

	// At this point the action is validated, authorized, and confirmed.
	// In a future sprint, the actual application service would be invoked here.
	return &ExecuteResult{
		ActionID:   action.ActionID,
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{
			"action_type": action.ActionType,
		},
	}, nil
}

// isSupportedAction returns true if the action type has a known implementation path.
func isSupportedAction(t domain.CommerceActionType) bool {
	switch t {
	case domain.ActionSearchListings, domain.ActionGetListing, domain.ActionCompareListings,
		domain.ActionCreateListing, domain.ActionUpdateListing, domain.ActionPublishListing, domain.ActionPauseListing, domain.ActionResumeListing,
		domain.ActionCreateOrder, domain.ActionGetOrder, domain.ActionCancelOrder,
		domain.ActionCheckInventory,
		domain.ActionInitiatePayment, domain.ActionGetPaymentStatus, domain.ActionCancelPayment,
		domain.ActionRefundPayment, domain.ActionAuthorizePayment, domain.ActionCapturePayment,
		domain.ActionGetDeliveryStatus,
		domain.ActionCreateCampaign, domain.ActionGetSellerReport:
		return true
	default:
		return false
	}
}
