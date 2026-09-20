package domain

import (
	"fmt"
	"time"
)

// ConfirmationState represents the confirmation state of a commerce action.
type ConfirmationState string

const (
	ConfirmationPending     ConfirmationState = "pending"
	ConfirmationRequested   ConfirmationState = "requested"
	ConfirmationApproved    ConfirmationState = "approved"
	ConfirmationRejected    ConfirmationState = "rejected"
	ConfirmationNotRequired ConfirmationState = "not_required"
)

// CommerceAction represents a canonical commerce action.
type CommerceAction struct {
	ActionID          string
	ActionType        CommerceActionType
	Actor             CommerceActor
	TenantID          string
	ConversationID    string
	Parameters        map[string]interface{}
	RequestedAt       int64
	IdempotencyKey    string
	Source            string // USER, AI, SYSTEM
	ConfirmationState ConfirmationState
	ConfirmationData  *ConfirmationData
}

// ConfirmationData represents confirmation details for actions requiring confirmation.
type ConfirmationData struct {
	Summary     string
	Details     map[string]interface{}
	RequestedAt *int64
}

// NewCommerceAction creates a new CommerceAction.
func NewCommerceAction(actionType CommerceActionType, actor CommerceActor, conversationID string) *CommerceAction {
	now := time.Now().Unix()
	return &CommerceAction{
		ActionID:          "",
		ActionType:        actionType,
		Actor:             actor,
		TenantID:          "",
		ConversationID:    conversationID,
		Parameters:        make(map[string]interface{}),
		RequestedAt:       now,
		Source:            "USER",
		ConfirmationState: ConfirmationNotRequired,
	}
}

// WithParameters sets parameters on the action.
func (a *CommerceAction) WithParameters(params map[string]interface{}) *CommerceAction {
	a.Parameters = params
	return a
}

// WithIdempotencyKey sets the idempotency key.
func (a *CommerceAction) WithIdempotencyKey(key string) *CommerceAction {
	a.IdempotencyKey = key
	return a
}

// WithTenant sets the tenant ID (resolved server-side, NOT from AI/client).
func (a *CommerceAction) WithTenant(tenantID string) *CommerceAction {
	a.TenantID = tenantID
	return a
}

// WithSource sets the action source.
func (a *CommerceAction) WithSource(source string) *CommerceAction {
	a.Source = source
	return a
}

// RequiresConfirmation returns true if the action requires confirmation.
func (a CommerceAction) RequiresConfirmation() bool {
	return a.ConfirmationState == ConfirmationRequested || a.ConfirmationState == ConfirmationPending
}

// IsConfirmed returns true if the action has been confirmed.
func (a CommerceAction) IsConfirmed() bool {
	return a.ConfirmationState == ConfirmationApproved
}

// IsIdempotent returns true if the action has an idempotency key set.
func (a CommerceAction) IsIdempotent() bool {
	return a.IdempotencyKey != ""
}

// Validate returns an error if the action is invalid.
func (a CommerceAction) Validate() error {
	if a.ActionType == "" {
		return fmt.Errorf("action type is required")
	}
	if a.Actor.ID == "" {
		return fmt.Errorf("actor ID is required")
	}
	if a.Actor.Type == "" {
		return fmt.Errorf("actor type is required")
	}
	if a.ConversationID == "" {
		return fmt.Errorf("conversation ID is required")
	}
	if a.ConfirmationState == "" {
		return fmt.Errorf("confirmation state is required")
	}
	return nil
}
