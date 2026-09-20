# Search Security Model

## 1. Principles

Search & Discovery must preserve the existing multi-tenant security model.

The core principles are:

* Private tenant data must never leak into public search results.
* Unpublished listings must never be visible in public search.
* Internal operational search may have different visibility rules but must still respect tenant boundaries.
* Location data must be exposed only when necessary.

## 2. Visibility Rules by Listing Status

| Status | Public Search | Internal Search | Notes |
|--------|---------------|-----------------|-------|
| `draft` | Excluded | Visible to seller/org | Not searchable publicly |
| `pending_review` | Excluded | Visible to seller/org | Moderation queue |
| `published` | Included | Included | Visible to all |
| `paused` | Excluded | Visible to seller/org | Temporarily hidden |
| `suspended` | Excluded | Visible to platform admin | Moderation action |
| `sold` | Excluded | Visible to seller/org | Terminal state |
| `closed` | Excluded | Visible to seller/org | Terminal state |
| `expired` | Excluded | Visible to seller/org | Terminal state |

### Public vs Internal Search

* **Public search**: accessible without authentication or with a public client context. Only `PUBLISHED` listings are eligible.
* **Internal search**: authenticated requests from seller, store staff, or organization members. Visibility follows the authorization matrix in the application layer.

## 3. Tenant Isolation in Search

The search index must never mix tenant data in a way that allows cross-tenant leakage.

### Index-Level Isolation

Each search document contains `organization_id`. Queries that operate within a tenant context must filter by `organization_id` at query time.

### Public Search

Public search queries do not filter by `organization_id`. They rely on `status = PUBLISHED` to enforce visibility.

### Internal Search

Internal search queries include `organization_id` in the filter clause. The application layer verifies the actor's membership in that organization before allowing the query.

## 4. Authorization Model

| Actor | Public Search | Internal Search |
|-------|---------------|-----------------|
| Anonymous | Allowed | Not allowed |
| Buyer / Public user | Allowed | Not allowed |
| Seller (own org) | Not allowed | Own listings only |
| Store Staff | Not allowed | Own store listings only |
| Org Admin / Owner | Not allowed | All org listings |
| Platform Admin | Not allowed | All listings |
| Platform Operator | Not allowed | All listings |

## 5. Location Privacy

### ListingLocation

* Listing locations are indexed for geo search.
* Coordinates are stored in the search index for distance calculation.
* Listing locations are visible in public search results for published listings.

### UserLocation

* UserLocation is NEVER indexed into the search index.
* UserLocation is never exposed through search APIs.
* UserLocation remains in PostgreSQL only, accessible only to the user and platform operators.

### SearchLocation

* SearchLocation is provided by the client at query time.
* SearchLocation is not stored persistently unless tied to a user preference.
* Free-text addresses passed as SearchLocation must not be logged in full.

## 6. Query Injection

Search queries must be sanitized before execution.

* Full-text queries must be escaped to prevent index injection.
* Attribute filters must validate operator and value types against the attribute definition.
* Geo queries must validate coordinate bounds.
* Pagination cursors must be opaque and validated server-side.

## 7. Rate Limiting and Abuse Prevention

* Search endpoints must enforce rate limits per IP and per authenticated user.
* Expensive queries (large radius, no text filter, high page size) must be throttled or rejected.
* Zero-result queries from the same actor in rapid succession should be monitored but not blocked.

## 8. Data Minimization

Search results must not expose:

* Internal moderation notes
* Seller personal contact information
* Private listing metadata
* Unpublished listing drafts

Search results are projections (`SearchResult`), not full domain entities.

## 9. Audit

All internal search queries must be logged with:

* actor_id (if authenticated)
* organization_id (if applicable)
* query_hash (not raw query text)
* result_count
* latency
* timestamp

Public search queries must be logged with:

* IP hash
* query_hash
* result_count
* latency
* timestamp

## 10. Cross-Tenant Query Leakage

The following scenarios must be prevented:

* A seller querying by category and receiving another tenant's unpublished listings
* An internal user querying without organization context and receiving cross-tenant results
* A public query using attribute filters to enumerate private listings

Prevention mechanisms:

* Public search always filters by `status = PUBLISHED`
* Internal search always includes `organization_id` in the query
* The application layer verifies actor membership before allowing internal search
