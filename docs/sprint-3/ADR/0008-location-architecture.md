# ADR 0008 — Location Architecture

## Contexte

La localisation est stratégique pour Trezutil.

Elle concerne :

* les Listings
* les Users
* la recherche

Elle ne peut pas se réduire à trois champs (`city`, `latitude`, `longitude`).

## Problème

Comment concevoir un modèle de localisation qui supporte :

* la hiérarchie administrative
* des positions exactes ou approximatives
* la confidentialité
* plusieurs usages sans confusion

## Options

### Option A — Champs simples dans Listing

* `city`, `latitude`, `longitude` directement dans `Listing`.
* Avantages : simple.
* Inconvénients : pas de hiérarchie, pas de réutilisation, confusion avec UserLocation.

### Option B — Entité Location séparée

* Entité `Location` + `ListingLocation` + `UserLocation`.
* Avantages : réutilisable, hiérarchique, séparation des concepts.
* Inconvénients : plus de jointures.

### Option C — PostGIS uniquement

* PostGIS comme source de vérité géospatiale.
* Avantages : puissant.
* Inconvénients : complexité, dépendance forte.

## Décision

**Option B — Entité Location séparée**

Modèle :

```text
Location
    ↓
ListingLocation
    ↓
Listing

Location
    ↓
UserLocation
    ↓
User
```

Règles :

* `ListingLocation` et `UserLocation` sont des entités séparées.
* `Location` contient la hiérarchie administrative et les coordonnées optionnelles.
* `UserLocation` n’est jamais exposée publiquement.
* `DiscoveryQuery.location` est distincte de `UserLocation`.

## Conséquences

* Le modèle est générique et réutilisable.
* La confidentialité est prise en compte.
* OpenSearch peut utiliser `geo_point` pour les recherches par rayon.
* PostGIS n’est pas requis pour le MVP.
