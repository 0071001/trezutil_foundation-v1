package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

type ProviderRepository struct {
	db *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

type providerRow struct {
	ID             string
	OrganizationID string
	Code           string
	Name           string
	Type           string
	Status         string
	Capabilities   []string
	Configuration  string
	SecretRef      sql.NullString
	Priority       int
	HealthStatus   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

func (r *ProviderRepository) Create(ctx context.Context, provider *domain.IntegrationProvider) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO providers (id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, provider.ID, provider.OrganizationID, provider.Code, provider.Name, provider.Type,
		string(provider.Status), pq.Array(encodeCapabilities(provider.Capabilities)),
		encodeProviderConfig(provider.Configuration), provider.SecretRef, provider.Priority,
		string(provider.HealthStatus), toTime(provider.CreatedAt), toTime(provider.UpdatedAt), provider.Version)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}
	return nil
}

func (r *ProviderRepository) FindByID(ctx context.Context, orgID, id string) (*domain.IntegrationProvider, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version
		FROM providers WHERE id = $1 AND organization_id = $2
	`, id, orgID)
	return scanProvider(row)
}

func (r *ProviderRepository) FindByCode(ctx context.Context, orgID, code string) (*domain.IntegrationProvider, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version
		FROM providers WHERE code = $1 AND organization_id = $2
	`, code, orgID)
	return scanProvider(row)
}

func (r *ProviderRepository) FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]*domain.IntegrationProvider, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version
		FROM providers WHERE organization_id = $1 ORDER BY priority DESC, created_at DESC
		OFFSET $2 LIMIT $3
	`, orgID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	defer rows.Close()

	return scanProviders(rows)
}

func (r *ProviderRepository) FindByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version
		FROM providers WHERE organization_id = $1
		ORDER BY priority DESC, created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers by capability: %w", err)
	}
	defer rows.Close()

	var providers []*domain.IntegrationProvider
	all, err := scanProviders(rows)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.SupportsCapability(capability) {
			providers = append(providers, p)
		}
	}
	if providers == nil {
		providers = []*domain.IntegrationProvider{}
	}
	return providers, nil
}

func (r *ProviderRepository) FindUsableByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, code, name, type, status, capabilities, configuration, secret_ref, priority, health_status, created_at, updated_at, version
		FROM providers WHERE organization_id = $1
		ORDER BY priority DESC, created_at DESC
	`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to find usable providers: %w", err)
	}
	defer rows.Close()

	var providers []*domain.IntegrationProvider
	all, err := scanProviders(rows)
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if p.IsUsable() && p.SupportsCapability(capability) {
			providers = append(providers, p)
		}
	}
	if providers == nil {
		providers = []*domain.IntegrationProvider{}
	}
	return providers, nil
}

func (r *ProviderRepository) Update(ctx context.Context, provider *domain.IntegrationProvider) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE providers SET
			name = $1, type = $2, status = $3, capabilities = $4, configuration = $5,
			secret_ref = $6, priority = $7, health_status = $8, updated_at = NOW(), version = version + 1
		WHERE id = $9 AND organization_id = $10 AND version = $11
	`, provider.Name, provider.Type, string(provider.Status), pq.Array(encodeCapabilities(provider.Capabilities)),
		encodeProviderConfig(provider.Configuration), provider.SecretRef, provider.Priority,
		string(provider.HealthStatus), provider.ID, provider.OrganizationID, provider.Version)
	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *ProviderRepository) Delete(ctx context.Context, orgID, id string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM providers WHERE id = $1 AND organization_id = $2
	`, id, orgID)
	if err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}
	return nil
}

func scanProvider(row *sql.Row) (*domain.IntegrationProvider, error) {
	var p providerRow
	err := row.Scan(
		&p.ID, &p.OrganizationID, &p.Code, &p.Name, &p.Type, &p.Status,
		pq.Array(&p.Capabilities), &p.Configuration, &p.SecretRef, &p.Priority,
		&p.HealthStatus, &p.CreatedAt, &p.UpdatedAt, &p.Version,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrProviderNotFound
		}
		return nil, fmt.Errorf("failed to scan provider: %w", err)
	}
	return unmarshalProvider(p), nil
}

func scanProviders(rows *sql.Rows) ([]*domain.IntegrationProvider, error) {
	var providers []*domain.IntegrationProvider
	for rows.Next() {
		var p providerRow
		err := rows.Scan(
			&p.ID, &p.OrganizationID, &p.Code, &p.Name, &p.Type, &p.Status,
			pq.Array(&p.Capabilities), &p.Configuration, &p.SecretRef, &p.Priority,
			&p.HealthStatus, &p.CreatedAt, &p.UpdatedAt, &p.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}
		providers = append(providers, unmarshalProvider(p))
	}
	if providers == nil {
		providers = []*domain.IntegrationProvider{}
	}
	return providers, nil
}

func unmarshalProvider(r providerRow) *domain.IntegrationProvider {
	var capabilities []domain.Capability
	for _, c := range r.Capabilities {
		capabilities = append(capabilities, domain.Capability(c))
	}
	if capabilities == nil {
		capabilities = []domain.Capability{}
	}

	return &domain.IntegrationProvider{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		Code:           r.Code,
		Name:           r.Name,
		Type:           r.Type,
		Status:         domain.ProviderStatus(r.Status),
		Capabilities:   capabilities,
		Configuration:  decodeProviderConfig(r.Configuration),
		SecretRef:      r.SecretRef.String,
		Priority:       r.Priority,
		HealthStatus:   domain.ProviderHealthStatus(r.HealthStatus),
		CreatedAt:      toUnix(r.CreatedAt),
		UpdatedAt:      toUnix(r.UpdatedAt),
		Version:        r.Version,
	}
}

func encodeCapabilities(caps []domain.Capability) []string {
	result := make([]string, len(caps))
	for i, c := range caps {
		result[i] = string(c)
	}
	return result
}

func encodeProviderConfig(config domain.ProviderConfig) string {
	if len(config.Endpoints) == 0 && len(config.Settings) == 0 {
		return ""
	}
	result := "{"
	first := true
	for k, v := range config.Endpoints {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`"endpoints.%s":"%s"`, k, v)
		first = false
	}
	for k, v := range config.Settings {
		if !first {
			result += ","
		}
		result += fmt.Sprintf(`"settings.%s":"%s"`, k, v)
		first = false
	}
	result += "}"
	return result
}

func decodeProviderConfig(s string) domain.ProviderConfig {
	config := domain.ProviderConfig{
		Endpoints: map[string]string{},
		Settings:  map[string]string{},
	}
	if s == "" {
		return config
	}
	return config
}
