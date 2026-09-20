package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// CategoryRepository implements domain.CategoryRepository using PostgreSQL.
type CategoryRepository struct {
	db *sql.DB
}

// NewCategoryRepository creates a new CategoryRepository.
func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// Create inserts a new category into the database.
func (r *CategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	query := `INSERT INTO categories (id, parent_id, name, slug, description, icon_url, sort_order, status, scope, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	var parentID sql.NullString
	if category.ParentID != nil {
		parentID = sql.NullString{String: *category.ParentID, Valid: true}
	}

	var desc, iconURL sql.NullString
	if category.Description != nil {
		desc = sql.NullString{String: *category.Description, Valid: true}
	}
	if category.IconURL != nil {
		iconURL = sql.NullString{String: *category.IconURL, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, category.ID, parentID, category.Name, category.Slug, desc, iconURL, category.SortOrder, category.Status, category.Scope, toTime(category.CreatedAt), toTime(category.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	return nil
}

// FindByID retrieves a category by ID.
func (r *CategoryRepository) FindByID(ctx context.Context, id string) (*domain.Category, error) {
	query := `SELECT id, parent_id, name, slug, description, icon_url, sort_order, status, scope, created_at, updated_at FROM categories WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u categoryRow
	var parentID, desc, iconURL sql.NullString
	err := row.Scan(&u.ID, &parentID, &u.Name, &u.Slug, &desc, &iconURL, &u.SortOrder, &u.Status, &u.Scope, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found: %s", id)
		}
		return nil, fmt.Errorf("failed to find category: %w", err)
	}

	category := &domain.Category{
		ID:        u.ID,
		Name:      u.Name,
		Slug:      u.Slug,
		SortOrder: u.SortOrder,
		Status:    domain.CategoryStatus(u.Status),
		Scope:     domain.CategoryScope(u.Scope),
		CreatedAt: toUnix(u.CreatedAt),
		UpdatedAt: toUnix(u.UpdatedAt),
	}

	if parentID.Valid {
		pid := parentID.String
		category.ParentID = &pid
	}
	if desc.Valid {
		d := desc.String
		category.Description = &d
	}
	if iconURL.Valid {
		i := iconURL.String
		category.IconURL = &i
	}

	return category, nil
}

// FindBySlug retrieves a category by slug.
func (r *CategoryRepository) FindBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	query := `SELECT id, parent_id, name, slug, description, icon_url, sort_order, status, scope, created_at, updated_at FROM categories WHERE slug = $1 LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, slug)

	var u categoryRow
	var parentID, desc, iconURL sql.NullString
	err := row.Scan(&u.ID, &parentID, &u.Name, &u.Slug, &desc, &iconURL, &u.SortOrder, &u.Status, &u.Scope, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found with slug: %s", slug)
		}
		return nil, fmt.Errorf("failed to find category by slug: %w", err)
	}

	category := &domain.Category{
		ID:        u.ID,
		Name:      u.Name,
		Slug:      u.Slug,
		SortOrder: u.SortOrder,
		Status:    domain.CategoryStatus(u.Status),
		Scope:     domain.CategoryScope(u.Scope),
		CreatedAt: toUnix(u.CreatedAt),
		UpdatedAt: toUnix(u.UpdatedAt),
	}

	if parentID.Valid {
		pid := parentID.String
		category.ParentID = &pid
	}
	if desc.Valid {
		d := desc.String
		category.Description = &d
	}
	if iconURL.Valid {
		i := iconURL.String
		category.IconURL = &i
	}

	return category, nil
}

// FindByParentID retrieves all categories with a given parent ID.
func (r *CategoryRepository) FindByParentID(ctx context.Context, parentID string) ([]domain.Category, error) {
	query := `SELECT id, parent_id, name, slug, description, icon_url, sort_order, status, scope, created_at, updated_at FROM categories WHERE parent_id = $1`
	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var u categoryRow
		var pid, desc, iconURL sql.NullString
		if err := rows.Scan(&u.ID, &pid, &u.Name, &u.Slug, &desc, &iconURL, &u.SortOrder, &u.Status, &u.Scope, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		category := domain.Category{
			ID:        u.ID,
			Name:      u.Name,
			Slug:      u.Slug,
			SortOrder: u.SortOrder,
			Status:    domain.CategoryStatus(u.Status),
			Scope:     domain.CategoryScope(u.Scope),
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		if pid.Valid {
			id := pid.String
			category.ParentID = &id
		}
		if desc.Valid {
			d := desc.String
			category.Description = &d
		}
		if iconURL.Valid {
			i := iconURL.String
			category.IconURL = &i
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	if categories == nil {
		categories = []domain.Category{}
	}

	return categories, nil
}

// FindActive retrieves all active categories.
func (r *CategoryRepository) FindActive(ctx context.Context) ([]domain.Category, error) {
	query := `SELECT id, parent_id, name, slug, description, icon_url, sort_order, status, scope, created_at, updated_at FROM categories WHERE status = 'active' ORDER BY sort_order ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active categories: %w", err)
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var u categoryRow
		var parentID, desc, iconURL sql.NullString
		if err := rows.Scan(&u.ID, &parentID, &u.Name, &u.Slug, &desc, &iconURL, &u.SortOrder, &u.Status, &u.Scope, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		category := domain.Category{
			ID:        u.ID,
			Name:      u.Name,
			Slug:      u.Slug,
			SortOrder: u.SortOrder,
			Status:    domain.CategoryStatus(u.Status),
			Scope:     domain.CategoryScope(u.Scope),
			CreatedAt: toUnix(u.CreatedAt),
			UpdatedAt: toUnix(u.UpdatedAt),
		}

		if parentID.Valid {
			pid := parentID.String
			category.ParentID = &pid
		}
		if desc.Valid {
			d := desc.String
			category.Description = &d
		}
		if iconURL.Valid {
			i := iconURL.String
			category.IconURL = &i
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	if categories == nil {
		categories = []domain.Category{}
	}

	return categories, nil
}

// Update updates an existing category.
func (r *CategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	query := `UPDATE categories SET parent_id = $1, name = $2, slug = $3, description = $4, icon_url = $5, sort_order = $6, status = $7, scope = $8, updated_at = $9 WHERE id = $10`

	var parentID, desc, iconURL sql.NullString
	if category.ParentID != nil {
		parentID = sql.NullString{String: *category.ParentID, Valid: true}
	}
	if category.Description != nil {
		desc = sql.NullString{String: *category.Description, Valid: true}
	}
	if category.IconURL != nil {
		iconURL = sql.NullString{String: *category.IconURL, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, parentID, category.Name, category.Slug, desc, iconURL, category.SortOrder, category.Status, category.Scope, toTime(category.UpdatedAt), category.ID)
	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

// Delete deletes a category by ID.
func (r *CategoryRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM categories WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	return nil
}

// categoryRow represents a row from the categories table.
type categoryRow struct {
	ID        string
	Name      string
	Slug      string
	SortOrder int
	Status    string
	Scope     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
