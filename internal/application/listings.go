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
	verticals           domain.VerticalResolver
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

// WithVerticals makes new listings resolve their vertical through the vertical
// catalogue, so an explicit vertical is checked against the listing type.
// Without it, a listing's vertical follows its listing type.
func (u *ListingUseCases) WithVerticals(verticals domain.VerticalResolver) *ListingUseCases {
	u.verticals = verticals
	return u
}

// resolveVertical returns the vertical of a new listing: the requested one when
// valid, otherwise the listing type's default (older clients send none).
func (u *ListingUseCases) resolveVertical(listing *domain.Listing) (domain.Vertical, error) {
	if u.verticals != nil {
		return u.verticals.ResolveForListing(listing.Vertical, listing.ListingType)
	}
	if listing.Vertical == "" {
		return domain.DefaultVerticalForListingType(listing.ListingType), nil
	}
	v := domain.NormalizeVertical(string(listing.Vertical))
	if !v.IsWellFormed() {
		return "", fmt.Errorf("%w: unknown vertical %q", domain.ErrInvalidParameter, listing.Vertical)
	}
	return v, nil
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

	vertical, err := u.resolveVertical(listing)
	if err != nil {
		return err
	}
	listing.Vertical = vertical

	// Server-controlled fields: identity and lifecycle are never taken from the
	// client. New listings always start as drafts and go live via PublishListing.
	now := time.Now().Unix()
	listing.ID = domain.NewID()
	listing.Status = domain.ListingStatusDraft
	listing.PublishedAt = nil
	listing.CreatedAt = now
	listing.UpdatedAt = now
	// A promotion exists only through SetListingPromotion, never from client metadata.
	listing.Metadata = domain.StripPromotion(listing.Metadata)

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
				"vertical":        string(listing.Vertical),
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
		// A new price at or above the regular price ends the sale; below it, it is
		// the new sale price.
		if _, marked := listing.Metadata[domain.PromotionMetadataKey]; marked && listing.Promotion() == nil {
			delete(listing.Metadata, domain.PromotionMetadataKey)
		}
	}
	if patch.Metadata != nil {
		// Replaces the seller's own metadata but never creates, edits or removes the promotion.
		meta := domain.StripPromotion(patch.Metadata)
		if promo, ok := listing.Metadata[domain.PromotionMetadataKey]; ok {
			meta[domain.PromotionMetadataKey] = promo
		}
		listing.Metadata = meta
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

// canChangePromotion: a sale can be started or ended while a listing is being
// prepared, live or paused; sold, closed, expired and suspended listings are final.
func canChangePromotion(status domain.ListingStatus) bool {
	switch status {
	case domain.ListingStatusDraft, domain.ListingStatusPendingReview, domain.ListingStatusPublished, domain.ListingStatusPaused:
		return true
	}
	return false
}

// manageableListing loads a listing and checks the actor may manage it.
func (u *ListingUseCases) manageableListing(ctx context.Context, actorID, listingID string) (*domain.Listing, error) {
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
	return listing, nil
}

// SetListingPromotion puts a listing on sale at salePrice, or changes the sale
// price of a listing already on sale. Unlike a plain price edit it works on a
// live listing (no pause needed), because a sale is a short-lived change the
// seller starts while the listing keeps selling. The listing's price becomes the
// sale price, so orders and payments charge exactly what customers see; the
// regular price is remembered so EndListingPromotion can restore it. Existing
// orders keep their pricing snapshot.
func (u *ListingUseCases) SetListingPromotion(ctx context.Context, actorID, listingID string, salePrice float64) (*domain.Listing, error) {
	listing, err := u.manageableListing(ctx, actorID, listingID)
	if err != nil {
		return nil, err
	}
	if !canChangePromotion(listing.Status) {
		return nil, domain.ErrInvalidStatusTransition
	}
	if err := listing.StartPromotion(salePrice); err != nil {
		return nil, err
	}
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return nil, fmt.Errorf("failed to update listing: %w", err)
	}
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: "listing.promotion_started", TargetType: "listing", TargetID: listing.ID})
	}
	return listing, nil
}

// EndListingPromotion ends a sale and restores the regular price. Ending a
// listing's sale when it has none is a no-op.
func (u *ListingUseCases) EndListingPromotion(ctx context.Context, actorID, listingID string) (*domain.Listing, error) {
	listing, err := u.manageableListing(ctx, actorID, listingID)
	if err != nil {
		return nil, err
	}
	if !canChangePromotion(listing.Status) {
		return nil, domain.ErrInvalidStatusTransition
	}
	if !listing.EndPromotion() {
		return listing, nil
	}
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return nil, fmt.Errorf("failed to update listing: %w", err)
	}
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: "listing.promotion_ended", TargetType: "listing", TargetID: listing.ID})
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

// CloseListing takes a listing off sale for good (the seller's "archive"): from published, paused or in
// review to closed, which search drops and nothing reopens. The seller side may do it, or platform staff.
func (u *ListingUseCases) CloseListing(ctx context.Context, actorID string, listingID string) error {
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}
	if !u.canManageListing(ctx, actorID, listing) {
		return domain.ErrInsufficientPermissions
	}
	if listing.Status == domain.ListingStatusClosed {
		return domain.ErrInvalidStatusTransition
	}
	if err := listing.ValidateTransition(domain.ListingStatusClosed); err != nil {
		return err
	}
	listing.MarkAsClosed()
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return fmt.Errorf("failed to close listing: %w", err)
	}
	u.record(ctx, actorID, "listing.closed", listing.ID, nil)
	return nil
}

// SuspendListing takes a listing off the platform (moderation): platform administrators and operators only,
// always with a reason, which goes to the audit trail. Published, paused and in-review listings can be
// suspended; the seller cannot lift it.
func (u *ListingUseCases) SuspendListing(ctx context.Context, actorID string, listingID string, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 || len(reason) > domain.MaxModerationReason {
		return domain.ErrReasonRequired
	}
	if !u.isPlatformStaff(ctx, actorID) {
		return domain.ErrInsufficientPermissions
	}
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}
	if listing.Status == domain.ListingStatusSuspended {
		return domain.ErrInvalidStatusTransition
	}
	if err := listing.ValidateTransition(domain.ListingStatusSuspended); err != nil {
		return err
	}
	from := listing.Status
	listing.Suspend()
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return fmt.Errorf("failed to suspend listing: %w", err)
	}
	u.record(ctx, actorID, "listing.suspended", listing.ID, map[string]interface{}{"reason": reason, "from": string(from), "organization_id": listing.OrganizationID})
	return nil
}

// ReinstateListing lifts a suspension: platform administrators and operators only. The listing comes back
// paused and its seller decides when it goes live again.
func (u *ListingUseCases) ReinstateListing(ctx context.Context, actorID string, listingID string, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > domain.MaxModerationReason {
		return domain.ErrReasonRequired
	}
	if !u.isPlatformStaff(ctx, actorID) {
		return domain.ErrInsufficientPermissions
	}
	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ErrListingNotFound
	}
	if err := listing.Reinstate(); err != nil {
		return err
	}
	if err := u.listingRepo.Update(ctx, listing); err != nil {
		return fmt.Errorf("failed to reinstate listing: %w", err)
	}
	meta := map[string]interface{}{"organization_id": listing.OrganizationID}
	if reason != "" {
		meta["reason"] = reason
	}
	u.record(ctx, actorID, "listing.reinstated", listing.ID, meta)
	return nil
}

// isPlatformStaff: an active platform administrator or operator.
func (u *ListingUseCases) isPlatformStaff(ctx context.Context, actorID string) bool {
	actor, err := u.userRepo.FindByID(ctx, actorID)
	return err == nil && actor.Status == domain.UserStatusActive && actor.HasAnyRole(domain.RoleAdmin, domain.RoleOperator)
}

func (u *ListingUseCases) record(ctx context.Context, actorID, action, listingID string, meta map[string]interface{}) {
	if u.auditLogger == nil {
		return
	}
	u.auditLogger.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: action, TargetType: "listing", TargetID: listingID, Metadata: meta})
}
