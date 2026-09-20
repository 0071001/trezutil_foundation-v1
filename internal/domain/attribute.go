package domain

import "time"

// AttributeType represents the data type of an attribute value.
type AttributeType string

const (
	AttributeTypeText        AttributeType = "text"
	AttributeTypeNumber      AttributeType = "number"
	AttributeTypeBoolean     AttributeType = "boolean"
	AttributeTypeDate        AttributeType = "date"
	AttributeTypeURL         AttributeType = "url"
	AttributeTypeSelect      AttributeType = "select"
	AttributeTypeMultiselect AttributeType = "multiselect"
)

// AttributeDefinition represents a definition of a listing attribute.
type AttributeDefinition struct {
	ID           string
	CategoryID   string
	Name         string
	Key          string
	Description  *string
	Type         AttributeType
	Unit         *string
	Required     bool
	Searchable   bool
	Filterable   bool
	Options      *string
	Validation   *string
	DisplayOrder int
	CreatedAt    int64
	UpdatedAt    int64
}

// NewAttributeDefinition creates a new AttributeDefinition.
func NewAttributeDefinition(categoryID, name, key string, attrType AttributeType) *AttributeDefinition {
	return &AttributeDefinition{
		CategoryID:   categoryID,
		Name:         name,
		Key:          key,
		Type:         attrType,
		Searchable:   false,
		Filterable:   false,
		DisplayOrder: 0,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
}

// AttributeValue represents a value for a listing attribute.
type AttributeValue struct {
	ID                    string
	ListingID             string
	AttributeDefinitionID string
	ValueText             *string
	ValueNumber           *float64
	ValueBoolean          *bool
	ValueDate             *int64
	ValueURL              *string
	ValueSelect           *string
	CreatedAt             int64
	UpdatedAt             int64
}

// NewAttributeValue creates a new AttributeValue.
func NewAttributeValue(listingID, attributeDefinitionID string) *AttributeValue {
	return &AttributeValue{
		ListingID:             listingID,
		AttributeDefinitionID: attributeDefinitionID,
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
	}
}
