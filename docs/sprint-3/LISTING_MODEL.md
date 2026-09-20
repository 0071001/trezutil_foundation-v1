# Listing Model

## Concept

`Listing` est l’entité centrale du Sprint 3.

Il représente une offre publiée par un `Seller` dans un `Store`, rattachée à une `Organization`.

Un Listing peut être :

* un produit physique
* un bien immobilier
* un véhicule
* une offre alimentaire
* un service
* une offre générique

Tous ces cas sont représentés par le même agrégat `Listing`, avec un `listing_type` et des attributs dynamiques.

## Champs universels

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

## Champs spécifiques par type

### PHYSICAL_PRODUCT

* attributs : marque, modèle, couleur, dimensions, poids, matériau
* disponibilité : `IN_STOCK`, `OUT_OF_STOCK`
* livraison : oui/non, zones de livraison

### PROPERTY

* attributs : chambres, salles de bain, surface, meublé, type de bien
* disponibilité : `AVAILABLE`, `RESERVED`, `SOLD`, `RENTED`
* localisation : exacte

### VEHICLE

* attributs : marque, modèle, année, carburant, transmission, kilométrage
* disponibilité : `AVAILABLE`, `RESERVED`, `SOLD`
* localisation : exacte

### FOOD

* attributs : catégorie alimentaire, ingredients, allergènes, date de péremption
* disponibilité : `AVAILABLE`, `SOLD_OUT`
* localisation : exacte ou rayon de livraison

### SERVICE

* attributs : durée, domaine, qualifications
* disponibilité : `AVAILABLE`, `BOOKED`, `UNAVAILABLE`
* localisation : zone de service ou exacte

### OTHER

* attributs : libres
* disponibilité : `AVAILABLE`, `UNAVAILABLE`
* localisation : selon le cas

## Lifecycle

```text
DRAFT
→ PENDING_REVIEW
→ PUBLISHED
→ PAUSED
→ SOLD / CLOSED / EXPIRED
```

Règles :

* Seul un `Seller` appartenant à l’`Organization` peut créer un Listing.
* Un Listing en `PUBLISHED` peut être modifié, mais passe en `PENDING_REVIEW` si validation requise.
* Un Listing `SOLD` / `CLOSED` / `EXPIRED` ne peut plus être modifié.
* `expires_at` est optionnel. S’il est défini, le système doit pouvoir expirer le Listing.

## Ownership

```text
Listing.organization_id  ← dérivé de Store.organization_id
Listing.seller_id        ← dérivé de Store.seller_id ou explicite
Listing.store_id         ← store de publication
```

Règle :

* Le client ne fournit jamais `organization_id` directement.
* Le serveur dérive `organization_id` depuis le `Store`.
* Le serveur vérifie que le `Seller` acteur appartient bien à l’`Organization`.

## Pricing

```text
price_amount        : number
price_currency      : string
price_unit          : string | null
pricing_model       : PricingModel
```

Modèles :

* `FIXED_PRICE` : prix fixe
* `NEGOTIABLE` : prix négociable
* `STARTING_FROM` : prix à partir de
* `PRICE_ON_REQUEST` : prix sur demande
* `FREE` : gratuit
* `PER_UNIT` : prix par unité
* `PER_HOUR` : prix par heure
* `PER_DAY` : prix par jour
* `PER_MONTH` : prix par mois

## Métadonnées

```text
metadata : JSONB
```

Utilisé pour :

* champs spécifiques par type
* extensions futures
* données de compatibilité

Règle :

* Les champs métier core ne doivent pas être stockés dans `metadata`.
* `metadata` est un complément, pas un substitut.
