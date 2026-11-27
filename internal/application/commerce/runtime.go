package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

	// 5. A completed action answers with its result (the key and the request must both match).
	idempotent := action.IsIdempotent() && r.idempotencyStore != nil
	key, print := "", ""
	if idempotent {
		key, print = scopedIdempotencyKey(action), fingerprint(action)
		if _, _, found := r.idempotencyStore.Check(ctx, key); found {
			claim, err := r.idempotencyStore.Claim(ctx, key, print)
			if err != nil {
				return failed(result, "IDEMPOTENCY_UNAVAILABLE", "The action cannot be recorded; nothing was done"), nil
			}
			if claim.State != ClaimAcquired {
				return answerClaim(result, claim), nil
			}
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

	// 7. Claim the key: from here on, the same request cannot run twice at the same time. Nothing runs unrecorded.
	completed := false
	if idempotent {
		claim, err := r.idempotencyStore.Claim(ctx, key, print)
		if err != nil {
			r.logger.Error().Err(err).Str("action_id", action.ActionID).Msg("Action journal unavailable")
			return failed(result, "IDEMPOTENCY_UNAVAILABLE", "The action cannot be recorded; nothing was done"), nil
		}
		if claim.State != ClaimAcquired {
			return answerClaim(result, claim), nil
		}
		defer func() {
			if !completed {
				if err := r.idempotencyStore.Release(context.WithoutCancel(ctx), key); err != nil {
					r.logger.Error().Err(err).Str("action_id", action.ActionID).Msg("Action claim not released; it expires")
				}
			}
		}()
	}

	// 8. Check usage policy and reserve credits. An action is free only when no policy prices it: if the policy
	// cannot be read, nothing runs.
	var reservationID string
	if r.usageService != nil && authenticatedActor.IsAuthenticated() {
		policy, err := r.usageService.ResolvePolicy(ctx, action.ActionType, domain.ChannelOther, authenticatedActor.Type)
		if err != nil && !errors.Is(err, domain.ErrUsagePolicyNotFound) {
			return failed(result, "USAGE_POLICY_FAILED", err.Error()), nil
		}
		if err == nil && policy.IsMetered() {
			// Find or create credit account for the authenticated actor
			account, err := r.usageService.FindOrCreateCreditAccountByOwner(ctx, authenticatedActor.Type, authenticatedActor.ID, action.TenantID, "", "")
			if err != nil {
				return failed(result, "USAGE_ACCOUNT_FAILED", err.Error()), nil
			}

			ok, err := r.usageService.Check(ctx, account.AccountID, policy.Cost)
			if err != nil {
				return failed(result, "USAGE_CHECK_FAILED", err.Error()), nil
			}
			if !ok {
				return failed(result, "INSUFFICIENT_CREDITS", "Insufficient credits for this action"), nil
			}
			res, err := r.usageService.Reserve(ctx, account.AccountID, action.ActionType, policy.Cost, action.ActionID, action.TenantID)
			if err != nil {
				return failed(result, "USAGE_RESERVE_FAILED", err.Error()), nil
			}
			// A reservation already consumed paid for this very action: it is not charged again.
			if res.Status != domain.UsageReservationConsumed {
				reservationID = res.ReservationID
			}
		}
	}

	// 9. Route to handler.
	handler, registered := r.registry.Get(action.ActionType)
	if !registered {
		r.release(ctx, reservationID)
		result.Status = domain.ActionResultNotImplemented
		result.MessageKey = "ACTION_NOT_IMPLEMENTED"
		result.Errors = append(result.Errors, domain.NewActionError("ACTION_NOT_IMPLEMENTED", fmt.Sprintf("Action %s is not yet implemented", action.ActionType), ""))
		return result, nil
	}

	// 10. Execute handler, after recording that it starts.
	if reservationID != "" {
		if err := r.usageService.MarkAction(ctx, reservationID, domain.UsageActionRunning); err != nil {
			r.release(ctx, reservationID)
			return failed(result, "USAGE_RESERVE_FAILED", err.Error()), nil
		}
	}
	r.logger.Info().Str("action_id", action.ActionID).Str("action_type", string(action.ActionType)).Str("actor", authenticatedActor.ID).Msg("Executing commerce action")

	data, err := handler.Handle(domain.WithEffectKey(ctx, effectKey(action)), authenticatedActor, action.Parameters)
	if err != nil {
		if reservationID != "" {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				// The effect may have happened: the credits stay reserved until a retry or a person settles them.
				r.logger.Error().Err(err).Str("action_id", action.ActionID).Str("reservation_id", reservationID).Msg("Action outcome unknown; credits held")
			} else {
				r.release(ctx, reservationID)
			}
		}
		result.Status = domain.ActionResultFailed
		result.MessageKey = "ACTION_FAILED"
		result.Errors = append(result.Errors, domain.NewActionError("ACTION_FAILED", err.Error(), ""))
		r.logger.Error().Err(err).Str("action_id", action.ActionID).Str("action_type", string(action.ActionType)).Msg("Commerce action failed")
		return result, nil
	}

	// The action happened: what follows is recorded even if the caller stopped waiting, and the claim is never given up.
	completed = true
	record := context.WithoutCancel(ctx)

	result.Status = domain.ActionResultSuccess
	result.MessageKey = "ACTION_SUCCESS"
	if data != nil {
		result.Data["data"] = data
	}

	// 11. Consume the reservation. If it cannot be done now, it is recorded as owed and consumed by the sweep.
	if reservationID != "" {
		r.consume(record, action.ActionID, reservationID)
	}

	// 12. Store result for idempotency.
	if idempotent {
		actionResult := domain.NewCommerceActionResult(action.ActionID, result.Status)
		actionResult.MessageKey = result.MessageKey
		actionResult.Data = result.Data
		actionResult.Errors = result.Errors
		if result.NextAction != nil {
			actionResult.SetNextAction(result.NextAction.ActionType, result.NextAction.Description)
		}
		if err := retry(func() error { return r.idempotencyStore.Store(record, key, *actionResult) }); err != nil {
			r.logger.Error().Err(err).Str("action_id", action.ActionID).Msg("Failed to store idempotency result")
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

// release returns reserved credits after an action that certainly did nothing. A release that fails is retried by the
// sweep, which sees the action as failed.
func (r *CommerceActionRuntime) release(ctx context.Context, reservationID string) {
	if reservationID == "" {
		return
	}
	ctx = context.WithoutCancel(ctx)
	if err := r.usageService.MarkAction(ctx, reservationID, domain.UsageActionFailed); err != nil {
		r.logger.Error().Err(err).Str("reservation_id", reservationID).Msg("Failed action not recorded on its reservation")
	}
	if err := retry(func() error { return r.usageService.Release(ctx, reservationID) }); err != nil {
		r.logger.Error().Err(err).Str("reservation_id", reservationID).Msg("Credits not released yet")
	}
}

// consume charges a successful action.
func (r *CommerceActionRuntime) consume(ctx context.Context, actionID, reservationID string) {
	if err := retry(func() error { return r.usageService.MarkAction(ctx, reservationID, domain.UsageActionSucceeded) }); err != nil {
		r.logger.Error().Err(err).Str("action_id", actionID).Str("reservation_id", reservationID).Msg("Successful action not recorded on its reservation")
	}
	err := retry(func() error {
		if err := r.usageService.Consume(ctx, reservationID); err != nil && !errors.Is(err, domain.ErrUsageAlreadyConsumed) {
			return err
		}
		return nil
	})
	if err != nil {
		r.logger.Error().Err(err).Str("action_id", actionID).Str("reservation_id", reservationID).Msg("Credits not consumed yet")
	}
}

// retry runs a database write a few times: the effect it records has already happened.
func retry(write func() error) (err error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err = write(); err == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return err
}

func failed(result *ExecuteResult, code, message string) *ExecuteResult {
	result.Status = domain.ActionResultFailed
	result.MessageKey = code
	result.Errors = append(result.Errors, domain.NewActionError(code, message, ""))
	return result
}

// answerClaim answers a request whose key is already used.
func answerClaim(result *ExecuteResult, claim Claim) *ExecuteResult {
	switch claim.State {
	case ClaimCompleted:
		result.Status = claim.Result.Status
		result.MessageKey = "ACTION_IDEMPOTENT_CACHE_HIT"
		if claim.Result.Data != nil {
			result.Data = claim.Result.Data
		}
		return result
	case ClaimConflict:
		return failed(result, "IDEMPOTENCY_CONFLICT", "This key was used for a different request")
	default:
		return failed(result, "ACTION_IN_PROGRESS", "This action is already being carried out")
	}
}

// scopedIdempotencyKey namespaces a client idempotency key by tenant, actor
// and action type, so one caller replaying another caller's key can never
// receive that caller's cached result.
func scopedIdempotencyKey(action *domain.CommerceAction) string {
	return action.TenantID + "|" + action.Actor.ID + "|" + string(action.ActionType) + "|" + action.IdempotencyKey
}

// readOnlyActions change nothing: a reused key simply answers with the first result.
var readOnlyActions = map[domain.CommerceActionType]bool{
	domain.ActionSearchListings: true, domain.ActionGetListing: true, domain.ActionCompareListings: true,
	domain.ActionSuggest: true, domain.ActionGetOrder: true, domain.ActionCheckAvailability: true,
	domain.ActionGetReservation: true, domain.ActionCheckInventory: true, domain.ActionGetFulfillmentStatus: true,
	domain.ActionGetPaymentStatus: true, domain.ActionGetRefundStatus: true, domain.ActionGetDeliveryStatus: true,
	domain.ActionGetSellerReport: true,
}

// fingerprint identifies what an action with an effect asks, so a reused key cannot stand for another request. It is
// empty for a read.
func fingerprint(action *domain.CommerceAction) string {
	if readOnlyActions[action.ActionType] {
		return ""
	}
	raw, err := json.Marshal(action.Parameters) // map keys are sorted
	if err != nil {
		raw = []byte(fmt.Sprint(action.Parameters))
	}
	sum := sha256.Sum256(append([]byte(string(action.ActionType)+"|"), raw...))
	return hex.EncodeToString(sum[:])
}

// effectKey names the business intention of an action for the service that creates its record (see domain.EffectKey).
func effectKey(action *domain.CommerceAction) string {
	named := action.EffectKey
	if named == "" {
		named = action.IdempotencyKey
	}
	if named == "" {
		named = action.ActionID
	}
	sum := sha256.Sum256([]byte(action.TenantID + "|" + action.Actor.ID + "|" + string(action.ActionType) + "|" + named))
	return "act-" + hex.EncodeToString(sum[:])
}
