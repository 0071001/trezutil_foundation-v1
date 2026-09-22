package domain

import (
	"context"
	"fmt"
	"time"
)

type AIProviderEntity struct {
	ID           string
	Code         string
	Name         string
	Type         string
	Status       AIProviderStatus
	ParsedConfig AIProviderConfig
	CreatedAt    int64
	UpdatedAt    int64
}

type AIProviderConfig struct {
	Endpoint  string
	ModelIDs  []string
	APIKeyRef string
	Extra     map[string]interface{}
}

func NewAIProviderEntity(code, name, ptype string) *AIProviderEntity {
	now := time.Now().Unix()
	return &AIProviderEntity{
		ID:           "",
		Code:         code,
		Name:         name,
		Type:         ptype,
		Status:       AIProviderStatusActive,
		ParsedConfig: AIProviderConfig{Extra: make(map[string]interface{})},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (p *AIProviderEntity) IsUsable() bool {
	return p.Status == AIProviderStatusActive
}

func (p *AIProviderEntity) Validate() error {
	if p.Code == "" {
		return fmt.Errorf("provider code is required")
	}
	if p.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if p.Type == "" {
		return fmt.Errorf("provider type is required")
	}
	if p.Status != AIProviderStatusActive && p.Status != AIProviderStatusInactive {
		return fmt.Errorf("invalid provider status: %s", p.Status)
	}
	return nil
}

func (p *AIProviderEntity) Enable() error {
	if p.Status == AIProviderStatusActive {
		return fmt.Errorf("provider is already active")
	}
	p.Status = AIProviderStatusActive
	p.UpdatedAt = time.Now().Unix()
	return nil
}

func (p *AIProviderEntity) Disable() error {
	if p.Status == AIProviderStatusInactive {
		return fmt.Errorf("provider is already inactive")
	}
	p.Status = AIProviderStatusInactive
	p.UpdatedAt = time.Now().Unix()
	return nil
}

type AIProviderRepository interface {
	Create(ctx context.Context, provider *AIProviderEntity) error
	FindByID(ctx context.Context, id string) (*AIProviderEntity, error)
	FindByCode(ctx context.Context, code string) (*AIProviderEntity, error)
	FindAll(ctx context.Context) ([]*AIProviderEntity, error)
	FindActive(ctx context.Context) ([]*AIProviderEntity, error)
	Update(ctx context.Context, provider *AIProviderEntity) error
	Delete(ctx context.Context, id string) error
}
