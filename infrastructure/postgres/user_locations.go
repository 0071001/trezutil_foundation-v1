package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// UserLocationRepository implements domain.UserLocationRepository using PostgreSQL.
type UserLocationRepository struct {
	db *sql.DB
}

// NewUserLocationRepository creates a new UserLocationRepository.
func NewUserLocationRepository(db *sql.DB) *UserLocationRepository {
	return &UserLocationRepository{db: db}
}

// Create inserts a new user location into the database.
func (r *UserLocationRepository) Create(ctx context.Context, ul *domain.UserLocation) error {
	query := `INSERT INTO user_locations (id, user_id, location_id, label, is_default, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var label sql.NullString
	if ul.Label != nil {
		label = sql.NullString{String: *ul.Label, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, ul.ID, ul.UserID, ul.LocationID, label, ul.IsDefault, toTime(ul.CreatedAt), toTime(ul.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create user location: %w", err)
	}
	return nil
}

// FindByUserID retrieves all locations for a user.
func (r *UserLocationRepository) FindByUserID(ctx context.Context, userID string) ([]domain.UserLocation, error) {
	query := `SELECT id, user_id, location_id, label, is_default, created_at, updated_at FROM user_locations WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user locations: %w", err)
	}
	defer rows.Close()

	var locations []domain.UserLocation
	for rows.Next() {
		var u userLocationRow
		var label sql.NullString
		if err := rows.Scan(&u.ID, &u.UserID, &u.LocationID, &label, &u.IsDefault, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user location: %w", err)
		}

		location := domain.UserLocation{
			ID:         u.ID,
			UserID:     u.UserID,
			LocationID: u.LocationID,
			IsDefault:  u.IsDefault,
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
		return nil, fmt.Errorf("error iterating user locations: %w", err)
	}

	if locations == nil {
		locations = []domain.UserLocation{}
	}

	return locations, nil
}

// FindDefaultByUserID retrieves the default location for a user.
func (r *UserLocationRepository) FindDefaultByUserID(ctx context.Context, userID string) (*domain.UserLocation, error) {
	query := `SELECT id, user_id, location_id, label, is_default, created_at, updated_at FROM user_locations WHERE user_id = $1 AND is_default = true LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, userID)

	var u userLocationRow
	var label sql.NullString
	err := row.Scan(&u.ID, &u.UserID, &u.LocationID, &label, &u.IsDefault, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("default user location not found for user %s", userID)
		}
		return nil, fmt.Errorf("failed to find default user location: %w", err)
	}

	location := &domain.UserLocation{
		ID:         u.ID,
		UserID:     u.UserID,
		LocationID: u.LocationID,
		IsDefault:  u.IsDefault,
		CreatedAt:  toUnix(u.CreatedAt),
		UpdatedAt:  toUnix(u.UpdatedAt),
	}

	if label.Valid {
		l := label.String
		location.Label = &l
	}

	return location, nil
}

// Update updates an existing user location.
func (r *UserLocationRepository) Update(ctx context.Context, ul *domain.UserLocation) error {
	query := `UPDATE user_locations SET location_id = $1, label = $2, is_default = $3, updated_at = $4 WHERE id = $5`

	var label sql.NullString
	if ul.Label != nil {
		label = sql.NullString{String: *ul.Label, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, ul.LocationID, label, ul.IsDefault, toTime(ul.UpdatedAt), ul.ID)
	if err != nil {
		return fmt.Errorf("failed to update user location: %w", err)
	}
	return nil
}

// Delete deletes a user location by ID.
func (r *UserLocationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM user_locations WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user location: %w", err)
	}
	return nil
}

// userLocationRow represents a row from the user_locations table.
type userLocationRow struct {
	ID         string
	UserID     string
	LocationID string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
