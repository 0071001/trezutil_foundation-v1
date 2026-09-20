# DiscoveryQuery Domain Model

## 1. Purpose

`DiscoveryQuery` is the canonical, channel-independent representation of a search or discovery request.

It is independent of:
* REST query parameters
* OpenSearch DSL
* PostgreSQL SQL
* WhatsApp message format
* any LLM provider

Natural language inputs from conversational channels must be translated into this model before reaching the Search Engine.

## 2. Conceptual Model

```text
DiscoveryQuery
├── Text
├── Category
├── Attributes
├── PriceRange
├── Currency
├── Availability
├── SearchLocation
├── Radius
├── Seller
├── Store
├── DeliveryRequirements
├── ListingType
├── Sort
└── Pagination
```

## 3. Fields

### Text

Free-text query string.

```text
"iPhone 15 256GB"
"maison à louer Gombe"
```

* Type: `string`
* Optional: yes
* Behavior: full-text search on title, description, category path, and searchable attributes

### Category

Target category or category path.

```text
category_id: "cat-123"
include_children: true
```

* Type: `CategoryFilter`
* Optional: yes

```text
CategoryFilter
├── category_id: string
└── include_children: boolean
```

### Attributes

Typed attribute filters.

```text
attributes:
  - key: "brand"
    operator: "eq"
    value: "Apple"
  - key: "storage"
    operator: "eq"
    value: "256GB"
  - key: "year"
    operator: "gte"
    value: 2023
```

* Type: `AttributeFilter[]`
* Optional: yes

```text
AttributeFilter
├── key: string
├── operator: FilterOperator
└── value: string | number | boolean | date
```

```text
FilterOperator
├── eq      (equal)
├── neq     (not equal)
├── gt      (greater than)
├── gte     (greater than or equal)
├── lt      (less than)
├── lte     (less than or equal)
├── in      (in list)
├── contains (text contains)
└── search  (full-text search)
```

### PriceRange

Price bounds.

```text
price_min: 100.00
price_max: 700.00
```

* Type: `PriceRange`
* Optional: yes

```text
PriceRange
├── min: number | null
└── max: number | null
```

### Currency

Currency filter.

```text
currency: "USD"
```

* Type: `string` (ISO 4217)
* Optional: yes
* Behavior: if specified, only listings with matching currency are returned

### Availability

Availability filter.

```text
availability: "in_stock"
```

* Type: `AvailabilityStatus`
* Optional: yes

```text
AvailabilityStatus
├── in_stock
├── out_of_stock
├── pre_order
├── reserved
├── available
└── unavailable
```

### SearchLocation

Origin location for geo queries.

```text
SearchLocation
├── location_id: string | null
├── latitude: number | null
├── longitude: number | null
└── address: string | null
```

* `location_id`: reference to a known `Location` entity
* `latitude` / `longitude`: explicit coordinates
* `address`: free-text address to be geocoded externally (not by Search)

Only one origin source should be provided. If multiple are provided, `location_id` takes precedence, then explicit coordinates.

### Radius

Search radius.

```text
radius: 5000
radius_unit: "m" | "km"
```

* Type: `number` + unit
* Optional: yes
* Default: no distance filter
* Behavior: filters listings whose `ListingLocation` falls within the radius from `SearchLocation`

### Seller

Seller filter.

```text
seller_id: "seller-123"
```

* Type: `string`
* Optional: yes

### Store

Store filter.

```text
store_id: "store-123"
```

* Type: `string`
* Optional: yes

### DeliveryRequirements

Delivery constraints.

```text
DeliveryRequirements
├── to_location_id: string
├── to_latitude: number | null
├── to_longitude: number | null
└── require_delivery: boolean
```

* Optional: yes
* Behavior: if provided, only listings whose delivery area covers the target location are returned
* This is a future capability; the contract is reserved now

### ListingType

Listing type filter.

```text
listing_type: "physical_product"
```

* Type: `ListingType`
* Optional: yes

```text
ListingType
├── physical_product
├── real_estate
├── vehicle
├── food
├── service
└── other
```

### Sort

Sort preference.

```text
Sort
├── field: SortField
└── direction: SortDirection
```

```text
SortField
├── relevance
├── distance
├── price_asc
├── price_desc
├── newest
├── oldest
└── availability
```

```text
SortDirection
├── asc
└── desc
```

* Optional: yes
* Default: `relevance` descending

### Pagination

Pagination model.

```text
Pagination
├── mode: PaginationMode
├── offset: number | null
├── cursor: string | null
└── page_size: number
```

```text
PaginationMode
├── offset
└── cursor
```

* `page_size`: recommended default 20, maximum 100
* `cursor`: opaque token returned by previous page
* Only one mode should be used per request

## 4. Validation Rules

* At least one of `Text`, `Category`, `Attributes`, `Seller`, `Store`, or `ListingType` must be provided. A query with no search criteria is invalid.
* `Radius` requires `SearchLocation`.
* `DeliveryRequirements` requires a target location.
* `page_size` must be between 1 and 100.
* `Sort.distance` requires `SearchLocation`.

## 5. Immutability

`DiscoveryQuery` is immutable after creation. All modifications produce a new instance.

## 6. Normalization

Before execution, `DiscoveryQuery` is normalized:

* `Text` is trimmed and lowercased for tokenization
* `Category.include_children` defaults to `true`
* `Radius` is converted to meters
* `Pagination.page_size` is clamped to [1, 100]

Normalization happens in the Query Parser layer, not in the domain model.

## 7. Channel Mapping Examples

### REST

```text
GET /v1/search?text=iphone+15&category_id=cat-123&price_max=700&radius=5000&lat=-4.44&lon=15.27&sort=distance&page=1&page_size=20
```

### gRPC

```protobuf
message DiscoveryQuery {
  string text = 1;
  CategoryFilter category = 2;
  repeated AttributeFilter attributes = 3;
  PriceRange price_range = 4;
  string currency = 5;
  string availability = 6;
  SearchLocation location = 7;
  double radius_meters = 8;
  string seller_id = 9;
  string store_id = 10;
  DeliveryRequirements delivery = 11;
  ListingType listing_type = 12;
  Sort sort = 13;
  Pagination pagination = 14;
}
```

### Conversational AI output

```text
{
  "text": "iphone 15",
  "category_id": "cat-123",
  "price_max": 700,
  "radius_meters": 5000,
  "location": { "latitude": -4.44, "longitude": 15.27 }
}
```

All three map to the same `DiscoveryQuery`.

## 8. Reserved Fields

The following fields are reserved for future use and must be accepted but ignored by current implementations:

* `delivery`
* `listing_type` (partially implemented)
* `attributes` with operators beyond `eq` and `contains`
