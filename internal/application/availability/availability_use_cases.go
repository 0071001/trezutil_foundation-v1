package availability

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// AvailabilityUseCases defines the interface for availability operations.
type AvailabilityUseCases interface {
	CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error)
}

// AvailabilityUseCasesImpl implements AvailabilityUseCases.
type AvailabilityUseCasesImpl struct {
	listingRepo domain.ListingRepository
	provider    domain.AvailabilityProvider
	auditLogger domain.AuditLogger
}

// NewAvailabilityUseCases creates a new AvailabilityUseCases.
func NewAvailabilityUseCases(
	listingRepo domain.ListingRepository,
	provider domain.AvailabilityProvider,
	auditLogger domain.AuditLogger,
) *AvailabilityUseCasesImpl {
	return &AvailabilityUseCasesImpl{
		listingRepo: listingRepo,
		provider:    provider,
		auditLogger: auditLogger,
	}
}

// CheckAvailability checks if a listing is available for the requested period.
func (u *AvailabilityUseCasesImpl) CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if actorID == "" {
		return domain.AvailabilityResult{}, domain.ErrInsufficientPermissions
	}
	if query.ListingID == "" {
		return domain.AvailabilityResult{}, fmt.Errorf("listing_id is required")
	}
	if query.StartAt == 0 || query.EndAt == 0 {
		return domain.AvailabilityResult{}, fmt.Errorf("start_at and end_at are required")
	}
	if query.EndAt <= query.StartAt {
		return domain.AvailabilityResult{}, fmt.Errorf("end_at must be after start_at")
	}

	listing, err := u.listingRepo.FindByID(ctx, query.ListingID)
	if err != nil {
		return domain.AvailabilityResult{}, fmt.Errorf("listing not found: %w", err)
	}

	tenantID := listing.OrganizationID
	if tenantID == "" {
		return domain.AvailabilityResult{}, fmt.Errorf("listing has no organization")
	}

	query.TenantID = tenantID
	result, err := u.provider.CheckAvailability(ctx, query)
	if err != nil {
		return domain.AvailabilityResult{}, fmt.Errorf("availability check failed: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "availability.checked",
			TargetType: "listing",
			TargetID:   query.ListingID,
			Metadata: map[string]interface{}{
				"organization_id": tenantID,
				"start_at":        query.StartAt,
				"end_at":          query.EndAt,
				"available":       result.Available,
				"remaining":       result.Remaining,
			},
		})
	}

	return result, nil
}
