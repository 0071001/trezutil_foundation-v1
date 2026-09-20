package domain

import (
	"context"
	"fmt"
)

// OTPStore defines the interface for OTP storage (e.g., Redis).
type OTPStore interface {
	Get(ctx context.Context, key string) (*string, error)
	Set(ctx context.Context, key, value string, expiration int64) error
	Delete(ctx context.Context, key string) error
}

// OTPProvider defines the interface for OTP delivery (e.g., SMS, Twilio).
type OTPProvider interface {
	Send(ctx context.Context, destination string, code string) error
}

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	AssignRole(ctx context.Context, userID string, role Role) error
	RemoveRole(ctx context.Context, userID string, role Role) error
	GetUserRoles(ctx context.Context, userID string) ([]Role, error)
	HasRole(ctx context.Context, userID string, role Role) (bool, error)
}

// RoleRepository defines the interface for role persistence.
type RoleRepository interface {
	Exists(ctx context.Context, name string) (bool, error)
	GetAll(ctx context.Context) ([]Role, error)
	GetPermissions(ctx context.Context, role Role) ([]Permission, error)
}

// OrganizationRepository defines the interface for organization persistence.
type OrganizationRepository interface {
	Create(ctx context.Context, org *Organization) error
	FindByID(ctx context.Context, id string) (*Organization, error)
	Update(ctx context.Context, org *Organization) error
	AddMember(ctx context.Context, member *OrganizationMember) error
	RemoveMember(ctx context.Context, organizationID, userID string) error
	GetMembers(ctx context.Context, organizationID string) ([]OrganizationMember, error)
	HasMember(ctx context.Context, organizationID, userID string) (bool, error)
	GetMemberRole(ctx context.Context, organizationID, userID string) (OrganizationRole, error)
}

// SellerRepository defines the interface for seller profile persistence.
type SellerRepository interface {
	Create(ctx context.Context, seller *SellerProfile) error
	FindByID(ctx context.Context, id string) (*SellerProfile, error)
	Update(ctx context.Context, seller *SellerProfile) error
	FindByUserID(ctx context.Context, userID string) (*SellerProfile, error)
	FindByOrganizationID(ctx context.Context, organizationID string) ([]SellerProfile, error)
}

// StoreRepository defines the interface for store persistence.
type StoreRepository interface {
	Create(ctx context.Context, store *Store) error
	FindByID(ctx context.Context, id string) (*Store, error)
	Update(ctx context.Context, store *Store) error
	FindByOrganizationID(ctx context.Context, organizationID string) ([]Store, error)
}

// TokenService defines the interface for JWT token operations.
type TokenService interface {
	IssueAccessToken(userID string, roles []Role) (string, error)
	IssueRefreshToken(userID string) (string, error)
	ValidateAccessToken(token string) (string, error) // returns userID
	RotateRefreshToken(oldToken string) (string, error)
}

// CategoryRepository defines the interface for category persistence.
type CategoryRepository interface {
	Create(ctx context.Context, category *Category) error
	FindByID(ctx context.Context, id string) (*Category, error)
	FindBySlug(ctx context.Context, slug string) (*Category, error)
	FindByParentID(ctx context.Context, parentID string) ([]Category, error)
	FindActive(ctx context.Context) ([]Category, error)
	Update(ctx context.Context, category *Category) error
	Delete(ctx context.Context, id string) error
}

// AttributeDefinitionRepository defines the interface for attribute definition persistence.
type AttributeDefinitionRepository interface {
	Create(ctx context.Context, attr *AttributeDefinition) error
	FindByID(ctx context.Context, id string) (*AttributeDefinition, error)
	FindByCategoryID(ctx context.Context, categoryID string) ([]AttributeDefinition, error)
	FindByKey(ctx context.Context, categoryID, key string) (*AttributeDefinition, error)
	Update(ctx context.Context, attr *AttributeDefinition) error
	Delete(ctx context.Context, id string) error
}

// AttributeValueRepository defines the interface for attribute value persistence.
type AttributeValueRepository interface {
	Create(ctx context.Context, value *AttributeValue) error
	FindByListingID(ctx context.Context, listingID string) ([]AttributeValue, error)
	FindByListingIDAndKey(ctx context.Context, listingID, key string) (*AttributeValue, error)
	Update(ctx context.Context, value *AttributeValue) error
	DeleteByListingID(ctx context.Context, listingID string) error
}

// ListingMediaRepository defines the interface for listing media persistence.
type ListingMediaRepository interface {
	Create(ctx context.Context, media *ListingMedia) error
	FindByListingID(ctx context.Context, listingID string) ([]ListingMedia, error)
	Update(ctx context.Context, media *ListingMedia) error
	Delete(ctx context.Context, id string) error
	DeleteByListingID(ctx context.Context, listingID string) error
}

// ListingAvailabilityRepository defines the interface for listing availability persistence.
type ListingAvailabilityRepository interface {
	Create(ctx context.Context, availability *ListingAvailability) error
	FindByListingID(ctx context.Context, listingID string) (*ListingAvailability, error)
	Update(ctx context.Context, availability *ListingAvailability) error
	Delete(ctx context.Context, id string) error
}

// LocationRepository defines the interface for location persistence.
type LocationRepository interface {
	Create(ctx context.Context, location *Location) error
	FindByID(ctx context.Context, id string) (*Location, error)
	FindByCountryAndCity(ctx context.Context, country, city string) ([]Location, error)
	FindByParentID(ctx context.Context, parentID string) ([]Location, error)
	FindAll(ctx context.Context) ([]Location, error)
	Update(ctx context.Context, location *Location) error
	Delete(ctx context.Context, id string) error
}

// ListingLocationRepository defines the interface for listing location persistence.
type ListingLocationRepository interface {
	Create(ctx context.Context, ll *ListingLocation) error
	FindByListingID(ctx context.Context, listingID string) ([]ListingLocation, error)
	FindPrimaryByListingID(ctx context.Context, listingID string) (*ListingLocation, error)
	DeleteByListingID(ctx context.Context, listingID string) error
}

// UserLocationRepository defines the interface for user location persistence.
type UserLocationRepository interface {
	Create(ctx context.Context, ul *UserLocation) error
	FindByUserID(ctx context.Context, userID string) ([]UserLocation, error)
	FindDefaultByUserID(ctx context.Context, userID string) (*UserLocation, error)
	Update(ctx context.Context, ul *UserLocation) error
	Delete(ctx context.Context, id string) error
}

// ListingRepository defines the interface for listing persistence.
type ListingRepository interface {
	Create(ctx context.Context, listing *Listing) error
	FindByID(ctx context.Context, id string) (*Listing, error)
	FindByOrganizationID(ctx context.Context, organizationID string, offset, limit int) ([]Listing, error)
	FindBySellerID(ctx context.Context, sellerID string, offset, limit int) ([]Listing, error)
	FindByStoreID(ctx context.Context, storeID string, offset, limit int) ([]Listing, error)
	FindByCategoryID(ctx context.Context, categoryID string, offset, limit int) ([]Listing, error)
	Update(ctx context.Context, listing *Listing) error
	Delete(ctx context.Context, id string) error
	CountByOrganizationID(ctx context.Context, organizationID string) (int, error)
}

// AuditLogger defines the interface for audit logging.
type AuditLogger interface {
	Record(ctx context.Context, entry AuditEntry) error
}

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]interface{}
	RequestID  string
	OccurredAt int64
}

// PhoneNormalizer defines the interface for phone number normalization.
type PhoneNormalizer interface {
	Normalize(phone string) (string, error)
}

// RateLimiter defines the interface for rate limiting.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// ErrInvalidParameter is returned when a required parameter is missing or invalid.
var ErrInvalidParameter = fmt.Errorf("invalid parameter")

// ErrInvalidOTP is returned when an OTP code is invalid.
var ErrInvalidOTP = fmt.Errorf("OTP is invalid or expired")

// ErrOTPExpired is returned when an OTP code has expired.
var ErrOTPExpired = fmt.Errorf("OTP is invalid or expired")

// ErrInsufficientPermissions is returned when a user lacks required permissions.
var ErrInsufficientPermissions = fmt.Errorf("insufficient permissions")

// ErrRoleAlreadyAssigned is returned when a role is already assigned to a user.
var ErrRoleAlreadyAssigned = fmt.Errorf("role already assigned to user")

// ErrRoleNotAssigned is returned when a role is not assigned to a user.
var ErrRoleNotAssigned = fmt.Errorf("role not assigned to user")

// ErrOrganizationNotFound is returned when an organization is not found.
var ErrOrganizationNotFound = fmt.Errorf("organization not found")

// ErrStoreNotFound is returned when a store is not found.
var ErrStoreNotFound = fmt.Errorf("store not found")

// ErrSellerNotFound is returned when a seller profile is not found.
var ErrSellerNotFound = fmt.Errorf("seller not found")

// ErrOrderNotFound is returned when an order is not found.
var ErrOrderNotFound = fmt.Errorf("order not found")

// ErrOrderCannotBeCancelled is returned when an order cannot be cancelled from its current status.
var ErrOrderCannotBeCancelled = fmt.Errorf("order cannot be cancelled from current status")

// OrderRepository defines the interface for order persistence.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, orderID string) (*Order, error)
	Update(ctx context.Context, order *Order) error
}

// ErrReservationNotFound is returned when a reservation is not found.
var ErrReservationNotFound = fmt.Errorf("reservation not found")

// ErrReservationConflict is returned when a reservation conflicts with existing availability.
var ErrReservationConflict = fmt.Errorf("reservation conflict: requested period is not available")

// ErrReservationCannotBeCancelled is returned when a reservation cannot be cancelled.
var ErrReservationCannotBeCancelled = fmt.Errorf("reservation cannot be cancelled from current status")

// ErrAvailabilityUnavailable is returned when availability check fails.
var ErrAvailabilityUnavailable = fmt.Errorf("availability unavailable")

// ReservationRepository defines the interface for reservation persistence.
type ReservationRepository interface {
	Create(ctx context.Context, reservation *Reservation) error
	FindByID(ctx context.Context, id string) (*Reservation, error)
	Update(ctx context.Context, reservation *Reservation) error
	FindByResourceAndPeriod(ctx context.Context, listingID string, startAt, endAt int64) ([]*Reservation, error)
}

// ErrDuplicateMembership is returned when a user is already a member.
var ErrDuplicateMembership = fmt.Errorf("user is already a member")

// ErrListingNotFound is returned when a listing is not found.
var ErrListingNotFound = fmt.Errorf("listing not found")

// ErrNotImplemented is returned when a feature is not yet implemented.
var ErrNotImplemented = fmt.Errorf("not implemented")

// ErrCategoryNotFound is returned when a category is not found.
var ErrCategoryNotFound = fmt.Errorf("category not found")

// Usage engine errors
var (
	ErrInsufficientCredits      = fmt.Errorf("insufficient credits")
	ErrUsagePolicyNotFound      = fmt.Errorf("usage policy not found")
	ErrUsageReservationNotFound = fmt.Errorf("usage reservation not found")
	ErrUsageReservationConflict = fmt.Errorf("usage reservation conflict")
	ErrUsageAlreadyConsumed     = fmt.Errorf("usage already consumed")
	ErrUsageAlreadyReleased     = fmt.Errorf("usage already released")
	ErrUsageAccountNotFound     = fmt.Errorf("usage account not found")
	ErrUsagePlanNotFound        = fmt.Errorf("usage plan not found")
	ErrDuplicateEvent           = fmt.Errorf("duplicate webhook event")
)
