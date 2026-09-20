package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// LocationRepository implements domain.LocationRepository using PostgreSQL.
type LocationRepository struct {
	db *sql.DB
}

// NewLocationRepository creates a new LocationRepository.
func NewLocationRepository(db *sql.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

// Create inserts a new location into the database.
func (r *LocationRepository) Create(ctx context.Context, location *domain.Location) error {
	query := `INSERT INTO locations (id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var parentID, province, city, neighborhood, timezone sql.NullString
	if location.ParentID != nil {
		parentID = sql.NullString{String: *location.ParentID, Valid: true}
	}
	if location.Province != nil {
		province = sql.NullString{String: *location.Province, Valid: true}
	}
	if location.City != nil {
		city = sql.NullString{String: *location.City, Valid: true}
	}
	if location.Neighborhood != nil {
		neighborhood = sql.NullString{String: *location.Neighborhood, Valid: true}
	}
	if location.Timezone != nil {
		timezone = sql.NullString{String: *location.Timezone, Valid: true}
	}

	var lat, lng sql.NullFloat64
	if location.Latitude != nil {
		lat = sql.NullFloat64{Float64: *location.Latitude, Valid: true}
	}
	if location.Longitude != nil {
		lng = sql.NullFloat64{Float64: *location.Longitude, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, location.ID, parentID, location.Name, location.Type, location.Country, province, city, neighborhood, lat, lng, timezone, toTime(location.CreatedAt), toTime(location.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}
	return nil
}

// FindByID retrieves a location by ID.
func (r *LocationRepository) FindByID(ctx context.Context, id string) (*domain.Location, error) {
	query := `SELECT id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone, created_at, updated_at FROM locations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u locationRow
	var parentID, province, city, neighborhood, timezone sql.NullString
	var lat, lng sql.NullFloat64
	err := row.Scan(&u.ID, &parentID, &u.Name, &u.Type, &u.Country, &province, &city, &neighborhood, &lat, &lng, &timezone, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("location not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find location: %w", err)
	}

	location := &domain.Location{
		ID:        u.ID,
		Name:      u.Name,
		Type:      domain.LocationType(u.Type),
		Country:   u.Country,
		CreatedAt: toUnix(u.CreatedAt),
		UpdatedAt: toUnix(u.UpdatedAt),
	}

	if parentID.Valid {
		pid := parentID.String
		location.ParentID = &pid
	}
	if province.Valid {
		p := province.String
		location.Province = &p
	}
	if city.Valid {
		c := city.String
		location.City = &c
	}
	if neighborhood.Valid {
		n := neighborhood.String
		location.Neighborhood = &n
	}
	if lat.Valid {
		la := lat.Float64
		location.Latitude = &la
	}
	if lng.Valid {
		ln := lng.Float64
		location.Longitude = &ln
	}
	if timezone.Valid {
		tz := timezone.String
		location.Timezone = &tz
	}

	return location, nil
}

// FindByCountryAndCity retrieves locations by country and city.
func (r *LocationRepository) FindByCountryAndCity(ctx context.Context, country, city string) ([]domain.Location, error) {
	query := `SELECT id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone, created_at, updated_at FROM locations WHERE country = $1 AND city = $2`
	rows, err := r.db.QueryContext(ctx, query, country, city)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	var locations []domain.Location
	for rows.Next() {
		var u locationRow
		var parentID, province, cityVal, neighborhood, timezone sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&u.ID, &parentID, &u.Name, &u.Type, &u.Country, &province, &cityVal, &neighborhood, &lat, &lng, &timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}

		location := domain.Location{
			ID:        u.ID,
			Name:      u.Name,
			Type:      domain.LocationType(u.Type),
			Country:   u.Country,
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		if parentID.Valid {
			pid := parentID.String
			location.ParentID = &pid
		}
		if province.Valid {
			p := province.String
			location.Province = &p
		}
		if cityVal.Valid {
			c := cityVal.String
			location.City = &c
		}
		if neighborhood.Valid {
			n := neighborhood.String
			location.Neighborhood = &n
		}
		if lat.Valid {
			la := lat.Float64
			location.Latitude = &la
		}
		if lng.Valid {
			ln := lng.Float64
			location.Longitude = &ln
		}
		if timezone.Valid {
			tz := timezone.String
			location.Timezone = &tz
		}

		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	if locations == nil {
		locations = []domain.Location{}
	}

	return locations, nil
}

// FindByParentID retrieves all locations with a given parent ID.
func (r *LocationRepository) FindByParentID(ctx context.Context, parentID string) ([]domain.Location, error) {
	query := `SELECT id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone, created_at, updated_at FROM locations WHERE parent_id = $1`
	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	var locations []domain.Location
	for rows.Next() {
		var u locationRow
		var pid, province, city, neighborhood, timezone sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&u.ID, &pid, &u.Name, &u.Type, &u.Country, &province, &city, &neighborhood, &lat, &lng, &timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}

		location := domain.Location{
			ID:        u.ID,
			Name:      u.Name,
			Type:      domain.LocationType(u.Type),
			Country:   u.Country,
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		if pid.Valid {
			id := pid.String
			location.ParentID = &id
		}
		if province.Valid {
			p := province.String
			location.Province = &p
		}
		if city.Valid {
			c := city.String
			location.City = &c
		}
		if neighborhood.Valid {
			n := neighborhood.String
			location.Neighborhood = &n
		}
		if lat.Valid {
			la := lat.Float64
			location.Latitude = &la
		}
		if lng.Valid {
			ln := lng.Float64
			location.Longitude = &ln
		}
		if timezone.Valid {
			tz := timezone.String
			location.Timezone = &tz
		}

		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	if locations == nil {
		locations = []domain.Location{}
	}

	return locations, nil
}

// FindAll retrieves all locations.
func (r *LocationRepository) FindAll(ctx context.Context) ([]domain.Location, error) {
	query := `SELECT id, parent_id, name, type, country, province, city, neighborhood, latitude, longitude, timezone, created_at, updated_at FROM locations`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	var locations []domain.Location
	for rows.Next() {
		var u locationRow
		var parentID, province, city, neighborhood, timezone sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&u.ID, &parentID, &u.Name, &u.Type, &u.Country, &province, &city, &neighborhood, &lat, &lng, &timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}

		location := domain.Location{
			ID:        u.ID,
			Name:      u.Name,
			Type:      domain.LocationType(u.Type),
			Country:   u.Country,
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		if parentID.Valid {
			pid := parentID.String
			location.ParentID = &pid
		}
		if province.Valid {
			p := province.String
			location.Province = &p
		}
		if city.Valid {
			c := city.String
			location.City = &c
		}
		if neighborhood.Valid {
			n := neighborhood.String
			location.Neighborhood = &n
		}
		if lat.Valid {
			la := lat.Float64
			location.Latitude = &la
		}
		if lng.Valid {
			ln := lng.Float64
			location.Longitude = &ln
		}
		if timezone.Valid {
			tz := timezone.String
			location.Timezone = &tz
		}

		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	if locations == nil {
		locations = []domain.Location{}
	}

	return locations, nil
}

// Update updates an existing location.
func (r *LocationRepository) Update(ctx context.Context, location *domain.Location) error {
	query := `UPDATE locations SET parent_id = $1, name = $2, type = $3, country = $4, province = $5, city = $6, neighborhood = $7, latitude = $8, longitude = $9, timezone = $10, updated_at = $11 WHERE id = $12`

	var parentID, province, city, neighborhood, timezone sql.NullString
	if location.ParentID != nil {
		parentID = sql.NullString{String: *location.ParentID, Valid: true}
	}
	if location.Province != nil {
		province = sql.NullString{String: *location.Province, Valid: true}
	}
	if location.City != nil {
		city = sql.NullString{String: *location.City, Valid: true}
	}
	if location.Neighborhood != nil {
		neighborhood = sql.NullString{String: *location.Neighborhood, Valid: true}
	}
	if location.Timezone != nil {
		timezone = sql.NullString{String: *location.Timezone, Valid: true}
	}

	var lat, lng sql.NullFloat64
	if location.Latitude != nil {
		lat = sql.NullFloat64{Float64: *location.Latitude, Valid: true}
	}
	if location.Longitude != nil {
		lng = sql.NullFloat64{Float64: *location.Longitude, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, parentID, location.Name, location.Type, location.Country, province, city, neighborhood, lat, lng, timezone, toTime(location.UpdatedAt), location.ID)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	return nil
}

// Delete deletes a location by ID.
func (r *LocationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM locations WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	return nil
}

// locationRow represents a row from the locations table.
type locationRow struct {
	ID        string
	Name      string
	Type      string
	Country   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
