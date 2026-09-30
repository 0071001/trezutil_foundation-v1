package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/domain"
)

// HotelRoomTypeRepository persists the hotel-only facts of room types
// (hotel_room_types, migration 000020).
type HotelRoomTypeRepository struct {
	db *sql.DB
}

// NewHotelRoomTypeRepository creates a HotelRoomTypeRepository.
func NewHotelRoomTypeRepository(db *sql.DB) *HotelRoomTypeRepository {
	return &HotelRoomTypeRepository{db: db}
}

func (r *HotelRoomTypeRepository) Create(ctx context.Context, rt *domain.HotelRoomType) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hotel_room_types (listing_id, organization_id, max_occupancy, bed_configuration, amenities, version, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8)
	`, rt.ListingID, rt.OrganizationID, rt.MaxOccupancy, rt.BedConfiguration, pq.Array(rt.Amenities), rt.Version, toTime(rt.CreatedAt), toTime(rt.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create hotel room type: %w", err)
	}
	return nil
}

const hotelRoomTypeColumns = `listing_id, organization_id, max_occupancy, COALESCE(bed_configuration, ''), amenities, version, created_at, updated_at`

func scanHotelRoomType(scan func(...interface{}) error) (*domain.HotelRoomType, error) {
	var rt domain.HotelRoomType
	var amenities pq.StringArray
	var created, updated time.Time
	if err := scan(&rt.ListingID, &rt.OrganizationID, &rt.MaxOccupancy, &rt.BedConfiguration, &amenities, &rt.Version, &created, &updated); err != nil {
		return nil, err
	}
	rt.Amenities = []string(amenities)
	if rt.Amenities == nil {
		rt.Amenities = []string{}
	}
	rt.CreatedAt, rt.UpdatedAt = toUnix(created), toUnix(updated)
	return &rt, nil
}

func (r *HotelRoomTypeRepository) FindByListingID(ctx context.Context, listingID string) (*domain.HotelRoomType, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+hotelRoomTypeColumns+` FROM hotel_room_types WHERE listing_id = $1`, listingID)
	rt, err := scanHotelRoomType(row.Scan)
	if err == sql.ErrNoRows {
		return nil, domain.ErrHotelRoomTypeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find hotel room type: %w", err)
	}
	return rt, nil
}

func (r *HotelRoomTypeRepository) FindByListingIDs(ctx context.Context, listingIDs []string) (map[string]*domain.HotelRoomType, error) {
	out := make(map[string]*domain.HotelRoomType, len(listingIDs))
	if len(listingIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+hotelRoomTypeColumns+` FROM hotel_room_types WHERE listing_id = ANY($1::uuid[])`, pq.Array(listingIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to list hotel room types: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		rt, err := scanHotelRoomType(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[rt.ListingID] = rt
	}
	return out, rows.Err()
}

// Update applies occupancy, beds and amenities with optimistic locking
// (rt.Version is the new version).
func (r *HotelRoomTypeRepository) Update(ctx context.Context, rt *domain.HotelRoomType) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE hotel_room_types SET max_occupancy = $1, bed_configuration = NULLIF($2, ''), amenities = $3, version = $4, updated_at = $5
		WHERE listing_id = $6 AND version = $7
	`, rt.MaxOccupancy, rt.BedConfiguration, pq.Array(rt.Amenities), rt.Version, toTime(rt.UpdatedAt), rt.ListingID, rt.Version-1)
	if err != nil {
		return fmt.Errorf("failed to update hotel room type: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}
