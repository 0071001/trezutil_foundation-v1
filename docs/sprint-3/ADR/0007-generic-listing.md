# ADR 0007 — Generic Listing instead of Product

## Contexte

Trezutil doit supporter plusieurs types de commerce :

* produits physiques
* immobilier
* véhicules
* alimentation
* services
* autres offres

Un modèle centré sur `Product` serait trop restrictif.

## Problème

Comment modéliser une offre publiée par un vendeur sans créer six systèmes indépendants ?

## Options

### Option A — Product-centric

* Entité `Product` avec champs spécifiques par type.
* Avantages : simple pour les produits physiques.
* Inconvénients : inadapté à l’immobilier, aux services, etc.

### Option B — Generic Listing

* Entité `Listing` avec `listing_type` et attributs dynamiques.
* Avantages : générique, extensible, un seul cycle de vie.
* Inconvénients : nécessite des attributs dynamiques.

### Option C — Six entités indépendantes

* `Product`, `Property`, `Vehicle`, `Food`, `Service`, `Other`
* Avantages : champs spécifiques explicites.
* Inconvénients : duplication, complexité, maintenance.

## Décision

**Option B — Generic Listing**

`Listing` est l’entité centrale, avec :

* `listing_type` pour distinguer les types
* `AttributeDefinition` / `AttributeValue` pour les champs spécifiques
* `ListingAvailability` pour la disponibilité
* `ListingLocation` pour la localisation

## Conséquences

* Un seul cycle de vie pour tous les types.
* Les attributs dynamiques sont requis.
* Le modèle reste compatible avec de futurs types.
* L’UI devra adapter l’affichage selon `listing_type`.
