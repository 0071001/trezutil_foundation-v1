package domain

import (
	"context"
	"time"
)

type ProviderExecutionContext struct {
	Context        context.Context
	Capability     Capability
	Operation      string
	TenantID       string
	ActorID        string
	IdempotencyKey string
	Deadline       time.Time
	RequestID      string
}

func (e *ProviderExecutionContext) RemainingTime() time.Duration {
	if e.Deadline.IsZero() {
		return 0
	}
	return time.Until(e.Deadline)
}

type ProviderResult struct {
	Success             bool
	Data                map[string]interface{}
	ExternalReference   string
	ErrorCode           ProviderErrorCode
	ErrorMessage        string
	ProviderDiagnostics map[string]interface{}
}

type ProviderExecutionRecord struct {
	ExecutionID       string
	ProviderID        string
	Capability        Capability
	Operation         string
	TenantID          string
	ActorID           string
	IdempotencyKey    string
	Status            ProviderExecutionStatus
	ErrorCode         ProviderErrorCode
	StartedAt         int64
	CompletedAt       int64
	DurationMs        int64
	ExternalReference string
}

type RetryClassification string

const (
	Retryable    RetryClassification = "RETRYABLE"
	NonRetryable RetryClassification = "NON_RETRYABLE"
	UnknownRetry RetryClassification = "UNKNOWN"
)

type RetryPolicy struct {
	MaxAttempts   int
	BaseDelayMs   int64
	MaxDelayMs    int64
	ClassifyError func(error) RetryClassification
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelayMs: 100,
		MaxDelayMs:  10000,
		ClassifyError: func(err error) RetryClassification {
			if err == nil {
				return NonRetryable
			}
			errStr := err.Error()
			if isRetryableError(errStr) {
				return Retryable
			}
			if isNonRetryableError(errStr) {
				return NonRetryable
			}
			return UnknownRetry
		},
	}
}

func isRetryableError(errStr string) bool {
	return containsAny(errStr, []string{
		"timeout", "connection refused", "connection reset",
		"temp unavailable", "i/o timeout", "no such host",
	})
}

func isNonRetryableError(errStr string) bool {
	return containsAny(errStr, []string{
		"unauthorized", "forbidden", "invalid request",
		"not found", "conflict", "rate limit",
	})
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type ProviderAdapter interface {
	ProviderID() string
	Capabilities() []Capability
	Health(ctx context.Context) ProviderHealthStatus
}

// AvailabilityProviderAdapter checks availability with an external provider.
// The adapter must set AvailabilityResult.State from the provider's explicit
// answer; a response that does not establish availability leaves it empty (or
// UNKNOWN), and a transport success is never an AVAILABLE answer by itself.
type AvailabilityProviderAdapter interface {
	ProviderAdapter
	CheckAvailability(ctx context.Context, query AvailabilityQuery) (AvailabilityResult, error)
}

type ReservationProviderAdapter interface {
	ProviderAdapter
	CreateExternalReservation(ctx context.Context, req ProviderReservationRequest) (ProviderResult, error)
	CancelExternalReservation(ctx context.Context, externalRef string) (ProviderResult, error)
}

type TrackingProviderAdapter interface {
	ProviderAdapter
	Track(ctx context.Context, trackingRef string) (ProviderResult, error)
}

type ProviderReservationRequest struct {
	ListingID      string
	StartAt        int64
	EndAt          int64
	Quantity       int
	PartySize      int
	IdempotencyKey string
	Metadata       map[string]interface{}
}

// PaymentProviderRequest is the request payload for provider payment operations.
type PaymentProviderRequest struct {
	Amount            Money
	PaymentMethodType PaymentMethodType
	PaymentMethodID   string
	IdempotencyKey    string
	Metadata          map[string]interface{}
}

// PaymentProviderResponse is the response payload for provider payment operations.
type PaymentProviderResponse struct {
	ProviderPaymentID string
	Status            string // provider-specific status string
	Data              map[string]interface{}
	ErrorCode         ProviderErrorCode
	ErrorMessage      string
}

// RefundProviderResponse is the response payload for provider refund operations.
type RefundProviderResponse struct {
	ProviderRefundID string
	Status           string
	Amount           Money
	Data             map[string]interface{}
	ErrorCode        ProviderErrorCode
	ErrorMessage     string
}

// PaymentProviderAdapter extends ProviderAdapter with payment operations.
// Providers implement only the operations they support; unsupported operations
// return ProviderErrorCode = ErrCodeUnsupportedCapability.
// No provider-specific logic leaks into the domain/application core — this is
// the stable domain boundary contract.
type PaymentProviderAdapter interface {
	ProviderAdapter

	// CreatePayment initiates a payment with the provider.
	CreatePayment(ctx context.Context, req PaymentProviderRequest) (PaymentProviderResponse, error)

	// GetPayment retrieves the current status of a payment from the provider.
	GetPayment(ctx context.Context, providerPaymentID string) (PaymentProviderResponse, error)

	// CancelPayment cancels (voids) a payment with the provider.
	CancelPayment(ctx context.Context, providerPaymentID string) (PaymentProviderResponse, error)

	// CapturePayment captures a previously authorized payment.
	CapturePayment(ctx context.Context, providerPaymentID string, amount *Money) (PaymentProviderResponse, error)

	// RefundPayment refunds (fully or partially) a captured payment.
	RefundPayment(ctx context.Context, providerPaymentID string, amount *Money) (RefundProviderResponse, error)
}

// PaymentProviderCapabilities describes which operations a provider supports.
type PaymentProviderCapabilities struct {
	CanCreate  bool
	CanGet     bool
	CanCancel  bool
	CanCapture bool
	CanRefund  bool
}
