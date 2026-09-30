package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

type ProviderOrchestrator struct {
	registry    *ProviderRegistry
	repo        domain.ProviderExecutionRepository
	logger      zerolog.Logger
	retryPolicy domain.RetryPolicy
}

type OrchestratorConfig struct {
	Logger      zerolog.Logger
	RetryPolicy domain.RetryPolicy
}

func NewProviderOrchestrator(
	registry *ProviderRegistry,
	repo domain.ProviderExecutionRepository,
	config OrchestratorConfig,
) *ProviderOrchestrator {
	if config.RetryPolicy.MaxAttempts == 0 {
		config.RetryPolicy = domain.DefaultRetryPolicy()
	}
	return &ProviderOrchestrator{
		registry:    registry,
		repo:        repo,
		logger:      config.Logger,
		retryPolicy: config.RetryPolicy,
	}
}

type ExecuteOptions struct {
	Capability     domain.Capability
	Operation      string
	IdempotencyKey string
	// RequestID correlates the attempts of this call in provider_executions
	// (generated when empty).
	RequestID string
	// Timeout bounds each attempt (default 30s). A timed-out attempt counts as
	// retryable, so it can fall back to the next provider.
	Timeout     time.Duration
	MaxAttempts int
}

type ExecuteResult struct {
	ProviderID        string
	Success           bool
	Data              map[string]interface{}
	ExternalReference string
	ErrorCode         domain.ProviderErrorCode
	ErrorMessage      string
	RetryCount        int
	DurationMs        int64
	RequestID         string
	Attempts          int
}

// ExecuteCapability runs perform against the organization's usable providers
// for the capability, highest priority first, and records every attempt.
//
//   - (Success true, nil): a provider performed the operation.
//   - (Success false, nil): a provider answered and refused (a business
//     outcome reported in its result, e.g. a decline); not retried.
//   - (Success false, err): no provider performed it; ErrorCode says why.
//
// Success comes from the provider result, never from the call returning
// without error. A retryable failure (timeout, connection error) is retried
// with backoff on the next provider in priority order (fallback); any other
// failure stops. The idempotency key reaches the adapter on every attempt, so
// a retry never repeats a side effect.
func (o *ProviderOrchestrator) ExecuteCapability(
	ctx context.Context,
	orgID, actorID string,
	opts ExecuteOptions,
	perform func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error),
) (*ExecuteResult, error) {
	if opts.Capability == "" {
		return nil, fmt.Errorf("capability is required")
	}
	if opts.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization ID is required")
	}

	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = o.retryPolicy.MaxAttempts
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	requestID := opts.RequestID
	if requestID == "" {
		requestID = generateRequestID()
	}
	if opts.IdempotencyKey != "" && domain.ProviderIdempotencyKey(ctx) == "" {
		ctx = domain.WithProviderIdempotencyKey(ctx, opts.IdempotencyKey)
	}

	entries, err := o.registry.FindByCapability(ctx, orgID, opts.Capability)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}
	if len(entries) == 0 {
		return nil, domain.ErrNoProvidersAvailable
	}

	var last *ExecuteResult
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		entry := entries[attempt%len(entries)]
		providerID := entry.GetProvider().ID
		started := time.Now()

		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		execCtx := &domain.ProviderExecutionContext{
			Context:        attemptCtx,
			Capability:     opts.Capability,
			Operation:      opts.Operation,
			TenantID:       orgID,
			ActorID:        actorID,
			IdempotencyKey: opts.IdempotencyKey,
			Deadline:       started.Add(timeout),
			RequestID:      requestID,
		}
		result, execErr := perform(attemptCtx, execCtx, entry.GetAdapter())
		timedOut := execErr != nil && (errors.Is(execErr, context.DeadlineExceeded) || errors.Is(attemptCtx.Err(), context.DeadlineExceeded))
		cancel()
		duration := time.Since(started)

		last = &ExecuteResult{
			ProviderID:        providerID,
			Data:              result.Data,
			ExternalReference: result.ExternalReference,
			RetryCount:        attempt,
			DurationMs:        duration.Milliseconds(),
			RequestID:         requestID,
			Attempts:          attempt + 1,
		}

		if execErr == nil {
			if result.Success {
				last.Success = true
				o.record(orgID, actorID, opts, requestID, attempt, providerID, domain.ExecutionStatusSuccess, "", "", started, duration, result.ExternalReference, nil)
				return last, nil
			}
			// The provider answered and refused: a business outcome, not a
			// failure to retry elsewhere.
			last.ErrorCode = result.ErrorCode
			if last.ErrorCode == "" {
				last.ErrorCode = domain.ErrCodeProviderRejected
			}
			last.ErrorMessage = result.ErrorMessage
			o.record(orgID, actorID, opts, requestID, attempt, providerID, domain.ExecutionStatusFailed, last.ErrorCode, domain.NonRetryable, started, duration, result.ExternalReference, nil)
			return last, nil
		}

		class := o.retryPolicy.ClassifyError(execErr)
		if timedOut {
			class = domain.Retryable
		}
		last.ErrorCode = normalizeErrorCode(execErr, result.ErrorCode, timedOut)
		last.ErrorMessage = execErr.Error()
		lastErr = execErr
		o.record(orgID, actorID, opts, requestID, attempt, providerID, domain.ExecutionStatusFailed, last.ErrorCode, class, started, duration, result.ExternalReference, execErr)

		if class != domain.Retryable || attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return last, lastErr
		case <-time.After(o.calculateBackoff(attempt)):
		}
	}
	return last, lastErr
}

// normalizeErrorCode maps a failed attempt to the normalized provider error
// codes: timeouts and unsupported capabilities are recognized, otherwise the
// adapter's code is kept, or UNKNOWN_PROVIDER_ERROR.
func normalizeErrorCode(err error, adapterCode domain.ProviderErrorCode, timedOut bool) domain.ProviderErrorCode {
	switch {
	case timedOut:
		return domain.ErrCodeProviderTimeout
	case errors.Is(err, domain.ErrUnsupportedCapability):
		return domain.ErrCodeUnsupportedCapability
	case adapterCode != "":
		return adapterCode
	default:
		return domain.ErrCodeUnknown
	}
}

func (o *ProviderOrchestrator) SelectProvider(
	ctx context.Context,
	orgID string,
	capability domain.Capability,
	eligibleChecker func(*domain.IntegrationProvider) bool,
) (*RegistryEntry, error) {
	entries, err := o.registry.FindByCapability(ctx, orgID, capability)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, domain.ErrNoProvidersAvailable
	}

	for _, entry := range entries {
		if !entry.GetProvider().IsUsable() {
			continue
		}
		if eligibleChecker != nil && !eligibleChecker(entry.GetProvider()) {
			continue
		}
		return entry, nil
	}

	return nil, domain.ErrProviderNotUsable
}

func (o *ProviderOrchestrator) calculateBackoff(attempt int) time.Duration {
	delay := o.retryPolicy.BaseDelayMs * (1 << uint(attempt))
	if delay > o.retryPolicy.MaxDelayMs {
		delay = o.retryPolicy.MaxDelayMs
	}
	return time.Duration(delay) * time.Millisecond
}

// record logs an attempt and persists it in provider_executions. Recording
// never changes the outcome of the call: it runs on its own short context
// (the call's may already be cancelled) and failures are only logged.
func (o *ProviderOrchestrator) record(
	orgID, actorID string,
	opts ExecuteOptions,
	requestID string,
	attempt int,
	providerID string,
	status domain.ProviderExecutionStatus,
	code domain.ProviderErrorCode,
	class domain.RetryClassification,
	started time.Time,
	duration time.Duration,
	externalRef string,
	err error,
) {
	event := o.logger.Info()
	if err != nil || status != domain.ExecutionStatusSuccess {
		observability.ProviderFailed(providerID, string(opts.Capability), string(code))
		event = o.logger.Warn().Err(err).Str("error_code", string(code)).Str("retry_class", string(class))
	}
	event.Str("provider", providerID).
		Str("capability", string(opts.Capability)).
		Str("operation", opts.Operation).
		Str("tenant", orgID).
		Str("actor", actorID).
		Str("request_id", requestID).
		Int("attempt", attempt+1).
		Dur("duration", duration).
		Str("status", string(status)).
		Msg("provider execution")

	if o.repo == nil {
		return
	}
	exec := &domain.ProviderExecution{
		ID:                domain.NewID(),
		ProviderID:        providerID,
		Capability:        opts.Capability,
		Operation:         opts.Operation,
		TenantID:          orgID,
		ActorID:           actorID,
		IdempotencyKey:    opts.IdempotencyKey,
		RequestID:         requestID,
		Attempt:           attempt + 1,
		RetryClass:        class,
		Status:            status,
		ErrorCode:         code,
		StartedAt:         started.Unix(),
		CompletedAt:       started.Add(duration).Unix(),
		DurationMs:        duration.Milliseconds(),
		ExternalReference: externalRef,
	}
	rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if rerr := o.repo.Create(rctx, exec); rerr != nil {
		o.logger.Warn().Err(rerr).Str("request_id", requestID).Msg("provider execution not recorded")
	}
}

func generateRequestID() string {
	return "req_" + domain.NewID()
}
