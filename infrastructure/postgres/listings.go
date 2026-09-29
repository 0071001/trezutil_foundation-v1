package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingRepository implements domain.ListingRepository using PostgreSQL.
type ListingRepository struct {
	db *sql.DB
}

// NewListingRepository creates a new ListingRepository.
func NewListingRepository(db *sql.DB) *ListingRepository {
	return &ListingRepository{db: db}
}

// Create inserts a new listing into the database.
func (r *ListingRepository) Create(ctx context.Context, listing *domain.Listing) error {
	query := `INSERT INTO listings (id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	var desc sql.NullString
	if listing.Description != nil {
		desc = sql.NullString{String: *listing.Description, Valid: true}
	}

	var metadata sql.NullString
	if listing.Metadata != nil && len(listing.Metadata) > 0 {
		metadata = encodeListingMetadata(listing.Metadata)
	}

	var publishedAt sql.NullTime
	if listing.PublishedAt != nil {
		publishedAt = sql.NullTime{Valid: true, Time: toTime(*listing.PublishedAt)}
	}

	_, err := r.db.ExecContext(ctx, query, listing.ID, listing.OrganizationID, listing.SellerID, listing.StoreID, listing.CategoryID, listing.Title, desc, listing.ListingType, listing.PricingModel, listing.PriceAmount, listing.PriceCurrency, listing.Status, metadata, publishedAt, toTime(listing.CreatedAt), toTime(listing.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create listing: %w", err)
	}
	return nil
}

// FindByID retrieves a listing by ID.
func (r *ListingRepository) FindByID(ctx context.Context, id string) (*domain.Listing, error) {
	query := `SELECT id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at FROM listings WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u listingRow
	var desc, metadata sql.NullString
	var publishedAt sql.NullTime
	err := row.Scan(&u.ID, &u.OrganizationID, &u.SellerID, &u.StoreID, &u.CategoryID, &u.Title, &desc, &u.ListingType, &u.PricingModel, &u.PriceAmount, &u.PriceCurrency, &u.Status, &metadata, &publishedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("listing not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find listing: %w", err)
	}

	listing := &domain.Listing{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		SellerID:       u.SellerID,
		StoreID:        u.StoreID,
		CategoryID:     u.CategoryID,
		Title:          u.Title,
		ListingType:    domain.ListingType(u.ListingType),
		PricingModel:   domain.PricingModel(u.PricingModel),
		PriceAmount:    u.PriceAmount,
		PriceCurrency:  u.PriceCurrency,
		Status:         domain.ListingStatus(u.Status),
		Metadata:       decodeListingMetadata(metadata),
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
	}

	if desc.Valid {
		d := desc.String
		listing.Description = &d
	}
	if publishedAt.Valid {
		ts := toUnix(publishedAt.Time)
		listing.PublishedAt = &ts
	}

	return listing, nil
}

// FindByOrganizationID retrieves all listings for an organization.
func (r *ListingRepository) FindByOrganizationID(ctx context.Context, organizationID string, offset, limit int) ([]domain.Listing, error) {
	query := `SELECT id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at FROM listings WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query listings: %w", err)
	}
	defer rows.Close()

	var listings []domain.Listing
	for rows.Next() {
		var u listingRow
		var desc, metadata sql.NullString
		var publishedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.SellerID, &u.StoreID, &u.CategoryID, &u.Title, &desc, &u.ListingType, &u.PricingModel, &u.PriceAmount, &u.PriceCurrency, &u.Status, &metadata, &publishedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing: %w", err)
		}

		listing := domain.Listing{
			ID:             u.ID,
			OrganizationID: u.OrganizationID,
			SellerID:       u.SellerID,
			StoreID:        u.StoreID,
			CategoryID:     u.CategoryID,
			Title:          u.Title,
			ListingType:    domain.ListingType(u.ListingType),
			PricingModel:   domain.PricingModel(u.PricingModel),
			PriceAmount:    u.PriceAmount,
			PriceCurrency:  u.PriceCurrency,
			Status:         domain.ListingStatus(u.Status),
			Metadata:       decodeListingMetadata(metadata),
			CreatedAt:      toUnix(u.CreatedAt),
			UpdatedAt:      toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			listing.Description = &d
		}
		if publishedAt.Valid {
			ts := toUnix(publishedAt.Time)
			listing.PublishedAt = &ts
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listings: %w", err)
	}

	if listings == nil {
		listings = []domain.Listing{}
	}

	return listings, nil
}

// FindBySellerID retrieves all listings for a seller.
func (r *ListingRepository) FindBySellerID(ctx context.Context, sellerID string, offset, limit int) ([]domain.Listing, error) {
	query := `SELECT id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at FROM listings WHERE seller_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query listings: %w", err)
	}
	defer rows.Close()

	var listings []domain.Listing
	for rows.Next() {
		var u listingRow
		var desc, metadata sql.NullString
		var publishedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.SellerID, &u.StoreID, &u.CategoryID, &u.Title, &desc, &u.ListingType, &u.PricingModel, &u.PriceAmount, &u.PriceCurrency, &u.Status, &metadata, &publishedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing: %w", err)
		}

		listing := domain.Listing{
			ID:             u.ID,
			OrganizationID: u.OrganizationID,
			SellerID:       u.SellerID,
			StoreID:        u.StoreID,
			CategoryID:     u.CategoryID,
			Title:          u.Title,
			ListingType:    domain.ListingType(u.ListingType),
			PricingModel:   domain.PricingModel(u.PricingModel),
			PriceAmount:    u.PriceAmount,
			PriceCurrency:  u.PriceCurrency,
			Status:         domain.ListingStatus(u.Status),
			Metadata:       decodeListingMetadata(metadata),
			CreatedAt:      toUnix(u.CreatedAt),
			UpdatedAt:      toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			listing.Description = &d
		}
		if publishedAt.Valid {
			ts := toUnix(publishedAt.Time)
			listing.PublishedAt = &ts
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listings: %w", err)
	}

	if listings == nil {
		listings = []domain.Listing{}
	}

	return listings, nil
}

// FindByStoreID retrieves all listings for a store.
func (r *ListingRepository) FindByStoreID(ctx context.Context, storeID string, offset, limit int) ([]domain.Listing, error) {
	query := `SELECT id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at FROM listings WHERE store_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, storeID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query listings: %w", err)
	}
	defer rows.Close()

	var listings []domain.Listing
	for rows.Next() {
		var u listingRow
		var desc, metadata sql.NullString
		var publishedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.SellerID, &u.StoreID, &u.CategoryID, &u.Title, &desc, &u.ListingType, &u.PricingModel, &u.PriceAmount, &u.PriceCurrency, &u.Status, &metadata, &publishedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing: %w", err)
		}

		listing := domain.Listing{
			ID:             u.ID,
			OrganizationID: u.OrganizationID,
			SellerID:       u.SellerID,
			StoreID:        u.StoreID,
			CategoryID:     u.CategoryID,
			Title:          u.Title,
			ListingType:    domain.ListingType(u.ListingType),
			PricingModel:   domain.PricingModel(u.PricingModel),
			PriceAmount:    u.PriceAmount,
			PriceCurrency:  u.PriceCurrency,
			Status:         domain.ListingStatus(u.Status),
			Metadata:       decodeListingMetadata(metadata),
			CreatedAt:      toUnix(u.CreatedAt),
			UpdatedAt:      toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			listing.Description = &d
		}
		if publishedAt.Valid {
			ts := toUnix(publishedAt.Time)
			listing.PublishedAt = &ts
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listings: %w", err)
	}

	if listings == nil {
		listings = []domain.Listing{}
	}

	return listings, nil
}

// FindByCategoryID retrieves all listings for a category.
func (r *ListingRepository) FindByCategoryID(ctx context.Context, categoryID string, offset, limit int) ([]domain.Listing, error) {
	query := `SELECT id, organization_id, seller_id, store_id, category_id, title, description, listing_type, pricing_model, price_amount, price_currency, status, metadata, published_at, created_at, updated_at FROM listings WHERE category_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, categoryID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query listings: %w", err)
	}
	defer rows.Close()

	var listings []domain.Listing
	for rows.Next() {
		var u listingRow
		var desc, metadata sql.NullString
		var publishedAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.SellerID, &u.StoreID, &u.CategoryID, &u.Title, &desc, &u.ListingType, &u.PricingModel, &u.PriceAmount, &u.PriceCurrency, &u.Status, &metadata, &publishedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing: %w", err)
		}

		listing := domain.Listing{
			ID:             u.ID,
			OrganizationID: u.OrganizationID,
			SellerID:       u.SellerID,
			StoreID:        u.StoreID,
			CategoryID:     u.CategoryID,
			Title:          u.Title,
			ListingType:    domain.ListingType(u.ListingType),
			PricingModel:   domain.PricingModel(u.PricingModel),
			PriceAmount:    u.PriceAmount,
			PriceCurrency:  u.PriceCurrency,
			Status:         domain.ListingStatus(u.Status),
			Metadata:       decodeListingMetadata(metadata),
			CreatedAt:      toUnix(u.CreatedAt),
			UpdatedAt:      toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			listing.Description = &d
		}
		if publishedAt.Valid {
			ts := toUnix(publishedAt.Time)
			listing.PublishedAt = &ts
		}

		listings = append(listings, listing)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating listings: %w", err)
	}

	if listings == nil {
		listings = []domain.Listing{}
	}

	return listings, nil
}

// Update updates an existing listing.
func (r *ListingRepository) Update(ctx context.Context, listing *domain.Listing) error {
	query := `UPDATE listings SET title = $1, description = $2, listing_type = $3, pricing_model = $4, price_amount = $5, price_currency = $6, status = $7, metadata = $8, published_at = $9, updated_at = $10 WHERE id = $11`

	var desc sql.NullString
	if listing.Description != nil {
		desc = sql.NullString{String: *listing.Description, Valid: true}
	}

	var metadata sql.NullString
	if listing.Metadata != nil && len(listing.Metadata) > 0 {
		metadata = encodeListingMetadata(listing.Metadata)
	}

	var publishedAt sql.NullTime
	if listing.PublishedAt != nil {
		publishedAt = sql.NullTime{Valid: true, Time: toTime(*listing.PublishedAt)}
	}

	_, err := r.db.ExecContext(ctx, query, listing.Title, desc, listing.ListingType, listing.PricingModel, listing.PriceAmount, listing.PriceCurrency, listing.Status, metadata, publishedAt, toTime(listing.UpdatedAt), listing.ID)
	if err != nil {
		return fmt.Errorf("failed to update listing: %w", err)
	}
	return nil
}

// Delete deletes a listing by ID.
func (r *ListingRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM listings WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete listing: %w", err)
	}
	return nil
}

// CountByOrganizationID counts listings for an organization.
func (r *ListingRepository) CountByOrganizationID(ctx context.Context, organizationID string) (int, error) {
	query := `SELECT COUNT(*) FROM listings WHERE organization_id = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, organizationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count listings: %w", err)
	}
	return count, nil
}

// listingRow represents a row from the listings table.
type listingRow struct {
	ID             string
	OrganizationID string
	SellerID       string
	StoreID        string
	CategoryID     string
	Title          string
	ListingType    string
	PricingModel   string
	PriceAmount    float64
	PriceCurrency  string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// encodeListingMetadata serializes listing metadata for the JSONB column.
func encodeListingMetadata(m map[string]interface{}) sql.NullString {
	b, err := json.Marshal(m)
	if err != nil {
		return sql.NullString{String: "{}", Valid: true}
	}
	return sql.NullString{String: string(b), Valid: true}
}

// decodeListingMetadata parses the JSONB column; invalid or empty values yield an empty map.
func decodeListingMetadata(v sql.NullString) map[string]interface{} {
	out := make(map[string]interface{})
	if v.Valid && v.String != "" {
		_ = json.Unmarshal([]byte(v.String), &out)
	}
	return out
}

// ListingChange is a (id, status, updated_at) tuple used by search projection sync.
type ListingChange struct {
	ID        string
	Status    domain.ListingStatus
	UpdatedAt time.Time
}

// ListChangedSince returns listings ordered by (updated_at, id) strictly after
// the given keyset position.
func (r *ListingRepository) ListChangedSince(ctx context.Context, afterUpdatedAt time.Time, afterID string, limit int) ([]ListingChange, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, status, updated_at FROM listings
		WHERE (updated_at, id::text) > ($1, $2)
		ORDER BY updated_at, id::text
		LIMIT $3`, afterUpdatedAt, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list changed listings: %w", err)
	}
	defer rows.Close()
	var out []ListingChange
	for rows.Next() {
		var c ListingChange
		var status string
		if err := rows.Scan(&c.ID, &status, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan listing change: %w", err)
		}
		c.Status = domain.ListingStatus(status)
		out = append(out, c)
	}
	return out, rows.Err()
}
