package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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

	query := `INSERT INTO reservations (id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, version, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err = r.db.ExecContext(ctx, query,
		reservation.ID, reservation.OrganizationID, reservation.CustomerID, reservation.ListingID,
		string(reservation.Status), toTime(reservation.StartAt), toTime(reservation.EndAt),
		reservation.Quantity, reservation.PartySize, string(pricingJSON),
		reservation.Version, toTime(reservation.CreatedAt), toTime(reservation.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create reservation: %w", err)
	}
	return nil
}

func (r *ReservationRepository) FindByID(ctx context.Context, id string) (*domain.Reservation, error) {
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, version, created_at, updated_at
	          FROM reservations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u reservationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID, &u.ListingID,
		&u.Status, &u.StartAt, &u.EndAt, &u.Quantity, &u.PartySize,
		&u.PricingSnapshotJSON, &u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reservation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find reservation: %w", err)
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
	          pricing_snapshot = $6, version = $7, updated_at = $8
	          WHERE id = $9 AND version = $6`

	result, err := r.db.ExecContext(ctx, query,
		string(reservation.Status), toTime(reservation.StartAt), toTime(reservation.EndAt),
		reservation.Quantity, reservation.PartySize, string(pricingJSON),
		reservation.Version, toTime(reservation.UpdatedAt), reservation.ID,
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
	query := `SELECT id, organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size, pricing_snapshot, version, created_at, updated_at
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
			&u.PricingSnapshotJSON, &u.Version, &u.CreatedAt, &u.UpdatedAt,
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
