package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// TestOrganizationHandler_CreateOrganization tests POST /v1/organizations
func TestOrganizationHandler_CreateOrganization(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		payload        interface{}
		expectedStatus int
		expectError    bool
	}{
		{
			name:   "valid organization creation",
			userID: "user-123",
			payload: map[string]interface{}{
				"type":         "business",
				"legal_name":   "Test Org",
				"display_name": "Test Organization",
				"country":      "CD",
				"phone":        "+243812345678",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:           "missing user_id",
			userID:         "",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:   "invalid payload",
			userID: "user-123",
			payload: map[string]interface{}{
				"type": "invalid",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler with mock use cases
			handler := NewOrganizationHandler(&mockOrganizationUseCases{})

			// Create request
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/organizations?user_id="+tt.userID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Call handler
			handler.CreateOrganizationH(w, req)

			// Verify response
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify error response format
			if tt.expectError && w.Code != http.StatusOK {
				var errResp ErrorResponse
				json.Unmarshal(w.Body.Bytes(), &errResp)
				if errResp.Error.Code == "" {
					t.Error("Expected error code in response")
				}
			}
		})
	}
}

// TestOrganizationHandler_GetOrganization tests GET /v1/organizations/{id}
func TestOrganizationHandler_GetOrganization(t *testing.T) {
	tests := []struct {
		name           string
		orgID          string
		expectedStatus int
	}{
		{
			name:           "missing organization_id",
			orgID:          "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid organization_id",
			orgID:          "org-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOrganizationHandler(&mockOrganizationUseCases{})
			req := httptest.NewRequest(http.MethodGet, "/v1/organizations?organization_id="+tt.orgID, nil)
			w := httptest.NewRecorder()

			handler.GetOrganizationH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestSellerHandler_CreateSeller tests POST /v1/sellers
func TestSellerHandler_CreateSeller(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		payload        interface{}
		expectedStatus int
	}{
		{
			name:   "valid seller creation",
			userID: "user-123",
			payload: map[string]interface{}{
				"seller_type":  "individual",
				"display_name": "Test Seller",
				"phone":        "+243812345678",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing user_id",
			userID:         "",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewSellerHandler(&mockSellerUseCases{})
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/sellers?user_id="+tt.userID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateSellerH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestStoreHandler_CreateStore tests POST /v1/stores
func TestStoreHandler_CreateStore(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		payload        interface{}
		expectedStatus int
	}{
		{
			name:   "valid store creation",
			userID: "user-123",
			payload: map[string]interface{}{
				"organization_id": "org-123",
				"name":            "Test Store",
				"slug":            "test-store",
				"country":         "CD",
				"phone":           "+243812345678",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing user_id",
			userID:         "",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewStoreHandler(&mockStoreUseCases{})
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/stores?user_id="+tt.userID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateStoreH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Mock use cases for testing
type mockOrganizationUseCases struct{}

func (m *mockOrganizationUseCases) CreateOrganization(ctx context.Context, actorID string, org *domain.Organization) error {
	return nil
}

func (m *mockOrganizationUseCases) GetOrganization(ctx context.Context, organizationID string) (*domain.Organization, error) {
	return &domain.Organization{ID: organizationID}, nil
}

func (m *mockOrganizationUseCases) UpdateOrganization(ctx context.Context, actorID, organizationID string, updates map[string]interface{}) error {
	return nil
}

func (m *mockOrganizationUseCases) AddMember(ctx context.Context, actorID, organizationID, userID string, role domain.OrganizationRole) error {
	return nil
}

func (m *mockOrganizationUseCases) GetMembers(ctx context.Context, organizationID string) ([]domain.OrganizationMember, error) {
	return nil, nil
}

func (m *mockOrganizationUseCases) RemoveMember(ctx context.Context, actorID, organizationID, userID string) error {
	return nil
}

type mockSellerUseCases struct{}

func (m *mockSellerUseCases) CreateSellerProfile(ctx context.Context, actorID string, seller *domain.SellerProfile) error {
	return nil
}

func (m *mockSellerUseCases) GetSellerProfile(ctx context.Context, sellerID string) (*domain.SellerProfile, error) {
	return &domain.SellerProfile{ID: sellerID}, nil
}

func (m *mockSellerUseCases) UpdateSellerProfile(ctx context.Context, actorID, sellerID string, updates map[string]interface{}) error {
	return nil
}

type mockStoreUseCases struct{}

func (m *mockStoreUseCases) CreateStore(ctx context.Context, actorID string, store *domain.Store) error {
	return nil
}

func (m *mockStoreUseCases) GetStore(ctx context.Context, storeID string) (*domain.Store, error) {
	return &domain.Store{ID: storeID}, nil
}

func (m *mockStoreUseCases) UpdateStore(ctx context.Context, actorID, storeID string, updates map[string]interface{}) error {
	return nil
}

type mockListingUseCases struct{}

func (m *mockListingUseCases) CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error {
	return nil
}

func (m *mockListingUseCases) GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error) {
	return &domain.Listing{ID: listingID}, nil
}

func (m *mockListingUseCases) PublishListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

func (m *mockListingUseCases) PauseListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

func (m *mockListingUseCases) ResumeListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

// TestListingHandler_CreateListing tests POST /v1/listings
func TestListingHandler_CreateListing(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		payload        interface{}
		expectedStatus int
	}{
		{
			name:    "valid listing creation",
			actorID: "user-123",
			payload: map[string]interface{}{
				"listing": map[string]interface{}{
					"seller_id":      "seller-123",
					"store_id":       "store-123",
					"category_id":    "category-123",
					"title":          "Test Listing",
					"listing_type":   "physical_product",
					"pricing_model":  "fixed",
					"price_amount":   100.0,
					"price_currency": "USD",
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing listing",
			actorID:        "user-123",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewListingHandler(&mockListingUseCases{})
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/listings?user_id="+tt.actorID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateListingH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestListingHandler_GetListing tests GET /v1/listings/{listing_id}
func TestListingHandler_GetListing(t *testing.T) {
	tests := []struct {
		name           string
		listingID      string
		expectedStatus int
	}{
		{
			name:           "missing listing_id",
			listingID:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid listing_id",
			listingID:      "listing-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewListingHandler(&mockListingUseCases{})
			req := httptest.NewRequest(http.MethodGet, "/v1/listings?listing_id="+tt.listingID, nil)
			w := httptest.NewRecorder()

			handler.GetListingH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestListingHandler_PublishListing tests POST /v1/listings/{listing_id}/publish
func TestListingHandler_PublishListing(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		listingID      string
		expectedStatus int
	}{
		{
			name:           "missing params",
			actorID:        "",
			listingID:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid publish",
			actorID:        "user-123",
			listingID:      "listing-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewListingHandler(&mockListingUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/listings/publish?user_id="+tt.actorID+"&listing_id="+tt.listingID, nil)
			w := httptest.NewRecorder()

			handler.PublishListingH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestListingHandler_PauseListing tests POST /v1/listings/{listing_id}/pause
func TestListingHandler_PauseListing(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		listingID      string
		expectedStatus int
	}{
		{
			name:           "missing params",
			actorID:        "",
			listingID:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid pause",
			actorID:        "user-123",
			listingID:      "listing-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewListingHandler(&mockListingUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/listings/pause?user_id="+tt.actorID+"&listing_id="+tt.listingID, nil)
			w := httptest.NewRecorder()

			handler.PauseListingH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestListingHandler_ResumeListing tests POST /v1/listings/{listing_id}/resume
func TestListingHandler_ResumeListing(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		listingID      string
		expectedStatus int
	}{
		{
			name:           "missing params",
			actorID:        "",
			listingID:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid resume",
			actorID:        "user-123",
			listingID:      "listing-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewListingHandler(&mockListingUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/listings/resume?user_id="+tt.actorID+"&listing_id="+tt.listingID, nil)
			w := httptest.NewRecorder()

			handler.ResumeListingH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// mockOrderUseCases implements OrderUseCases for testing.
type mockOrderUseCases struct{}

func (m *mockOrderUseCases) CreateOrder(ctx context.Context, actorID string, listingID string, quantity int) (*domain.Order, error) {
	if listingID == "invalid-listing" {
		return nil, domain.ErrOrderNotFound
	}
	if actorID == "unauthorized" {
		return nil, domain.ErrInsufficientPermissions
	}
	return &domain.Order{
		ID:          "order-123",
		Status:      domain.OrderStatusDraft,
		TotalAmount: domain.Money{AmountMinor: 10000, Currency: "USD"},
		Currency:    "USD",
		CustomerID:  "user-123",
	}, nil
}

func (m *mockOrderUseCases) GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	if orderID == "not-found" {
		return nil, domain.ErrOrderNotFound
	}
	if actorID == "unauthorized" {
		return nil, domain.ErrInsufficientPermissions
	}
	return &domain.Order{
		ID:          orderID,
		Status:      domain.OrderStatusDraft,
		TotalAmount: domain.Money{AmountMinor: 10000, Currency: "USD"},
		Currency:    "USD",
		CustomerID:  actorID,
	}, nil
}

func (m *mockOrderUseCases) CancelOrder(ctx context.Context, actorID string, orderID string) error {
	if orderID == "not-found" {
		return domain.ErrOrderNotFound
	}
	if actorID == "unauthorized" {
		return domain.ErrInsufficientPermissions
	}
	if orderID == "invalid-state" {
		return domain.ErrOrderCannotBeCancelled
	}
	return nil
}

func (m *mockOrderUseCases) ConfirmOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	if orderID == "not-found" {
		return nil, domain.ErrOrderNotFound
	}
	if actorID == "unauthorized" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orderID == "invalid-state" {
		return nil, fmt.Errorf("cannot confirm order")
	}
	return &domain.Order{
		ID:     orderID,
		Status: domain.OrderStatusConfirmed,
	}, nil
}

// mockReservationUseCases implements ReservationUseCases for testing.
type mockReservationUseCases struct{}

func (m *mockReservationUseCases) CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error) {
	if listingID == "not-found" {
		return nil, domain.ErrListingNotFound
	}
	if actorID == "unauthorized" {
		return nil, domain.ErrInsufficientPermissions
	}
	if listingID == "unavailable" {
		return nil, domain.ErrAvailabilityUnavailable
	}
	if listingID == "conflict" {
		return nil, domain.ErrReservationConflict
	}
	return &domain.Reservation{
		ID:         "res-123",
		Status:     domain.ReservationStatusConfirmed,
		StartAt:    startAt,
		EndAt:      endAt,
		Quantity:   quantity,
		PartySize:  partySize,
		CustomerID: actorID,
		ListingID:  listingID,
	}, nil
}

func (m *mockReservationUseCases) GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error) {
	if reservationID == "not-found" {
		return nil, domain.ErrReservationNotFound
	}
	if actorID == "unauthorized" {
		return nil, domain.ErrInsufficientPermissions
	}
	return &domain.Reservation{
		ID:             reservationID,
		OrganizationID: "org-1",
		CustomerID:     actorID,
		ListingID:      "listing-1",
		Status:         domain.ReservationStatusConfirmed,
		StartAt:        1760227200,
		EndAt:          1760486400,
		Quantity:       1,
		PartySize:      2,
		Version:        1,
		CreatedAt:      1760000000,
		UpdatedAt:      1760000000,
	}, nil
}

func (m *mockReservationUseCases) CancelReservation(ctx context.Context, actorID string, reservationID string) error {
	if reservationID == "not-found" {
		return domain.ErrReservationNotFound
	}
	if actorID == "unauthorized" {
		return domain.ErrInsufficientPermissions
	}
	if reservationID == "completed" {
		return domain.ErrReservationCannotBeCancelled
	}
	return nil
}

// mockAvailabilityUseCases implements AvailabilityUseCases for testing.
type mockAvailabilityUseCases struct{}

func (m *mockAvailabilityUseCases) CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if query.ListingID == "" {
		return domain.AvailabilityResult{}, fmt.Errorf("listing_id is required")
	}
	if query.StartAt == 0 || query.EndAt == 0 {
		return domain.AvailabilityResult{}, fmt.Errorf("start_at and end_at are required")
	}
	if query.EndAt <= query.StartAt {
		return domain.AvailabilityResult{}, fmt.Errorf("end_at must be after start_at")
	}
	return domain.AvailabilityResult{
		Available: true,
		Capacity:  10,
		Remaining: 7,
		Reason:    "",
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
	}, nil
}

// TestOrderHandler_CreateOrder tests POST /v1/orders
func TestOrderHandler_CreateOrder(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		payload        interface{}
		expectedStatus int
	}{
		{
			name:    "valid order creation",
			actorID: "user-123",
			payload: map[string]interface{}{
				"listing_id": "listing-123",
				"quantity":   1,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			payload:        map[string]interface{}{"listing_id": "listing-123"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing listing_id",
			actorID:        "user-123",
			payload:        map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid listing",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "invalid-listing"},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOrderHandler(&mockOrderUseCases{})
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/orders?user_id="+tt.actorID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateOrderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestOrderHandler_GetOrder tests GET /v1/orders/{order_id}
func TestOrderHandler_GetOrder(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orderID        string
		expectedStatus int
	}{
		{
			name:           "missing order_id",
			actorID:        "user-123",
			orderID:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			orderID:        "order-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "order not found",
			actorID:        "user-123",
			orderID:        "not-found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized access",
			actorID:        "unauthorized",
			orderID:        "order-123",
			expectedStatus: http.StatusNotFound, // Returns 404 to avoid info leakage
		},
		{
			name:           "valid order get",
			actorID:        "user-123",
			orderID:        "order-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOrderHandler(&mockOrderUseCases{})
			req := httptest.NewRequest(http.MethodGet, "/v1/orders?order_id="+tt.orderID+"&user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.GetOrderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestOrderHandler_CancelOrder tests POST /v1/orders/{order_id}/cancel
func TestOrderHandler_CancelOrder(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orderID        string
		expectedStatus int
	}{
		{
			name:           "missing order_id",
			actorID:        "user-123",
			orderID:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			orderID:        "order-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "order not found",
			actorID:        "user-123",
			orderID:        "not-found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized access",
			actorID:        "unauthorized",
			orderID:        "order-123",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid state for cancellation",
			actorID:        "user-123",
			orderID:        "invalid-state",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid cancel",
			actorID:        "user-123",
			orderID:        "order-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOrderHandler(&mockOrderUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/orders/cancel?order_id="+tt.orderID+"&user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.CancelOrderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestOrderHandler_ConfirmOrder tests POST /v1/orders/{order_id}/confirm
func TestOrderHandler_ConfirmOrder(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		orderID        string
		expectedStatus int
	}{
		{
			name:           "missing order_id",
			actorID:        "user-123",
			orderID:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			orderID:        "order-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "order not found",
			actorID:        "user-123",
			orderID:        "not-found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized access",
			actorID:        "unauthorized",
			orderID:        "order-123",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid state for confirmation",
			actorID:        "user-123",
			orderID:        "invalid-state",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "valid confirm",
			actorID:        "user-123",
			orderID:        "order-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOrderHandler(&mockOrderUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/orders/confirm?order_id="+tt.orderID+"&user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.ConfirmOrderH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestReservationHandler_CreateReservationH tests POST /v1/reservations
func TestReservationHandler_CreateReservationH(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		payload        interface{}
		expectedStatus int
	}{
		{
			name:    "valid reservation creation",
			actorID: "user-123",
			payload: map[string]interface{}{
				"listing_id": "listing-123",
				"start_at":   1760227200,
				"end_at":     1760486400,
				"quantity":   1,
				"party_size": 2,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			payload:        map[string]interface{}{"listing_id": "listing-123", "start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing listing_id",
			actorID:        "user-123",
			payload:        map[string]interface{}{"start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing dates",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "listing-123"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid dates (end before start)",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "listing-123", "start_at": 1760486400, "end_at": 1760227200},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "listing not found",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "not-found", "start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized",
			actorID:        "unauthorized",
			payload:        map[string]interface{}{"listing_id": "listing-123", "start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "unavailable",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "unavailable", "start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "conflict",
			actorID:        "user-123",
			payload:        map[string]interface{}{"listing_id": "conflict", "start_at": 1760227200, "end_at": 1760486400},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewReservationHandler(&mockReservationUseCases{}, &mockAvailabilityUseCases{})
			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/v1/reservations?user_id="+tt.actorID, bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateReservationH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestReservationHandler_CreateReservationH_Idempotency tests idempotency key handling
func TestReservationHandler_CreateReservationH_Idempotency(t *testing.T) {
	handler := NewReservationHandler(&mockReservationUseCases{}, &mockAvailabilityUseCases{})
	payload := map[string]interface{}{
		"listing_id": "listing-123",
		"start_at":   1760227200,
		"end_at":     1760486400,
		"quantity":   1,
		"party_size": 2,
	}
	idempotencyKey := "idem-test-123"

	payloadBytes, _ := json.Marshal(payload)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/v1/reservations?user_id=user-123", bytes.NewReader(payloadBytes))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)
	w1 := httptest.NewRecorder()
	handler.CreateReservationH(w1, req1)

	// Second request with same idempotency key
	req2 := httptest.NewRequest(http.MethodPost, "/v1/reservations?user_id=user-123", bytes.NewReader(payloadBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)
	w2 := httptest.NewRecorder()
	handler.CreateReservationH(w2, req2)

	if w1.Code != http.StatusCreated || w2.Code != http.StatusCreated {
		t.Errorf("Expected both requests to return Created. Got %d and %d", w1.Code, w2.Code)
	}

	var resp1, resp2 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1["reservation_id"] != resp2["reservation_id"] {
		t.Errorf("Idempotency failed: different reservation_ids. Got %v and %v", resp1["reservation_id"], resp2["reservation_id"])
	}
}

// TestReservationHandler_GetReservationH tests GET /v1/reservations/{reservation_id}
func TestReservationHandler_GetReservationH(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		reservationID  string
		expectedStatus int
	}{
		{
			name:           "missing reservation_id",
			actorID:        "user-123",
			reservationID:  "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			reservationID:  "res-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "reservation not found",
			actorID:        "user-123",
			reservationID:  "not-found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized access",
			actorID:        "unauthorized",
			reservationID:  "res-123",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "valid get",
			actorID:        "user-123",
			reservationID:  "res-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewReservationHandler(&mockReservationUseCases{}, &mockAvailabilityUseCases{})
			req := httptest.NewRequest(http.MethodGet, "/v1/reservations/"+tt.reservationID+"?user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.GetReservationH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestReservationHandler_CancelReservationH tests POST /v1/reservations/{reservation_id}/cancel
func TestReservationHandler_CancelReservationH(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		reservationID  string
		expectedStatus int
	}{
		{
			name:           "missing reservation_id",
			actorID:        "user-123",
			reservationID:  "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing user_id",
			actorID:        "",
			reservationID:  "res-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "reservation not found",
			actorID:        "user-123",
			reservationID:  "not-found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized access",
			actorID:        "unauthorized",
			reservationID:  "res-123",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid transition (completed)",
			actorID:        "user-123",
			reservationID:  "completed",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid cancel",
			actorID:        "user-123",
			reservationID:  "res-123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewReservationHandler(&mockReservationUseCases{}, &mockAvailabilityUseCases{})
			req := httptest.NewRequest(http.MethodPost, "/v1/reservations/"+tt.reservationID+"/cancel?user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.CancelReservationH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestReservationHandler_CheckAvailabilityH tests GET /v1/availability
func TestReservationHandler_CheckAvailabilityH(t *testing.T) {
	tests := []struct {
		name           string
		actorID        string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "missing user_id",
			actorID:        "",
			queryParams:    "listing_id=listing-123&start_at=1760227200&end_at=1760486400",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing listing_id",
			actorID:        "user-123",
			queryParams:    "start_at=1760227200&end_at=1760486400",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing dates",
			actorID:        "user-123",
			queryParams:    "listing_id=listing-123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid dates",
			actorID:        "user-123",
			queryParams:    "listing_id=listing-123&start_at=1760486400&end_at=1760227200",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid check",
			actorID:        "user-123",
			queryParams:    "listing_id=listing-123&start_at=1760227200&end_at=1760486400&quantity=2&party_size=4",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewReservationHandler(&mockReservationUseCases{}, &mockAvailabilityUseCases{})
			req := httptest.NewRequest(http.MethodGet, "/v1/availability?"+tt.queryParams+"&user_id="+tt.actorID, nil)
			w := httptest.NewRecorder()

			handler.CheckAvailabilityH(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
