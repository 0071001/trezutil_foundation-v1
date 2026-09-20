package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingLocationRepository implements domain.ListingLocationRepository using PostgreSQL.
type ListingLocationRepository struct {
	db *sql.DB
}

// NewListingLocationRepository creates a new ListingLocationRepository.
func NewListingLocationRepository(db *sql.DB) *ListingLocationRepository {
	return &ListingLocationRepository{db: db}
}

// Create inserts a new listing location into the database.
func (r *ListingLocationRepository) Create(ctx context.Context, ll *domain.ListingLocation) error {
	query := `INSERT INTO listing_locations (id, listing_id, location_id, label, is_primary, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var label sql.NullString
	if ll.Label != nil {
		label = sql.NullString{String: *ll.Label, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, ll.ID, ll.ListingID, ll.LocationID, label, ll.IsPrimary, toTime(ll.CreatedAt), toTime(ll.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create listing location: %w", err)
	}
	return nil
}

// FindByListingID retrieves all locations for a listing.
func (r *ListingLocationRepository) FindByListingID(ctx context.Context, listingID string) ([]domain.ListingLocation, error) {
	query := `SELECT id, listing_id, location_id, label, is_primary, created_at, updated_at FROM listing_locations WHERE listing_id = $1`
	rows, err := r.db.QueryContext(ctx, query, listingID)
	if err != nil {
		return nil, fmt.Errorf("failed to query listing locations: %w", err)
	}
	defer rows.Close()

	var locations []domain.ListingLocation
	for rows.Next() {
		var u listingLocationRow
		var label sql.NullString
		if err := rows.Scan(&u.ID, &u.ListingID, &u.LocationID, &label, &u.IsPrimary, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing location: %w", err)
		}

		location := domain.ListingLocation{
			ID:         u.ID,
			ListingID:  u.ListingID,
			LocationID: u.LocationID,
			IsPrimary:  u.IsPrimary,
			CreatedAt:  toUnix(u.CreatedAt),
			UpdatedAt:  toUnix(u.UpdatedAt),
		}

		if label.Valid {
			l := label.String
			location.Label = &l
		}

		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listing locations: %w", err)
	}

	if locations == nil {
		locations = []domain.ListingLocation{}
	}

	return locations, nil
}

// FindPrimaryByListingID retrieves the primary location for a listing.
func (r *ListingLocationRepository) FindPrimaryByListingID(ctx context.Context, listingID string) (*domain.ListingLocation, error) {
	query := `SELECT id, listing_id, location_id, label, is_primary, created_at, updated_at FROM listing_locations WHERE listing_id = $1 AND is_primary = true LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, listingID)

	var u listingLocationRow
	var label sql.NullString
	err := row.Scan(&u.ID, &u.ListingID, &u.LocationID, &label, &u.IsPrimary, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("primary listing location not found for listing %s", listingID)
		}
		return nil, fmt.Errorf("failed to find primary listing location: %w", err)
	}

	location := &domain.ListingLocation{
		ID:         u.ID,
		ListingID:  u.ListingID,
		LocationID: u.LocationID,
		IsPrimary:  u.IsPrimary,
		CreatedAt:  toUnix(u.CreatedAt),
		UpdatedAt:  toUnix(u.UpdatedAt),
	}

	if label.Valid {
		l := label.String
		location.Label = &l
	}

	return location, nil
}

// DeleteByListingID deletes all locations for a listing.
func (r *ListingLocationRepository) DeleteByListingID(ctx context.Context, listingID string) error {
	query := `DELETE FROM listing_locations WHERE listing_id = $1`
	_, err := r.db.ExecContext(ctx, query, listingID)
	if err != nil {
		return fmt.Errorf("failed to delete listing locations: %w", err)
	}
	return nil
}

// listingLocationRow represents a row from the listing_locations table.
type listingLocationRow struct {
	ID         string
	ListingID  string
	LocationID string
	IsPrimary  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
