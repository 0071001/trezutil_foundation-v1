package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type inventoryResourceRow struct {
	ID           string
	TenantID     string
	ListingID    string
	ResourceType string
	Total        int64
	Reserved     int64
	Consumed     int64
	Version      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type InventoryRepository struct {
	db *sql.DB
}

func NewInventoryRepository(db *sql.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) Create(ctx context.Context, resource *domain.InventoryResource) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO inventory_resources (id, tenant_id, listing_id, resource_type, total, reserved, consumed, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, resource.ID, resource.TenantID, resource.ListingID, string(resource.ResourceType),
		resource.Total, resource.Reserved, resource.Consumed, resource.Version,
		toTime(resource.CreatedAt), toTime(resource.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create inventory resource: %w", err)
	}
	return nil
}

func (r *InventoryRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.InventoryResource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, listing_id, resource_type, total, reserved, consumed, version, created_at, updated_at
		FROM inventory_resources WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanInventoryResource(row)
}

func (r *InventoryRepository) FindByListingID(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, listing_id, resource_type, total, reserved, consumed, version, created_at, updated_at
		FROM inventory_resources WHERE listing_id = $1 AND tenant_id = $2
	`, listingID, tenantID)
	return scanInventoryResource(row)
}

func (r *InventoryRepository) FindByListingIDForUpdate(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, listing_id, resource_type, total, reserved, consumed, version, created_at, updated_at
		FROM inventory_resources WHERE listing_id = $1 AND tenant_id = $2 FOR UPDATE
	`, listingID, tenantID)
	return scanInventoryResource(row)
}

func (r *InventoryRepository) Update(ctx context.Context, resource *domain.InventoryResource) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE inventory_resources SET total = $1, reserved = $2, consumed = $3, version = $4, updated_at = $5
		WHERE id = $6 AND version = $7
	`, resource.Total, resource.Reserved, resource.Consumed, resource.Version, toTime(resource.UpdatedAt), resource.ID, resource.Version-1)
	if err != nil {
		return fmt.Errorf("failed to update inventory resource: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *InventoryRepository) FindByTenantID(ctx context.Context, tenantID string, offset, limit int) ([]*domain.InventoryResource, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, listing_id, resource_type, total, reserved, consumed, version, created_at, updated_at
		FROM inventory_resources WHERE tenant_id = $1
		ORDER BY created_at DESC OFFSET $2 LIMIT $3
	`, tenantID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory resources: %w", err)
	}
	defer rows.Close()

	var resources []*domain.InventoryResource
	for rows.Next() {
		res, err := scanInventoryResourceFromRow(rows)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res)
	}
	if resources == nil {
		resources = []*domain.InventoryResource{}
	}
	return resources, nil
}

func (r *InventoryRepository) Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE inventory_resources SET reserved = reserved + $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND (reserved + $1 + consumed) <= total AND $1 > 0
	`, quantity, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to reserve inventory: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *InventoryRepository) Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE inventory_resources SET reserved = reserved - $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND reserved >= $1 AND $1 > 0
	`, quantity, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to release inventory: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *InventoryRepository) Consume(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE inventory_resources SET reserved = reserved - $1, consumed = consumed + $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND (reserved >= $1 OR (reserved + $1 + consumed) <= total) AND $1 > 0
	`, quantity, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to consume inventory: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func scanInventoryResource(row *sql.Row) (*domain.InventoryResource, error) {
	var r inventoryResourceRow
	err := row.Scan(&r.ID, &r.TenantID, &r.ListingID, &r.ResourceType,
		&r.Total, &r.Reserved, &r.Consumed, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrInventoryNotFound
		}
		return nil, fmt.Errorf("failed to scan inventory resource: %w", err)
	}
	return unmarshalInventoryResource(r), nil
}

func scanInventoryResourceFromRow(rows *sql.Rows) (*domain.InventoryResource, error) {
	var r inventoryResourceRow
	err := rows.Scan(&r.ID, &r.TenantID, &r.ListingID, &r.ResourceType,
		&r.Total, &r.Reserved, &r.Consumed, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan inventory resource: %w", err)
	}
	return unmarshalInventoryResource(r), nil
}

func unmarshalInventoryResource(r inventoryResourceRow) *domain.InventoryResource {
	return &domain.InventoryResource{
		ID:           r.ID,
		TenantID:     r.TenantID,
		ListingID:    r.ListingID,
		ResourceType: domain.ResourceType(r.ResourceType),
		Total:        r.Total,
		Reserved:     r.Reserved,
		Consumed:     r.Consumed,
		Version:      r.Version,
		CreatedAt:    toUnix(r.CreatedAt),
		UpdatedAt:    toUnix(r.UpdatedAt),
	}
}
