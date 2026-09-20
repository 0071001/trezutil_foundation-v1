# Availability Model

## Concept

La disponibilité d’un `Listing` est distincte de son `status`.

* `Listing.status` : cycle de vie du Listing.
* `ListingAvailability.status` : disponibilité de l’offre.

Exemples :

```text
Listing.status = PUBLISHED
ListingAvailability.status = OUT_OF_STOCK
```

## Entité

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

Règles :

* Un `Listing` a une seule `ListingAvailability` active.
* `quantity` est la quantité totale disponible.
* `reserved_quantity` est la quantité réservée.
* `available_from` / `available_to` bornent la période de disponibilité.
* Si `available_from` / `available_to` ne sont pas définis, le Listing est disponible sans limite de temps.

## Status

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

Règles par type :

### PHYSICAL_PRODUCT

* `IN_STOCK` : stock disponible
* `OUT_OF_STOCK` : stock épuisé

### SERVICE

* `AVAILABLE` : créneau disponible
* `BOOKED` : créneau réservé
* `UNAVAILABLE` : indisponible

### PROPERTY

* `AVAILABLE` : bien disponible
* `RESERVED` : bien réservé
* `SOLD` : bien vendu
* `RENTED` : bien loué

### VEHICLE

* `AVAILABLE` : véhicule disponible
* `RESERVED` : véhicule réservé
* `SOLD` : véhicule vendu

### FOOD

* `AVAILABLE` : plat disponible
* `SOLD_OUT` : épuisé

### OTHER

* `AVAILABLE`
* `UNAVAILABLE`

## Quantity

* `quantity` : quantité totale.
* `reserved_quantity` : quantité réservée.
* `available_quantity = quantity - reserved_quantity`.

Règles :

* Si `quantity` est `null`, la quantité est illimitée.
* `reserved_quantity` ne peut pas dépasser `quantity`.

## Période

* `available_from` : début de disponibilité.
* `available_to` : fin de disponibilité.
* Si non définis, le Listing est disponible sans limite de temps.

Règles :

* `available_to` doit être postérieur à `available_from`.
* Les systèmes de réservation doivent vérifier ces bornes.

## Règles métier

* Un `Listing` ne peut être `PUBLISHED` si sa disponibilité est `OUT_OF_STOCK` ou `UNAVAILABLE`.
* Un `Listing` passé en `SOLD` / `RENTED` / `EXPIRED` voit sa disponibilité figée.
* Les modifications de disponibilité doivent être auditées.
