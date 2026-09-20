package domain

import "time"

// SellerType represents the type of seller.
type SellerType string

const (
	SellerTypeIndividual   SellerType = "individual"
	SellerTypeBusiness     SellerType = "business"
	SellerTypeOrganization SellerType = "organization"
)

// SellerStatus represents the status of a seller profile.
type SellerStatus string

const (
	SellerStatusActive    SellerStatus = "active"
	SellerStatusSuspended SellerStatus = "suspended"
	SellerStatusClosed    SellerStatus = "closed"
)

// SellerProfile represents a seller in the system.
type SellerProfile struct {
	ID                 string
	UserID             *string
	OrganizationID     *string
	SellerType         SellerType
	DisplayName        string
	Bio                *string
	Phone              string
	Email              *string
	Status             SellerStatus
	VerificationStatus VerificationStatus
	TrustLevel         int
	CreatedAt          int64
	UpdatedAt          int64
}

// NewSellerProfile creates a new SellerProfile.
func NewSellerProfile(sellerType SellerType, displayName, phone string, userID, organizationID *string) *SellerProfile {
	return &SellerProfile{
		UserID:             userID,
		OrganizationID:     organizationID,
		SellerType:         sellerType,
		DisplayName:        displayName,
		Phone:              phone,
		Status:             SellerStatusActive,
		VerificationStatus: VerificationStatusUnverified,
		TrustLevel:         0,
		CreatedAt:          time.Now().Unix(),
		UpdatedAt:          time.Now().Unix(),
	}
}
