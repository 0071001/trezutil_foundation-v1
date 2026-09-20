# Sprint 7 — Workflows

## Commerce Action Lifecycle

```
Conversation Layer
    ↓ (intent extraction)
CommerceAction
    ↓ (actor from auth, not AI)
CommerceActionRuntime.Execute
    ↓
1. Identity Protection (override actor with authenticated)
2. Tenant Resolution (server-side from actor)
3. Action Validation (structure, actor, conversation)
4. Parameter Validation (required fields per type)
5. Idempotency Check (cache hit → return cached)
6. Confirmation Gate (WRITE requires confirmation)
7. Handler Dispatch
    ↓
Handler
    ↓ (validate inputs, call domain)
ListingUseCases
    ↓ (auth, tenant, state machine)
Persistence Layer
    ↓
CommerceActionResult
```

## Search Listings Workflow

```
1. User: "Find Samsung phones"
2. Intent: SEARCH_LISTINGS
3. CommerceAction with text="Samsung"
4. Runtime → SearchHandler
5. SearchHandler → SearchUseCases.ExecuteSearch(actorID, DiscoveryQuery)
6. SearchUseCases resolves visibility (public/internal)
7. SearchUseCases authorizes search
8. SearchUseCases injects visibility constraints (SellerID, StoreID)
9. SearchEnginePort.ExecuteSearch → OpenSearch
10. Returns paginated results
```

## Publish Listing Workflow

```
1. User: "Publish listing 123"
2. Intent: PUBLISH_LISTING
3. CommerceAction with listing_id="123"
4. Runtime → PublishListingHandler
5. Handler validates listing_id parameter
6. Handler → ListingUseCases.PublishListing(actorID, listingID)
7. ListingUseCases:
   a. Find listing
   b. Verify actor is seller owner OR org admin/manager OR platform admin/operator
   c. Validate transition: DRAFT/PENDING_REVIEW/PAUSED → PUBLISHED
   d. listing.Publish()
   e. Update listing in persistence
8. Returns success result
```

## Pause Listing Workflow

```
1. User: "Pause listing 123"
2. Intent: PAUSE_LISTING
3. CommerceAction with listing_id="123"
4. Runtime → PauseListingHandler
5. Handler → ListingUseCases.PauseListing(actorID, listingID)
6. Same auth + transition validation as Publish
7. listing.Pause(): PUBLISHED → PAUSED
8. Returns success result
```

## Create Listing Workflow

```
1. User: "Create a new listing"
2. Intent: CREATE_LISTING
3. CommerceAction with listing, attributes, media, availability, locations
4. Runtime → CreateListingHandler
5. Handler validates listing parameter exists
6. Handler → ListingUseCases.CreateListing(actorID, listing, ...)
7. ListingUseCases:
   a. Validate store exists
   b. Derive org_id from store
   c. Validate seller exists and belongs to org
   d. Verify actor permission (seller owner, org admin, platform admin)
   e. Validate category exists
   f. Create listing
   g. Create attributes, media, availability, locations
8. Returns created listing ID
```

## Order Workflow (Future — BLOCKED)

```
1. User: "Buy this listing"
2. Intent: CREATE_ORDER
3. CommerceAction with listing_id, quantity
4. Runtime → [CreateOrder handler — not yet implemented]
5. Handler → [OrderUseCases — not yet exists]
6. Order aggregate: DRAFT → PENDING_PAYMENT → PAID → CONFIRMED → ...
7. BLOCKED: No OrderRepository or OrderUseCases interface
```

## Real Estate Scenario

```
Customer: "Je cherche une maison à Limete."
→ SEARCH_LISTINGS (DiscoveryQuery with text="maison Limete")

Customer: "Cette maison est-elle disponible ?"
→ GET_LISTING (listing_id)
→ OR → PUBLISH_LISTING (if seller wants to update)

Customer: "Je veux acheter."
→ CREATE_ORDER (BLOCKED - no order persistence)
```

## Architecture

```
Conversation
    ↓
CommerceAction
    ↓
CommerceActionRuntime
    ↓
Handler (per action type)
    ↓
ListingUseCases / SearchUseCases
    ↓
Domain (Listing, Order, SearchQuery)
    ↓
Persistence (Repository interfaces)
```
