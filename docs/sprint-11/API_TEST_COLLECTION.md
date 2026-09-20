# Sprint 11 — API Test Collection

## Overview

This document provides a reproducible API test collection using the project's existing Go testing approach.

## Test Structure

```
internal/api/
├── handlers.go                   # Shared error/response helpers
├── handlers_test.go              # Existing: Organization, Seller, Store, Listing, Order tests
├── search_handlers.go            # Search and Suggest handlers
├── search_handlers_test.go       # Existing: Search, Suggest tests
├── whatsapp_handler.go           # WhatsApp webhook handler
└── whatsapp_handler_test.go      # Existing: WhatsApp webhook tests
```

---

## Running Tests

```bash
# All API tests
go test -buildvcs=false ./internal/api/... -v

# Specific test suites
go test -buildvcs=false ./internal/api/... -run TestSearch -v
go test -buildvcs=false ./internal/api/... -run TestListing -v
go test -buildvcs=false ./internal/api/... -run TestOrder -v

# Integration tests (requires PostgreSQL)
go test -buildvcs=false ./infrastructure/postgres/integration/... -v
```

---

## Test Fixtures

### Common Test Setup

```go
func setupTestDB(t *testing.T) *TestDB {
    db := NewTestDB(t)
    // Creates organizations, users, sellers, stores, categories
    // Returns cleanup function
    return db
}

func createTestActors(t *testing.T, db *TestDB) (customer, seller string) {
    // Creates test users with roles
    return "user-customer-001", "seller-001"
}

func createTestListing(t *testing.T, db *TestDB, sellerID string) string {
    // Creates published listing with attributes
    return "listing-test-001"
}
```

---

## Search Tests

### TestSearchHandler_SearchH_Success

```go
func TestSearchHandler_SearchH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    
    h := NewSearchHandler(searchUseCases)
    
    body := `{"text": "iphone", "page_size": 10}`
    req := httptest.NewRequest("POST", "/v1/search?user_id=user-001", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.SearchH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var resp SearchResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NotEmpty(t, resp.Items)
}
```

### TestSearchHandler_SearchH_MissingUserID

```go
func TestSearchHandler_SearchH_MissingUserID(t *testing.T) {
    h := NewSearchHandler(searchUseCases)
    
    req := httptest.NewRequest("POST", "/v1/search", strings.NewReader(`{}`))
    w := httptest.NewRecorder()
    
    h.SearchH(w, req)
    
    assert.Equal(t, http.StatusUnauthorized, w.Code)
    assert.Contains(t, w.Body.String(), "MISSING_USER_ID")
}
```

### TestSearchHandler_SearchH_InvalidRequest

```go
func TestSearchHandler_SearchH_InvalidRequest(t *testing.T) {
    h := NewSearchHandler(searchUseCases)
    
    req := httptest.NewRequest("POST", "/v1/search?user_id=user-001", strings.NewReader("not json"))
    w := httptest.NewRecorder()
    
    h.SearchH(w, req)
    
    assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

### TestSearchHandler_SearchH_Pagination

```go
func TestSearchHandler_SearchH_Pagination(t *testing.T) {
    // Tests cursor-based pagination
    // First page
    // Follow next_cursor
    // Verify has_next flag
}
```

### TestSearchHandler_SearchH_Sorting

```go
func TestSearchHandler_SearchH_Sorting(t *testing.T) {
    // Test sort_field: price_asc, price_desc, newest, oldest, relevance
    // Test sort_direction: asc, desc
}
```

### TestSearchHandler_SuggestH_Success

```go
func TestSearchHandler_SuggestH_Success(t *testing.T) {
    h := NewSearchHandler(searchUseCases)
    
    req := httptest.NewRequest("GET", "/v1/search/suggest?user_id=user-001&text=iph&limit=5", nil)
    w := httptest.NewRecorder()
    
    h.SuggestH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var resp SuggestResponseDTO
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.LessOrEqual(t, len(resp.Suggestions), 5)
}
```

---

## Listing Tests

### TestListingHandler_CreateListingH_Success

```go
func TestListingHandler_CreateListingH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    
    h := NewListingHandler(listingUseCases)
    
    body := `{
        "listing": {
            "organization_id": "org-001",
            "seller_id": "seller-001",
            "store_id": "store-001",
            "category_id": "cat-electronics",
            "title": "Test Product",
            "listing_type": "physical_product",
            "pricing_model": "fixed",
            "price_amount": 99.99,
            "price_currency": "USD"
        }
    }`
    
    req := httptest.NewRequest("POST", "/v1/listings?user_id=seller-001", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.CreateListingH(w, req)
    
    assert.Equal(t, http.StatusCreated, w.Code)
    var resp map[string]string
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NotEmpty(t, resp["listing_id"])
}
```

### TestListingHandler_CreateListingH_Forbidden

```go
func TestListingHandler_CreateListingH_Forbidden(t *testing.T) {
    // Actor not authorized to create listing for seller
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("POST", "/v1/listings?user_id=unauthorized-user", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.CreateListingH(w, req)
    
    assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### TestListingHandler_GetListingH_Success

```go
func TestListingHandler_GetListingH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    listingID := createTestListing(t, db, "seller-001")
    
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("GET", "/v1/listings/"+listingID+"?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.GetListingH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var listing domain.Listing
    json.Unmarshal(w.Body.Bytes(), &listing)
    assert.Equal(t, listingID, listing.ID)
}
```

### TestListingHandler_GetListingH_NotFound

```go
func TestListingHandler_GetListingH_NotFound(t *testing.T) {
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("GET", "/v1/listings/non-existent?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.GetListingH(w, req)
    
    assert.Equal(t, http.StatusNotFound, w.Code)
}
```

### TestListingHandler_GetListingH_Forbidden_Draft

```go
func TestListingHandler_GetListingH_Forbidden_Draft(t *testing.T) {
    // Draft listing not visible to public
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("GET", "/v1/listings/draft-listing?user_id=random-user", nil)
    w := httptest.NewRecorder()
    
    h.GetListingH(w, req)
    
    assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### TestListingHandler_PublishListingH_Success

```go
func TestListingHandler_PublishListingH_Success(t *testing.T) {
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("POST", "/v1/listings/listing-001/publish?user_id=seller-001", nil)
    w := httptest.NewRecorder()
    
    h.PublishListingH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### TestListingHandler_PauseListingH_Success

```go
func TestListingHandler_PauseListingH_Success(t *testing.T) {
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("POST", "/v1/listings/listing-001/pause?user_id=seller-001", nil)
    w := httptest.NewRecorder()
    
    h.PauseListingH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### TestListingHandler_ResumeListingH_Success

```go
func TestListingHandler_ResumeListingH_Success(t *testing.T) {
    h := NewListingHandler(listingUseCases)
    
    req := httptest.NewRequest("POST", "/v1/listings/listing-001/resume?user_id=seller-001", nil)
    w := httptest.NewRecorder()
    
    h.ResumeListingH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### TestListingHandler_Idempotency

```go
func TestListingHandler_CreateListingH_Idempotency(t *testing.T) {
    // Same Idempotency-Key returns cached response
    h := NewListingHandler(listingUseCases)
    
    body := `{"listing": {...}}`
    key := "idem-key-001"
    
    // First request
    req1 := httptest.NewRequest("POST", "/v1/listings?user_id=seller-001", strings.NewReader(body))
    req1.Header.Set("Idempotency-Key", key)
    w1 := httptest.NewRecorder()
    h.CreateListingH(w1, req1)
    
    // Second request with same key
    req2 := httptest.NewRequest("POST", "/v1/listings?user_id=seller-001", strings.NewReader(body))
    req2.Header.Set("Idempotency-Key", key)
    w2 := httptest.NewRecorder()
    h.CreateListingH(w2, req2)
    
    // Should return same listing_id
    var resp1, resp2 map[string]string
    json.Unmarshal(w1.Body.Bytes(), &resp1)
    json.Unmarshal(w2.Body.Bytes(), &resp2)
    assert.Equal(t, resp1["listing_id"], resp2["listing_id"])
}
```

---

## Order Tests (EXISTING — in handlers_test.go)

### TestOrderHandler_CreateOrderH_Success

```go
func TestOrderHandler_CreateOrderH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    listingID := createTestListing(t, db, "seller-001")
    
    h := NewOrderHandler(orderUseCases)
    
    body := `{"listing_id": "` + listingID + `", "quantity": 2}`
    
    req := httptest.NewRequest("POST", "/v1/orders?user_id=user-001", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.CreateOrderH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NotEmpty(t, resp["order_id"])
    assert.Equal(t, "draft", resp["status"])
}
```

### TestOrderHandler_CreateOrderH_ListingNotFound

```go
func TestOrderHandler_CreateOrderH_ListingNotFound(t *testing.T) {
    h := NewOrderHandler(orderUseCases)
    
    body := `{"listing_id": "non-existent", "quantity": 1}`
    
    req := httptest.NewRequest("POST", "/v1/orders?user_id=user-001", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.CreateOrderH(w, req)
    
    assert.Equal(t, http.StatusNotFound, w.Code)
}
```

### TestOrderHandler_CreateOrderH_InvalidQuantity

```go
func TestOrderHandler_CreateOrderH_InvalidQuantity(t *testing.T) {
    h := NewOrderHandler(orderUseCases)
    
    body := `{"listing_id": "listing-001", "quantity": 0}`
    
    req := httptest.NewRequest("POST", "/v1/orders?user_id=user-001", strings.NewReader(body))
    w := httptest.NewRecorder()
    
    h.CreateOrderH(w, req)
    
    assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

### TestOrderHandler_CreateOrderH_Idempotency

```go
func TestOrderHandler_CreateOrderH_Idempotency(t *testing.T) {
    // Same Idempotency-Key returns same order
    key := "idem-order-001"
    
    req1 := httptest.NewRequest("POST", "/v1/orders?user_id=user-001", strings.NewReader(body))
    req1.Header.Set("Idempotency-Key", key)
    
    req2 := httptest.NewRequest("POST", "/v1/orders?user_id=user-001", strings.NewReader(body))
    req2.Header.Set("Idempotency-Key", key)
    
    // Both should return same order_id
}
```

### TestOrderHandler_GetOrderH_Success

```go
func TestOrderHandler_GetOrderH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    orderID := createTestOrder(t, db, "user-001")
    
    h := NewOrderHandler(orderUseCases)
    
    req := httptest.NewRequest("GET", "/v1/orders/"+orderID+"?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.GetOrderH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var order domain.Order
    json.Unmarshal(w.Body.Bytes(), &order)
    assert.Equal(t, orderID, order.ID)
}
```

### TestOrderHandler_GetOrderH_Forbidden

```go
func TestOrderHandler_GetOrderH_Forbidden(t *testing.T) {
    // User B cannot access User A's order
    orderID := createTestOrder(t, db, "user-A")
    
    req := httptest.NewRequest("GET", "/v1/orders/"+orderID+"?user_id=user-B", nil)
    w := httptest.NewRecorder()
    
    h.GetOrderH(w, req)
    
    assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### TestOrderHandler_GetOrderH_NotFound

```go
func TestOrderHandler_GetOrderH_NotFound(t *testing.T) {
    req := httptest.NewRequest("GET", "/v1/orders/non-existent?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.GetOrderH(w, req)
    
    assert.Equal(t, http.StatusNotFound, w.Code)
}
```

### TestOrderHandler_CancelOrderH_Success

```go
func TestOrderHandler_CancelOrderH_Success(t *testing.T) {
    db := setupTestDB(t)
    defer db.Cleanup(t)
    orderID := createTestOrder(t, db, "user-001")
    // Order must be in draft or pending_payment
    
    h := NewOrderHandler(orderUseCases)
    
    req := httptest.NewRequest("POST", "/v1/orders/"+orderID+"/cancel?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.CancelOrderH(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    var resp map[string]string
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Equal(t, "cancelled", resp["status"])
}
```

### TestOrderHandler_CancelOrderH_Forbidden

```go
func TestOrderHandler_CancelOrderH_Forbidden(t *testing.T) {
    // User B cannot cancel User A's order
    orderID := createTestOrder(t, db, "user-A")
    
    req := httptest.NewRequest("POST", "/v1/orders/"+orderID+"/cancel?user_id=user-B", nil)
    w := httptest.NewRecorder()
    
    h.CancelOrderH(w, req)
    
    assert.Equal(t, http.StatusForbidden, w.Code)
}
```

### TestOrderHandler_CancelOrderH_InvalidTransition

```go
func TestOrderHandler_CancelOrderH_InvalidTransition(t *testing.T) {
    // Paid order cannot be cancelled
    orderID := createPaidOrder(t, db, "user-001")
    
    req := httptest.NewRequest("POST", "/v1/orders/"+orderID+"/cancel?user_id=user-001", nil)
    w := httptest.NewRecorder()
    
    h.CancelOrderH(w, req)
    
    assert.Equal(t, http.StatusBadRequest, w.Code)
    assert.Contains(t, w.Body.String(), "INVALID_TRANSITION")
}
```

---

## Cross-Channel Tests (DOCUMENTED — shared use cases enforce consistency)

### TestCrossChannel_Search_REST_vs_WhatsApp

```go
func TestCrossChannel_Search_REST_vs_WhatsApp(t *testing.T) {
    // Same query, same use case, both channels
    
    // REST
    restReq := httptest.NewRequest("POST", "/v1/search?user_id=user-001", strings.NewReader(`{"text": "iphone"}`))
    restW := httptest.NewRecorder()
    restHandler.SearchH(restW, restReq)
    
    // WhatsApp
    whatsappReq := buildWhatsAppSearchRequest("iphone")
    whatsappW := httptest.NewRecorder()
    whatsappHandler.WebhookH(whatsappW, whatsappReq)
    
    // Both should return same results structure
    // Both use SearchUseCases.ExecuteSearch
}
```

### TestCrossChannel_Order_REST_vs_WhatsApp

```go
func TestCrossChannel_Order_REST_vs_WhatsApp(t *testing.T) {
    // Same listing, same actor, both channels
    // Both use OrderUseCases.CreateOrder
    // Both enforce same authorization
}
```

### TestCrossChannel_TenantIsolation

```go
func TestCrossChannel_TenantIsolation(t *testing.T) {
    // User in Org A cannot access Org B's listings
    // Works for both REST and WhatsApp
}
```

---

## Usage Tests (EXISTING — see internal/application/usage_test.go)

### TestUsage_MeteredAction

```go
func TestUsage_MeteredAction(t *testing.T) {
    // Create order → credits reserved → consumed
    // Verify balance decreases
}
```

### TestUsage_InsufficientCredits

```go
func TestUsage_InsufficientCredits(t *testing.T) {
    // Zero balance account → create order → ErrInsufficientCredits
}
```

### TestUsage_FreeAction

```go
func TestUsage_FreeAction(t *testing.T) {
    // Search → no credits consumed
}
```

---

## Integration Tests

### TestListingRepository_CRUD

```go
func TestListingRepository_CRUD(t *testing.T) {
    db := NewTestDB(t)
    defer db.Cleanup(t)
    
    repo := NewListingRepository(db.DB)
    
    listing := &domain.Listing{...}
    err := repo.Create(ctx, listing)
    assert.NoError(t, err)
    
    found, _ := repo.FindByID(ctx, listing.ID)
    assert.Equal(t, listing.Title, found.Title)
    
    listing.Title = "Updated"
    err = repo.Update(ctx, listing)
    assert.NoError(t, err)
    
    updated, _ := repo.FindByID(ctx, listing.ID)
    assert.Equal(t, "Updated", updated.Title)
}
```

### TestOrderRepository_CRUD

```go
func TestOrderRepository_CRUD(t *testing.T) {
    db := NewTestDB(t)
    defer db.Cleanup(t)
    
    repo := NewOrderRepository(db.DB)
    
    order := &domain.Order{...}
    err := repo.Create(ctx, order)
    assert.NoError(t, err)
    
    found, _ := repo.FindByID(ctx, order.ID)
    assert.Equal(t, order.TotalAmount, found.TotalAmount)
}
```

### TestConversationStore_CRUD

```go
func TestConversationStore_CRUD(t *testing.T) {
    db := NewTestDB(t)
    defer db.Cleanup(t)
    
    repo := NewConversationStore(db.DB)
    
    conv := domain.NewConversation(...)
    err := repo.Create(ctx, conv)
    assert.NoError(t, err)
    
    found, _ := repo.FindByChannelExternalID(ctx, domain.ChannelWhatsApp, "conv-123")
    assert.Equal(t, conv.ID, found.ID)
}
```

---

## CI/CD Integration

```yaml
# .github/workflows/test.yml
test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
    - run: go test -buildvcs=false ./internal/api/... ./internal/application/... ./infrastructure/postgres/integration/...
```

---

## Coverage Targets

| Package | Target |
|---------|--------|
| `internal/api` | 80% |
| `internal/application` | 80% |
| `internal/domain` | 90% |
| `infrastructure/postgres` | 70% (integration) |