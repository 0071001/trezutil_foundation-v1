package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingAvailabilityRepository implements domain.ListingAvailabilityRepository using PostgreSQL.
type ListingAvailabilityRepository struct {
	db *sql.DB
}

// NewListingAvailabilityRepository creates a new ListingAvailabilityRepository.
func NewListingAvailabilityRepository(db *sql.DB) *ListingAvailabilityRepository {
	return &ListingAvailabilityRepository{db: db}
}

// Create inserts a new listing availability into the database.
func (r *ListingAvailabilityRepository) Create(ctx context.Context, availability *domain.ListingAvailability) error {
	query := `INSERT INTO listing_availability (id, listing_id, status, quantity, reserved_quantity, available_from, available_to, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	var quantity, reservedQuantity sql.NullInt64
	if availability.Quantity != nil {
		quantity = sql.NullInt64{Int64: int64(*availability.Quantity), Valid: true}
	}
	if availability.ReservedQuantity != nil {
		reservedQuantity = sql.NullInt64{Int64: int64(*availability.ReservedQuantity), Valid: true}
	}

	var availableFrom, availableTo sql.NullTime
	if availability.AvailableFrom != nil {
		availableFrom = sql.NullTime{Valid: true, Time: toTime(*availability.AvailableFrom)}
	}
	if availability.AvailableTo != nil {
		availableTo = sql.NullTime{Valid: true, Time: toTime(*availability.AvailableTo)}
	}

	_, err := r.db.ExecContext(ctx, query, availability.ID, availability.ListingID, availability.Status, quantity, reservedQuantity, availableFrom, availableTo, toTime(availability.CreatedAt), toTime(availability.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create listing availability: %w", err)
	}
	return nil
}

// FindByListingID retrieves the availability for a listing.
func (r *ListingAvailabilityRepository) FindByListingID(ctx context.Context, listingID string) (*domain.ListingAvailability, error) {
	query := `SELECT id, listing_id, status, quantity, reserved_quantity, available_from, available_to, created_at, updated_at FROM listing_availability WHERE listing_id = $1`
	row := r.db.QueryRowContext(ctx, query, listingID)

	var u listingAvailabilityRow
	var quantity, reservedQuantity sql.NullInt64
	var availableFrom, availableTo sql.NullTime
	err := row.Scan(&u.ID, &u.ListingID, &u.Status, &quantity, &reservedQuantity, &availableFrom, &availableTo, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w for listing %s", domain.ErrListingAvailabilityNotFound, listingID)
		}
		return nil, fmt.Errorf("failed to find listing availability: %w", err)
	}

	availability := &domain.ListingAvailability{
		ID:        u.ID,
		ListingID: u.ListingID,
		Status:    domain.AvailabilityStatus(u.Status),
		CreatedAt: toUnix(u.CreatedAt),
		UpdatedAt: toUnix(u.UpdatedAt),
	}

	if quantity.Valid {
		q := int(quantity.Int64)
		availability.Quantity = &q
	}
	if reservedQuantity.Valid {
		rq := int(reservedQuantity.Int64)
		availability.ReservedQuantity = &rq
	}
	if availableFrom.Valid {
		af := toUnix(availableFrom.Time)
		availability.AvailableFrom = &af
	}
	if availableTo.Valid {
		at := toUnix(availableTo.Time)
		availability.AvailableTo = &at
	}

	return availability, nil
}

// Update updates an existing listing availability.
func (r *ListingAvailabilityRepository) Update(ctx context.Context, availability *domain.ListingAvailability) error {
	query := `UPDATE listing_availability SET status = $1, quantity = $2, reserved_quantity = $3, available_from = $4, available_to = $5, updated_at = $6 WHERE id = $7`

	var quantity, reservedQuantity sql.NullInt64
	if availability.Quantity != nil {
		quantity = sql.NullInt64{Int64: int64(*availability.Quantity), Valid: true}
	}
	if availability.ReservedQuantity != nil {
		reservedQuantity = sql.NullInt64{Int64: int64(*availability.ReservedQuantity), Valid: true}
	}

	var availableFrom, availableTo sql.NullTime
	if availability.AvailableFrom != nil {
		availableFrom = sql.NullTime{Valid: true, Time: toTime(*availability.AvailableFrom)}
	}
	if availability.AvailableTo != nil {
		availableTo = sql.NullTime{Valid: true, Time: toTime(*availability.AvailableTo)}
	}

	_, err := r.db.ExecContext(ctx, query, availability.Status, quantity, reservedQuantity, availableFrom, availableTo, toTime(availability.UpdatedAt), availability.ID)
	if err != nil {
		return fmt.Errorf("failed to update listing availability: %w", err)
	}
	return nil
}

// Delete deletes a listing availability by ID.
func (r *ListingAvailabilityRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM listing_availability WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete listing availability: %w", err)
	}
	return nil
}

// listingAvailabilityRow represents a row from the listing_availability table.
type listingAvailabilityRow struct {
	ID        string
	ListingID string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
