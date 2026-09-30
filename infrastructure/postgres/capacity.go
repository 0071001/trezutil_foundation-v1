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

// ReserveCapacity takes quantity places in every capacity window of the
// listing's time-windowed resource (rooms, seats, time slots…) that overlaps
// [startAt, endAt). The windows are locked in one transaction; the period must
// be covered without gaps and every window must have room, otherwise nothing
// changes (ErrInsufficientCapacity). Listings without a time-windowed resource
// (stock, single bookable units) have no capacity to reserve.
func (r *CapacityRepository) ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	if quantity <= 0 {
		return domain.ErrInvalidCapacityQuantity
	}
	return r.adjustCapacity(ctx, listingID, startAt, endAt, quantity)
}

// ReleaseCapacity gives back quantity places in every window of the period,
// in one transaction.
func (r *CapacityRepository) ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	if quantity <= 0 {
		return domain.ErrInvalidCapacityQuantity
	}
	return r.adjustCapacity(ctx, listingID, startAt, endAt, -quantity)
}

func (r *CapacityRepository) adjustCapacity(ctx context.Context, listingID string, startAt, endAt, delta int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin capacity transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var inventoryID string
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM inventory_resources
		WHERE listing_id = $1 AND resource_type <> 'COUNTABLE'
		ORDER BY created_at LIMIT 1
	`, listingID).Scan(&inventoryID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to find inventory: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at
		FROM capacity_units WHERE inventory_id = $1 AND start_at < $3 AND end_at > $2
		ORDER BY start_at ASC FOR UPDATE
	`, inventoryID, toTime(startAt), toTime(endAt))
	if err != nil {
		return fmt.Errorf("failed to lock capacity units: %w", err)
	}
	var units []*domain.CapacityUnit
	for rows.Next() {
		u, err := scanCapacityUnitFromRow(rows)
		if err != nil {
			rows.Close()
			return err
		}
		units = append(units, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if delta > 0 {
		available, _, covered := domain.CapacityCoverage(units, startAt, endAt)
		if !covered || available < delta {
			return domain.ErrInsufficientCapacity
		}
	} else {
		for _, u := range units {
			if u.Reserved < -delta {
				return domain.ErrInvalidCapacityQuantity
			}
		}
	}
	for _, u := range units {
		if _, err := tx.ExecContext(ctx, `
			UPDATE capacity_units SET reserved = reserved + $1, version = version + 1, updated_at = NOW()
			WHERE id = $2
		`, delta, u.ID); err != nil {
			return fmt.Errorf("failed to update capacity unit: %w", err)
		}
	}
	return tx.Commit()
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

// SetWindowCapacities sets the capacity of exact windows of one resource in a
// single transaction: an existing window is updated, a missing one created,
// and capacity 0 removes it. Lowering a window below what is already reserved
// in it, or overlapping a window with different boundaries, is refused with
// ErrCapacityConflict and nothing changes.
func (r *CapacityRepository) SetWindowCapacities(ctx context.Context, inventoryID string, windows []domain.CapacityWindow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin capacity transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	for _, w := range windows {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, start_at, end_at, reserved FROM capacity_units
			WHERE inventory_id = $1 AND start_at < $3 AND end_at > $2 FOR UPDATE
		`, inventoryID, toTime(w.StartAt), toTime(w.EndAt))
		if err != nil {
			return fmt.Errorf("failed to lock capacity window: %w", err)
		}
		type existing struct {
			id         string
			start, end time.Time
			reserved   int64
		}
		var found []existing
		for rows.Next() {
			var e existing
			if err := rows.Scan(&e.id, &e.start, &e.end, &e.reserved); err != nil {
				rows.Close()
				return err
			}
			found = append(found, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		date := time.Unix(w.StartAt, 0).UTC().Format("2006-01-02")
		switch {
		case len(found) > 1 || (len(found) == 1 && (found[0].start.Unix() != w.StartAt || found[0].end.Unix() != w.EndAt)):
			return fmt.Errorf("%w: %s overlaps a capacity window with other boundaries", domain.ErrCapacityConflict, date)
		case len(found) == 1 && w.Capacity < found[0].reserved:
			return fmt.Errorf("%w: %s already has %d booked", domain.ErrCapacityConflict, date, found[0].reserved)
		case len(found) == 1 && w.Capacity == 0:
			if _, err := tx.ExecContext(ctx, `DELETE FROM capacity_units WHERE id = $1`, found[0].id); err != nil {
				return fmt.Errorf("failed to close capacity window: %w", err)
			}
		case len(found) == 1:
			if _, err := tx.ExecContext(ctx, `
				UPDATE capacity_units SET capacity = $1, version = version + 1, updated_at = $2 WHERE id = $3
			`, w.Capacity, now, found[0].id); err != nil {
				return fmt.Errorf("failed to update capacity window: %w", err)
			}
		case w.Capacity > 0:
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO capacity_units (id, inventory_id, start_at, end_at, capacity, reserved, version, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, 0, 1, $6, $6)
			`, domain.NewID(), inventoryID, toTime(w.StartAt), toTime(w.EndAt), w.Capacity, now); err != nil {
				return fmt.Errorf("failed to create capacity window: %w", err)
			}
		}
	}
	return tx.Commit()
}

// MaxWindowCapacityFrom returns the largest capacity of the resource's
// windows ending after from (0 when none).
func (r *CapacityRepository) MaxWindowCapacityFrom(ctx context.Context, inventoryID string, from int64) (int64, error) {
	var max sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `
		SELECT MAX(capacity) FROM capacity_units WHERE inventory_id = $1 AND end_at > $2
	`, inventoryID, toTime(from)).Scan(&max); err != nil {
		return 0, fmt.Errorf("failed to read capacity: %w", err)
	}
	return max.Int64, nil
}
