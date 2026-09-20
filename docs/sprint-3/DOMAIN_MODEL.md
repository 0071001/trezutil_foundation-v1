# Domain Model — Catalog, Listing & Location Discovery

## Principe

Le domaine Sprint 3 est centré sur `Listing`.

Un `Listing` est une offre publiée par un `Seller` dans un `Store`, rattachée à une `Organization`, modélisée par :

* une identité
* un type
* un cycle de vie
* un prix
* une disponibilité
* des attributs
* des médias
* une localisation
* des métadonnées

Les entités existantes (`User`, `Organization`, `Seller`, `Store`, `Role`) sont réputées déjà définies dans `docs/DOMAIN_MODEL.md`.

## Agrégats

```text
Organization
    ↓
Seller
    ↓
Store
    ↓
Listing
    ↓
ListingAttribute
ListingMedia
ListingAvailability
ListingLocation

Category
    ↓
CategoryAttribute

AttributeDefinition
AttributeValue

Location
    ↓
ListingLocation
UserLocation
```

## Règles d’isolation

* Une `Organization` possède des `Seller`s.
* Un `Seller` appartient à une `Organization`.
* Un `Store` appartient à une `Organization` et à un `Seller`.
* Un `Listing` appartient à un `Store` et donc implicitement à un `Seller` et à une `Organization`.
* Les opérations sur `Listing` doivent toujours vérifier l’appartenance organization/seller/store.

Aucune modification d’`organization_id`, `seller_id`, `store_id` ne peut venir du client sans validation serveur.

## Entités principales

### Listing

```text
id
organization_id
seller_id
store_id
category_id
title
description
listing_type
status
pricing_model
price_amount
price_currency
price_unit
availability_status
metadata
published_at
expires_at
created_at
updated_at
version
```

### ListingAttribute

```text
id
listing_id
attribute_definition_id
attribute_value_id
value_text
value_number
value_boolean
value_date
value_json
created_at
updated_at
```

### ListingMedia

```text
id
listing_id
type
url
object_key
mime_type
size
width
height
duration
ordering
primary
metadata
created_at
```

### ListingAvailability

```text
id
listing_id
status
quantity
reserved_quantity
available_from
available_to
created_at
updated_at
```

### ListingLocation

```text
id
listing_id
location_id
label
is_primary
created_at
updated_at
```

### Category

```text
id
parent_id
name
slug
description
status
display_order
is_system
organization_id
created_at
updated_at
```

### AttributeDefinition

```text
id
category_id
name
key
data_type
unit
validation_json
searchable
filterable
display_order
created_at
updated_at
```

### AttributeValue

```text
id
attribute_definition_id
value_text
value_number
value_boolean
value_date
value_json
created_at
updated_at
```

### Location

```text
id
country
province
city
municipality
neighborhood
zone
latitude
longitude
accuracy
source
metadata
created_at
updated_at
```

### UserLocation

```text
id
user_id
location_id
source
accuracy
captured_at
created_at
updated_at
```

## Enums

### ListingType

```text
PHYSICAL_PRODUCT
PROPERTY
VEHICLE
FOOD
SERVICE
OTHER
```

### ListingStatus

```text
DRAFT
PENDING_REVIEW
PUBLISHED
PAUSED
SOLD
CLOSED
EXPIRED
```

### PricingModel

```text
FIXED_PRICE
NEGOTIABLE
STARTING_FROM
PRICE_ON_REQUEST
FREE
PER_UNIT
PER_HOUR
PER_DAY
PER_MONTH
```

### AvailabilityStatus

```text
IN_STOCK
OUT_OF_STOCK
AVAILABLE
BOOKED
UNAVAILABLE
RESERVED
SOLD
RENTED
```

### MediaType

```text
IMAGE
VIDEO
DOCUMENT
AUDIO
```

### LocationSource

```text
GPS
USER_INPUT
SEARCH_CONTEXT
DERIVED
```

### DataType

```text
TEXT
NUMBER
BOOLEAN
DATE
ENUM
JSON
```

## Concepts distincts

* `Listing.status` : état du listing dans son cycle de vie.
* `ListingAvailability.status` : état de disponibilité de l’offre.
* `ListingLocation` : localisation du listing.
* `UserLocation` : localisation de l’utilisateur.
* `DiscoveryQuery.location` : localisation de recherche.

Ces concepts ne doivent pas être fusionnés.
