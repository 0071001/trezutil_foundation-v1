package domain

import (
	"testing"
)

func TestNewCategory(t *testing.T) {
	category := NewCategory("Electronics", "electronics", nil)

	if category.Name != "Electronics" {
		t.Errorf("Expected Name 'Electronics', got '%s'", category.Name)
	}
	if category.Slug != "electronics" {
		t.Errorf("Expected Slug 'electronics', got '%s'", category.Slug)
	}
	if category.Status != CategoryStatusActive {
		t.Errorf("Expected Status 'active', got '%s'", category.Status)
	}
	if category.Scope != CategoryScopeGlobal {
		t.Errorf("Expected Scope 'global', got '%s'", category.Scope)
	}
	if category.SortOrder != 0 {
		t.Errorf("Expected SortOrder 0, got %d", category.SortOrder)
	}
}

func TestNewCategoryWithParent(t *testing.T) {
	parentID := "parent-1"
	category := NewCategory("Smartphones", "smartphones", &parentID)

	if category.ParentID == nil || *category.ParentID != "parent-1" {
		t.Errorf("Expected ParentID 'parent-1', got %v", category.ParentID)
	}
}
