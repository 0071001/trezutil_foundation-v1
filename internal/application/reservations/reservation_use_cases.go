package reservations

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ReservationUseCases defines the interface for reservation operations.
type ReservationUseCases interface {
	CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error)
	GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error)
	CancelReservation(ctx context.Context, actorID string, reservationID string) error
	ModifyReservation(ctx context.Context, actorID string, reservationID string, startAt int64, endAt int64, quantity int, partySize int) (*domain.Reservation, error)
}

// ReservationUseCasesImpl implements ReservationUseCases.
type ReservationUseCasesImpl struct {
	reservationRepo    domain.ReservationRepository
	listingRepo        domain.ListingRepository
	sellerRepo         domain.SellerRepository
	userRepo           domain.UserRepository
	orgRepo            domain.OrganizationRepository
	provider           domain.AvailabilityProvider
	capacityRepo       domain.CapacityRepository
	inventoryRepo      domain.InventoryRepository
	fulfillmentRepo    domain.FulfillmentRepository
	auditLogger        domain.AuditLogger
	confirmationPolicy ConfirmationPolicy
}

// ConfirmationPolicy defines rules for reservation confirmation.
type ConfirmationPolicy struct {
	RequiresConfirmation bool
	AutoConfirm          bool
}

// NewReservationUseCases creates a new ReservationUseCases.
func NewReservationUseCases(
	reservationRepo domain.ReservationRepository,
	listingRepo domain.ListingRepository,
	sellerRepo domain.SellerRepository,
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	provider domain.AvailabilityProvider,
	capacityRepo domain.CapacityRepository,
	inventoryRepo domain.InventoryRepository,
	fulfillmentRepo domain.FulfillmentRepository,
	auditLogger domain.AuditLogger,
	confirmationPolicy ConfirmationPolicy,
) *ReservationUseCasesImpl {
	return &ReservationUseCasesImpl{
		reservationRepo:    reservationRepo,
		listingRepo:        listingRepo,
		sellerRepo:         sellerRepo,
		userRepo:           userRepo,
		orgRepo:            orgRepo,
		provider:           provider,
		capacityRepo:       capacityRepo,
		inventoryRepo:      inventoryRepo,
		fulfillmentRepo:    fulfillmentRepo,
		auditLogger:        auditLogger,
		confirmationPolicy: confirmationPolicy,
	}
}

// CreateReservation creates a new reservation with availability check and conflict prevention.
func (u *ReservationUseCasesImpl) CreateReservation(
	ctx context.Context,
	actorID string,
	listingID string,
	startAt int64,
	endAt int64,
	quantity int,
	partySize int,
	idempotencyKey string,
) (*domain.Reservation, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if listingID == "" {
		return nil, fmt.Errorf("listing_id is required")
	}
	if startAt == 0 || endAt == 0 {
		return nil, fmt.Errorf("start_at and end_at are required")
	}
	if endAt <= startAt {
		return nil, fmt.Errorf("end_at must be after start_at")
	}
	if quantity <= 0 {
		quantity = 1
	}

	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, domain.ErrListingNotFound
	}

	tenantID := listing.OrganizationID
	if tenantID == "" {
		return nil, fmt.Errorf("listing has no organization")
	}

	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !u.canCreateReservation(ctx, actor, tenantID) {
		return nil, domain.ErrInsufficientPermissions
	}

	availabilityQuery := domain.AvailabilityQuery{
		ListingID: listingID,
		StartAt:   startAt,
		EndAt:     endAt,
		Quantity:  quantity,
		PartySize: partySize,
	}
	result, err := u.provider.CheckAvailability(ctx, availabilityQuery)
	if err != nil {
		return nil, fmt.Errorf("availability check failed: %w", err)
	}
	if !result.Available {
		return nil, domain.ErrAvailabilityUnavailable
	}

	if idempotencyKey != "" {
		existing, err := u.findIdempotentReservation(ctx, actorID, listingID, idempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	now := time.Now().Unix()
	reservation := domain.NewReservation(tenantID, actorID, listingID, startAt, endAt, quantity, partySize, now)
	reservation.IdempotencyKey = idempotencyKey

	// Reserve capacity units if capacity repo is available
	if u.capacityRepo != nil {
		if err := u.capacityRepo.ReserveCapacity(ctx, tenantID, listingID, startAt, endAt, int64(quantity), reservation.ID); err != nil {
			return nil, fmt.Errorf("failed to reserve capacity: %w", err)
		}
	}

	// Reserve inventory if inventory repo is available
	if u.inventoryRepo != nil {
		inv, err := u.inventoryRepo.FindByListingIDForUpdate(ctx, tenantID, listingID)
		if err != nil {
			// If no inventory resource exists, skip (non-inventory-controlled)
			if err != domain.ErrInventoryNotFound {
				return nil, err
			}
		} else {
			if err := inv.Reserve(int64(quantity), now); err != nil {
				return nil, fmt.Errorf("failed to reserve inventory: %w", err)
			}
			if err := u.inventoryRepo.Update(ctx, inv); err != nil {
				return nil, fmt.Errorf("failed to update inventory: %w", err)
			}
		}
	}

	if !u.confirmationPolicy.RequiresConfirmation || u.confirmationPolicy.AutoConfirm {
		if err := reservation.Confirm(now); err != nil {
			return nil, fmt.Errorf("failed to confirm reservation: %w", err)
		}
	}

	if err := u.reservationRepo.Create(ctx, reservation); err != nil {
		return nil, fmt.Errorf("failed to create reservation: %w", err)
	}

	// Create fulfillment if needed for confirmed reservations
	if u.fulfillmentRepo != nil && (reservation.Status == domain.ReservationStatusConfirmed || u.confirmationPolicy.AutoConfirm) {
		ftype := u.inferFulfillmentType(listing)
		if ftype != domain.FulfillmentTypeNone {
			fulfillment := domain.NewFulfillment(
				tenantID,
				nil,
				&reservation.ID,
				ftype,
				now,
			)
			if err := u.fulfillmentRepo.Create(ctx, fulfillment); err != nil {
				return nil, fmt.Errorf("failed to create fulfillment: %w", err)
			}
		}
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "reservation.created",
			TargetType: "reservation",
			TargetID:   reservation.ID,
			Metadata: map[string]interface{}{
				"organization_id": tenantID,
				"listing_id":      listingID,
				"start_at":        startAt,
				"end_at":          endAt,
				"quantity":        quantity,
				"status":          string(reservation.Status),
			},
		})
	}

	return reservation, nil
}

// GetReservation retrieves a reservation by ID with authorization.
func (u *ReservationUseCasesImpl) GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if reservationID == "" {
		return nil, domain.ErrReservationNotFound
	}

	reservation, err := u.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, domain.ErrReservationNotFound
	}

	if !u.canAccessReservation(ctx, reservation, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	return reservation, nil
}

// CancelReservation cancels a reservation by ID.
func (u *ReservationUseCasesImpl) CancelReservation(ctx context.Context, actorID string, reservationID string) error {
	if actorID == "" {
		return domain.ErrInsufficientPermissions
	}
	if reservationID == "" {
		return domain.ErrReservationNotFound
	}

	reservation, err := u.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return domain.ErrReservationNotFound
	}

	if !u.canAccessReservation(ctx, reservation, actorID) {
		return domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	if err := reservation.Cancel(now); err != nil {
		return domain.ErrReservationCannotBeCancelled
	}

	// Release capacity
	if u.capacityRepo != nil {
		if err := u.capacityRepo.ReleaseCapacity(ctx, reservation.OrganizationID, reservation.ListingID, reservation.StartAt, reservation.EndAt, int64(reservation.Quantity), reservation.ID); err != nil {
			return fmt.Errorf("failed to release capacity: %w", err)
		}
	}

	// Release inventory if it was reserved
	if u.inventoryRepo != nil {
		inv, err := u.inventoryRepo.FindByListingIDForUpdate(ctx, reservation.OrganizationID, reservation.ListingID)
		if err == nil {
			if err := inv.Release(int64(reservation.Quantity), now); err != nil {
				return fmt.Errorf("failed to release inventory: %w", err)
			}
			if err := u.inventoryRepo.Update(ctx, inv); err != nil {
				return fmt.Errorf("failed to update inventory: %w", err)
			}
		}
	}

	if err := u.reservationRepo.Update(ctx, reservation); err != nil {
		return fmt.Errorf("failed to cancel reservation: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "reservation.cancelled",
			TargetType: "reservation",
			TargetID:   reservationID,
			Metadata: map[string]interface{}{
				"previous_status": string(reservation.Status),
			},
		})
	}

	return nil
}

// ModifyReservation modifies a reservation's period or quantity.
func (u *ReservationUseCasesImpl) ModifyReservation(
	ctx context.Context,
	actorID string,
	reservationID string,
	startAt int64,
	endAt int64,
	quantity int,
	partySize int,
) (*domain.Reservation, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if reservationID == "" {
		return nil, domain.ErrReservationNotFound
	}

	reservation, err := u.reservationRepo.FindByID(ctx, reservationID)
	if err != nil {
		return nil, domain.ErrReservationNotFound
	}

	if !u.canAccessReservation(ctx, reservation, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	return nil, domain.ErrNotImplemented
}

func (u *ReservationUseCasesImpl) canCreateReservation(ctx context.Context, actor *domain.User, tenantID string) bool {
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}
	memberRole, err := u.orgRepo.GetMemberRole(ctx, tenantID, actor.ID)
	if err == nil {
		if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
			return true
		}
	}
	seller, err := u.sellerRepo.FindByUserID(ctx, actor.ID)
	if err == nil && seller != nil {
		if seller.UserID != nil && *seller.UserID == actor.ID {
			return true
		}
	}
	return false
}

func (u *ReservationUseCasesImpl) canAccessReservation(ctx context.Context, reservation *domain.Reservation, actorID string) bool {
	if reservation.CustomerID == actorID {
		return true
	}
	if reservation.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, reservation.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				return true
			}
		}
	}
	return false
}

func (u *ReservationUseCasesImpl) findIdempotentReservation(ctx context.Context, actorID, listingID, idempotencyKey string) (*domain.Reservation, error) {
	return u.reservationRepo.FindByIdempotencyKey(ctx, idempotencyKey)
}

func (u *ReservationUseCasesImpl) inferFulfillmentType(listing *domain.Listing) domain.FulfillmentType {
	switch listing.ListingType {
	case domain.ListingTypeService:
		return domain.FulfillmentTypeAppointment
	case domain.ListingTypeRealEstate:
		return domain.FulfillmentTypePropertyVisit
	case domain.ListingTypeVehicle:
		return domain.FulfillmentTypePickup
	default:
		return domain.FulfillmentTypeNone
	}
}
