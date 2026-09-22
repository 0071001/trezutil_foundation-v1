package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// FulfillmentUseCases handles fulfillment-related operations.
type FulfillmentUseCases struct {
	fulfillmentRepo domain.FulfillmentRepository
	orderRepo       domain.OrderRepository
	reservationRepo domain.ReservationRepository
	userRepo        domain.UserRepository
	orgRepo         domain.OrganizationRepository
	auditLogger     domain.AuditLogger
}

func NewFulfillmentUseCases(
	fulfillmentRepo domain.FulfillmentRepository,
	orderRepo domain.OrderRepository,
	reservationRepo domain.ReservationRepository,
	userRepo domain.UserRepository,
	orgRepo domain.OrganizationRepository,
	auditLogger domain.AuditLogger,
) *FulfillmentUseCases {
	return &FulfillmentUseCases{
		fulfillmentRepo: fulfillmentRepo,
		orderRepo:       orderRepo,
		reservationRepo: reservationRepo,
		userRepo:        userRepo,
		orgRepo:         orgRepo,
		auditLogger:     auditLogger,
	}
}

func (uc *FulfillmentUseCases) GetFulfillment(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error) {
	return uc.fulfillmentRepo.FindByID(ctx, tenantID, id)
}

func (uc *FulfillmentUseCases) GetFulfillmentByOrder(ctx context.Context, tenantID, orderID string) (*domain.Fulfillment, error) {
	return uc.fulfillmentRepo.FindByOrderID(ctx, tenantID, orderID)
}

func (uc *FulfillmentUseCases) GetFulfillmentByReservation(ctx context.Context, tenantID, reservationID string) (*domain.Fulfillment, error) {
	return uc.fulfillmentRepo.FindByReservationID(ctx, tenantID, reservationID)
}

func (uc *FulfillmentUseCases) TransitionFulfillment(ctx context.Context, actorID, tenantID, fulfillmentID string, targetStatus domain.FulfillmentStatus, idempotencyKey string) (*domain.Fulfillment, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if fulfillmentID == "" {
		return nil, domain.ErrFulfillmentNotFound
	}

	fulfillment, err := uc.fulfillmentRepo.FindByID(ctx, tenantID, fulfillmentID)
	if err != nil {
		return nil, err
	}

	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !uc.canTransitionFulfillment(ctx, actor, fulfillment, targetStatus) {
		return nil, domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	if err := fulfillment.TransitionTo(targetStatus, actorID, now); err != nil {
		return nil, err
	}

	if err := uc.fulfillmentRepo.Update(ctx, fulfillment); err != nil {
		return nil, err
	}

	if uc.auditLogger != nil {
		uc.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "fulfillment.transitioned",
			TargetType: "fulfillment",
			TargetID:   fulfillmentID,
			Metadata: map[string]interface{}{
				"previous_status": string(fulfillment.Status),
				"new_status":      string(targetStatus),
			},
		})
	}

	return fulfillment, nil
}

func (uc *FulfillmentUseCases) AssignFulfillment(ctx context.Context, actorID, tenantID, fulfillmentID string, idempotencyKey string) (*domain.Fulfillment, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if fulfillmentID == "" {
		return nil, domain.ErrFulfillmentNotFound
	}

	fulfillment, err := uc.fulfillmentRepo.FindByID(ctx, tenantID, fulfillmentID)
	if err != nil {
		return nil, err
	}

	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !uc.canAssignFulfillment(ctx, actor, fulfillment) {
		return nil, domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	if err := fulfillment.Assign(actorID, now); err != nil {
		return nil, err
	}

	if err := uc.fulfillmentRepo.Update(ctx, fulfillment); err != nil {
		return nil, err
	}

	if uc.auditLogger != nil {
		uc.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "fulfillment.assigned",
			TargetType: "fulfillment",
			TargetID:   fulfillmentID,
		})
	}

	return fulfillment, nil
}

func (uc *FulfillmentUseCases) UpdateTracking(ctx context.Context, actorID, tenantID, fulfillmentID, trackingRef string, idempotencyKey string) (*domain.Fulfillment, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if fulfillmentID == "" {
		return nil, domain.ErrFulfillmentNotFound
	}

	fulfillment, err := uc.fulfillmentRepo.FindByID(ctx, tenantID, fulfillmentID)
	if err != nil {
		return nil, err
	}

	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !uc.canUpdateTracking(ctx, actor, fulfillment) {
		return nil, domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	if err := fulfillment.SetTrackingReference(trackingRef, now); err != nil {
		return nil, err
	}

	if err := uc.fulfillmentRepo.Update(ctx, fulfillment); err != nil {
		return nil, err
	}

	if uc.auditLogger != nil {
		uc.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "fulfillment.tracking_updated",
			TargetType: "fulfillment",
			TargetID:   fulfillmentID,
			Metadata: map[string]interface{}{
				"tracking_ref": trackingRef,
			},
		})
	}

	return fulfillment, nil
}

func (uc *FulfillmentUseCases) canTransitionFulfillment(ctx context.Context, actor *domain.User, fulfillment *domain.Fulfillment, target domain.FulfillmentStatus) bool {
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}
	if fulfillment.AssignedTo != nil && *fulfillment.AssignedTo == actor.ID {
		return true
	}
	if fulfillment.TenantID != "" {
		role, err := uc.orgRepo.GetMemberRole(ctx, fulfillment.TenantID, actor.ID)
		if err == nil {
			switch role {
			case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager:
				return true
			}
		}
	}
	return false
}

func (uc *FulfillmentUseCases) canAssignFulfillment(ctx context.Context, actor *domain.User, fulfillment *domain.Fulfillment) bool {
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}
	if fulfillment.TenantID != "" {
		role, err := uc.orgRepo.GetMemberRole(ctx, fulfillment.TenantID, actor.ID)
		if err == nil {
			switch role {
			case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager:
				return true
			}
		}
	}
	return false
}

func (uc *FulfillmentUseCases) canUpdateTracking(ctx context.Context, actor *domain.User, fulfillment *domain.Fulfillment) bool {
	return uc.canTransitionFulfillment(ctx, actor, fulfillment, fulfillment.Status)
}
