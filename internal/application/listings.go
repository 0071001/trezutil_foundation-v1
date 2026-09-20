package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingUseCases handles listing-related operations.
type ListingUseCases struct {
	listingRepo         domain.ListingRepository
	categoryRepo        domain.CategoryRepository
	attributeDefRepo    domain.AttributeDefinitionRepository
	attributeValRepo    domain.AttributeValueRepository
	mediaRepo           domain.ListingMediaRepository
	availabilityRepo    domain.ListingAvailabilityRepository
	locationRepo        domain.LocationRepository
	listingLocationRepo domain.ListingLocationRepository
	orgRepo             domain.OrganizationRepository
	sellerRepo          domain.SellerRepository
	storeRepo           domain.StoreRepository
	userRepo            domain.UserRepository
	auditLogger         domain.AuditLogger
}

// NewListingUseCases creates a new ListingUseCases.
func NewListingUseCases(
	listingRepo domain.ListingRepository,
	categoryRepo domain.CategoryRepository,
	attributeDefRepo domain.AttributeDefinitionRepository,
	attributeValRepo domain.AttributeValueRepository,
	mediaRepo domain.ListingMediaRepository,
	availabilityRepo domain.ListingAvailabilityRepository,
	locationRepo domain.LocationRepository,
	listingLocationRepo domain.ListingLocationRepository,
	orgRepo domain.OrganizationRepository,
	sellerRepo domain.SellerRepository,
	storeRepo domain.StoreRepository,
	userRepo domain.UserRepository,
	auditLogger domain.AuditLogger,
) *ListingUseCases {
	return &ListingUseCases{
		listingRepo:         listingRepo,
		categoryRepo:        categoryRepo,
		attributeDefRepo:    attributeDefRepo,
		attributeValRepo:    attributeValRepo,
		mediaRepo:           mediaRepo,
		availabilityRepo:    availabilityRepo,
		locationRepo:        locationRepo,
		listingLocationRepo: listingLocationRepo,
		orgRepo:             orgRepo,
		sellerRepo:          sellerRepo,
		storeRepo:           storeRepo,
		userRepo:            userRepo,
		auditLogger:         auditLogger,
	}
}

// CreateListing creates a new listing.
func (u *ListingUseCases) CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error {
	// Validate store exists
	store, err := u.storeRepo.FindByID(ctx, listing.StoreID)
	if err != nil {
		return fmt.Errorf("store not found: %w", err)
	}

	// Derive organization_id from store
	listing.OrganizationID = store.OrganizationID

	// Validate seller exists
	seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return fmt.Errorf("seller not found: %w", err)
	}

	// Verify seller belongs to the same organization as the store
	if seller.OrganizationID != nil && *seller.OrganizationID != store.OrganizationID {
		return domain.ErrInsufficientPermissions
	}

	// Verify actor permission: must be seller owner or org admin/owner/manager
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	hasPermission := false

	// Check if actor is the seller owner
	if seller.UserID != nil && *seller.UserID == actorID {
		hasPermission = true
	}

	// Check if actor is org admin/owner/manager
	if !hasPermission && seller.OrganizationID != nil && *seller.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				hasPermission = true
			}
		}
	}

	// Check if actor is platform admin/operator
	if !hasPermission && (actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		hasPermission = true
	}

	if !hasPermission {
		return domain.ErrInsufficientPermissions
	}

	// Validate category exists
	_, err = u.categoryRepo.FindByID(ctx, listing.CategoryID)
	if err != nil {
		return fmt.Errorf("category not found: %w", err)
	}

	// Set timestamps
	now := time.Now().Unix()
	listing.CreatedAt = now
	listing.UpdatedAt = now

	// Create listing
	err = u.listingRepo.Create(ctx, listing)
	if err != nil {
		return fmt.Errorf("failed to create listing: %w", err)
	}

	// Create attribute values
	for _, attr := range attributes {
		attr.ListingID = listing.ID
		attr.CreatedAt = now
		attr.UpdatedAt = now
		err = u.attributeValRepo.Create(ctx, attr)
		if err != nil {
			return fmt.Errorf("failed to create attribute value: %w", err)
		}
	}

	// Create media
	for i, m := range media {
		m.ListingID = listing.ID
		m.SortOrder = i
		m.CreatedAt = now
		m.UpdatedAt = now
		err = u.mediaRepo.Create(ctx, m)
		if err != nil {
			return fmt.Errorf("failed to create media: %w", err)
		}
	}

	// Create availability
	if availability != nil {
		availability.ListingID = listing.ID
		availability.CreatedAt = now
		availability.UpdatedAt = now
		err = u.availabilityRepo.Create(ctx, availability)
		if err != nil {
			return fmt.Errorf("failed to create availability: %w", err)
		}
	}

	// Create listing locations
	for _, ll := range locations {
		ll.ListingID = listing.ID
		ll.CreatedAt = now
		ll.UpdatedAt = now
		err = u.listingLocationRepo.Create(ctx, ll)
		if err != nil {
			return fmt.Errorf("failed to create listing location: %w", err)
		}
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "listing.created",
			TargetType: "listing",
			TargetID:   listing.ID,
			Metadata: map[string]interface{}{
				"organization_id": listing.OrganizationID,
				"seller_id":       listing.SellerID,
				"store_id":        listing.StoreID,
			},
		})
	}

	return nil
}

// GetListing retrieves a listing by ID.
func (u *ListingUseCases) GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error) {
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, domain.ErrListingNotFound
	}

	// Public listings are visible to everyone
	if listing.IsPublished() {
		return listing, nil
	}

	// For non-published listings, check permissions
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}

	// Get actor
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	// Check if actor is the seller owner
	seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find seller: %w", err)
	}

	if seller.UserID != nil && *seller.UserID == actorID {
		return listing, nil
	}

	// Check if actor is org admin/owner/manager
	if seller.OrganizationID != nil && *seller.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				return listing, nil
			}
		}
	}

	// Check if actor is platform admin/operator
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return listing, nil
	}

	return nil, domain.ErrInsufficientPermissions
}

// PublishListing publishes a listing.
func (u *ListingUseCases) PublishListing(ctx context.Context, actorID string, listingID string) error {
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}

	// Verify actor permission
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return fmt.Errorf("failed to find seller: %w", err)
	}

	hasPermission := false

	// Check if actor is the seller owner
	if seller.UserID != nil && *seller.UserID == actorID {
		hasPermission = true
	}

	// Check if actor is org admin/owner/manager
	if !hasPermission && seller.OrganizationID != nil && *seller.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				hasPermission = true
			}
		}
	}

	// Check if actor is platform admin/operator
	if !hasPermission && (actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		hasPermission = true
	}

	if !hasPermission {
		return domain.ErrInsufficientPermissions
	}

	// Validate transition
	if err := listing.ValidateTransition(domain.ListingStatusPublished); err != nil {
		return err
	}

	listing.Publish()

	err = u.listingRepo.Update(ctx, listing)
	if err != nil {
		return fmt.Errorf("failed to publish listing: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "listing.published",
			TargetType: "listing",
			TargetID:   listing.ID,
		})
	}

	return nil
}

// PauseListing pauses a listing.
func (u *ListingUseCases) PauseListing(ctx context.Context, actorID string, listingID string) error {
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}

	// Verify actor permission
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return fmt.Errorf("failed to find seller: %w", err)
	}

	hasPermission := false

	// Check if actor is the seller owner
	if seller.UserID != nil && *seller.UserID == actorID {
		hasPermission = true
	}

	// Check if actor is org admin/owner/manager
	if !hasPermission && seller.OrganizationID != nil && *seller.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				hasPermission = true
			}
		}
	}

	// Check if actor is platform admin/operator
	if !hasPermission && (actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		hasPermission = true
	}

	if !hasPermission {
		return domain.ErrInsufficientPermissions
	}

	// Validate transition
	if err := listing.ValidateTransition(domain.ListingStatusPaused); err != nil {
		return err
	}

	listing.Pause()

	err = u.listingRepo.Update(ctx, listing)
	if err != nil {
		return fmt.Errorf("failed to pause listing: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "listing.paused",
			TargetType: "listing",
			TargetID:   listing.ID,
		})
	}

	return nil
}

// ResumeListing resumes a paused listing.
func (u *ListingUseCases) ResumeListing(ctx context.Context, actorID string, listingID string) error {
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}

	// Verify actor permission (same as PublishListing)
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return fmt.Errorf("failed to find seller: %w", err)
	}

	hasPermission := false

	if seller.UserID != nil && *seller.UserID == actorID {
		hasPermission = true
	}

	if !hasPermission && seller.OrganizationID != nil && *seller.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				hasPermission = true
			}
		}
	}

	if !hasPermission && (actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		hasPermission = true
	}

	if !hasPermission {
		return domain.ErrInsufficientPermissions
	}

	// Validate transition
	if err := listing.ValidateTransition(domain.ListingStatusPublished); err != nil {
		return err
	}

	listing.Resume()

	err = u.listingRepo.Update(ctx, listing)
	if err != nil {
		return fmt.Errorf("failed to resume listing: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "listing.resumed",
			TargetType: "listing",
			TargetID:   listing.ID,
		})
	}

	return nil
}
