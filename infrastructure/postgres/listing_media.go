package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingMediaRepository implements domain.ListingMediaRepository using PostgreSQL.
type ListingMediaRepository struct {
	db *sql.DB
}

// NewListingMediaRepository creates a new ListingMediaRepository.
func NewListingMediaRepository(db *sql.DB) *ListingMediaRepository {
	return &ListingMediaRepository{db: db}
}

// Create inserts a new listing media into the database.
func (r *ListingMediaRepository) Create(ctx context.Context, media *domain.ListingMedia) error {
	query := `INSERT INTO listing_media (id, listing_id, type, url, is_primary, sort_order, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.ExecContext(ctx, query, media.ID, media.ListingID, media.Type, media.URL, media.Primary, media.SortOrder, toTime(media.CreatedAt), toTime(media.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create listing media: %w", err)
	}
	return nil
}

// FindByListingID retrieves all media for a listing.
func (r *ListingMediaRepository) FindByListingID(ctx context.Context, listingID string) ([]domain.ListingMedia, error) {
	query := `SELECT id, listing_id, type, url, is_primary, sort_order, created_at, updated_at FROM listing_media WHERE listing_id = $1 ORDER BY sort_order ASC`
	rows, err := r.db.QueryContext(ctx, query, listingID)
	if err != nil {
		return nil, fmt.Errorf("failed to query listing media: %w", err)
	}
	defer rows.Close()

	var mediaList []domain.ListingMedia
	for rows.Next() {
		var u listingMediaRow
		if err := rows.Scan(&u.ID, &u.ListingID, &u.Type, &u.URL, &u.Primary, &u.SortOrder, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing media: %w", err)
		}

		media := domain.ListingMedia{
			ID:        u.ID,
			ListingID: u.ListingID,
			Type:      domain.MediaType(u.Type),
			URL:       u.URL,
			Primary:   u.Primary,
			SortOrder: u.SortOrder,
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		mediaList = append(mediaList, media)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listing media: %w", err)
	}

	if mediaList == nil {
		mediaList = []domain.ListingMedia{}
	}

	return mediaList, nil
}

// Update updates an existing listing media.
func (r *ListingMediaRepository) Update(ctx context.Context, media *domain.ListingMedia) error {
	query := `UPDATE listing_media SET type = $1, url = $2, primary = $3, sort_order = $4, updated_at = $5 WHERE id = $6`
	_, err := r.db.ExecContext(ctx, query, media.Type, media.URL, media.Primary, media.SortOrder, toTime(media.UpdatedAt), media.ID)
	if err != nil {
		return fmt.Errorf("failed to update listing media: %w", err)
	}
	return nil
}

// Delete deletes a listing media by ID.
func (r *ListingMediaRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM listing_media WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete listing media: %w", err)
	}
	return nil
}

// DeleteByListingID deletes all media for a listing.
func (r *ListingMediaRepository) DeleteByListingID(ctx context.Context, listingID string) error {
	query := `DELETE FROM listing_media WHERE listing_id = $1`
	_, err := r.db.ExecContext(ctx, query, listingID)
	if err != nil {
		return fmt.Errorf("failed to delete listing media: %w", err)
	}
	return nil
}

// listingMediaRow represents a row from the listing_media table.
type listingMediaRow struct {
	ID        string
	ListingID string
	Type      string
	URL       string
	Primary   bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}
