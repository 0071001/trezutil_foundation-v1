package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// AdminCommerceReader supplies the commerce lists of the administration console.
type AdminCommerceReader interface {
	ListOrders(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminOrder, int, error)
	ListReservations(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminReservation, int, error)
	ListOrganizations(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminOrganization, int, error)
}

// WithCommerce attaches the commerce lists.
func (h *AdminHandler) WithCommerce(reader AdminCommerceReader) *AdminHandler {
	h.commerce = reader
	return h
}

// The statuses each list can be narrowed to; anything else is refused rather than passed to the database.
var (
	adminOrderStatuses = []string{"draft", "pending_payment", "paid", "confirmed", "processing", "ready_for_fulfillment", "fulfilled", "completed", "cancelled", "refunded", "disputed", "expired"}
	adminBookingStatus = []string{"pending", "confirmed", "completed", "cancelled"}
	adminOrgStatuses   = []string{"active", "suspended", "closed"}
)

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// listParams reads ?q=&status=&page=&page_size= and answers 400 for a status the list does not know.
func listParams(w http.ResponseWriter, r *http.Request, statuses []string) (f domain.AdminListFilter, limit, offset, page int, ok bool) {
	q := r.URL.Query()
	f.Query = strings.TrimSpace(q.Get("q"))
	if len(f.Query) > 100 {
		f.Query = f.Query[:100]
	}
	f.Status = q.Get("status")
	if f.Status != "" && !contains(statuses, f.Status) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_FILTER", Message: "Unknown status"}}, http.StatusBadRequest)
		return f, 0, 0, 0, false
	}
	limit = atoiClamp(q.Get("page_size"), 25, 1, 100)
	page = atoiClamp(q.Get("page"), 1, 1, 100000)
	return f, limit, (page - 1) * limit, page, true
}

func (h *AdminHandler) commerceList(statuses []string, load func(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]map[string]interface{}, int, error)) http.HandlerFunc {
	return h.require(func(w http.ResponseWriter, r *http.Request) {
		if h.commerce == nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "This list is not available"}}, http.StatusServiceUnavailable)
			return
		}
		f, limit, offset, page, ok := listParams(w, r, statuses)
		if !ok {
			return
		}
		items, total, err := load(r.Context(), f, limit, offset)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ADMIN_QUERY_FAILED", Message: "The list could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": limit}, http.StatusOK)
	}, domain.RoleAdmin, domain.RoleOperator)
}

// OrdersH handles GET /api/v1/admin/orders?q=&status=&page=&page_size= (admin or operator).
func (h *AdminHandler) OrdersH() http.HandlerFunc {
	return h.commerceList(adminOrderStatuses, func(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]map[string]interface{}, int, error) {
		orders, total, err := h.commerce.ListOrders(ctx, f, limit, offset)
		items := make([]map[string]interface{}, 0, len(orders))
		for _, o := range orders {
			items = append(items, map[string]interface{}{
				"id": o.ID, "status": o.Status, "total_minor": o.TotalMinor, "currency": o.Currency, "title": o.Title, "item_count": o.ItemCount,
				"organization_id": o.OrganizationID, "organization_name": o.OrganizationName, "customer_id": o.CustomerID, "customer_name": o.CustomerName,
				"created_at": isoTime(o.CreatedAt), "updated_at": isoTime(o.UpdatedAt),
			})
		}
		return items, total, err
	})
}

// ReservationsH handles GET /api/v1/admin/reservations?q=&status=&page=&page_size= (admin or operator).
func (h *AdminHandler) ReservationsH() http.HandlerFunc {
	return h.commerceList(adminBookingStatus, func(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]map[string]interface{}, int, error) {
		bookings, total, err := h.commerce.ListReservations(ctx, f, limit, offset)
		items := make([]map[string]interface{}, 0, len(bookings))
		for _, b := range bookings {
			items = append(items, map[string]interface{}{
				"id": b.ID, "status": b.Status, "title": b.Title, "listing_id": b.ListingID,
				"organization_id": b.OrganizationID, "organization_name": b.OrganizationName, "customer_id": b.CustomerID, "customer_name": b.CustomerName,
				"start_at": isoTime(b.StartAt), "end_at": isoTime(b.EndAt), "quantity": b.Quantity, "party_size": b.PartySize, "created_at": isoTime(b.CreatedAt),
			})
		}
		return items, total, err
	})
}

// OrganizationsH handles GET /api/v1/admin/organizations?q=&status=&page=&page_size= (admin or operator).
// The list carries no phone number or email address.
func (h *AdminHandler) OrganizationsH() http.HandlerFunc {
	return h.commerceList(adminOrgStatuses, func(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]map[string]interface{}, int, error) {
		orgs, total, err := h.commerce.ListOrganizations(ctx, f, limit, offset)
		items := make([]map[string]interface{}, 0, len(orgs))
		for _, o := range orgs {
			items = append(items, map[string]interface{}{
				"id": o.ID, "display_name": o.DisplayName, "type": o.Type, "status": o.Status, "verification_status": o.VerificationStatus,
				"city": o.City, "country": o.Country, "members": o.Members, "stores": o.Stores, "published_listings": o.PublishedListings,
				"orders": o.Orders, "created_at": isoTime(o.CreatedAt),
			})
		}
		return items, total, err
	})
}
