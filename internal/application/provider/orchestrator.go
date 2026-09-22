package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
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
	Timeout        time.Duration
	MaxAttempts    int
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
}

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

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entries, err := o.registry.FindByCapability(ctx, orgID, opts.Capability)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}
	if len(entries) == 0 {
		return nil, domain.ErrNoProvidersAvailable
	}

	var lastErr error
	var lastResult *ExecuteResult

	for attempt := 0; attempt < maxAttempts; attempt++ {
		entry := entries[attempt%len(entries)]

		if !entry.GetProvider().IsUsable() {
			lastErr = fmt.Errorf("provider %s is not usable", entry.GetProvider().ID)
			continue
		}

		startTime := time.Now()

		execCtx := &domain.ProviderExecutionContext{
			Context:        ctx,
			Capability:     opts.Capability,
			Operation:      opts.Operation,
			TenantID:       orgID,
			ActorID:        actorID,
			IdempotencyKey: opts.IdempotencyKey,
			Deadline:       time.Now().Add(timeout),
			RequestID:      generateRequestID(),
		}

		result, execErr := perform(ctx, execCtx, entry.GetAdapter())

		duration := time.Since(startTime)

		lastResult = &ExecuteResult{
			ProviderID:        entry.GetProvider().ID,
			ExternalReference: result.ExternalReference,
			Data:              result.Data,
			RetryCount:        attempt,
			DurationMs:        duration.Milliseconds(),
		}

		if execErr != nil {
			lastErr = execErr
			classification := o.retryPolicy.ClassifyError(execErr)

			if attempt < maxAttempts-1 && classification == domain.Retryable {
				backoff := o.calculateBackoff(attempt)
				o.logger.Warn().
					Str("provider", entry.GetProvider().ID).
					Str("capability", string(opts.Capability)).
					Err(execErr).
					Int("attempt", attempt+1).
					Dur("backoff", backoff).
					Msg("Provider execution failed, retrying")
				time.Sleep(backoff)
				continue
			}

			lastResult.Success = false
			lastResult.ErrorCode = result.ErrorCode
			lastResult.ErrorMessage = execErr.Error()

			if classification == domain.NonRetryable || classification == domain.UnknownRetry {
				break
			}
			o.logExecution(entry.GetProvider().ID, orgID, actorID, opts, false, duration, execErr)
			return lastResult, nil
		}

		lastResult.Success = true
		o.logExecution(entry.GetProvider().ID, orgID, actorID, opts, true, duration, nil)
		return lastResult, nil
	}

	if lastResult != nil {
		lastResult.ErrorCode = domain.ErrCodeProviderUnavailable
		lastResult.ErrorMessage = lastErr.Error()
	}
	return lastResult, lastErr
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

func (o *ProviderOrchestrator) logExecution(
	providerID, orgID, actorID string,
	opts ExecuteOptions,
	success bool,
	duration time.Duration,
	err error,
) {
	if err != nil {
		o.logger.Error().
			Str("provider", providerID).
			Str("capability", string(opts.Capability)).
			Str("operation", opts.Operation).
			Str("tenant", orgID).
			Str("actor", actorID).
			Dur("duration", duration).
			Err(err).
			Msg("Provider execution failed")
	} else {
		o.logger.Info().
			Str("provider", providerID).
			Str("capability", string(opts.Capability)).
			Str("operation", opts.Operation).
			Str("tenant", orgID).
			Str("actor", actorID).
			Dur("duration", duration).
			Bool("success", success).
			Msg("Provider execution completed")
	}
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
