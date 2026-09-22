package domain

import "fmt"

type AIIntent struct {
	Intent      CommerceActionType
	Parameters  map[string]interface{}
	Confidence  float64
	Explanation string
}

func NewAIIntent() *AIIntent {
	return &AIIntent{
		Parameters: make(map[string]interface{}),
		Confidence: 0.0,
	}
}

func (i *AIIntent) Validate() error {
	if i.Intent == "" {
		return fmt.Errorf("intent is required")
	}
	if !i.IsAllowlisted() {
		return fmt.Errorf("intent %s is not in the AI action allowlist", i.Intent)
	}
	if i.Confidence < 0 || i.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	if i.HasUntrustedFields() {
		return fmt.Errorf("AI intent contains untrusted sensitive fields")
	}
	return nil
}

func (i *AIIntent) IsAllowlisted() bool {
	for _, a := range AIAcceptedActions {
		if a == i.Intent {
			return true
		}
	}
	return false
}

func (i *AIIntent) ConfidenceOK(threshold float64) bool {
	return i.Confidence >= threshold
}

func (i *AIIntent) HasUntrustedFields() bool {
	for k := range i.Parameters {
		if isSensitiveField(k) {
			return true
		}
	}
	return false
}

func isSensitiveField(field string) bool {
	sensitiveFields := map[string]bool{
		"actor_id":           true,
		"tenant_id":          true,
		"user_id":            true,
		"organization_id":    true,
		"role":               true,
		"permissions":        true,
		"confirmation_state": true,
		"confirmed":          true,
		"secret_ref":         true,
	}
	return sensitiveFields[field]
}

var AIAcceptedActions = []CommerceActionType{
	ActionSearchListings,
	ActionGetListing,
	ActionCompareListings,
	ActionSuggest,
	ActionCheckAvailability,
	ActionGetOrder,
	ActionGetReservation,
	ActionGetFulfillmentStatus,
	ActionGetDeliveryStatus,
	ActionCheckInventory,
	ActionGetPaymentStatus,
}

func AIActionRequiresConfirmation(actionType CommerceActionType) bool {
	switch actionType {
	case ActionCreateOrder, ActionCreateReservation, ActionCancelOrder,
		ActionCancelReservation, ActionModifyReservation,
		ActionInitiatePayment:
		return true
	default:
		return false
	}
}

func AIActionCategory(actionType CommerceActionType) ActionCategory {
	return actionType.GetCategory()
}

type AIOrchestratedResponse struct {
	Intent            *AIIntent
	AssistantMessage  *AssistantMessage
	ActionResult      *CommerceActionResult
	Action            *CommerceAction
	Confirmation      *ConfirmationRequest
	NeedsConfirmation bool
	Error             *AIOrchestratorError
}

type AssistantMessage struct {
	Text   string
	Locale string
}

type ConfirmationRequest struct {
	ActionID  string
	Summary   string
	Details   map[string]interface{}
	ExpiresAt int64
}

type AIOrchestratorError struct {
	Code    AIErrorCode
	Message string
}

func NewAIOrchestratedResponse() *AIOrchestratedResponse {
	return &AIOrchestratedResponse{
		AssistantMessage: &AssistantMessage{},
	}
}
