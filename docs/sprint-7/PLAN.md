# Sprint 7 — Commerce Core Actions — Plan

## Implementation Priority

### Phase 1: Discovery (harden existing)
1. **SEARCH_LISTINGS** — Updated to use SearchUseCases interface for visibility resolution and authorization
2. **GET_LISTING** — Updated to use ListingUseCases interface for authorization and tenant isolation

### Phase 2: Commerce Availability
3. **PUBLISH_LISTING** — New handler using ListingUseCases.PublishListing
4. **PAUSE_LISTING** — New handler using ListingUseCases.PauseListing

### Phase 3: Create Listing
5. **CREATE_LISTING** — New handler using ListingUseCases.CreateListing

### Not Implemented (insufficient domain support)
6. **COMPARE_LISTINGS** — No comparison domain model exists
7. **CHECK_INVENTORY** — No inventory abstraction
8. **CREATE_ORDER** — No OrderRepository/OrderUseCases exists despite Order aggregate being complete
9. **GET_ORDER** — No OrderRepository
10. **CANCEL_ORDER** — No OrderRepository (though Order.Cancel() state machine exists)
11. **INITIATE_PAYMENT** — No payment abstraction
12. **GET_PAYMENT_STATUS** — No payment abstraction
13. **GET_DELIVERY_STATUS** — No delivery abstraction
14. **CREATE_MARKETING_CAMPAIGN** — No marketing abstraction
15. **GET_SELLER_REPORT** — No analytics abstraction
