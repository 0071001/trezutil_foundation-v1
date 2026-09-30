package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/domain"
)

type reservationRow struct {
	ID                  string
	OrganizationID      string
	CustomerID          string
	ListingID           string
	Status              string
	StartAt             time.Time
	EndAt               time.Time
	Quantity            int
	PartySize           int
	PricingSnapshotJSON []byte
	IdempotencyKey      sql.NullString
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ReservationRepository struct {
	db *sql.DB
}

func NewReservationRepository(db *sql.DB) *ReservationRepository {
	return &ReservationRepository{db: db}
}

func (r *ReservationRepository) Create(ctx context.Context, reservation *domain.Reservation) error {
	pricingJSON, err := json.Marshal(reservation.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing snapshot: %w", err)
	}

	if reservation.ID == "" {
		query := `INSERT INTO reservations (organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at, capacity_managed)
		          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id`
		err = r.db.QueryRowContext(ctx, query,
			reservation.OrganizationID, reservation.CustomerID, reservation.ListingID,
			string(reservation.Status), toTime(reservation.StartAt), toTime(reservation.EndAt),
			reservation.Quantity, reservation.PartySize, string(pricingJSON),
			nullString(reservation.IdempotencyKey),
			reservation.Version, toTime(reservation.CreatedAt), toTime(reservation.UpdatedAt),
			reservation.CapacityManaged,
		).Scan(&reservation.ID)
		if err != nil {
			return mapReservationInsertError(err)
		}
		return nil
	}

	query := `INSERT INTO reservations (id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at, capacity_managed)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err = r.db.ExecContext(ctx, query,
		reservation.ID, reservation.OrganizationID, reservation.CustomerID, reservation.ListingID,
		string(reservation.Status), toTime(reservation.StartAt), toTime(reservation.EndAt),
		reservation.Quantity, reservation.PartySize, string(pricingJSON),
		nullString(reservation.IdempotencyKey),
		reservation.Version, toTime(reservation.CreatedAt), toTime(reservation.UpdatedAt),
		reservation.CapacityManaged,
	)
	if err != nil {
		return mapReservationInsertError(err)
	}
	return nil
}

// mapReservationInsertError translates constraint violations into domain errors:
// the no_overlapping_reservations exclusion constraint (23P01) is the
// authoritative double-booking guard; a duplicate idempotency key (23505) means
// a concurrent retry already created the reservation.
func mapReservationInsertError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23P01":
			return domain.ErrReservationConflict
		case "23505":
			return domain.ErrIdempotencyConflict
		}
	}
	return fmt.Errorf("failed to create reservation: %w", err)
}

func (r *ReservationRepository) FindByID(ctx context.Context, id string) (*domain.Reservation, error) {
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u reservationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
		&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
		&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reservation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find reservation: %w", err)
	}

	return unmarshalReservation(u), nil
}

// FindByIDForUpdate retrieves a reservation by ID with row-level lock.
func (r *ReservationRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.Reservation, error) {
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations WHERE id = $1 FOR UPDATE`
	row := r.db.QueryRowContext(ctx, query, id)

	var u reservationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
		&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
		&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reservation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find reservation for update: %w", err)
	}

	return unmarshalReservation(u), nil
}

func (r *ReservationRepository) Update(ctx context.Context, reservation *domain.Reservation) error {
	pricingJSON, err := json.Marshal(reservation.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing snapshot: %w", err)
	}

	query := `UPDATE reservations SET
          status = $1, start_at = $2, end_at = $3, quantity = $4, party_size = $5,
          pricing_snapshot = $6, idempotency_key = $7, version = version + 1, updated_at = $8
          WHERE id = $9 AND version = $10`

	result, err := r.db.ExecContext(ctx, query,
		string(reservation.Status), toTime(reservation.StartAt), toTime(reservation.EndAt),
		reservation.Quantity, reservation.PartySize, string(pricingJSON),
		nullString(reservation.IdempotencyKey),
		toTime(reservation.UpdatedAt), reservation.ID, reservation.Version-1,
	)
	if err != nil {
		return fmt.Errorf("failed to update reservation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("reservation not found or version conflict: %s", reservation.ID)
	}

	return nil
}

func (r *ReservationRepository) FindByResourceAndPeriod(ctx context.Context, listingID string, startAt, endAt int64) ([]*domain.Reservation, error) {
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations
	          WHERE listing_id = $1
	            AND status <> 'cancelled'
	            AND start_at < $3
	            AND end_at > $2
	          ORDER BY start_at ASC`

	rows, err := r.db.QueryContext(ctx, query, listingID, toTime(startAt), toTime(endAt))
	if err != nil {
		return nil, fmt.Errorf("failed to query reservations: %w", err)
	}
	defer rows.Close()

	var reservations []*domain.Reservation
	for rows.Next() {
		var u reservationRow
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
			&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
			&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reservation: %w", err)
		}

		reservations = append(reservations, unmarshalReservation(u))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reservations: %w", err)
	}

	if reservations == nil {
		reservations = []*domain.Reservation{}
	}

	return reservations, nil
}

// FindByIdempotencyKey retrieves a reservation by its idempotency key.
func (r *ReservationRepository) FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (*domain.Reservation, error) {
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations WHERE idempotency_key = $1`
	row := r.db.QueryRowContext(ctx, query, idempotencyKey)

	var u reservationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
		&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
		&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find reservation by idempotency key: %w", err)
	}

	return unmarshalReservation(u), nil
}

func unmarshalReservation(u reservationRow) *domain.Reservation {
	reservation := &domain.Reservation{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		CustomerID:     u.CustomerID,
		ListingID:      u.ListingID,
		Status:         domain.ReservationStatus(u.Status),
		StartAt:        toUnix(u.StartAt),
		EndAt:          toUnix(u.EndAt),
		Quantity:       u.Quantity,
		PartySize:      u.PartySize,
		IdempotencyKey: nullStringToString(u.IdempotencyKey),
		Version:        u.Version,
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
	}

	if len(u.PricingSnapshotJSON) > 0 && string(u.PricingSnapshotJSON) != "null" {
		var ps domain.PricingSnapshot
		if err := json.Unmarshal(u.PricingSnapshotJSON, &ps); err == nil {
			reservation.PricingSnapshot = &ps
		}
	}

	return reservation
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullStringToString(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

// FindByCustomerID lists a customer's reservations, most recent start first.
func (r *ReservationRepository) FindByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]*domain.Reservation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations WHERE customer_id = $1 ORDER BY start_at DESC, id LIMIT $2 OFFSET $3`, customerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list reservations: %w", err)
	}
	defer rows.Close()
	var out []*domain.Reservation
	for rows.Next() {
		var u reservationRow
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
			&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
			&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reservation: %w", err)
		}
		out = append(out, unmarshalReservation(u))
	}
	return out, rows.Err()
}

// FindByOrganizationID lists an organization's reservations by start time.
func (r *ReservationRepository) FindByOrganizationID(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Reservation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, idempotency_key, version, created_at, updated_at
	          FROM reservations WHERE organization_id = $1 ORDER BY start_at DESC, id LIMIT $2 OFFSET $3`, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization reservations: %w", err)
	}
	defer rows.Close()
	var out []*domain.Reservation
	for rows.Next() {
		var u reservationRow
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
			&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
			&u.PricingSnapshotJSON, &u.IdempotencyKey, &u.Version, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reservation: %w", err)
		}
		out = append(out, unmarshalReservation(u))
	}
	return out, rows.Err()
}
