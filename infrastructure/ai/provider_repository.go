package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

type AIProviderRepository struct {
	db *sql.DB
}

func NewAIProviderRepository(db *sql.DB) *AIProviderRepository {
	return &AIProviderRepository{db: db}
}

func (r *AIProviderRepository) Create(ctx context.Context, provider *domain.AIProviderEntity) error {
	configJSON, _ := json.Marshal(provider.ParsedConfig)
	query := `INSERT INTO ai_providers (id, code, name, type, status, config, created_at, updated_at) 
	          VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NOW()) RETURNING id`
	return r.db.QueryRowContext(ctx, query, provider.Code, provider.Name, provider.Type, provider.Status, configJSON).Scan(&provider.ID)
}

func (r *AIProviderRepository) FindByID(ctx context.Context, id string) (*domain.AIProviderEntity, error) {
	var p domain.AIProviderEntity
	var configJSON []byte
	var createdAt, updatedAt time.Time
	query := `SELECT id, code, name, type, status, config, created_at, updated_at FROM ai_providers WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Code, &p.Name, &p.Type, &p.Status, &configJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(configJSON, &p.ParsedConfig)
	p.CreatedAt = toUnix(createdAt)
	p.UpdatedAt = toUnix(updatedAt)
	return &p, nil
}

func (r *AIProviderRepository) FindByCode(ctx context.Context, code string) (*domain.AIProviderEntity, error) {
	var p domain.AIProviderEntity
	var configJSON []byte
	var createdAt, updatedAt time.Time
	query := `SELECT id, code, name, type, status, config, created_at, updated_at FROM ai_providers WHERE code = $1`
	err := r.db.QueryRowContext(ctx, query, code).Scan(&p.ID, &p.Code, &p.Name, &p.Type, &p.Status, &configJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(configJSON, &p.ParsedConfig)
	p.CreatedAt = toUnix(createdAt)
	p.UpdatedAt = toUnix(updatedAt)
	return &p, nil
}

func (r *AIProviderRepository) FindAll(ctx context.Context) ([]*domain.AIProviderEntity, error) {
	query := `SELECT id, code, name, type, status, config, created_at, updated_at FROM ai_providers ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*domain.AIProviderEntity
	for rows.Next() {
		var p domain.AIProviderEntity
		var configJSON []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Type, &p.Status, &configJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(configJSON, &p.ParsedConfig)
		p.CreatedAt = toUnix(createdAt)
		p.UpdatedAt = toUnix(updatedAt)
		providers = append(providers, &p)
	}
	return providers, nil
}

func (r *AIProviderRepository) FindActive(ctx context.Context) ([]*domain.AIProviderEntity, error) {
	query := `SELECT id, code, name, type, status, config, created_at, updated_at FROM ai_providers WHERE status = 'ACTIVE' ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*domain.AIProviderEntity
	for rows.Next() {
		var p domain.AIProviderEntity
		var configJSON []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Type, &p.Status, &configJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(configJSON, &p.ParsedConfig)
		p.CreatedAt = toUnix(createdAt)
		p.UpdatedAt = toUnix(updatedAt)
		providers = append(providers, &p)
	}
	return providers, nil
}

func (r *AIProviderRepository) Update(ctx context.Context, provider *domain.AIProviderEntity) error {
	configJSON, _ := json.Marshal(provider.ParsedConfig)
	provider.UpdatedAt = time.Now().Unix()
	query := `UPDATE ai_providers SET code = $1, name = $2, type = $3, status = $4, config = $5, updated_at = NOW() WHERE id = $6`
	_, err := r.db.ExecContext(ctx, query, provider.Code, provider.Name, provider.Type, provider.Status, configJSON, provider.ID)
	return err
}

func (r *AIProviderRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM ai_providers WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
