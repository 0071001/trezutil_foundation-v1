package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/observability"
)

// ErrorResponse represents a standard API error response.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// Error represents a structured API error.
type Error struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// readJSONBody reads and decodes the JSON body of an HTTP request.
func readJSONBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// reservationFailureReason maps a reservation error to a short, bounded reason
// for logs and metrics.
func reservationFailureReason(err error) string {
	switch {
	case errors.Is(err, domain.ErrListingNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrInsufficientPermissions):
		return "forbidden"
	case errors.Is(err, domain.ErrAvailabilityUnavailable):
		return "unavailable"
	case errors.Is(err, domain.ErrReservationConflict):
		return "conflict"
	case errors.Is(err, domain.ErrInsufficientCapacity):
		return "capacity"
	case errors.Is(err, domain.ErrListingUnavailable):
		return "listing_unavailable"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, domain.ErrAvailabilityUnknown):
		return "unknown_availability"
	case errors.Is(err, domain.ErrInvalidParameter):
		return "invalid"
	default:
		return "error"
	}
}

// InternalErrorMessage is the only text a client ever receives for a 5xx.
const InternalErrorMessage = "An internal error occurred. Quote the request id (X-Request-ID) if you contact support."

// writeFailure answers an unexpected failure of an operation: a permission
// denial is the 403 it always was, an unavailable dependency is 503, anything
// else a generic 500 whose detail goes to the log only (see writeJSON).
func writeFailure(w http.ResponseWriter, code string, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientPermissions):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeJSON(w, ErrorResponse{Error: Error{Code: "SERVICE_UNAVAILABLE", Message: err.Error()}}, http.StatusServiceUnavailable)
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: code, Message: err.Error()}}, http.StatusInternalServerError)
	}
}

// writeJSON writes a JSON response with the given status code.
//
// Error bodies never carry internal detail on a 5xx: handlers may pass the raw
// error text as the message, but it is replaced by InternalErrorMessage and the
// original goes to the server log (with the request id, route and error code)
// through the internal X-Error-* headers that the service middleware consumes
// and removes before the response is sent.
func writeJSON(w http.ResponseWriter, v interface{}, statusCode ...int) {
	w.Header().Set("Content-Type", "application/json")
	if er, ok := v.(ErrorResponse); ok {
		w.Header().Set(common.ErrorCodeHeader, er.Error.Code)
		if len(statusCode) > 0 && statusCode[0] >= 500 {
			if er.Error.Message != "" && er.Error.Message != InternalErrorMessage {
				w.Header().Set(common.ErrorDetailHeader, er.Error.Message)
			}
			er.Error.Message = InternalErrorMessage
			v = er
		}
	}
	if len(statusCode) > 0 {
		w.WriteHeader(statusCode[0])
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(v)
}

// OrganizationUseCases interface for organization operations.
type OrganizationUseCases interface {
	CreateOrganization(ctx context.Context, actorID string, org *domain.Organization) error
	GetOrganization(ctx context.Context, organizationID string) (*domain.Organization, error)
	UpdateOrganization(ctx context.Context, actorID, organizationID string, updates map[string]interface{}) error
	AddMember(ctx context.Context, actorID, organizationID, userID string, role domain.OrganizationRole) error
	GetMembers(ctx context.Context, organizationID string) ([]domain.OrganizationMember, error)
	RemoveMember(ctx context.Context, actorID, organizationID, userID string) error
}

// SellerUseCases interface for seller operations.
type SellerUseCases interface {
	CreateSellerProfile(ctx context.Context, actorID string, seller *domain.SellerProfile) error
	GetSellerProfile(ctx context.Context, sellerID string) (*domain.SellerProfile, error)
	UpdateSellerProfile(ctx context.Context, actorID, sellerID string, updates map[string]interface{}) error
}

// StoreUseCases interface for store operations.
type StoreUseCases interface {
	CreateStore(ctx context.Context, actorID string, store *domain.Store) error
	GetStore(ctx context.Context, storeID string) (*domain.Store, error)
	UpdateStore(ctx context.Context, actorID, storeID string, updates map[string]interface{}) error
}

// ListingUseCases interface for listing operations.
type ListingUseCases interface {
	CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error
	GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error)
	PublishListing(ctx context.Context, actorID string, listingID string) error
	PauseListing(ctx context.Context, actorID string, listingID string) error
	ResumeListing(ctx context.Context, actorID string, listingID string) error
}

// OrderUseCases interface for order operations.
type OrderUseCases interface {
	CreateOrder(ctx context.Context, actorID string, listingID string, quantity int) (*domain.Order, error)
	GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
	CancelOrder(ctx context.Context, actorID string, orderID string) error
	ConfirmOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
}

// orderLifecycle is implemented by order use cases that carry an order through the seller-side steps
// after confirmation (application.OrderUseCases).
type orderLifecycle interface {
	StartProcessing(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
	ReadyForFulfillment(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
	FulfilOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
	CompleteOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
}

// idempotentOrderCreator is implemented by order use cases that support
// idempotent checkout (application.OrderUseCases).
type idempotentOrderCreator interface {
	CreateOrderIdempotent(ctx context.Context, actorID string, listingID string, quantity int, idempotencyKey string) (*domain.Order, error)
}

// cartOrderCreator is implemented by order use cases that accept several
// items in one order (application.OrderUseCases).
type cartOrderCreator interface {
	CreateOrderWithLines(ctx context.Context, actorID string, lines []domain.OrderLine, idempotencyKey string) (*domain.Order, error)
}

// ReservationUseCases interface for reservation operations.
type ReservationUseCases interface {
	CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error)
	GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error)
	CancelReservation(ctx context.Context, actorID string, reservationID string) error
}

// AvailabilityUseCases interface for availability operations.
type AvailabilityUseCases interface {
	CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error)
}

// ProviderUseCases interface for provider operations.
type ProviderUseCases interface {
	CreateProvider(ctx context.Context, actorID string, orgID string, req provider.CreateProviderRequest) (*provider.ProviderResponse, error)
	GetProvider(ctx context.Context, actorID string, orgID string, providerID string) (*provider.ProviderResponse, error)
	ListProviders(ctx context.Context, actorID string, orgID string, offset, limit int) ([]*provider.ProviderResponse, error)
	EnableProvider(ctx context.Context, actorID string, orgID string, providerID string) error
	DisableProvider(ctx context.Context, actorID string, orgID string, providerID string) error
	GetProviderCapabilities(ctx context.Context, actorID string, orgID string, providerID string) ([]string, error)
}

// OrganizationHandler handles organization HTTP requests.
type OrganizationHandler struct {
	orgUseCases OrganizationUseCases
}

// NewOrganizationHandler creates a new OrganizationHandler.
func NewOrganizationHandler(orgUseCases OrganizationUseCases) *OrganizationHandler {
	return &OrganizationHandler{orgUseCases: orgUseCases}
}

// CreateOrganizationH handles POST /v1/organizations
func (h *OrganizationHandler) CreateOrganizationH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req domain.Organization
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.orgUseCases.CreateOrganization(r.Context(), userID, &req)
	if err != nil {
		writeFailure(w, "ORGANIZATION_CREATE_FAILED", err)
		return
	}

	writeJSON(w, req, http.StatusCreated)
}

// GetOrganizationH handles GET /v1/organizations/{organization_id}
func (h *OrganizationHandler) GetOrganizationH(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "Organization ID required"}}, http.StatusBadRequest)
		return
	}

	org, err := h.orgUseCases.GetOrganization(r.Context(), orgID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "ORGANIZATION_NOT_FOUND", Message: "Organization not found"}}, http.StatusNotFound)
		return
	}

	writeJSON(w, org, http.StatusOK)
}

// UpdateOrganizationH handles PATCH /v1/organizations/{organization_id}
func (h *OrganizationHandler) UpdateOrganizationH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	orgID := r.URL.Query().Get("organization_id")
	if userID == "" || orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Organization ID required"}}, http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := readJSONBody(r, &updates); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.orgUseCases.UpdateOrganization(r.Context(), userID, orgID, updates)
	if err != nil {
		writeFailure(w, "ORGANIZATION_UPDATE_FAILED", err)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// AddMemberH handles POST /v1/organizations/{organization_id}/members
func (h *OrganizationHandler) AddMemberH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	orgID := r.URL.Query().Get("organization_id")
	if actorID == "" || orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Organization ID required"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		UserID         string `json:"user_id"`
		MembershipRole string `json:"membership_role"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	role := domain.OrganizationRole(req.MembershipRole)
	err := h.orgUseCases.AddMember(r.Context(), actorID, orgID, req.UserID, role)
	if err != nil {
		writeFailure(w, "ADD_MEMBER_FAILED", err)
		return
	}

	writeJSON(w, map[string]string{"status": "member_added"}, http.StatusCreated)
}

// GetMembersH handles GET /v1/organizations/{organization_id}/members
func (h *OrganizationHandler) GetMembersH(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORGANIZATION_ID", Message: "Organization ID required"}}, http.StatusBadRequest)
		return
	}

	members, err := h.orgUseCases.GetMembers(r.Context(), orgID)
	if err != nil {
		writeFailure(w, "GET_MEMBERS_FAILED", err)
		return
	}

	writeJSON(w, map[string]interface{}{"items": members}, http.StatusOK)
}

// RemoveMemberH handles DELETE /v1/organizations/{organization_id}/members/{user_id}
func (h *OrganizationHandler) RemoveMemberH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	orgID := r.URL.Query().Get("organization_id")
	userID := r.URL.Query().Get("member_user_id") // the member being removed
	if actorID == "" || orgID == "" || userID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Organization ID required"}}, http.StatusBadRequest)
		return
	}

	err := h.orgUseCases.RemoveMember(r.Context(), actorID, orgID, userID)
	if err != nil {
		writeFailure(w, "REMOVE_MEMBER_FAILED", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SellerHandler handles seller HTTP requests.
type SellerHandler struct {
	sellerUseCases SellerUseCases
	access         ProfileAccess
}

// NewSellerHandler creates a new SellerHandler.
func NewSellerHandler(sellerUseCases SellerUseCases) *SellerHandler {
	return &SellerHandler{sellerUseCases: sellerUseCases}
}

// WithAccess says who may read a whole seller profile; everyone else reads the public projection.
func (h *SellerHandler) WithAccess(access ProfileAccess) *SellerHandler {
	h.access = access
	return h
}

// CreateSellerH handles POST /v1/sellers
func (h *SellerHandler) CreateSellerH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req domain.SellerProfile
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.sellerUseCases.CreateSellerProfile(r.Context(), userID, &req)
	if err != nil {
		writeFailure(w, "SELLER_CREATE_FAILED", err)
		return
	}

	writeJSON(w, req, http.StatusCreated)
}

// GetSellerH handles GET /v1/sellers/{seller_id}. The whole profile (phone, email, account and
// organization ids) is for the seller, the members of its organization and platform staff; any
// other caller, a guest included, gets the public projection.
func (h *SellerHandler) GetSellerH(w http.ResponseWriter, r *http.Request) {
	sellerID := r.URL.Query().Get("seller_id")
	if sellerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_SELLER_ID", Message: "Seller ID required"}}, http.StatusBadRequest)
		return
	}

	seller, err := h.sellerUseCases.GetSellerProfile(r.Context(), sellerID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "SELLER_NOT_FOUND", Message: "Seller not found"}}, http.StatusNotFound)
		return
	}

	if !concerned(r.Context(), h.access, auth.UserID(r.Context()), deref(seller.UserID), deref(seller.OrganizationID)) {
		writeJSON(w, publicSeller(seller), http.StatusOK)
		return
	}
	writeJSON(w, seller, http.StatusOK)
}

// UpdateSellerH handles PATCH /v1/sellers/{seller_id}
func (h *SellerHandler) UpdateSellerH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	sellerID := r.URL.Query().Get("seller_id")
	if userID == "" || sellerID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Seller ID required"}}, http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := readJSONBody(r, &updates); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.sellerUseCases.UpdateSellerProfile(r.Context(), userID, sellerID, updates)
	if err != nil {
		writeFailure(w, "SELLER_UPDATE_FAILED", err)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// StoreHandler handles store HTTP requests.
type StoreHandler struct {
	storeUseCases StoreUseCases
	access        ProfileAccess
}

// NewStoreHandler creates a new StoreHandler.
func NewStoreHandler(storeUseCases StoreUseCases) *StoreHandler {
	return &StoreHandler{storeUseCases: storeUseCases}
}

// WithAccess says who may read a whole store record; everyone else reads the public projection.
func (h *StoreHandler) WithAccess(access ProfileAccess) *StoreHandler {
	h.access = access
	return h
}

// CreateStoreH handles POST /v1/stores
func (h *StoreHandler) CreateStoreH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if userID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req domain.Store
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.storeUseCases.CreateStore(r.Context(), userID, &req)
	if err != nil {
		writeFailure(w, "STORE_CREATE_FAILED", err)
		return
	}

	writeJSON(w, req, http.StatusCreated)
}

// GetStoreH handles GET /v1/stores/{store_id}. The whole record (phone, email, organization id) is
// for the members of the owning organization and platform staff; any other caller, a guest
// included, gets the public projection.
func (h *StoreHandler) GetStoreH(w http.ResponseWriter, r *http.Request) {
	storeID := r.URL.Query().Get("store_id")
	if storeID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_STORE_ID", Message: "Store ID required"}}, http.StatusBadRequest)
		return
	}

	store, err := h.storeUseCases.GetStore(r.Context(), storeID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "STORE_NOT_FOUND", Message: "Store not found"}}, http.StatusNotFound)
		return
	}

	if !concerned(r.Context(), h.access, auth.UserID(r.Context()), "", store.OrganizationID) {
		writeJSON(w, publicStore(store), http.StatusOK)
		return
	}
	writeJSON(w, store, http.StatusOK)
}

// UpdateStoreH handles PATCH /v1/stores/{store_id}
func (h *StoreHandler) UpdateStoreH(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	storeID := r.URL.Query().Get("store_id")
	if userID == "" || storeID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Store ID required"}}, http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := readJSONBody(r, &updates); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	err := h.storeUseCases.UpdateStore(r.Context(), userID, storeID, updates)
	if err != nil {
		writeFailure(w, "STORE_UPDATE_FAILED", err)
		return
	}

	writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
}

// OrderHandler handles order HTTP requests.
type OrderHandler struct {
	orderUseCases OrderUseCases
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(orderUseCases OrderUseCases) *OrderHandler {
	return &OrderHandler{orderUseCases: orderUseCases}
}

// writeOrderCreateFailure answers an order-creation error that is not one of
// the known public outcomes: an unavailable dependency (deadline, cancelled
// request) is 503, everything else a generic 500. The detail is logged, never sent.
func writeOrderCreateFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "SERVICE_UNAVAILABLE", Message: err.Error()}}, http.StatusServiceUnavailable)
		return
	}
	writeFailure(w, "ORDER_CREATE_FAILED", err)
}

// createCartOrder handles POST /v1/orders with an items array.
func (h *OrderHandler) createCartOrder(w http.ResponseWriter, r *http.Request, actorID string, items []struct {
	ListingID string `json:"listing_id"`
	Quantity  int    `json:"quantity"`
}) {
	creator, ok := h.orderUseCases.(cartOrderCreator)
	if !ok {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_SUPPORTED", Message: "Multi-item orders are not supported"}}, http.StatusNotImplemented)
		return
	}
	lines := make([]domain.OrderLine, 0, len(items))
	for _, it := range items {
		q := it.Quantity
		if q == 0 {
			q = 1
		}
		lines = append(lines, domain.OrderLine{ListingID: it.ListingID, Quantity: q})
	}
	order, err := creator.CreateOrderWithLines(r.Context(), actorID, lines, r.Header.Get("Idempotency-Key"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidOrderLines):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_ORDER_ITEMS", Message: err.Error()}}, http.StatusBadRequest)
		case errors.Is(err, domain.ErrOrderMixedStores):
			writeJSON(w, ErrorResponse{Error: Error{Code: "MIXED_STORES", Message: err.Error()}}, http.StatusBadRequest)
		case errors.Is(err, domain.ErrOrderNotFound):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case errors.Is(err, domain.ErrInsufficientPermissions):
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case errors.Is(err, domain.ErrListingUnavailable):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_UNAVAILABLE", Message: "An item is not available for purchase"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrIdempotencyConflict):
			writeJSON(w, ErrorResponse{Error: Error{Code: "IDEMPOTENCY_CONFLICT", Message: "Idempotency-Key was already used for a different order"}}, http.StatusConflict)
		default:
			writeOrderCreateFailure(w, err)
		}
		return
	}
	writeJSON(w, map[string]interface{}{
		"order_id":    order.ID,
		"status":      string(order.Status),
		"total":       order.TotalAmount,
		"currency":    order.Currency,
		"customer_id": order.CustomerID,
		"items":       len(order.Items),
	}, http.StatusCreated)
}

// CreateOrderH handles POST /v1/orders
func (h *OrderHandler) CreateOrderH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req struct {
		ListingID string `json:"listing_id"`
		Quantity  int    `json:"quantity"`
		Items     []struct {
			ListingID string `json:"listing_id"`
			Quantity  int    `json:"quantity"`
		} `json:"items"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if len(req.Items) > 0 {
		h.createCartOrder(w, r, actorID, req.Items)
		return
	}

	if req.ListingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_LISTING_ID", Message: "Listing ID required"}}, http.StatusBadRequest)
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	// Retried checkouts carrying the same Idempotency-Key return the original order.
	idempotencyKey := r.Header.Get("Idempotency-Key")
	var order *domain.Order
	var err error
	if creator, ok := h.orderUseCases.(idempotentOrderCreator); ok && idempotencyKey != "" {
		order, err = creator.CreateOrderIdempotent(r.Context(), actorID, req.ListingID, req.Quantity, idempotencyKey)
	} else {
		order, err = h.orderUseCases.CreateOrder(r.Context(), actorID, req.ListingID, req.Quantity)
	}
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case errors.Is(err, domain.ErrInsufficientPermissions):
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case errors.Is(err, domain.ErrListingUnavailable):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_UNAVAILABLE", Message: "This listing is not available for purchase"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrIdempotencyConflict):
			writeJSON(w, ErrorResponse{Error: Error{Code: "IDEMPOTENCY_CONFLICT", Message: "Idempotency-Key was already used for a different order"}}, http.StatusConflict)
		default:
			writeOrderCreateFailure(w, err)
		}
		return
	}

	response := map[string]interface{}{
		"order_id":    order.ID,
		"status":      string(order.Status),
		"total":       order.TotalAmount,
		"currency":    order.Currency,
		"customer_id": order.CustomerID,
	}

	writeJSON(w, response, http.StatusCreated)
}

// GetOrderH handles GET /v1/orders/{order_id}
func (h *OrderHandler) GetOrderH(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORDER_ID", Message: "Order ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	order, err := h.orderUseCases.GetOrder(r.Context(), actorID, orderID)
	if err != nil {
		if err == domain.ErrOrderNotFound || err == domain.ErrInsufficientPermissions {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_NOT_FOUND", Message: "Order not found"}}, http.StatusNotFound)
		} else {
			writeFailure(w, "ORDER_GET_FAILED", err)
		}
		return
	}

	writeJSON(w, order, http.StatusOK)
}

// CancelOrderH handles POST /v1/orders/{order_id}/cancel
func (h *OrderHandler) CancelOrderH(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORDER_ID", Message: "Order ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	err := h.orderUseCases.CancelOrder(r.Context(), actorID, orderID)
	if err != nil {
		switch err {
		case domain.ErrOrderNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_NOT_FOUND", Message: "Order not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrOrderCannotBeCancelled:
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "Order cannot be cancelled in current state"}}, http.StatusBadRequest)
		default:
			writeFailure(w, "ORDER_CANCEL_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"order_id": orderID, "status": "cancelled"}, http.StatusOK)
}

// ConfirmOrderH handles POST /v1/orders/{order_id}/confirm
func (h *OrderHandler) ConfirmOrderH(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORDER_ID", Message: "Order ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	order, err := h.orderUseCases.ConfirmOrder(r.Context(), actorID, orderID)
	if err != nil {
		writeOrderStepFailure(w, "CONFIRM_ORDER_FAILED", err)
		return
	}

	writeJSON(w, map[string]interface{}{
		"order_id": order.ID,
		"status":   string(order.Status),
	}, http.StatusOK)
}

// writeOrderStepFailure maps the outcome of a seller-side order step: an unknown order is 404, a caller who
// may not manage it 403, a step the order's status does not allow (or a concurrent click that got there
// first) 409, anything else a server failure.
func writeOrderStepFailure(w http.ResponseWriter, code string, err error) {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_NOT_FOUND", Message: "Order not found"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrInsufficientPermissions):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalidOrderTransition):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "The order is not in a state that allows this step"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrOptimisticLock):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_CHANGED", Message: "The order was just changed by someone else; reload it"}}, http.StatusConflict)
	default:
		writeFailure(w, code, err)
	}
}

// OrderStepH returns the handler of one seller-side step after confirmation:
// POST /api/v1/orders/{order_id}/process | ready | fulfil | complete.
// The use cases decide who may do it (the seller side, never the buyer) and from which status.
func (h *OrderHandler) OrderStepH(step string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORDER_ID", Message: "Order ID required"}}, http.StatusBadRequest)
			return
		}
		actorID := auth.UserID(r.Context())
		if actorID == "" {
			writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
			return
		}
		lifecycle, ok := h.orderUseCases.(orderLifecycle)
		if !ok {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_IMPLEMENTED", Message: "Order steps are not available"}}, http.StatusNotImplemented)
			return
		}

		var (
			order *domain.Order
			err   error
		)
		switch step {
		case "process":
			order, err = lifecycle.StartProcessing(r.Context(), actorID, orderID)
		case "ready":
			order, err = lifecycle.ReadyForFulfillment(r.Context(), actorID, orderID)
		case "fulfil":
			order, err = lifecycle.FulfilOrder(r.Context(), actorID, orderID)
		case "complete":
			order, err = lifecycle.CompleteOrder(r.Context(), actorID, orderID)
		default:
			writeJSON(w, ErrorResponse{Error: Error{Code: "UNKNOWN_STEP", Message: "Unknown order step"}}, http.StatusNotFound)
			return
		}
		if err != nil {
			writeOrderStepFailure(w, "ORDER_STEP_FAILED", err)
			return
		}
		writeJSON(w, map[string]interface{}{"order_id": order.ID, "status": string(order.Status)}, http.StatusOK)
	}
}

// ListingHandler handles listing HTTP requests.
type ListingHandler struct {
	listingUseCases ListingUseCases
}

// NewListingHandler creates a new ListingHandler.
func NewListingHandler(listingUseCases ListingUseCases) *ListingHandler {
	return &ListingHandler{listingUseCases: listingUseCases}
}

// CreateListingH handles POST /v1/listings
func (h *ListingHandler) CreateListingH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req struct {
		Listing      *domain.Listing             `json:"listing"`
		Attributes   []*domain.AttributeValue    `json:"attributes"`
		Media        []*domain.ListingMedia      `json:"media"`
		Availability *domain.ListingAvailability `json:"availability"`
		Locations    []*domain.ListingLocation   `json:"locations"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.Listing == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_LISTING", Message: "Listing is required"}}, http.StatusBadRequest)
		return
	}

	err := h.listingUseCases.CreateListing(r.Context(), actorID, req.Listing, req.Attributes, req.Media, req.Availability, req.Locations)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidParameter):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: err.Error()}}, http.StatusBadRequest)
		case errors.Is(err, domain.ErrInsufficientPermissions):
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		default:
			writeFailure(w, "LISTING_CREATE_FAILED", err)
		}
		return
	}

	writeJSON(w, req.Listing, http.StatusCreated)
}

// GetListingH handles GET /v1/listings/{listing_id}
func (h *ListingHandler) GetListingH(w http.ResponseWriter, r *http.Request) {
	listingID := r.URL.Query().Get("listing_id")
	if listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_LISTING_ID", Message: "Listing ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	listing, err := h.listingUseCases.GetListing(r.Context(), actorID, listingID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		return
	}

	// Media is added next to the listing fields (the listing shape is unchanged).
	media := []domain.ListingMedia{}
	if src, ok := h.listingUseCases.(listingMediaSource); ok {
		if items, err := src.ListingMedia(r.Context(), listing.ID); err == nil && items != nil {
			media = items
		}
	}
	writeJSON(w, struct {
		*domain.Listing
		Media []domain.ListingMedia
	}{listing, media}, http.StatusOK)
}

// listingMediaSource is implemented by use cases that can list a listing's media.
type listingMediaSource interface {
	ListingMedia(ctx context.Context, listingID string) ([]domain.ListingMedia, error)
}

// PublishListingH handles POST /v1/listings/{listing_id}/publish
func (h *ListingHandler) PublishListingH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	listingID := r.URL.Query().Get("listing_id")
	if actorID == "" || listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Listing ID required"}}, http.StatusBadRequest)
		return
	}

	err := h.listingUseCases.PublishListing(r.Context(), actorID, listingID)
	if err != nil {
		writeFailure(w, "LISTING_PUBLISH_FAILED", err)
		return
	}

	writeJSON(w, map[string]string{"status": "published"}, http.StatusOK)
}

// PauseListingH handles POST /v1/listings/{listing_id}/pause
func (h *ListingHandler) PauseListingH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	listingID := r.URL.Query().Get("listing_id")
	if actorID == "" || listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Listing ID required"}}, http.StatusBadRequest)
		return
	}

	err := h.listingUseCases.PauseListing(r.Context(), actorID, listingID)
	if err != nil {
		switch err {
		case domain.ErrListingNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrInvalidStatusTransition:
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "Invalid status transition"}}, http.StatusBadRequest)
		default:
			writeFailure(w, "LISTING_PAUSE_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"status": "paused"}, http.StatusOK)
}

// ResumeListingH handles POST /v1/listings/{listing_id}/resume
func (h *ListingHandler) ResumeListingH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	listingID := r.URL.Query().Get("listing_id")
	if actorID == "" || listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "User ID and Listing ID required"}}, http.StatusBadRequest)
		return
	}

	err := h.listingUseCases.ResumeListing(r.Context(), actorID, listingID)
	if err != nil {
		switch err {
		case domain.ErrListingNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrInvalidStatusTransition:
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "Invalid status transition"}}, http.StatusBadRequest)
		default:
			writeFailure(w, "LISTING_RESUME_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"status": "published"}, http.StatusOK)
}

// ReservationHandler handles reservation HTTP requests.
type ReservationHandler struct {
	reservationUseCases  ReservationUseCases
	availabilityUseCases AvailabilityUseCases
}

// NewReservationHandler creates a new ReservationHandler.
func NewReservationHandler(reservationUseCases ReservationUseCases, availabilityUseCases AvailabilityUseCases) *ReservationHandler {
	return &ReservationHandler{
		reservationUseCases:  reservationUseCases,
		availabilityUseCases: availabilityUseCases,
	}
}

// CreateReservationH handles POST /v1/reservations
func (h *ReservationHandler) CreateReservationH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req struct {
		ListingID      string `json:"listing_id"`
		StartAt        int64  `json:"start_at"`
		EndAt          int64  `json:"end_at"`
		Quantity       int    `json:"quantity"`
		PartySize      int    `json:"party_size"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.ListingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_LISTING_ID", Message: "Listing ID required"}}, http.StatusBadRequest)
		return
	}
	if req.StartAt == 0 || req.EndAt == 0 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_DATES", Message: "Start and end timestamps required"}}, http.StatusBadRequest)
		return
	}
	if req.EndAt <= req.StartAt {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_DATES", Message: "End must be after start"}}, http.StatusBadRequest)
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.PartySize < 0 {
		req.PartySize = 0
	}

	// Check idempotency key
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = r.Header.Get("Idempotency-Key")
	}

	reservation, err := h.reservationUseCases.CreateReservation(r.Context(), actorID, req.ListingID, req.StartAt, req.EndAt, req.Quantity, req.PartySize, idempotencyKey)
	if err != nil {
		observability.ReservationFailed(r.Context(), req.ListingID, reservationFailureReason(err), err)
		switch {
		case errors.Is(err, domain.ErrListingNotFound):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
		case errors.Is(err, domain.ErrInsufficientPermissions):
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case errors.Is(err, domain.ErrAvailabilityUnavailable):
			writeJSON(w, ErrorResponse{Error: Error{Code: "UNAVAILABLE", Message: "Requested period not available"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrReservationConflict):
			writeJSON(w, ErrorResponse{Error: Error{Code: "CONFLICT", Message: "Reservation conflict: period not available"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrListingUnavailable):
			writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_UNAVAILABLE", Message: "This listing cannot be booked"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrIdempotencyConflict):
			writeJSON(w, ErrorResponse{Error: Error{Code: "IDEMPOTENCY_CONFLICT", Message: "Idempotency-Key was already used for a different booking"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrAvailabilityUnknown):
			writeJSON(w, ErrorResponse{Error: Error{Code: "AVAILABILITY_UNKNOWN", Message: "Availability could not be confirmed right now"}}, http.StatusServiceUnavailable)
		case errors.Is(err, domain.ErrInsufficientCapacity):
			// Lost a race for the last rooms or places: the capacity windows refused it.
			writeJSON(w, ErrorResponse{Error: Error{Code: "UNAVAILABLE", Message: "Requested period not available"}}, http.StatusConflict)
		case errors.Is(err, domain.ErrInvalidParameter):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: err.Error()}}, http.StatusBadRequest)
		default:
			writeFailure(w, "RESERVATION_CREATE_FAILED", err)
		}
		return
	}

	response := map[string]interface{}{
		"reservation_id": reservation.ID,
		"status":         string(reservation.Status),
		"start_at":       reservation.StartAt,
		"end_at":         reservation.EndAt,
		"quantity":       reservation.Quantity,
		"party_size":     reservation.PartySize,
		"customer_id":    reservation.CustomerID,
		"listing_id":     reservation.ListingID,
	}

	writeJSON(w, response, http.StatusCreated)
}

// GetReservationH handles GET /v1/reservations/{reservation_id}
func (h *ReservationHandler) GetReservationH(w http.ResponseWriter, r *http.Request) {
	// Extract reservation_id from path: /v1/reservations/{reservation_id}
	path := r.URL.Path
	reservationID := ""
	if len(path) > len("/v1/reservations/") {
		reservationID = path[len("/v1/reservations/"):]
		// Remove any trailing query string or additional path segments
		if idx := strings.Index(reservationID, "?"); idx >= 0 {
			reservationID = reservationID[:idx]
		}
		if idx := strings.Index(reservationID, "/"); idx >= 0 {
			reservationID = reservationID[:idx]
		}
	}
	if reservationID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_RESERVATION_ID", Message: "Reservation ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	reservation, err := h.reservationUseCases.GetReservation(r.Context(), actorID, reservationID)
	if err != nil {
		switch err {
		case domain.ErrReservationNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "RESERVATION_NOT_FOUND", Message: "Reservation not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		default:
			writeFailure(w, "RESERVATION_GET_FAILED", err)
		}
		return
	}

	writeJSON(w, reservation, http.StatusOK)
}

// CancelReservationH handles POST /v1/reservations/{reservation_id}/cancel
func (h *ReservationHandler) CancelReservationH(w http.ResponseWriter, r *http.Request) {
	// Extract reservation_id from path: /v1/reservations/{reservation_id}/cancel
	path := r.URL.Path
	reservationID := ""
	prefix := "/v1/reservations/"
	suffix := "/cancel"
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		reservationID = path[len(prefix) : len(path)-len(suffix)]
	}
	if reservationID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_RESERVATION_ID", Message: "Reservation ID required"}}, http.StatusBadRequest)
		return
	}

	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	err := h.reservationUseCases.CancelReservation(r.Context(), actorID, reservationID)
	if err != nil {
		switch err {
		case domain.ErrReservationNotFound:
			writeJSON(w, ErrorResponse{Error: Error{Code: "RESERVATION_NOT_FOUND", Message: "Reservation not found"}}, http.StatusNotFound)
		case domain.ErrInsufficientPermissions:
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
		case domain.ErrReservationCannotBeCancelled:
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "Reservation cannot be cancelled in current state"}}, http.StatusBadRequest)
		default:
			writeFailure(w, "RESERVATION_CANCEL_FAILED", err)
		}
		return
	}

	writeJSON(w, map[string]string{"reservation_id": reservationID, "status": "cancelled"}, http.StatusOK)
}

// CheckAvailabilityH handles GET /v1/availability
func (h *ReservationHandler) CheckAvailabilityH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	listingID := r.URL.Query().Get("listing_id")
	if listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_LISTING_ID", Message: "Listing ID required"}}, http.StatusBadRequest)
		return
	}

	startAtStr := r.URL.Query().Get("start_at")
	endAtStr := r.URL.Query().Get("end_at")
	if startAtStr == "" || endAtStr == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_DATES", Message: "Start and end timestamps required"}}, http.StatusBadRequest)
		return
	}

	var startAt, endAt int64
	_, err := fmt.Sscanf(startAtStr, "%d", &startAt)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_START_AT", Message: "Invalid start_at format"}}, http.StatusBadRequest)
		return
	}
	_, err = fmt.Sscanf(endAtStr, "%d", &endAt)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_END_AT", Message: "Invalid end_at format"}}, http.StatusBadRequest)
		return
	}

	if endAt <= startAt {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_DATES", Message: "End must be after start"}}, http.StatusBadRequest)
		return
	}

	quantity := 1
	if qStr := r.URL.Query().Get("quantity"); qStr != "" {
		fmt.Sscanf(qStr, "%d", &quantity)
	}
	if quantity <= 0 {
		quantity = 1
	}

	partySize := 0
	if psStr := r.URL.Query().Get("party_size"); psStr != "" {
		fmt.Sscanf(psStr, "%d", &partySize)
	}
	if partySize < 0 {
		partySize = 0
	}

	result, err := h.availabilityUseCases.CheckAvailability(r.Context(), actorID, domain.AvailabilityQuery{
		ListingID: listingID,
		StartAt:   startAt,
		EndAt:     endAt,
		Quantity:  quantity,
		PartySize: partySize,
	})
	if err != nil {
		if errors.Is(err, domain.ErrAvailabilityUnknown) {
			// An external provider could not establish availability: say so, never "available".
			writeJSON(w, ErrorResponse{Error: Error{Code: "AVAILABILITY_UNKNOWN", Message: "Availability could not be confirmed right now"}}, http.StatusServiceUnavailable)
			return
		}
		writeFailure(w, "AVAILABILITY_CHECK_FAILED", err)
		return
	}

	writeJSON(w, result, http.StatusOK)
}

// InventoryUseCases interface for inventory operations.
type InventoryUseCases interface {
	GetInventory(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error)
	GetInventoryByID(ctx context.Context, tenantID, id string) (*domain.InventoryResource, error)
	ListInventory(ctx context.Context, tenantID string, offset, limit int) ([]*domain.InventoryResource, error)
	AdjustInventory(ctx context.Context, actorID, tenantID, listingID string, deltaTotal, deltaReserved, deltaConsumed int64, reason string, idempotencyKey string) (*domain.InventoryResource, error)
}

// CapacityUseCases interface for capacity operations.
type CapacityUseCases interface {
	GetCapacityUnits(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error)
	CheckCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int) (domain.CapacityResult, error)
	ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error
	ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error
}

// FulfillmentUseCases interface for fulfillment operations.
type FulfillmentUseCases interface {
	GetFulfillment(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error)
	GetFulfillmentByOrder(ctx context.Context, tenantID, orderID string) (*domain.Fulfillment, error)
	GetFulfillmentByReservation(ctx context.Context, tenantID, reservationID string) (*domain.Fulfillment, error)
	TransitionFulfillment(ctx context.Context, actorID, tenantID, fulfillmentID string, targetStatus domain.FulfillmentStatus, idempotencyKey string) (*domain.Fulfillment, error)
	AssignFulfillment(ctx context.Context, actorID, tenantID, fulfillmentID string, idempotencyKey string) (*domain.Fulfillment, error)
	UpdateTracking(ctx context.Context, actorID, tenantID, fulfillmentID, trackingRef string, idempotencyKey string) (*domain.Fulfillment, error)
}

// InventoryHandler handles inventory HTTP requests.
type InventoryHandler struct {
	inventoryUseCases InventoryUseCases
	listingUseCases   ListingUseCases
}

func NewInventoryHandler(inventoryUseCases InventoryUseCases, listingUseCases ListingUseCases) *InventoryHandler {
	return &InventoryHandler{
		inventoryUseCases: inventoryUseCases,
		listingUseCases:   listingUseCases,
	}
}

func (h *InventoryHandler) ListInventory(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TENANT_ID", Message: "tenant_id is required"}}, http.StatusBadRequest)
		return
	}

	offset, limit := 0, 20
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	resources, err := h.inventoryUseCases.ListInventory(r.Context(), tenantID, offset, limit)
	if err != nil {
		writeFailure(w, "INVENTORY_LIST_FAILED", err)
		return
	}

	writeJSON(w, map[string]interface{}{
		"data": resources,
		"pagination": map[string]interface{}{
			"offset": offset,
			"limit":  limit,
		},
	}, http.StatusOK)
}

func (h *InventoryHandler) GetInventory(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/inventory/")
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TENANT_ID", Message: "tenant_id is required"}}, http.StatusBadRequest)
		return
	}

	resource, err := h.inventoryUseCases.GetInventoryByID(r.Context(), tenantID, id)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVENTORY_NOT_FOUND", Message: "Inventory not found"}}, http.StatusNotFound)
		return
	}

	writeJSON(w, resource, http.StatusOK)
}

// FulfillmentHandler handles fulfillment HTTP requests.
type FulfillmentHandler struct {
	fulfillmentUseCases FulfillmentUseCases
}

func NewFulfillmentHandler(fulfillmentUseCases FulfillmentUseCases) *FulfillmentHandler {
	return &FulfillmentHandler{
		fulfillmentUseCases: fulfillmentUseCases,
	}
}

func (h *FulfillmentHandler) GetFulfillment(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/fulfillments/")
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TENANT_ID", Message: "tenant_id is required"}}, http.StatusBadRequest)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.GetFulfillment(r.Context(), tenantID, id)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "FULFILLMENT_NOT_FOUND", Message: "Fulfillment not found"}}, http.StatusNotFound)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}

func (h *FulfillmentHandler) GetFulfillmentByOrder(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimPrefix(r.URL.Path, "/v1/orders/")
	orderID = strings.TrimSuffix(orderID, "/fulfillment")
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TENANT_ID", Message: "tenant_id is required"}}, http.StatusBadRequest)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.GetFulfillmentByOrder(r.Context(), tenantID, orderID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "FULFILLMENT_NOT_FOUND", Message: "Fulfillment not found"}}, http.StatusNotFound)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}

func (h *FulfillmentHandler) GetFulfillmentByReservation(w http.ResponseWriter, r *http.Request) {
	reservationID := strings.TrimPrefix(r.URL.Path, "/v1/reservations/")
	reservationID = strings.TrimSuffix(reservationID, "/fulfillment")
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TENANT_ID", Message: "tenant_id is required"}}, http.StatusBadRequest)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.GetFulfillmentByReservation(r.Context(), tenantID, reservationID)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "FULFILLMENT_NOT_FOUND", Message: "Fulfillment not found"}}, http.StatusNotFound)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}

func (h *FulfillmentHandler) TransitionFulfillment(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/v1/fulfillments/"
	suffix := "/transition"
	id := ""
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		id = path[len(prefix) : len(path)-len(suffix)]
	}
	if id == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_FULFILLMENT_ID", Message: "Fulfillment ID required"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		TargetStatus   string `json:"target_status"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request body"}}, http.StatusBadRequest)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	targetStatus := domain.FulfillmentStatus(req.TargetStatus)
	if !domain.IsValidFulfillmentStatus(targetStatus) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_STATUS", Message: "Invalid target status"}}, http.StatusBadRequest)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.TransitionFulfillment(r.Context(), actorID, tenantID, id, targetStatus, req.IdempotencyKey)
	if err != nil {
		writeFailure(w, "FULFILLMENT_TRANSITION_FAILED", err)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}

func (h *FulfillmentHandler) AssignFulfillment(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/v1/fulfillments/"
	suffix := "/assign"
	id := ""
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		id = path[len(prefix) : len(path)-len(suffix)]
	}
	if id == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_FULFILLMENT_ID", Message: "Fulfillment ID required"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request body"}}, http.StatusBadRequest)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.AssignFulfillment(r.Context(), actorID, tenantID, id, req.IdempotencyKey)
	if err != nil {
		writeFailure(w, "FULFILLMENT_ASSIGN_FAILED", err)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}

func (h *FulfillmentHandler) UpdateTracking(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	prefix := "/v1/fulfillments/"
	suffix := "/tracking"
	id := ""
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		id = path[len(prefix) : len(path)-len(suffix)]
	}
	if id == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_FULFILLMENT_ID", Message: "Fulfillment ID required"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		TrackingRef    string `json:"tracking_ref"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request body"}}, http.StatusBadRequest)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	actorID := auth.UserID(r.Context())
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	fulfillment, err := h.fulfillmentUseCases.UpdateTracking(r.Context(), actorID, tenantID, id, req.TrackingRef, req.IdempotencyKey)
	if err != nil {
		writeFailure(w, "FULFILLMENT_TRACKING_FAILED", err)
		return
	}

	writeJSON(w, fulfillment, http.StatusOK)
}
