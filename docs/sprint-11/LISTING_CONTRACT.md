# Sprint 11 — Listing Contract

## Overview

This document defines how a single `Listing` domain model represents all commercial offer types without vertical-specific core models.

## Core Philosophy

```
ONE Listing MODEL
      ↓
CONFIGURED BY ATTRIBUTES
      ↓
PRODUCT / SERVICE / RENTAL / BOOKABLE OFFER
```

**No vertical-specific Listing types in core domain.**

---

## Core Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Auto | UUID |
| `organization_id` | string | Yes | Owning organization |
| `seller_id` | string | Yes | Seller profile |
| `store_id` | string | Yes | Physical/online store |
| `category_id` | string | Yes | Category reference |
| `title` | string | Yes | Display title |
| `description` | *string | No | Rich description |
| `listing_type` | ListingType | Yes | `physical_product\|real_estate\|vehicle\|food\|service\|other` |
| `pricing_model` | PricingModel | Yes | `fixed\|negotiable\|auction` |
| `price_amount` | float64 | Yes | Major units (e.g., 19.99) |
| `price_currency` | string | Yes | ISO 4217 (USD, EUR, CDF) |
| `status` | ListingStatus | Auto | `draft\|pending_review\|published\|paused\|sold\|closed\|expired\|suspended` |
| `metadata` | map[string]interface{} | No | Extensible |
| `published_at` | *int64 | Auto | Unix timestamp |
| `created_at` | int64 | Auto | Unix timestamp |
| `updated_at` | int64 | Auto | Unix timestamp |

---

## Configuration by Category & Attributes

The listing's **category** + **attributes** determine its semantics.

### Category Hierarchy

```
Electronics (category)
  ↓
  Attributes: brand, model, storage, condition, color
  → Represents: Product

Home Services (category)
  ↓
  Attributes: service_type, duration, includes_materials, team_size
  → Represents: Service

Vehicle Rental (category)
  ↓
  Attributes: vehicle_class, transmission, fuel_type, mileage_limit
  → Represents: Rental

Hotel Room (category)
  ↓
  Attributes: room_type, capacity, amenities, check_in_time, check_out_time
  → Represents: Bookable Offer
```

---

## Attribute System

Attributes are defined per category via `AttributeDefinition`:

```go
type AttributeDefinition struct {
    ID            string
    CategoryID    string
    Name          string
    Key           string      // e.g., "brand", "storage"
    Type          AttributeType  // text, number, boolean, date, url, select
    Searchable    bool
    Required      bool
    Options       []string    // for select type
}
```

Values stored per listing via `AttributeValue`:

```go
type AttributeValue struct {
    ID                      string
    ListingID               string
    AttributeDefinitionID   string
    ValueText   *string
    ValueNumber *float64
    ValueBoolean *bool
    ValueDate   *int64
    ValueURL    *string
    ValueSelect *string
}
```

---

## Pricing

| Model | Semantics | Use Case |
|-------|-----------|----------|
| `fixed` | Price is final | Products, standard services |
| `negotiable` | Starting price, buyer can counter | Vehicles, real estate, custom services |
| `auction` | Starting bid, highest wins | Collectibles, surplus |

**Money representation:** `PriceAmount` in major units (float64 for display), converted to minor units (int64) in `Money` for calculations.

---

## Availability

Separate from Listing: `ListingAvailability`

```go
type ListingAvailability struct {
    ListingID   string
    Status      AvailabilityStatus  // in_stock, out_of_stock, pre_order, reserved, available, unavailable
    Quantity    *int                // for products
}
```

**Important:** Availability ≠ Listing. A listing is the offer; availability is the answer to "can this be purchased/booked now?"

---

## Location

```go
type ListingLocation struct {
    ListingID string
    LocationID string
    IsPrimary bool
}
```

Links to `Location` (city, district, coordinates, country).

---

## Media

```go
type ListingMedia struct {
    ListingID  string
    Type       MediaType  // image, video, document
    URL        string
    Caption    *string
    SortOrder  int
    IsPrimary  bool
}
```

---

## Status Lifecycle

```
DRAFT → PENDING_REVIEW → PUBLISHED ↔ PAUSED
                    ↓
                SOLD / CLOSED / EXPIRED / SUSPENDED
```

### Transitions

| From | To | Valid? | Trigger |
|------|-----|--------|---------|
| DRAFT | PENDING_REVIEW | ✅ | Seller submits |
| PENDING_REVIEW | PUBLISHED | ✅ | Admin approves |
| PENDING_REVIEW | DRAFT | ✅ | Admin rejects |
| PUBLISHED | PAUSED | ✅ | Seller pauses |
| PAUSED | PUBLISHED | ✅ | Seller resumes |
| PUBLISHED | SOLD | ✅ | Order completes (products) |
| PUBLISHED | CLOSED | ✅ | Seller closes |
| PUBLISHED | EXPIRED | ✅ | Auto (time-based) |
| PUBLISHED | SUSPENDED | ✅ | Admin action |

---

## Examples

### 1. Smartphone (Product)

```json
{
  "title": "iPhone 15 Pro 256GB Natural Titanium",
  "listing_type": "physical_product",
  "pricing_model": "fixed",
  "price_amount": 1199.00,
  "price_currency": "USD",
  "category_id": "cat-electronics-smartphones",
  "attributes": [
    { "key": "brand", "value_text": "Apple" },
    { "key": "model", "value_text": "iPhone 15 Pro" },
    { "key": "storage", "value_text": "256GB" },
    { "key": "color", "value_text": "Natural Titanium" },
    { "key": "condition", "value_text": "new" }
  ],
  "availability": { "status": "in_stock", "quantity": 15 }
}
```

### 2. Repair Service (Service)

```json
{
  "title": "iPhone Screen Replacement",
  "listing_type": "service",
  "pricing_model": "fixed",
  "price_amount": 129.99,
  "price_currency": "USD",
  "category_id": "cat-services-electronics-repair",
  "attributes": [
    { "key": "service_type", "value_text": "screen_replacement" },
    { "key": "duration_minutes", "value_number": 60 },
    { "key": "includes_parts", "value_boolean": true },
    { "key": "warranty_months", "value_number": 3 }
  ],
  "availability": { "status": "available" }
}
```

### 3. Vehicle Rental (Rental)

```json
{
  "title": "Toyota Camry 2024 - Compact Sedan",
  "listing_type": "vehicle",
  "pricing_model": "fixed",
  "price_amount": 45.00,
  "price_currency": "USD",
  "category_id": "cat-vehicles-rental-cars",
  "attributes": [
    { "key": "vehicle_class", "value_text": "compact" },
    { "key": "transmission", "value_text": "automatic" },
    { "key": "fuel_type", "value_text": "gasoline" },
    { "key": "mileage_limit_daily", "value_number": 200 },
    { "key": "insurance_included", "value_boolean": true },
    { "key": "fuel_policy", "value_text": "full_to_full" }
  ],
  "availability": { "status": "available" }
}
```

### 4. Hotel Room (Bookable Offer)

```json
{
  "title": "Deluxe King Room - Ocean View",
  "listing_type": "real_estate",
  "pricing_model": "fixed",
  "price_amount": 299.00,
  "price_currency": "USD",
  "category_id": "cat-hospitality-hotel-rooms",
  "attributes": [
    { "key": "room_type", "value_text": "deluxe_king" },
    { "key": "capacity", "value_number": 2 },
    { "key": "check_in_time", "value_text": "15:00" },
    { "key": "check_out_time", "value_text": "11:00" },
    { "key": "amenities", "value_select": "wifi,pool,breakfast,gym,ac" },
    { "key": "view", "value_text": "ocean" },
    { "key": "cancellation_policy", "value_text": "free_until_24h" }
  ],
  "availability": { "status": "available" }
}
```

---

## Visibility & Authorization

| Status | Visible To |
|--------|------------|
| `draft` | Seller owner, org admin/owner/manager, platform admin |
| `pending_review` | Same as draft |
| `published` | Public (all actors) |
| `paused` | Same as draft |
| `sold/closed/expired/suspended` | Seller, org admin, platform admin |

---

## Search Projection

`SearchResult` is the channel-neutral projection for search results:

```go
type SearchResult struct {
    ListingID          string
    Title              string
    PriceAmount        float64
    PriceCurrency      string
    AvailabilityStatus string
    DistanceMeters     *float64
    Score              float64
    MatchedAttributes  []MatchedAttribute
    Location           *SearchResultLocation
    Seller             *SearchResultSeller
    Store              *SearchResultStore
    PrimaryMediaURL    *string
    PublishedAt        int64
}
```

---

## What NOT to Add to Core Listing

| Anti-Pattern | Correct Approach |
|--------------|------------------|
| `hotel_room_number` field | Attribute: `room_number` |
| `vehicle_vin` field | Attribute: `vin` |
| `service_duration_hours` field | Attribute: `duration_minutes` |
| `product_weight_kg` field | Attribute: `weight_kg` |
| `hotel_star_rating` field | Attribute: `star_rating` |
| `vehicle_make` / `vehicle_model` fields | Attributes: `make`, `model` |

**Rule:** If it's specific to one vertical, it's an attribute.

---

## Persistence

| Table | Description |
|-------|-------------|
| `listings` | Core listing fields |
| `listing_attributes` | Attribute values (FK to `attribute_definitions`) |
| `listing_media` | Media references |
| `listing_availability` | Availability status & quantity |
| `listing_locations` | Location links |
| `categories` | Category hierarchy |
| `attribute_definitions` | Attribute schema per category |

---

## Listing Types vs. Categories

| ListingType | Example Categories |
|-------------|-------------------|
| `physical_product` | Electronics, Clothing, Food, Vehicles (for sale) |
| `real_estate` | Hotel Rooms, Apartments, Commercial Space |
| `vehicle` | Cars (rental), Motorcycles, Trucks |
| `food` | Restaurant dishes, Grocery, Catering |
| `service` | Repairs, Beauty, Consulting, Training |
| `other` | Fallback for uncategorized |

---

## Versioning & Metadata

```go
type Listing struct {
    // ... core fields ...
    Metadata map[string]interface{}  // Extensible, not queried
}
```

Use `Metadata` for:
- Feature flags
- Experiment data
- Integration-specific fields (e.g., `external_sku`)

---

## Summary

| Aspect | Approach |
|--------|----------|
| Vertical distinction | Category + Attributes |
| Pricing variety | PricingModel enum |
| Availability | Separate entity (ListingAvailability) |
| Rich data | Attributes (typed, searchable) |
| Media | Separate table (ListingMedia) |
| Location | Separate table (ListingLocation) |
| Extensibility | Metadata map |
| Search | SearchResult projection |
| No vertical core models | ✅ Enforced by architecture |