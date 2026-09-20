package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// AttributeValueRepository implements domain.AttributeValueRepository using PostgreSQL.
type AttributeValueRepository struct {
	db *sql.DB
}

// NewAttributeValueRepository creates a new AttributeValueRepository.
func NewAttributeValueRepository(db *sql.DB) *AttributeValueRepository {
	return &AttributeValueRepository{db: db}
}

// Create inserts a new attribute value into the database.
func (r *AttributeValueRepository) Create(ctx context.Context, value *domain.AttributeValue) error {
	query := `INSERT INTO listing_attributes (id, listing_id, attribute_definition_id, value_text, value_number, value_boolean, value_date, value_url, value_select, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	var valueText, valueURL, valueSelect sql.NullString
	if value.ValueText != nil {
		valueText = sql.NullString{String: *value.ValueText, Valid: true}
	}
	if value.ValueURL != nil {
		valueURL = sql.NullString{String: *value.ValueURL, Valid: true}
	}
	if value.ValueSelect != nil {
		valueSelect = sql.NullString{String: *value.ValueSelect, Valid: true}
	}

	var valueNumber sql.NullFloat64
	if value.ValueNumber != nil {
		valueNumber = sql.NullFloat64{Float64: *value.ValueNumber, Valid: true}
	}

	var valueBoolean sql.NullBool
	if value.ValueBoolean != nil {
		valueBoolean = sql.NullBool{Bool: *value.ValueBoolean, Valid: true}
	}

	var valueDate sql.NullTime
	if value.ValueDate != nil {
		valueDate = sql.NullTime{Valid: true, Time: toTime(*value.ValueDate)}
	}

	_, err := r.db.ExecContext(ctx, query, value.ID, value.ListingID, value.AttributeDefinitionID, valueText, valueNumber, valueBoolean, valueDate, valueURL, valueSelect, toTime(value.CreatedAt), toTime(value.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create attribute value: %w", err)
	}
	return nil
}

// FindByListingID retrieves all attribute values for a listing.
func (r *AttributeValueRepository) FindByListingID(ctx context.Context, listingID string) ([]domain.AttributeValue, error) {
	query := `SELECT id, listing_id, attribute_definition_id, value_text, value_number, value_boolean, value_date, value_url, value_select, created_at, updated_at FROM listing_attributes WHERE listing_id = $1`
	rows, err := r.db.QueryContext(ctx, query, listingID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attribute values: %w", err)
	}
	defer rows.Close()

	var values []domain.AttributeValue
	for rows.Next() {
		var u attributeValueRow
		var valueText, valueURL, valueSelect sql.NullString
		var valueNumber sql.NullFloat64
		var valueBoolean sql.NullBool
		var valueDate sql.NullTime
		if err := rows.Scan(&u.ID, &u.ListingID, &u.AttributeDefinitionID, &valueText, &valueNumber, &valueBoolean, &valueDate, &valueURL, &valueSelect, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan attribute value: %w", err)
		}

		value := domain.AttributeValue{
			ID:                    u.ID,
			ListingID:             u.ListingID,
			AttributeDefinitionID: u.AttributeDefinitionID,
			CreatedAt:             toUnix(u.CreatedAt),
			UpdatedAt:             toUnix(u.UpdatedAt),
		}

		if valueText.Valid {
			vt := valueText.String
			value.ValueText = &vt
		}
		if valueNumber.Valid {
			vn := valueNumber.Float64
			value.ValueNumber = &vn
		}
		if valueBoolean.Valid {
			vb := valueBoolean.Bool
			value.ValueBoolean = &vb
		}
		if valueDate.Valid {
			vd := toUnix(valueDate.Time)
			value.ValueDate = &vd
		}
		if valueURL.Valid {
			vu := valueURL.String
			value.ValueURL = &vu
		}
		if valueSelect.Valid {
			vs := valueSelect.String
			value.ValueSelect = &vs
		}

		values = append(values, value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attribute values: %w", err)
	}

	if values == nil {
		values = []domain.AttributeValue{}
	}

	return values, nil
}

// FindByListingIDAndKey retrieves an attribute value by listing ID and key.
func (r *AttributeValueRepository) FindByListingIDAndKey(ctx context.Context, listingID, key string) (*domain.AttributeValue, error) {
	query := `SELECT av.id, av.listing_id, av.attribute_definition_id, av.value_text, av.value_number, av.value_boolean, av.value_date, av.value_url, av.value_select, av.created_at, av.updated_at 
	          FROM listing_attributes av 
	          INNER JOIN attribute_definitions ad ON av.attribute_definition_id = ad.id 
	          WHERE av.listing_id = $1 AND ad.key = $2 LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, listingID, key)

	var u attributeValueRow
	var valueText, valueURL, valueSelect sql.NullString
	var valueNumber sql.NullFloat64
	var valueBoolean sql.NullBool
	var valueDate sql.NullTime
	err := row.Scan(&u.ID, &u.ListingID, &u.AttributeDefinitionID, &valueText, &valueNumber, &valueBoolean, &valueDate, &valueURL, &valueSelect, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attribute value not found for listing %s and key %s", listingID, key)
		}
		return nil, fmt.Errorf("failed to find attribute value by key: %w", err)
	}

	value := &domain.AttributeValue{
		ID:                    u.ID,
		ListingID:             u.ListingID,
		AttributeDefinitionID: u.AttributeDefinitionID,
		CreatedAt:             toUnix(u.CreatedAt),
		UpdatedAt:             toUnix(u.UpdatedAt),
	}

	if valueText.Valid {
		vt := valueText.String
		value.ValueText = &vt
	}
	if valueNumber.Valid {
		vn := valueNumber.Float64
		value.ValueNumber = &vn
	}
	if valueBoolean.Valid {
		vb := valueBoolean.Bool
		value.ValueBoolean = &vb
	}
	if valueDate.Valid {
		vd := toUnix(valueDate.Time)
		value.ValueDate = &vd
	}
	if valueURL.Valid {
		vu := valueURL.String
		value.ValueURL = &vu
	}
	if valueSelect.Valid {
		vs := valueSelect.String
		value.ValueSelect = &vs
	}

	return value, nil
}

// Update updates an existing attribute value.
func (r *AttributeValueRepository) Update(ctx context.Context, value *domain.AttributeValue) error {
	query := `UPDATE listing_attributes SET value_text = $1, value_number = $2, value_boolean = $3, value_date = $4, value_url = $5, value_select = $6, updated_at = $7 WHERE id = $8`

	var valueText, valueURL, valueSelect sql.NullString
	if value.ValueText != nil {
		valueText = sql.NullString{String: *value.ValueText, Valid: true}
	}
	if value.ValueURL != nil {
		valueURL = sql.NullString{String: *value.ValueURL, Valid: true}
	}
	if value.ValueSelect != nil {
		valueSelect = sql.NullString{String: *value.ValueSelect, Valid: true}
	}

	var valueNumber sql.NullFloat64
	if value.ValueNumber != nil {
		valueNumber = sql.NullFloat64{Float64: *value.ValueNumber, Valid: true}
	}

	var valueBoolean sql.NullBool
	if value.ValueBoolean != nil {
		valueBoolean = sql.NullBool{Bool: *value.ValueBoolean, Valid: true}
	}

	var valueDate sql.NullTime
	if value.ValueDate != nil {
		valueDate = sql.NullTime{Valid: true, Time: toTime(*value.ValueDate)}
	}

	_, err := r.db.ExecContext(ctx, query, valueText, valueNumber, valueBoolean, valueDate, valueURL, valueSelect, toTime(value.UpdatedAt), value.ID)
	if err != nil {
		return fmt.Errorf("failed to update attribute value: %w", err)
	}
	return nil
}

// DeleteByListingID deletes all attribute values for a listing.
func (r *AttributeValueRepository) DeleteByListingID(ctx context.Context, listingID string) error {
	query := `DELETE FROM listing_attributes WHERE listing_id = $1`
	_, err := r.db.ExecContext(ctx, query, listingID)
	if err != nil {
		return fmt.Errorf("failed to delete attribute values: %w", err)
	}
	return nil
}

// attributeValueRow represents a row from the listing_attributes table.
type attributeValueRow struct {
	ID                    string
	ListingID             string
	AttributeDefinitionID string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
