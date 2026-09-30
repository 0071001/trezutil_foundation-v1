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
	ValidateAccessToken(token string) (string, error)  // returns userID; rejects refresh tokens
	ValidateRefreshToken(token string) (string, error) // returns userID; rejects access tokens
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

// ErrConversationForbidden is returned when an actor addresses a conversation
// owned by someone else.
var ErrConversationForbidden = fmt.Errorf("conversation belongs to another user")

// ErrDuplicateMessage is returned when an inbound message with the same
// idempotency key was already recorded for the conversation.
var ErrDuplicateMessage = fmt.Errorf("duplicate message")

// ErrListingAvailabilityNotFound is returned when a listing has no availability record.
var ErrListingAvailabilityNotFound = fmt.Errorf("listing availability not found")

// ErrUserNotFound is returned when a user does not exist.
var ErrUserNotFound = fmt.Errorf("user not found")

// ErrUserSuspended is returned when a suspended user tries to authenticate.
var ErrUserSuspended = fmt.Errorf("user account is suspended")

// ErrOTPRateLimited is returned when too many OTP codes were requested for a phone.
var ErrOTPRateLimited = fmt.Errorf("too many verification codes requested; try again later")

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
	FindByIDForUpdate(ctx context.Context, orderID string) (*Order, error)
	FindByIDAndOrg(ctx context.Context, orgID, orderID string) (*Order, error)
	// FindByCustomerIdempotencyKey returns ErrOrderNotFound when no order matches.
	FindByCustomerIdempotencyKey(ctx context.Context, customerID, idempotencyKey string) (*Order, error)
	// Update persists a single state transition; it fails with ErrOptimisticLock
	// when the stored version is not order.Version-1 (concurrent modification).
	Update(ctx context.Context, order *Order) error
}

// ErrProviderDeclined marks a definitive provider refusal (e.g. card declined):
// retrying the same request cannot succeed, so the payment must fail.
var ErrProviderDeclined = fmt.Errorf("payment declined by provider")

// ErrOrderNotPayable is returned when an order cannot accept a (new) payment:
// it is not awaiting payment, or it already has a live payment.
var ErrOrderNotPayable = fmt.Errorf("order is not awaiting payment")

// ErrListingUnavailable is returned when a listing cannot be ordered or booked
// (e.g. it is not published).
var ErrListingUnavailable = fmt.Errorf("listing is not available")

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
	FindByIDForUpdate(ctx context.Context, id string) (*Reservation, error)
	Update(ctx context.Context, reservation *Reservation) error
	FindByResourceAndPeriod(ctx context.Context, listingID string, startAt, endAt int64) ([]*Reservation, error)
	FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (*Reservation, error)
}

// InventoryRepository defines the interface for inventory persistence.
type InventoryRepository interface {
	Create(ctx context.Context, resource *InventoryResource) error
	FindByID(ctx context.Context, tenantID, id string) (*InventoryResource, error)
	FindByListingID(ctx context.Context, tenantID, listingID string) (*InventoryResource, error)
	FindByListingIDForUpdate(ctx context.Context, tenantID, listingID string) (*InventoryResource, error)
	Update(ctx context.Context, resource *InventoryResource) error
	FindByTenantID(ctx context.Context, tenantID string, offset, limit int) ([]*InventoryResource, error)
	Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error
	Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error
	Consume(ctx context.Context, id string, quantity int64, expectedVersion int64) error
}

// CapacityRepository defines the interface for capacity persistence.
type CapacityRepository interface {
	Create(ctx context.Context, unit *CapacityUnit) error
	FindByID(ctx context.Context, tenantID, id string) (*CapacityUnit, error)
	FindByInventoryID(ctx context.Context, tenantID, inventoryID string) ([]*CapacityUnit, error)
	FindByInventoryIDAndRange(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*CapacityUnit, error)
	FindByInventoryIDAndRangeForUpdate(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*CapacityUnit, error)
	Update(ctx context.Context, unit *CapacityUnit) error
	Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error
	Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error
	ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error
	ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error
}

// FulfillmentRepository defines the interface for fulfillment persistence.
type FulfillmentRepository interface {
	Create(ctx context.Context, fulfillment *Fulfillment) error
	FindByID(ctx context.Context, tenantID, id string) (*Fulfillment, error)
	FindByIDForUpdate(ctx context.Context, tenantID, id string) (*Fulfillment, error)
	FindByOrderID(ctx context.Context, tenantID, orderID string) (*Fulfillment, error)
	FindByReservationID(ctx context.Context, tenantID, reservationID string) (*Fulfillment, error)
	FindByAssignedTo(ctx context.Context, tenantID, actorID string, status *FulfillmentStatus) ([]*Fulfillment, error)
	Update(ctx context.Context, fulfillment *Fulfillment) error
}

// ErrDuplicateMembership is returned when a user is already a member.
var ErrDuplicateMembership = fmt.Errorf("user is already a member")

// ErrListingNotFound is returned when a listing is not found.
var ErrListingNotFound = fmt.Errorf("listing not found")

// Inventory errors
var (
	ErrInsufficientInventory = fmt.Errorf("insufficient inventory available")
	ErrInvalidQuantity       = fmt.Errorf("invalid quantity")
	ErrInvalidAdjustment     = fmt.Errorf("adjustment would violate invariants")
	ErrInventoryNotFound     = fmt.Errorf("inventory not found")
	ErrOptimisticLock        = fmt.Errorf("optimistic lock conflict")
)

// Capacity errors
var (
	ErrInsufficientCapacity    = fmt.Errorf("insufficient capacity available")
	ErrInvalidCapacityQuantity = fmt.Errorf("invalid capacity quantity")
	ErrCapacityNotFound        = fmt.Errorf("capacity unit not found")
)

// Fulfillment errors
var (
	ErrInvalidFulfillmentTransition = fmt.Errorf("invalid fulfillment transition")
	ErrTrackingNotSupported         = fmt.Errorf("tracking reference not supported for this fulfillment type")
	ErrFulfillmentNotFound          = fmt.Errorf("fulfillment not found")
	ErrNotAssigned                  = fmt.Errorf("actor not assigned to this fulfillment")
)

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

	ErrProviderNotFound      = fmt.Errorf("provider not found")
	ErrProviderNotUsable     = fmt.Errorf("provider is not usable")
	ErrNoProvidersAvailable  = fmt.Errorf("no providers available for capability")
	ErrUnsupportedCapability = fmt.Errorf("unsupported capability")

	// Payment errors
	ErrPaymentNotFound              = fmt.Errorf("payment not found")
	ErrPaymentAlreadyExists         = fmt.Errorf("payment already exists")
	ErrIdempotencyConflict          = fmt.Errorf("idempotency key conflict")
	ErrPaymentInvalidTransition     = fmt.Errorf("invalid payment state transition")
	ErrPaymentNotCapturable         = fmt.Errorf("payment is not in a capturable state")
	ErrPaymentNotCancellable        = fmt.Errorf("payment is not in a cancellable state")
	ErrPaymentNotRefundable         = fmt.Errorf("payment is not in a refundable state")
	ErrRefundNotFound               = fmt.Errorf("refund not found")
	ErrRefundAlreadyExists          = fmt.Errorf("refund already exists")
	ErrRefundExceedsRemaining       = fmt.Errorf("refund amount exceeds the refundable balance")
	ErrRefundExceedsCapturedAmount  = fmt.Errorf("refund amount exceeds captured amount")
	ErrRefundInvalidTransition      = fmt.Errorf("invalid refund state transition")
	ErrPaymentOperationNotSupported = fmt.Errorf("payment operation not supported by provider")
	ErrWebhookSignatureInvalid      = fmt.Errorf("webhook signature verification failed")
	ErrWebhookEventDuplicate        = fmt.Errorf("webhook event already processed")
	ErrLedgerAlreadyPosted          = fmt.Errorf("ledger entry already posted")
)

// PaymentRepository defines the interface for payment persistence.
// All query methods are scoped by tenantID to enforce tenant isolation.
type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	FindByID(ctx context.Context, tenantID, paymentID string) (*Payment, error)
	FindByIDForUpdate(ctx context.Context, tenantID, paymentID string) (*Payment, error)
	FindByOrderID(ctx context.Context, tenantID, orderID string) (*Payment, error)
	FindByProviderPaymentID(ctx context.Context, tenantID, providerID, providerPaymentID string) (*Payment, error)
	FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*Payment, error)
	Update(ctx context.Context, payment *Payment) error
}

// RefundRepository defines the interface for refund persistence.
// All query methods are scoped by tenantID to enforce tenant isolation.
type RefundRepository interface {
	Create(ctx context.Context, refund *Refund) error
	FindByID(ctx context.Context, tenantID, refundID string) (*Refund, error)
	FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*Refund, error)
	FindByProviderRefundID(ctx context.Context, tenantID, providerID, providerRefundID string) (*Refund, error)
	FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*Refund, error)
	Update(ctx context.Context, refund *Refund) error
}

// AggregateRefundSummary returns the total refunded amount for a payment.
func AggregateRefundSummary(refunds []*Refund) Money {
	var total Money
	for _, r := range refunds {
		if r.Status == RefundStatusSucceeded || r.Status == RefundStatusPending {
			if total.Currency == "" {
				total.Currency = r.Amount.Currency
			}
			total.AmountMinor += r.Amount.AmountMinor
		}
	}
	return total
}

// WebhookSignatureVerifier defines the interface for verifying payment provider
// webhook signatures. Provider-specific implementations must NOT leak into
// the domain/application core.
type WebhookSignatureVerifier interface {
	Verify(providerCode string, payload []byte, signature string) error
}
