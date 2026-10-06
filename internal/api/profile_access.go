package api

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * What a caller may read of a seller profile or a store.
 *
 * Both are reachable by anyone who has seen a public search result (the seller
 * and store ids are in it), so the full record (phone, email, the account behind
 * the seller, the organization id that keys every tenant-scoped API) is only for
 * the people it concerns: platform administrators and operators, members of the
 * owning organization, and the seller's own account. Everyone else, including a
 * guest, gets a public projection with what a buyer needs to know who they are
 * dealing with. Findings S-1 and S-2 of docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md.
 */

// ProfileAccess decides who counts as concerned by a profile.
type ProfileAccess interface {
	IsPlatformStaff(ctx context.Context, userID string) bool
	IsMember(ctx context.Context, organizationID, userID string) bool
}

type repoProfileAccess struct {
	users domain.UserRepository
	orgs  domain.OrganizationRepository
}

// NewProfileAccess answers from the user and organization repositories.
func NewProfileAccess(users domain.UserRepository, orgs domain.OrganizationRepository) ProfileAccess {
	return &repoProfileAccess{users: users, orgs: orgs}
}

func (a *repoProfileAccess) IsPlatformStaff(ctx context.Context, userID string) bool {
	u, err := a.users.FindByID(ctx, userID)
	return err == nil && u != nil && (u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator))
}

func (a *repoProfileAccess) IsMember(ctx context.Context, organizationID, userID string) bool {
	if organizationID == "" {
		return false
	}
	member, err := a.orgs.HasMember(ctx, organizationID, userID)
	return err == nil && member
}

// concerned reports whether the caller may read the whole record. With no access
// rules configured, nobody does: the safe default is the public projection.
func concerned(ctx context.Context, access ProfileAccess, callerID, ownerUserID, organizationID string) bool {
	if callerID == "" || access == nil {
		return false
	}
	if ownerUserID != "" && ownerUserID == callerID {
		return true
	}
	return access.IsPlatformStaff(ctx, callerID) || access.IsMember(ctx, organizationID, callerID)
}

// PublicSeller is what anyone may read of a seller. Field names are those of
// the full record, so a client reading the public fields is unaffected.
type PublicSeller struct {
	ID                 string
	SellerType         domain.SellerType
	DisplayName        string
	Bio                *string
	Status             domain.SellerStatus
	VerificationStatus domain.VerificationStatus
	TrustLevel         int
	CreatedAt          int64
}

func publicSeller(s *domain.SellerProfile) PublicSeller {
	return PublicSeller{
		ID: s.ID, SellerType: s.SellerType, DisplayName: s.DisplayName, Bio: s.Bio, Status: s.Status,
		VerificationStatus: s.VerificationStatus, TrustLevel: s.TrustLevel, CreatedAt: s.CreatedAt,
	}
}

// PublicStore is what anyone may read of a store: where it is and what it is
// called. Its phone, email and organization id are not public.
type PublicStore struct {
	ID          string
	Name        string
	Slug        string
	Description *string
	Country     string
	Province    *string
	City        *string
	Address     *string
	Latitude    *float64
	Longitude   *float64
	Status      domain.StoreStatus
}

func publicStore(s *domain.Store) PublicStore {
	return PublicStore{
		ID: s.ID, Name: s.Name, Slug: s.Slug, Description: s.Description, Country: s.Country, Province: s.Province,
		City: s.City, Address: s.Address, Latitude: s.Latitude, Longitude: s.Longitude, Status: s.Status,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
