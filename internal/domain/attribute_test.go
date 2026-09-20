package domain

import (
	"testing"
)

func TestNewAttributeDefinition(t *testing.T) {
	attr := NewAttributeDefinition("cat-1", "Brand", "brand", AttributeTypeText)

	if attr.CategoryID != "cat-1" {
		t.Errorf("Expected CategoryID 'cat-1', got '%s'", attr.CategoryID)
	}
	if attr.Name != "Brand" {
		t.Errorf("Expected Name 'Brand', got '%s'", attr.Name)
	}
	if attr.Key != "brand" {
		t.Errorf("Expected Key 'brand', got '%s'", attr.Key)
	}
	if attr.Type != AttributeTypeText {
		t.Errorf("Expected Type 'text', got '%s'", attr.Type)
	}
	if attr.Searchable {
		t.Error("Expected Searchable to be false")
	}
	if attr.Filterable {
		t.Error("Expected Filterable to be false")
	}
}

func TestNewAttributeValue(t *testing.T) {
	value := NewAttributeValue("listing-1", "attr-1")

	if value.ListingID != "listing-1" {
		t.Errorf("Expected ListingID 'listing-1', got '%s'", value.ListingID)
	}
	if value.AttributeDefinitionID != "attr-1" {
		t.Errorf("Expected AttributeDefinitionID 'attr-1', got '%s'", value.AttributeDefinitionID)
	}
}
