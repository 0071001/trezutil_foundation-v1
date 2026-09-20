package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// AttributeDefinitionRepository implements domain.AttributeDefinitionRepository using PostgreSQL.
type AttributeDefinitionRepository struct {
	db *sql.DB
}

// NewAttributeDefinitionRepository creates a new AttributeDefinitionRepository.
func NewAttributeDefinitionRepository(db *sql.DB) *AttributeDefinitionRepository {
	return &AttributeDefinitionRepository{db: db}
}

// Create inserts a new attribute definition into the database.
func (r *AttributeDefinitionRepository) Create(ctx context.Context, attr *domain.AttributeDefinition) error {
	query := `INSERT INTO attribute_definitions (id, category_id, name, key, description, type, unit, required, searchable, filterable, options, validation, display_order, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	var desc, unit, options, validation sql.NullString
	if attr.Description != nil {
		desc = sql.NullString{String: *attr.Description, Valid: true}
	}
	if attr.Unit != nil {
		unit = sql.NullString{String: *attr.Unit, Valid: true}
	}
	if attr.Options != nil {
		options = sql.NullString{String: *attr.Options, Valid: true}
	}
	if attr.Validation != nil {
		validation = sql.NullString{String: *attr.Validation, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, attr.ID, attr.CategoryID, attr.Name, attr.Key, desc, attr.Type, unit, attr.Required, attr.Searchable, attr.Filterable, options, validation, attr.DisplayOrder, toTime(attr.CreatedAt), toTime(attr.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create attribute definition: %w", err)
	}
	return nil
}

// FindByID retrieves an attribute definition by ID.
func (r *AttributeDefinitionRepository) FindByID(ctx context.Context, id string) (*domain.AttributeDefinition, error) {
	query := `SELECT id, category_id, name, key, description, type, unit, required, searchable, filterable, options, validation, display_order, created_at, updated_at FROM attribute_definitions WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u attributeDefinitionRow
	var desc, unit, options, validation sql.NullString
	err := row.Scan(&u.ID, &u.CategoryID, &u.Name, &u.Key, &desc, &u.Type, &unit, &u.Required, &u.Searchable, &u.Filterable, &options, &validation, &u.DisplayOrder, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attribute definition not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find attribute definition: %w", err)
	}

	attr := &domain.AttributeDefinition{
		ID:           u.ID,
		CategoryID:   u.CategoryID,
		Name:         u.Name,
		Key:          u.Key,
		Type:         domain.AttributeType(u.Type),
		Required:     u.Required,
		Searchable:   u.Searchable,
		Filterable:   u.Filterable,
		DisplayOrder: u.DisplayOrder,
		CreatedAt:    toUnix(u.CreatedAt),
		UpdatedAt:    toUnix(u.UpdatedAt),
	}

	if desc.Valid {
		d := desc.String
		attr.Description = &d
	}
	if unit.Valid {
		u := unit.String
		attr.Unit = &u
	}
	if options.Valid {
		o := options.String
		attr.Options = &o
	}
	if validation.Valid {
		v := validation.String
		attr.Validation = &v
	}

	return attr, nil
}

// FindByCategoryID retrieves all attribute definitions for a category.
func (r *AttributeDefinitionRepository) FindByCategoryID(ctx context.Context, categoryID string) ([]domain.AttributeDefinition, error) {
	query := `SELECT id, category_id, name, key, description, type, unit, required, searchable, filterable, options, validation, display_order, created_at, updated_at FROM attribute_definitions WHERE category_id = $1 ORDER BY display_order ASC`
	rows, err := r.db.QueryContext(ctx, query, categoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attribute definitions: %w", err)
	}
	defer rows.Close()

	var attrs []domain.AttributeDefinition
	for rows.Next() {
		var u attributeDefinitionRow
		var desc, unit, options, validation sql.NullString
		if err := rows.Scan(&u.ID, &u.CategoryID, &u.Name, &u.Key, &desc, &u.Type, &unit, &u.Required, &u.Searchable, &u.Filterable, &options, &validation, &u.DisplayOrder, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan attribute definition: %w", err)
		}

		attr := domain.AttributeDefinition{
			ID:           u.ID,
			CategoryID:   u.CategoryID,
			Name:         u.Name,
			Key:          u.Key,
			Type:         domain.AttributeType(u.Type),
			Required:     u.Required,
			Searchable:   u.Searchable,
			Filterable:   u.Filterable,
			DisplayOrder: u.DisplayOrder,
			CreatedAt:    toUnix(u.CreatedAt),
			UpdatedAt:    toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			attr.Description = &d
		}
		if unit.Valid {
			u := unit.String
			attr.Unit = &u
		}
		if options.Valid {
			o := options.String
			attr.Options = &o
		}
		if validation.Valid {
			v := validation.String
			attr.Validation = &v
		}

		attrs = append(attrs, attr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attribute definitions: %w", err)
	}

	if attrs == nil {
		attrs = []domain.AttributeDefinition{}
	}

	return attrs, nil
}

// FindByKey retrieves an attribute definition by category ID and key.
func (r *AttributeDefinitionRepository) FindByKey(ctx context.Context, categoryID, key string) (*domain.AttributeDefinition, error) {
	query := `SELECT id, category_id, name, key, description, type, unit, required, searchable, filterable, options, validation, display_order, created_at, updated_at FROM attribute_definitions WHERE category_id = $1 AND key = $2 LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, categoryID, key)

	var u attributeDefinitionRow
	var desc, unit, options, validation sql.NullString
	err := row.Scan(&u.ID, &u.CategoryID, &u.Name, &u.Key, &desc, &u.Type, &unit, &u.Required, &u.Searchable, &u.Filterable, &options, &validation, &u.DisplayOrder, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("attribute definition not found for category %s and key %s", categoryID, key)
		}
		return nil, fmt.Errorf("failed to find attribute definition by key: %w", err)
	}

	attr := &domain.AttributeDefinition{
		ID:           u.ID,
		CategoryID:   u.CategoryID,
		Name:         u.Name,
		Key:          u.Key,
		Type:         domain.AttributeType(u.Type),
		Required:     u.Required,
		Searchable:   u.Searchable,
		Filterable:   u.Filterable,
		DisplayOrder: u.DisplayOrder,
		CreatedAt:    toUnix(u.CreatedAt),
		UpdatedAt:    toUnix(u.UpdatedAt),
	}

	if desc.Valid {
		d := desc.String
		attr.Description = &d
	}
	if unit.Valid {
		u := unit.String
		attr.Unit = &u
	}
	if options.Valid {
		o := options.String
		attr.Options = &o
	}
	if validation.Valid {
		v := validation.String
		attr.Validation = &v
	}

	return attr, nil
}

// Update updates an existing attribute definition.
func (r *AttributeDefinitionRepository) Update(ctx context.Context, attr *domain.AttributeDefinition) error {
	query := `UPDATE attribute_definitions SET name = $1, key = $2, description = $3, type = $4, unit = $5, required = $6, searchable = $7, filterable = $8, options = $9, validation = $10, display_order = $11, updated_at = $12 WHERE id = $13`

	var desc, unit, options, validation sql.NullString
	if attr.Description != nil {
		desc = sql.NullString{String: *attr.Description, Valid: true}
	}
	if attr.Unit != nil {
		unit = sql.NullString{String: *attr.Unit, Valid: true}
	}
	if attr.Options != nil {
		options = sql.NullString{String: *attr.Options, Valid: true}
	}
	if attr.Validation != nil {
		validation = sql.NullString{String: *attr.Validation, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, attr.Name, attr.Key, desc, attr.Type, unit, attr.Required, attr.Searchable, attr.Filterable, options, validation, attr.DisplayOrder, toTime(attr.UpdatedAt), attr.ID)
	if err != nil {
		return fmt.Errorf("failed to update attribute definition: %w", err)
	}
	return nil
}

// Delete deletes an attribute definition by ID.
func (r *AttributeDefinitionRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM attribute_definitions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete attribute definition: %w", err)
	}
	return nil
}

// attributeDefinitionRow represents a row from the attribute_definitions table.
type attributeDefinitionRow struct {
	ID           string
	CategoryID   string
	Name         string
	Key          string
	Type         string
	Required     bool
	Searchable   bool
	Filterable   bool
	DisplayOrder int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
