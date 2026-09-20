# Test Strategy — Sprint 3

## Principe

Les tests sont définis avant l’implémentation.

Ils couvrent :

* le domaine
* PostgreSQL
* l’API
* le Discovery

## Domain

### Listing

* création valide
* création invalide (champs manquants)
* lifecycle transitions
* pricing valide
* pricing invalide
* métadonnées
* ownership derivation

### Category

* création hiérarchique
* unicité slug par scope
* suppression interdite si Listings actifs
* catégories système

### Attribute

* définition par catégorie
* valeur par type
* validation
* searchable / filterable
* unicité key par catégorie

### Media

* upload conceptuel
* primary flag
* ordering
* types supportés

### Availability

* transitions de status
* quantity / reserved_quantity
* période available_from / available_to

### Location

* hiérarchie
* ListingLocation multiple
* UserLocation séparée
* confidentialité

## PostgreSQL

* FK Listing → Store / Seller / Organization
* FK Listing → Category
* FK ListingAttribute → AttributeDefinition / AttributeValue
* FK ListingMedia → Listing
* FK ListingAvailability → Listing
* FK ListingLocation → Listing / Location
* FK UserLocation → User / Location
* UNIQUE slug par scope (Category)
* UNIQUE (listing_id, attribute_definition_id)
* UNIQUE (listing_id, location_id)
* PK user_roles / role_permissions déjà couverts
* tenant isolation Listing / Category / Attribute / Media / Availability / Location

## API

* request validation
* authorization Seller / Store Staff / Org Admin / Public
* tenant boundaries
* public/private visibility
* pagination
* sorting
* rate limiting

## Discovery

* category filtering
* attribute filtering
* radius search
* city / neighborhood search
* price range
* availability filter
* pagination
* sorting
* ranking score

## Outils

* Go tests unitaires pour le domaine.
* PostgreSQL integration tests pour les repositories.
* HTTP tests pour les handlers.
* OpenSearch tests pour l’indexation et la recherche (hors implémentation Sprint 3).

## Non-objectifs Sprint 3

* tests d’intégration Redis
* tests d’intégration Kafka
* tests de charge
* tests E2E complets
