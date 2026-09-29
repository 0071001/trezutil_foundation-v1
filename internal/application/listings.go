package application

import (
	"context"
	"fmt"
	"strings"
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

	// Verify seller belongs to the same organization as the store. A seller
	// without an organization cannot attach listings to any organization's store.
	if seller.OrganizationID == nil || *seller.OrganizationID != store.OrganizationID {
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

	// Server-controlled fields: identity and lifecycle are never taken from the
	// client. New listings always start as drafts and go live via PublishListing.
	now := time.Now().Unix()
	listing.ID = domain.NewID()
	listing.Status = domain.ListingStatusDraft
	listing.PublishedAt = nil
	listing.CreatedAt = now
	listing.UpdatedAt = now
	if listing.Metadata == nil {
		listing.Metadata = make(map[string]interface{})
	}

	// Create listing
	err = u.listingRepo.Create(ctx, listing)
	if err != nil {
		return fmt.Errorf("failed to create listing: %w", err)
	}

	// Create attribute values
	for _, attr := range attributes {
		attr.ID = domain.NewID()
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
		m.ID = domain.NewID()
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
		availability.ID = domain.NewID()
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
		ll.ID = domain.NewID()
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

// ListingMedia returns a listing's media in display order. Callers must have
// resolved the listing through GetListing first (visibility is checked there).
func (u *ListingUseCases) ListingMedia(ctx context.Context, listingID string) ([]domain.ListingMedia, error) {
	return u.mediaRepo.FindByListingID(ctx, listingID)
}

// ListingPatch holds the seller-editable listing fields. Identity, ownership,
// organization, store, seller and lifecycle status are never editable here.
type ListingPatch struct {
	Title       *string
	Description *string
	PriceAmount *float64
	Metadata    map[string]interface{}
}

// UpdateListing applies a patch to a draft, pending-review or paused listing
// (a live listing must be paused first). Existing orders keep their pricing
// snapshot, so a price change never alters what a customer already agreed to.
func (u *ListingUseCases) UpdateListing(ctx context.Context, actorID string, listingID string, patch ListingPatch) (*domain.Listing, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, domain.ErrListingNotFound
	}
	if !u.canManageListing(ctx, actorID, listing) {
		return nil, domain.ErrInsufficientPermissions
	}
	if !listing.CanBeModified() {
		return nil, domain.ErrInvalidStatusTransition
	}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" || len(title) > 255 {
			return nil, fmt.Errorf("%w: title must be 1-255 characters", domain.ErrInvalidParameter)
		}
		listing.Title = title
	}
	if patch.Description != nil {
		d := strings.TrimSpace(*patch.Description)
		listing.Description = &d
	}
	if patch.PriceAmount != nil {
		if *patch.PriceAmount <= 0 || *patch.PriceAmount > 1e9 {
			return nil, fmt.Errorf("%w: price must be positive", domain.ErrInvalidParameter)
		}
		listing.PriceAmount = *patch.PriceAmount
	}
	if patch.Metadata != nil {
		listing.Metadata = patch.Metadata
	}
	listing.UpdatedAt = time.Now().Unix()
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return nil, fmt.Errorf("failed to update listing: %w", err)
	}
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: "listing.updated", TargetType: "listing", TargetID: listing.ID})
	}
	return listing, nil
}

// canManageListing: the seller owner, the organization's owner/admin/manager,
// or a platform admin/operator.
func (u *ListingUseCases) canManageListing(ctx context.Context, actorID string, listing *domain.Listing) bool {
	if seller, err := u.sellerRepo.FindByID(ctx, listing.SellerID); err == nil && seller.UserID != nil && *seller.UserID == actorID {
		return true
	}
	if role, err := u.orgRepo.GetMemberRole(ctx, listing.OrganizationID, actorID); err == nil {
		if role == domain.OrganizationRoleOwner || role == domain.OrganizationRoleAdmin || role == domain.OrganizationRoleManager {
			return true
		}
	}
	if actor, err := u.userRepo.FindByID(ctx, actorID); err == nil && !actor.IsSuspended() &&
		(actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		return true
	}
	return false
}
