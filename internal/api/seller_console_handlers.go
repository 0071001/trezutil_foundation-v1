package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// SellerContextSources are the reads behind GET /api/v1/sellers/me.
type SellerContextSources struct {
	Memberships interface {
		FindMembershipsByUserID(ctx context.Context, userID string) ([]domain.OrganizationMember, error)
	}
	Orgs    domain.OrganizationRepository
	Sellers domain.SellerRepository
	Stores  domain.StoreRepository
}

type sellerOrgJSON struct {
	OrganizationID string              `json:"organization_id"`
	DisplayName    string              `json:"display_name"`
	Status         string              `json:"status"`
	Verification   string              `json:"verification_status"`
	Role           string              `json:"role"`
	SellerProfiles []sellerProfileJSON `json:"seller_profiles"`
	Stores         []map[string]string `json:"stores"`
}

type sellerProfileJSON struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	Verification string `json:"verification_status"`
	Mine         bool   `json:"mine"`
}

// SellerContextH handles GET /api/v1/sellers/me: the organizations the caller
// belongs to, with their stores and seller profiles. It never reveals other
// organizations.
func SellerContextH(src SellerContextSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		memberships, err := src.Memberships.FindMembershipsByUserID(r.Context(), userID)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SELLER_CONTEXT_FAILED", Message: "Could not load seller context"}}, http.StatusInternalServerError)
			return
		}
		out := make([]sellerOrgJSON, 0, len(memberships))
		for _, m := range memberships {
			org, err := src.Orgs.FindByID(r.Context(), m.OrganizationID)
			if err != nil {
				continue
			}
			item := sellerOrgJSON{
				OrganizationID: org.ID, DisplayName: org.DisplayName, Status: string(org.Status),
				Verification: string(org.VerificationStatus), Role: string(m.Role),
				SellerProfiles: []sellerProfileJSON{}, Stores: []map[string]string{},
			}
			if sellers, err := src.Sellers.FindByOrganizationID(r.Context(), org.ID); err == nil {
				for _, s := range sellers {
					item.SellerProfiles = append(item.SellerProfiles, sellerProfileJSON{
						ID: s.ID, DisplayName: s.DisplayName, Status: string(s.Status),
						Verification: string(s.VerificationStatus), Mine: s.UserID != nil && *s.UserID == userID,
					})
				}
			}
			if stores, err := src.Stores.FindByOrganizationID(r.Context(), org.ID); err == nil {
				for _, st := range stores {
					item.Stores = append(item.Stores, map[string]string{"id": st.ID, "name": st.Name, "status": string(st.Status)})
				}
			}
			out = append(out, item)
		}
		writeJSON(w, map[string]interface{}{"organizations": out}, http.StatusOK)
	}
}

// OrganizationListingLister lists all listings (every status) of an organization.
type OrganizationListingLister interface {
	FindByOrganizationID(ctx context.Context, organizationID string, offset, limit int) ([]domain.Listing, error)
}

// ManageListingsH handles GET /v1/listings/manage?organization_id= (members only;
// enforced by the caller's organization guard).
func ManageListingsH(listings OrganizationListingLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageParams(r)
		items, err := listings.FindByOrganizationID(r.Context(), r.URL.Query().Get("organization_id"), offset, limit)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_LIST_FAILED", Message: "Could not load listings"}}, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []domain.Listing{}
		}
		writeJSON(w, map[string]interface{}{"items": items, "limit": limit, "offset": offset}, http.StatusOK)
	}
}

// ListingUpdater edits seller-editable listing fields.
type ListingUpdater interface {
	UpdateListing(ctx context.Context, actorID, listingID string, patch application.ListingPatch) (*domain.Listing, error)
}

// UpdateListingH handles POST /v1/listings/update?listing_id=
func UpdateListingH(updater ListingUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		var req struct {
			Title       *string                `json:"title"`
			Description *string                `json:"description"`
			PriceAmount *float64               `json:"price_amount"`
			Metadata    map[string]interface{} `json:"metadata"`
		}
		if err := readJSONBody(r, &req); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
			return
		}
		listing, err := updater.UpdateListing(r.Context(), userID, r.URL.Query().Get("listing_id"), application.ListingPatch{
			Title: req.Title, Description: req.Description, PriceAmount: req.PriceAmount, Metadata: req.Metadata,
		})
		switch {
		case err == nil:
			writeJSON(w, listing, http.StatusOK)
		case errors.Is(err, domain.ErrListingNotFound):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case errors.Is(err, domain.ErrInsufficientPermissions):
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case errors.Is(err, domain.ErrInvalidStatusTransition):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_EDITABLE", Message: "Pause the listing before editing it"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrInvalidParameter):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: err.Error()}}, http.StatusBadRequest)
		default:
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_UPDATE_FAILED", Message: "Could not update listing"}}, http.StatusInternalServerError)
		}
	}
}

// OrganizationOrderLister lists an organization's orders.
type OrganizationOrderLister interface {
	FindByOrganizationID(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Order, error)
}

// OrganizationReservationLister lists an organization's reservations.
type OrganizationReservationLister interface {
	FindByOrganizationID(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Reservation, error)
}

// IncomingOrdersH handles GET /api/v1/orders/incoming?tenant_id= (staff guard).
func IncomingOrdersH(orders OrganizationOrderLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageParams(r)
		items, err := orders.FindByOrganizationID(r.Context(), r.URL.Query().Get("tenant_id"), limit, offset)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_LIST_FAILED", Message: "Could not load orders"}}, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []*domain.Order{}
		}
		writeJSON(w, map[string]interface{}{"items": items, "limit": limit, "offset": offset}, http.StatusOK)
	}
}

// IncomingReservationsH handles GET /api/v1/reservations/incoming?tenant_id= (staff guard).
func IncomingReservationsH(reservations OrganizationReservationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := pageParams(r)
		items, err := reservations.FindByOrganizationID(r.Context(), r.URL.Query().Get("tenant_id"), limit, offset)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "RESERVATION_LIST_FAILED", Message: "Could not load reservations"}}, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []*domain.Reservation{}
		}
		writeJSON(w, map[string]interface{}{"items": items, "limit": limit, "offset": offset}, http.StatusOK)
	}
}
