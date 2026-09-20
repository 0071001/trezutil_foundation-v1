package domain

import "time"

// StoreStatus represents the status of a store.
type StoreStatus string

const (
	StoreStatusDraft     StoreStatus = "draft"
	StoreStatusActive    StoreStatus = "active"
	StoreStatusSuspended StoreStatus = "suspended"
	StoreStatusClosed    StoreStatus = "closed"
)

// Store represents a physical or virtual store belonging to an organization.
type Store struct {
	ID             string
	OrganizationID string
	Name           string
	Slug           string
	Description    *string
	Phone          string
	Email          *string
	Country        string
	Province       *string
	City           *string
	Address        *string
	Latitude       *float64
	Longitude      *float64
	Status         StoreStatus
	CreatedAt      int64
	UpdatedAt      int64
}

// NewStore creates a new Store.
func NewStore(organizationID, name, slug, country, phone string) *Store {
	return &Store{
		OrganizationID: organizationID,
		Name:           name,
		Slug:           slug,
		Country:        country,
		Phone:          phone,
		Status:         StoreStatusActive,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
}
