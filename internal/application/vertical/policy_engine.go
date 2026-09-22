package vertical

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type PolicyEngineConfig struct {
	Logger zerolog.Logger
}

type PolicyEngine struct {
	logger zerolog.Logger
}

type PolicyEvaluationResult struct {
	Allowed  bool
	Reason   string
	PackCode domain.PackCode
}

func NewPolicyEngine(config PolicyEngineConfig) *PolicyEngine {
	if config.Logger.GetLevel() == 0 {
		config.Logger = zerolog.Nop()
	}
	return &PolicyEngine{
		logger: config.Logger,
	}
}

func (e *PolicyEngine) Evaluate(
	pack *domain.VerticalPack,
	action domain.CommerceActionType,
	ctx map[string]interface{},
) *PolicyEvaluationResult {
	if pack == nil {
		return &PolicyEvaluationResult{
			Allowed: false,
			Reason:  "pack is nil",
		}
	}

	if !pack.IsActive() {
		return &PolicyEvaluationResult{
			Allowed:  false,
			Reason:   fmt.Sprintf("pack %s is not active (status: %s)", pack.Code, pack.Status),
			PackCode: pack.Code,
		}
	}

	if !pack.SupportsAction(action) {
		return &PolicyEvaluationResult{
			Allowed:  false,
			Reason:   fmt.Sprintf("action %s is not supported by pack %s", action, pack.Code),
			PackCode: pack.Code,
		}
	}

	for _, policy := range pack.Policies {
		if !e.evaluatePolicy(policy, action, ctx) {
			return &PolicyEvaluationResult{
				Allowed:  false,
				Reason:   fmt.Sprintf("policy %s denied action %s", policy.Key, action),
				PackCode: pack.Code,
			}
		}
	}

	return &PolicyEvaluationResult{
		Allowed:  true,
		Reason:   "all policies satisfied",
		PackCode: pack.Code,
	}
}

func (e *PolicyEngine) evaluatePolicy(policy domain.PackPolicy, action domain.CommerceActionType, ctx map[string]interface{}) bool {
	key := strings.ToLower(strings.ReplaceAll(policy.Key, " ", "_"))
	value := policy.Value

	switch key {
	case "require_payment_before_fulfillment":
		if value == "true" {
			paymentStatus, ok := ctx["payment_status"]
			if !ok {
				return true
			}
			return isPaymentComplete(paymentStatus)
		}
	case "require_payment_before_reservation":
		if value == "true" {
			paymentStatus, ok := ctx["payment_status"]
			if !ok {
				return true
			}
			return isPaymentComplete(paymentStatus)
		}
	case "inventory_decrement_on":
		return true
	case "allow_overbooking":
		if value == "false" {
			return true
		}
		return true
	case "min_advance_booking_hours":
		hours, err := strconv.Atoi(value)
		if err != nil {
			return true
		}
		bookingTime, ok := ctx["booking_time"]
		if !ok {
			return true
		}
		if bt, ok := bookingTime.(int64); ok {
			return isInFuture(bt, hours)
		}
		return true
	case "allow_scheduling_into_future_days":
		return true
	case "require_provider_assignment":
		if value == "true" {
			providerID, ok := ctx["provider_id"]
			if !ok {
				return true
			}
			return providerID != nil && providerID != ""
		}
	case "allow_partial_reservations":
		return true
	default:
		e.logger.Warn().
			Str("policy_key", policy.Key).
			Str("policy_value", policy.Value).
			Msg("Unknown policy, allowing by default")
		return true
	}
	return true
}

func isPaymentComplete(status interface{}) bool {
	s, ok := status.(string)
	if !ok {
		return true
	}
	switch strings.ToLower(s) {
	case "paid", "completed", "settled", "captured":
		return true
	default:
		return false
	}
}

func isInFuture(t int64, minHours int) bool {
	return t > 0 && minHours >= 0
}

func (e *PolicyEngine) EvaluateAll(
	registry *PackRegistry,
	action domain.CommerceActionType,
	ctx map[string]interface{},
) []*PolicyEvaluationResult {
	var results []*PolicyEvaluationResult
	for _, pack := range registry.GetAll() {
		result := e.Evaluate(pack, action, ctx)
		results = append(results, result)
	}
	return results
}

func (e *PolicyEngine) FindAllowedPacks(
	registry *PackRegistry,
	action domain.CommerceActionType,
	ctx map[string]interface{},
) []domain.PackCode {
	var allowed []domain.PackCode
	for _, pack := range registry.GetAll() {
		result := e.Evaluate(pack, action, ctx)
		if result.Allowed {
			allowed = append(allowed, pack.Code)
		}
	}
	return allowed
}
