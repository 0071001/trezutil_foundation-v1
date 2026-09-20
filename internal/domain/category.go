package domain

import "time"

// CategoryStatus represents the status of a category.
type CategoryStatus string

const (
	CategoryStatusActive   CategoryStatus = "active"
	CategoryStatusInactive CategoryStatus = "inactive"
	CategoryStatusSystem   CategoryStatus = "system"
)

// CategoryScope represents the scope of a category.
type CategoryScope string

const (
	CategoryScopeGlobal CategoryScope = "global"
	CategoryScopeSystem CategoryScope = "system"
	CategoryScopeTenant CategoryScope = "tenant"
)

// Category represents a product or service category.
type Category struct {
	ID          string
	ParentID    *string
	Name        string
	Slug        string
	Description *string
	IconURL     *string
	SortOrder   int
	Status      CategoryStatus
	Scope       CategoryScope
	CreatedAt   int64
	UpdatedAt   int64
}

// NewCategory creates a new Category.
func NewCategory(name, slug string, parentID *string) *Category {
	return &Category{
		ParentID:  parentID,
		Name:      name,
		Slug:      slug,
		Status:    CategoryStatusActive,
		Scope:     CategoryScopeGlobal,
		SortOrder: 0,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
}
