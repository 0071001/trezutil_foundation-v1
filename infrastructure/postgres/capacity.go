package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type capacityUnitRow struct {
	ID          string
	InventoryID string
	StartAt     time.Time
	EndAt       time.Time
	Capacity    int64
	Reserved    int64
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CapacityRepository struct {
	db *sql.DB
}

func NewCapacityRepository(db *sql.DB) *CapacityRepository {
	return &CapacityRepository{db: db}
}

func (r *CapacityRepository) Create(ctx context.Context, unit *domain.CapacityUnit) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO capacity_units (id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, unit.ID, unit.InventoryID, toTime(unit.StartAt), toTime(unit.EndAt),
		unit.Capacity, unit.Reserved, unit.Version, toTime(unit.CreatedAt), toTime(unit.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create capacity unit: %w", err)
	}
	return nil
}

func (r *CapacityRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.CapacityUnit, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at
		FROM capacity_units WHERE id = $1
	`, id)
	return scanCapacityUnit(row)
}

func (r *CapacityRepository) FindByInventoryID(ctx context.Context, tenantID, inventoryID string) ([]*domain.CapacityUnit, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at
		FROM capacity_units WHERE inventory_id = $1 ORDER BY start_at ASC
	`, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to find capacity units: %w", err)
	}
	defer rows.Close()

	var units []*domain.CapacityUnit
	for rows.Next() {
		u, err := scanCapacityUnitFromRow(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	if units == nil {
		units = []*domain.CapacityUnit{}
	}
	return units, nil
}

func (r *CapacityRepository) FindByInventoryIDAndRange(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at
		FROM capacity_units WHERE inventory_id = $1 AND start_at < $3 AND end_at > $2
		ORDER BY start_at ASC
	`, inventoryID, toTime(startAt), toTime(endAt))
	if err != nil {
		return nil, fmt.Errorf("failed to find capacity units by range: %w", err)
	}
	defer rows.Close()

	var units []*domain.CapacityUnit
	for rows.Next() {
		u, err := scanCapacityUnitFromRow(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	if units == nil {
		units = []*domain.CapacityUnit{}
	}
	return units, nil
}

func (r *CapacityRepository) FindByInventoryIDAndRangeForUpdate(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at
		FROM capacity_units WHERE inventory_id = $1 AND start_at < $3 AND end_at > $2
		ORDER BY start_at ASC FOR UPDATE
	`, inventoryID, toTime(startAt), toTime(endAt))
	if err != nil {
		return nil, fmt.Errorf("failed to find capacity units for update: %w", err)
	}
	defer rows.Close()

	var units []*domain.CapacityUnit
	for rows.Next() {
		u, err := scanCapacityUnitFromRow(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	if units == nil {
		units = []*domain.CapacityUnit{}
	}
	return units, nil
}

func (r *CapacityRepository) Update(ctx context.Context, unit *domain.CapacityUnit) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE capacity_units SET reserved = $1, version = $2, updated_at = $3
		WHERE id = $4 AND version = $5
	`, unit.Reserved, unit.Version, toTime(unit.UpdatedAt), unit.ID, unit.Version-1)
	if err != nil {
		return fmt.Errorf("failed to update capacity unit: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *CapacityRepository) Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE capacity_units SET reserved = reserved + $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND (reserved + $1) <= capacity AND $1 > 0
	`, quantity, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to reserve capacity: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *CapacityRepository) Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE capacity_units SET reserved = reserved - $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3 AND reserved >= $1 AND $1 > 0
	`, quantity, id, expectedVersion)
	if err != nil {
		return fmt.Errorf("failed to release capacity: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

// ReserveCapacity reserves capacity across all units in the given time range for a listing.
func (r *CapacityRepository) ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	// First find the inventory for this listing
	invRow := r.db.QueryRowContext(ctx, `
		SELECT id FROM inventory_resources WHERE listing_id = $1
	`, listingID)
	var inventoryID string
	if err := invRow.Scan(&inventoryID); err != nil {
		if err == sql.ErrNoRows {
			return nil // No inventory, no capacity to reserve
		}
		return fmt.Errorf("failed to find inventory: %w", err)
	}

	units, err := r.FindByInventoryIDAndRangeForUpdate(ctx, tenantID, inventoryID, startAt, endAt)
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Available() < quantity {
			return domain.ErrInsufficientCapacity
		}
	}

	now := time.Now().Unix()
	for _, u := range units {
		if err := r.Reserve(ctx, u.ID, quantity, u.Version); err != nil {
			return err
		}
		// Note: audit logging is handled by the caller
		_ = now
		_ = reservationID
	}
	return nil
}

// ReleaseCapacity releases capacity across all units in the given time range for a listing.
func (r *CapacityRepository) ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	// First find the inventory for this listing
	invRow := r.db.QueryRowContext(ctx, `
		SELECT id FROM inventory_resources WHERE listing_id = $1
	`, listingID)
	var inventoryID string
	if err := invRow.Scan(&inventoryID); err != nil {
		if err == sql.ErrNoRows {
			return nil // No inventory, no capacity to release
		}
		return fmt.Errorf("failed to find inventory: %w", err)
	}

	units, err := r.FindByInventoryIDAndRangeForUpdate(ctx, tenantID, inventoryID, startAt, endAt)
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Reserved < quantity {
			return domain.ErrInvalidCapacityQuantity
		}
	}

	now := time.Now().Unix()
	for _, u := range units {
		if err := r.Release(ctx, u.ID, quantity, u.Version); err != nil {
			return err
		}
		// Note: audit logging is handled by the caller
		_ = now
		_ = reservationID
	}
	return nil
}

func scanCapacityUnit(row *sql.Row) (*domain.CapacityUnit, error) {
	var r capacityUnitRow
	err := row.Scan(&r.ID, &r.InventoryID, &r.StartAt, &r.EndAt,
		&r.Capacity, &r.Reserved, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrCapacityNotFound
		}
		return nil, fmt.Errorf("failed to scan capacity unit: %w", err)
	}
	return unmarshalCapacityUnit(r), nil
}

func scanCapacityUnitFromRow(rows *sql.Rows) (*domain.CapacityUnit, error) {
	var r capacityUnitRow
	err := rows.Scan(&r.ID, &r.InventoryID, &r.StartAt, &r.EndAt,
		&r.Capacity, &r.Reserved, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan capacity unit: %w", err)
	}
	return unmarshalCapacityUnit(r), nil
}

func unmarshalCapacityUnit(r capacityUnitRow) *domain.CapacityUnit {
	return &domain.CapacityUnit{
		ID:          r.ID,
		InventoryID: r.InventoryID,
		StartAt:     toUnix(r.StartAt),
		EndAt:       toUnix(r.EndAt),
		Capacity:    r.Capacity,
		Reserved:    r.Reserved,
		Version:     r.Version,
		CreatedAt:   toUnix(r.CreatedAt),
		UpdatedAt:   toUnix(r.UpdatedAt),
	}
}
