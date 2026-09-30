package commerce

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// CommerceActionRuntime orchestrates commerce action execution.
// It handles validation, authorization, confirmation, idempotency,
// identity protection, tenant resolution, usage checking, and handler dispatch.
type CommerceActionRuntime struct {
	logger             zerolog.Logger
	registry           *HandlerRegistry
	idempotencyStore   IdempotencyStore
	confirmationPolicy ConfirmationPolicy
	parameterValidator ParameterValidator
	usageService       domain.UsageService
	reservations       map[string]*domain.UsageReservation
	mu                 sync.RWMutex
}

// RuntimeConfig configures the CommerceActionRuntime.
type RuntimeConfig struct {
	Logger             zerolog.Logger
	HandlerRegistry    *HandlerRegistry
	IdempotencyStore   IdempotencyStore
	ConfirmationPolicy ConfirmationPolicy
	ParameterValidator ParameterValidator
	UsageService       domain.UsageService
}

// NewCommerceActionRuntime creates a new CommerceActionRuntime.
func NewCommerceActionRuntime(config RuntimeConfig) *CommerceActionRuntime {
	return &CommerceActionRuntime{
		logger:             config.Logger,
		registry:           config.HandlerRegistry,
		idempotencyStore:   config.IdempotencyStore,
		confirmationPolicy: config.ConfirmationPolicy,
		parameterValidator: config.ParameterValidator,
		usageService:       config.UsageService,
		reservations:       make(map[string]*domain.UsageReservation),
	}
}

// ExecuteResult represents the result of executing a commerce action.
type ExecuteResult struct {
	ActionID             string
	Status               domain.ActionResultStatus
	MessageKey           string
	Data                 map[string]interface{}
	Errors               []domain.ActionError
	NextAction           *domain.NextAction
	RequiresConfirmation bool
}

// Execute orchestrates commerce action execution with full lifecycle management.
// The authenticatedActor is the real actor from JWT/session. The action's Actor
// field is always overridden with authenticatedActor for identity protection.
func (r *CommerceActionRuntime) Execute(
	ctx context.Context,
	action *domain.CommerceAction,
	authenticatedActor domain.CommerceActor,
) (*ExecuteResult, error) {
	if action == nil {
		return nil, fmt.Errorf("action is required")
	}

	result := &ExecuteResult{
		ActionID: action.ActionID,
		Data:     make(map[string]interface{}),
		Errors:   []domain.ActionError{},
	}

	// 1. Identity protection: always use authenticated actor, never AI-supplied actor.
	action.Actor = authenticatedActor

	// 2. Resolve tenant server-side (not from AI/client).
	action.TenantID = authenticatedActor.OrgID
	if action.TenantID == "" {
		action.TenantID = authenticatedActor.ID
	}

	// 3. Validate action structure.
	if err := action.Validate(); err != nil {
		result.Status = domain.ActionResultValidationFailed
		result.MessageKey = "ACTION_VALIDATION_FAILED"
		result.Errors = append(result.Errors, domain.NewActionError("INVALID_ACTION", err.Error(), ""))
		return result, nil
	}

	// 4. Validate parameters.
	if r.parameterValidator != nil {
		if err := r.parameterValidator.Validate(ctx, action.ActionType, action.Parameters); err != nil {
			field := ""
			var missing *MissingParameterError
			var invalid *InvalidParameterError
			if errors.As(err, &missing) {
				field = missing.Param
			} else if errors.As(err, &invalid) {
				field = invalid.Param
			}
			result.Status = domain.ActionResultValidationFailed
			result.MessageKey = "ACTION_VALIDATION_FAILED"
			result.Errors = append(result.Errors, domain.NewActionError("INVALID_PARAM", err.Error(), field))
			return result, nil
		}
	}

	// 5. Check idempotency.
	if action.IsIdempotent() && r.idempotencyStore != nil {
		if status, data, found := r.idempotencyStore.Check(ctx, scopedIdempotencyKey(action)); found {
			result.Status = status
			result.MessageKey = "ACTION_IDEMPOTENT_CACHE_HIT"
			if data != nil {
				result.Data = data
			}
			return result, nil
		}
	}

	// 6. Check confirmation policy.
	if r.confirmationPolicy != nil {
		needsConfirmation, reason := r.confirmationPolicy.RequiresConfirmation(ctx, action)
		if needsConfirmation && !action.IsConfirmed() {
			result.Status = domain.ActionResultRequiresConfirmation
			result.MessageKey = "ACTION_REQUIRES_CONFIRMATION"
			result.RequiresConfirmation = true
			result.Data["action_type"] = action.ActionType
			result.Data["reason"] = reason
			return result, nil
		}
	}

	// 7. Check usage policy and reserve credits.
	var reservationID string
	if r.usageService != nil && authenticatedActor.IsAuthenticated() {
		policy, err := r.usageService.ResolvePolicy(ctx, action.ActionType, domain.ChannelOther, authenticatedActor.Type)
		if err == nil && policy.IsMetered() {
			// Find or create credit account for the authenticated actor
			account, err := r.usageService.FindOrCreateCreditAccountByOwner(ctx, authenticatedActor.Type, authenticatedActor.ID, action.TenantID, "", "")
			if err != nil {
				result.Status = domain.ActionResultFailed
				result.MessageKey = "USAGE_ACCOUNT_FAILED"
				result.Errors = append(result.Errors, domain.NewActionError("USAGE_ACCOUNT_FAILED", err.Error(), ""))
				return result, nil
			}

			ok, err := r.usageService.Check(ctx, account.AccountID, policy.Cost)
			if err != nil {
				result.Status = domain.ActionResultFailed
				result.MessageKey = "USAGE_CHECK_FAILED"
				result.Errors = append(result.Errors, domain.NewActionError("USAGE_CHECK_FAILED", err.Error(), ""))
				return result, nil
			}
			if !ok {
				result.Status = domain.ActionResultFailed
				result.MessageKey = "INSUFFICIENT_CREDITS"
				result.Errors = append(result.Errors, domain.NewActionError("INSUFFICIENT_CREDITS", "Insufficient credits for this action", ""))
				return result, nil
			}
			res, err := r.usageService.Reserve(ctx, account.AccountID, action.ActionType, policy.Cost, action.ActionID, action.TenantID)
			if err != nil {
				result.Status = domain.ActionResultFailed
				result.MessageKey = "USAGE_RESERVE_FAILED"
				result.Errors = append(result.Errors, domain.NewActionError("USAGE_RESERVE_FAILED", err.Error(), ""))
				return result, nil
			}
			reservationID = res.ReservationID
			r.reserveReservation(ctx, res)
		}
	}

	// 8. Route to handler.
	handler, registered := r.registry.Get(action.ActionType)
	if !registered {
		if reservationID != "" {
			_ = r.releaseReservation(ctx, reservationID)
		}
		result.Status = domain.ActionResultNotImplemented
		result.MessageKey = "ACTION_NOT_IMPLEMENTED"
		result.Errors = append(result.Errors, domain.NewActionError("ACTION_NOT_IMPLEMENTED", fmt.Sprintf("Action %s is not yet implemented", action.ActionType), ""))
		return result, nil
	}

	// 9. Execute handler.
	r.logger.Info().Str("action_id", action.ActionID).Str("action_type", string(action.ActionType)).Str("actor", authenticatedActor.ID).Msg("Executing commerce action")

	data, err := handler.Handle(ctx, authenticatedActor, action.Parameters)
	if err != nil {
		if reservationID != "" {
			_ = r.releaseReservation(ctx, reservationID)
		}
		result.Status = domain.ActionResultFailed
		result.MessageKey = "ACTION_FAILED"
		result.Errors = append(result.Errors, domain.NewActionError("ACTION_FAILED", err.Error(), ""))
		r.logger.Error().Err(err).Str("action_id", action.ActionID).Str("action_type", string(action.ActionType)).Msg("Commerce action failed")
		return result, nil
	}

	result.Status = domain.ActionResultSuccess
	result.MessageKey = "ACTION_SUCCESS"
	if data != nil {
		result.Data["data"] = data
	}

	// 10. Consume usage reservation on success.
	if reservationID != "" {
		if err := r.consumeReservation(ctx, reservationID); err != nil {
			r.logger.Warn().Err(err).Str("action_id", action.ActionID).Str("reservation_id", reservationID).Msg("Failed to consume usage reservation")
		}
	}

	// 11. Store result for idempotency.
	if action.IsIdempotent() && r.idempotencyStore != nil {
		actionResult := domain.NewCommerceActionResult(action.ActionID, result.Status)
		actionResult.MessageKey = result.MessageKey
		actionResult.Data = result.Data
		actionResult.Errors = result.Errors
		if result.NextAction != nil {
			actionResult.SetNextAction(result.NextAction.ActionType, result.NextAction.Description)
		}
		if err := r.idempotencyStore.Store(ctx, scopedIdempotencyKey(action), *actionResult); err != nil {
			r.logger.Warn().Err(err).Str("action_id", action.ActionID).Msg("Failed to store idempotency result")
		}
	}

	r.logger.Info().Str("action_id", action.ActionID).Str("action_type", string(action.ActionType)).Msg("Commerce action completed")
	return result, nil
}

// IsSupported returns true if the action type has a registered handler.
func (r *CommerceActionRuntime) IsSupported(actionType domain.CommerceActionType) bool {
	return r.registry.Has(actionType)
}

// RegisterHandler registers a handler for an action type.
func (r *CommerceActionRuntime) RegisterHandler(handler Handler) {
	r.registry.Register(handler)
}

func (r *CommerceActionRuntime) reserveReservation(ctx context.Context, reservation *domain.UsageReservation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reservations[reservation.ReservationID] = reservation
}

func (r *CommerceActionRuntime) getReservation(reservationID string) (*domain.UsageReservation, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.reservations[reservationID]
	return res, ok
}

func (r *CommerceActionRuntime) consumeReservation(ctx context.Context, reservationID string) error {
	_, ok := r.getReservation(reservationID)
	if !ok {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}
	if err := r.usageService.Consume(ctx, reservationID); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.reservations, reservationID)
	r.mu.Unlock()
	return nil
}

func (r *CommerceActionRuntime) releaseReservation(ctx context.Context, reservationID string) error {
	_, ok := r.getReservation(reservationID)
	if !ok {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}
	if err := r.usageService.Release(ctx, reservationID); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.reservations, reservationID)
	r.mu.Unlock()
	return nil
}

// scopedIdempotencyKey namespaces a client idempotency key by tenant, actor
// and action type, so one caller replaying another caller's key can never
// receive that caller's cached result.
func scopedIdempotencyKey(action *domain.CommerceAction) string {
	return action.TenantID + "|" + action.Actor.ID + "|" + string(action.ActionType) + "|" + action.IdempotencyKey
}
