package domain

import (
	"context"
	"fmt"
	"time"
)

type ProviderStatus string

const (
	ProviderStatusActive    ProviderStatus = "ACTIVE"
	ProviderStatusInactive  ProviderStatus = "INACTIVE"
	ProviderStatusSuspended ProviderStatus = "SUSPENDED"
	ProviderStatusDegraded  ProviderStatus = "DEGRADED"
)

type Capability string

const (
	CapabilityAvailability   Capability = "AVAILABILITY"
	CapabilityReservation    Capability = "RESERVATION"
	CapabilityBooking        Capability = "BOOKING"
	CapabilityCatalog        Capability = "CATALOG"
	CapabilityDelivery       Capability = "DELIVERY"
	CapabilityTracking       Capability = "TRACKING"
	CapabilityLocation       Capability = "LOCATION"
	CapabilityNotification   Capability = "NOTIFICATION"
	CapabilityIdentityVerify Capability = "IDENTITY_VERIFICATION"
	CapabilityPayment        Capability = "PAYMENT"
	CapabilityFulfillment    Capability = "FULFILLMENT"
)

type ProviderConfig struct {
	Endpoints map[string]string `json:"endpoints,omitempty"`
	Settings  map[string]string `json:"settings,omitempty"`
}

type IntegrationProvider struct {
	ID             string
	OrganizationID string
	Code           string
	Name           string
	Type           string
	Status         ProviderStatus
	Capabilities   []Capability
	Configuration  ProviderConfig
	SecretRef      string
	Priority       int
	HealthStatus   ProviderHealthStatus
	CreatedAt      int64
	UpdatedAt      int64
	Version        int64
}

type ProviderHealthStatus string

const (
	ProviderHealthUnknown     ProviderHealthStatus = "UNKNOWN"
	ProviderHealthHealthy     ProviderHealthStatus = "HEALTHY"
	ProviderHealthDegraded    ProviderHealthStatus = "DEGRADED"
	ProviderHealthUnavailable ProviderHealthStatus = "UNAVAILABLE"
)

func NewProvider(orgID, code, name, providerType string, capabilities []Capability, config ProviderConfig, secretRef string, priority int) *IntegrationProvider {
	now := time.Now().Unix()
	return &IntegrationProvider{
		OrganizationID: orgID,
		Code:           code,
		Name:           name,
		Type:           providerType,
		Status:         ProviderStatusInactive,
		Capabilities:   capabilities,
		Configuration:  config,
		SecretRef:      secretRef,
		Priority:       priority,
		HealthStatus:   ProviderHealthUnknown,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
	}
}

func (p *IntegrationProvider) Enable(now int64) error {
	if p.Status == ProviderStatusActive {
		return fmt.Errorf("provider is already active")
	}
	p.Status = ProviderStatusActive
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *IntegrationProvider) Disable(now int64) error {
	if p.Status == ProviderStatusInactive || p.Status == ProviderStatusSuspended {
		return fmt.Errorf("provider is already inactive or suspended")
	}
	p.Status = ProviderStatusInactive
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *IntegrationProvider) Suspend(now int64) error {
	if p.Status == ProviderStatusSuspended {
		return fmt.Errorf("provider is already suspended")
	}
	p.Status = ProviderStatusSuspended
	p.UpdatedAt = now
	p.Version++
	return nil
}

func (p *IntegrationProvider) SetHealth(status ProviderHealthStatus, now int64) {
	p.HealthStatus = status
	p.UpdatedAt = now
	p.Version++
}

func (p *IntegrationProvider) SupportsCapability(cap Capability) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (p *IntegrationProvider) IsUsable() bool {
	if p.Status != ProviderStatusActive {
		return false
	}
	if p.HealthStatus == ProviderHealthUnavailable {
		return false
	}
	return true
}

func (p *IntegrationProvider) ValidateInvariants() error {
	if p.Code == "" {
		return fmt.Errorf("provider code is required")
	}
	if p.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if p.Status != ProviderStatusActive &&
		p.Status != ProviderStatusInactive &&
		p.Status != ProviderStatusSuspended &&
		p.Status != ProviderStatusDegraded {
		return fmt.Errorf("invalid provider status: %s", p.Status)
	}
	if p.Version < 1 {
		return fmt.Errorf("version must be >= 1")
	}
	return nil
}

type ProviderRepository interface {
	Create(ctx context.Context, provider *IntegrationProvider) error
	FindByID(ctx context.Context, orgID, id string) (*IntegrationProvider, error)
	FindByCode(ctx context.Context, orgID, code string) (*IntegrationProvider, error)
	FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]*IntegrationProvider, error)
	FindByCapability(ctx context.Context, orgID string, capability Capability) ([]*IntegrationProvider, error)
	Update(ctx context.Context, provider *IntegrationProvider) error
	Delete(ctx context.Context, orgID, id string) error
}

type ProviderExecutionStatus string

const (
	ExecutionStatusSuccess   ProviderExecutionStatus = "SUCCESS"
	ExecutionStatusFailed    ProviderExecutionStatus = "FAILED"
	ExecutionStatusCancelled ProviderExecutionStatus = "CANCELLED"
)

// ProviderExecution records one attempt of an orchestrated provider call.
type ProviderExecution struct {
	ID             string
	ProviderID     string
	Capability     Capability
	Operation      string
	TenantID       string
	ActorID        string
	IdempotencyKey string
	// RequestID correlates the attempts of one orchestrated call; Attempt is
	// 1-based; RetryClass classifies a failed attempt.
	RequestID         string
	Attempt           int
	RetryClass        RetryClassification
	Status            ProviderExecutionStatus
	ErrorCode         ProviderErrorCode
	StartedAt         int64
	CompletedAt       int64
	DurationMs        int64
	ExternalReference string
}

type ProviderErrorCode string

const (
	ErrCodeProviderUnavailable   ProviderErrorCode = "PROVIDER_UNAVAILABLE"
	ErrCodeProviderTimeout       ProviderErrorCode = "PROVIDER_TIMEOUT"
	ErrCodeProviderAuthFailed    ProviderErrorCode = "PROVIDER_AUTH_FAILED"
	ErrCodeRateLimited           ProviderErrorCode = "PROVIDER_RATE_LIMITED"
	ErrCodeInvalidRequest        ProviderErrorCode = "INVALID_REQUEST"
	ErrCodeUnsupportedCapability ProviderErrorCode = "UNSUPPORTED_CAPABILITY"
	ErrCodeResourceNotFound      ProviderErrorCode = "RESOURCE_NOT_FOUND"
	ErrCodeResourceUnavailable   ProviderErrorCode = "RESOURCE_UNAVAILABLE"
	ErrCodeConflict              ProviderErrorCode = "CONFLICT"
	ErrCodeProviderRejected      ProviderErrorCode = "PROVIDER_REJECTED"
	// ErrCodeInvalidResponse: the provider answered but the answer is
	// missing or contradictory (e.g. no availability state).
	ErrCodeInvalidResponse ProviderErrorCode = "PROVIDER_INVALID_RESPONSE"
	ErrCodeUnknown         ProviderErrorCode = "UNKNOWN_PROVIDER_ERROR"
)

type ProviderExecutionRepository interface {
	Create(ctx context.Context, exec *ProviderExecution) error
	FindByID(ctx context.Context, orgID, id string) (*ProviderExecution, error)
	FindByIdempotencyKey(ctx context.Context, orgID, key string) (*ProviderExecution, error)
	FindByProviderID(ctx context.Context, orgID, providerID string, offset, limit int) ([]*ProviderExecution, error)
}
