package domain

// ActionResultStatus represents the status of a commerce action result.
type ActionResultStatus string

const (
	ActionResultSuccess              ActionResultStatus = "SUCCESS"
	ActionResultRequiresConfirmation ActionResultStatus = "REQUIRES_CONFIRMATION"
	ActionResultValidationFailed     ActionResultStatus = "VALIDATION_FAILED"
	ActionResultUnauthorized         ActionResultStatus = "UNAUTHORIZED"
	ActionResultForbidden            ActionResultStatus = "FORBIDDEN"
	ActionResultNotFound             ActionResultStatus = "NOT_FOUND"
	ActionResultConflict             ActionResultStatus = "CONFLICT"
	ActionResultNotImplemented       ActionResultStatus = "NOT_IMPLEMENTED"
	ActionResultTemporaryFailure     ActionResultStatus = "TEMPORARY_FAILURE"
	ActionResultFailed               ActionResultStatus = "FAILED"
)

// ActionError represents a structured error in an action result.
type ActionError struct {
	Code    string
	Message string
	Field   string
}

// NewActionError creates a new action error.
func NewActionError(code, message, field string) ActionError {
	return ActionError{
		Code:    code,
		Message: message,
		Field:   field,
	}
}

// CommerceActionResult represents the result of a commerce action.
type CommerceActionResult struct {
	ActionID   string
	Status     ActionResultStatus
	MessageKey string
	Data       map[string]interface{}
	Errors     []ActionError
	NextAction *NextAction
}

// NextAction represents a suggested next step.
type NextAction struct {
	ActionType  CommerceActionType
	Description string
	Parameters  map[string]interface{}
}

// NewCommerceActionResult creates a new CommerceActionResult.
func NewCommerceActionResult(actionID string, status ActionResultStatus) *CommerceActionResult {
	return &CommerceActionResult{
		ActionID: actionID,
		Status:   status,
		Data:     make(map[string]interface{}),
		Errors:   []ActionError{},
	}
}

// AddError adds an error to the result.
func (r *CommerceActionResult) AddError(err ActionError) {
	r.Errors = append(r.Errors, err)
}

// AddData adds a key-value pair to the result data.
func (r *CommerceActionResult) AddData(key string, value interface{}) {
	r.Data[key] = value
}

// SetNextAction sets the next action suggestion.
func (r *CommerceActionResult) SetNextAction(action CommerceActionType, description string) {
	r.NextAction = &NextAction{
		ActionType:  action,
		Description: description,
	}
}

// HasErrors returns true if the result has errors.
func (r CommerceActionResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// IsSuccess returns true if the result is a success.
func (r CommerceActionResult) IsSuccess() bool {
	return r.Status == ActionResultSuccess
}

// IsFailure returns true if the result is any failure type.
func (r CommerceActionResult) IsFailure() bool {
	switch r.Status {
	case ActionResultUnauthorized, ActionResultForbidden, ActionResultNotFound,
		ActionResultConflict, ActionResultNotImplemented, ActionResultTemporaryFailure,
		ActionResultFailed, ActionResultValidationFailed:
		return true
	default:
		return false
	}
}

// RequiresUserAction returns true if the user needs to take action (e.g., confirm).
func (r CommerceActionResult) RequiresUserAction() bool {
	return r.Status == ActionResultRequiresConfirmation || r.Status == ActionResultValidationFailed
}
