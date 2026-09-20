package domain

import "time"

// OrganizationType represents the type of organization.
type OrganizationType string

const (
	OrganizationTypeIndividual  OrganizationType = "individual"
	OrganizationTypeBusiness    OrganizationType = "business"
	OrganizationTypeCompany     OrganizationType = "company"
	OrganizationTypeCooperative OrganizationType = "cooperative"
	OrganizationTypeAssociation OrganizationType = "association"
	OrganizationTypeOther       OrganizationType = "other"
)

// OrganizationStatus represents the status of an organization.
type OrganizationStatus string

const (
	OrganizationStatusActive    OrganizationStatus = "active"
	OrganizationStatusSuspended OrganizationStatus = "suspended"
	OrganizationStatusClosed    OrganizationStatus = "closed"
)

// VerificationStatus represents the verification status of an entity.
type VerificationStatus string

const (
	VerificationStatusUnverified VerificationStatus = "unverified"
	VerificationStatusPending    VerificationStatus = "pending"
	VerificationStatusVerified   VerificationStatus = "verified"
	VerificationStatusRejected   VerificationStatus = "rejected"
	VerificationStatusSuspended  VerificationStatus = "suspended"
)

// Organization represents a business entity in the system.
type Organization struct {
	ID                 string
	Type               OrganizationType
	LegalName          string
	DisplayName        string
	Description        *string
	Phone              string
	Email              *string
	Status             OrganizationStatus
	Country            string
	Province           *string
	City               *string
	Address            *string
	VerificationStatus VerificationStatus
	CreatedAt          int64
	UpdatedAt          int64
}

// NewOrganization creates a new Organization.
func NewOrganization(orgType OrganizationType, legalName, displayName, country, phone string) *Organization {
	return &Organization{
		Type:               orgType,
		LegalName:          legalName,
		DisplayName:        displayName,
		Country:            country,
		Phone:              phone,
		Status:             OrganizationStatusActive,
		VerificationStatus: VerificationStatusUnverified,
		CreatedAt:          time.Now().Unix(),
		UpdatedAt:          time.Now().Unix(),
	}
}
