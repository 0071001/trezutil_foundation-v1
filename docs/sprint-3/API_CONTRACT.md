# API Contract — OpenAPI (Conceptuel)

## Base

```text
Host: api.trezutil.com
BasePath: /v1
Auth: Bearer JWT
```

## Endpoints

### Listings

```text
POST   /v1/listings
GET    /v1/listings/{listing_id}
PATCH  /v1/listings/{listing_id}
DELETE /v1/listings/{listing_id}
```

### Listing Actions

```text
POST   /v1/listings/{listing_id}/publish
POST   /v1/listings/{listing_id}/pause
```

### Discovery

```text
GET    /v1/discovery/listings
```

### Categories

```text
GET    /v1/categories
GET    /v1/categories/{category_id}
```

### Locations

```text
GET    /v1/locations/cities
GET    /v1/locations/provinces
GET    /v1/locations/countries
```

## Request / Response (conceptuel)

### POST /v1/listings

Request :

```json
{
  "store_id": "store-123",
  "category_id": "category-456",
  "title": "Samsung Galaxy S25",
  "description": "Brand new...",
  "listing_type": "PHYSICAL_PRODUCT",
  "pricing_model": "FIXED_PRICE",
  "price_amount": 999.99,
  "price_currency": "USD",
  "attributes": [
    { "attribute_definition_id": "attr-1", "value_text": "Samsung" },
    { "attribute_definition_id": "attr-2", "value_number": 12 }
  ],
  "media": [
    { "type": "IMAGE", "url": "https://...", "primary": true }
  ],
  "availability": {
    "status": "IN_STOCK",
    "quantity": 10
  },
  "locations": [
    { "location_id": "loc-1", "label": "Store", "is_primary": true }
  ]
}
```

Response :

```json
{
  "id": "listing-789",
  "organization_id": "org-123",
  "seller_id": "seller-456",
  "store_id": "store-123",
  "category_id": "category-456",
  "title": "Samsung Galaxy S25",
  "status": "DRAFT",
  "created_at": "2026-09-16T00:00:00Z"
}
```

### GET /v1/discovery/listings

Query parameters :

```text
q
category_id
attribute[brand]=Samsung
price_min
price_max
currency
location
radius
availability
sort
page
page_size
```

Response :

```json
{
  "items": [ ... ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

## Authorization

* `POST /v1/listings` : Seller / Store Staff
* `PATCH /v1/listings/{listing_id}` : Seller owner / Store Staff
* `DELETE /v1/listings/{listing_id}` : Seller owner / Org Admin
* `POST /v1/listings/{listing_id}/publish` : Seller / Org Admin
* `GET /v1/listings/{listing_id}` : Seller / Store Staff / Public (si PUBLISHED)
* `GET /v1/discovery/listings` : Public
* `GET /v1/categories` : Public

## Tenant boundaries

* Toutes les réponses retournent uniquement les Listings appartenant à l’`Organization` de l’acteur, sauf pour les endpoints publics.
* Les endpoints de Discovery ne retournent que les Listings `PUBLISHED`.
* Les endpoints privés vérifient l’`organization_id` dérivé du `Store`.

## Errors

Codes standards :

```text
400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
422 Unprocessable Entity
429 Too Many Requests
500 Internal Server Error
```

Body :

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid listing_type",
    "details": []
  }
}
```

## Pagination

* `page` : numéro de page (1-based)
* `page_size` : taille de page (max 100)
* `total` : nombre total d’éléments

## Sorting

Valeurs autorisées :

```text
RELEVANCE
DISTANCE
PRICE_ASC
PRICE_DESC
NEWEST
OLDEST
```

## Validation

* `listing_type` : enum
* `pricing_model` : enum
* `price_amount` : nombre positif
* `currency` : ISO 4217
* `attributes` : validation par `AttributeDefinition`
* `media` : validation de type MIME
* `locations` : validation de coordonnées

## Rate limiting

* Discovery : limité par IP / utilisateur.
* Écriture : limité par Seller / Store.
