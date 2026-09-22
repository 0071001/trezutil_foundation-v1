package integration

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/ai"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestAIProviderRepository_CreateAndGet(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	provider := domain.NewAIProviderEntity("openai", "OpenAI GPT", "openai")
	provider.ParsedConfig = domain.AIProviderConfig{
		Endpoint:  "https://api.openai.com/v1",
		ModelIDs:  []string{"gpt-4", "gpt-3.5-turbo"},
		APIKeyRef: "secret/openai/api_key",
	}

	err := repo.Create(ctx, provider)
	if err != nil {
		t.Fatalf("failed to create AI provider: %v", err)
	}
	if provider.ID == "" {
		t.Fatal("expected provider ID to be generated")
	}

	found, err := repo.FindByID(ctx, provider.ID)
	if err != nil {
		t.Fatalf("failed to find provider by ID: %v", err)
	}
	if found.Code != "openai" {
		t.Errorf("expected code 'openai', got '%s'", found.Code)
	}
	if found.Name != "OpenAI GPT" {
		t.Errorf("expected name 'OpenAI GPT', got '%s'", found.Name)
	}
	if found.Type != "openai" {
		t.Errorf("expected type 'openai', got '%s'", found.Type)
	}
	if found.Status != domain.AIProviderStatusActive {
		t.Errorf("expected status 'active', got '%s'", found.Status)
	}
	if found.ParsedConfig.Endpoint != "https://api.openai.com/v1" {
		t.Errorf("expected endpoint 'https://api.openai.com/v1', got '%s'", found.ParsedConfig.Endpoint)
	}
}

func TestAIProviderRepository_FindByCode(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	provider := domain.NewAIProviderEntity("anthropic", "Anthropic Claude", "anthropic")
	if err := repo.Create(ctx, provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	found, err := repo.FindByCode(ctx, "anthropic")
	if err != nil {
		t.Fatalf("failed to find provider by code: %v", err)
	}
	if found.Name != "Anthropic Claude" {
		t.Errorf("expected name 'Anthropic Claude', got '%s'", found.Name)
	}
}

func TestAIProviderRepository_FindByCodeNotFound(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	_, err := repo.FindByCode(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent provider code")
	}
}

func TestAIProviderRepository_FindAll(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	p1 := domain.NewAIProviderEntity("openai", "OpenAI", "openai")
	p2 := domain.NewAIProviderEntity("anthropic", "Anthropic", "anthropic")
	p3 := domain.NewAIProviderEntity("google", "Google", "google")

	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create provider 1: %v", err)
	}
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("failed to create provider 2: %v", err)
	}
	if err := repo.Create(ctx, p3); err != nil {
		t.Fatalf("failed to create provider 3: %v", err)
	}

	providers, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("failed to find all providers: %v", err)
	}
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}
}

func TestAIProviderRepository_FindActive(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	p1 := domain.NewAIProviderEntity("active-provider", "Active Provider", "openai")
	p2 := domain.NewAIProviderEntity("inactive-provider", "Inactive Provider", "anthropic")

	if err := repo.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create active provider: %v", err)
	}
	if err := repo.Create(ctx, p2); err != nil {
		t.Fatalf("failed to create inactive provider: %v", err)
	}

	if err := p2.Disable(); err != nil {
		t.Fatalf("failed to disable provider: %v", err)
	}
	if err := repo.Update(ctx, p2); err != nil {
		t.Fatalf("failed to update provider: %v", err)
	}

	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("failed to find active providers: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active provider, got %d", len(active))
	}
	if active[0].Code != "active-provider" {
		t.Errorf("expected active provider 'active-provider', got '%s'", active[0].Code)
	}
}

func TestAIProviderRepository_Update(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	provider := domain.NewAIProviderEntity("openai", "OpenAI GPT", "openai")
	if err := repo.Create(ctx, provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	provider.Name = "OpenAI Updated"
	provider.ParsedConfig.Endpoint = "https://updated.openai.com/v1"

	if err := repo.Update(ctx, provider); err != nil {
		t.Fatalf("failed to update provider: %v", err)
	}

	found, err := repo.FindByID(ctx, provider.ID)
	if err != nil {
		t.Fatalf("failed to find provider after update: %v", err)
	}
	if found.Name != "OpenAI Updated" {
		t.Errorf("expected name 'OpenAI Updated', got '%s'", found.Name)
	}
	if found.ParsedConfig.Endpoint != "https://updated.openai.com/v1" {
		t.Errorf("expected endpoint 'https://updated.openai.com/v1', got '%s'", found.ParsedConfig.Endpoint)
	}
}

func TestAIProviderRepository_Delete(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	provider := domain.NewAIProviderEntity("to-delete", "To Delete", "openai")
	if err := repo.Create(ctx, provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if err := repo.Delete(ctx, provider.ID); err != nil {
		t.Fatalf("failed to delete provider: %v", err)
	}

	_, err := repo.FindByID(ctx, provider.ID)
	if err == nil {
		t.Fatal("expected error after deleting provider")
	}
}

func TestAIProviderRepository_EnableDisable(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	repo := ai.NewAIProviderRepository(db.DB)

	provider := domain.NewAIProviderEntity("toggle-test", "Toggle Test", "openai")
	if err := repo.Create(ctx, provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if err := provider.Disable(); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}
	if err := repo.Update(ctx, provider); err != nil {
		t.Fatalf("failed to update after disable: %v", err)
	}

	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("failed to find active providers: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active providers, got %d", len(active))
	}

	if err := provider.Enable(); err != nil {
		t.Fatalf("failed to enable: %v", err)
	}
	if err := repo.Update(ctx, provider); err != nil {
		t.Fatalf("failed to update after enable: %v", err)
	}

	active, err = repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("failed to find active providers: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active provider, got %d", len(active))
	}
}
