# ADR 0011 — Geo Search Architecture

## Contexte

La recherche géospatiale est un besoin central pour Trezutil.

Les utilisateurs vont rechercher des listings :
* près de leur position actuelle
* dans un quartier spécifique (ex: "Gombe")
* dans un rayon autour d'un point
* accessibles par livraison dans une zone

Les concepts géographiques suivants doivent rester séparés :

* `UserLocation` — position de l'utilisateur
* `SearchLocation` — origine de la requête de recherche
* `ListingLocation` — position du listing
* `DeliveryArea` — zone de livraison (futur)

## Problème

Comment concevoir l'architecture géospatiale sans coupler la recherche à une technologie spécifique tout en gardant des performances acceptables en RDC ?

## Options

### Option A — PostgreSQL + PostGIS uniquement

* Toutes les requêtes géospatiales passent par PostGIS.
* Avantages : pas de dépendance supplémentaire, transactions ACID.
* Inconvénients : performance sur les requêtes plein-texte + geo combinées, scaling limité, pas de ranking géospatial avancé.

### Option B — OpenSearch Geo uniquement

* Toutes les requêtes géospatiales passent par OpenSearch.
* Avantages : geo queries performantes, combinaison avec full-text.
* Inconvénients : dépendance à OpenSearch, complexité de synchronisation.

### Option C — Hybride avec dénormalisation

* Les coordonnées sont stockées dans PostgreSQL (dans `locations`) et dénormalisées dans l'index OpenSearch.
* Les requêtes utilisent l'outil approprié selon le type de filtre.
* Avantages : flexibilité, performance, cohérence avec le reste de l'architecture.
* Inconvénients : deux systèmes à maintenir.

## Décision

**Option C — Hybride avec dénormalisation**

### Architecture retenue

```
SearchLocation (client-provided)
    ↓
Search Service
    ├── PostgreSQL : filtres exacts, tenant, availability
    └── OpenSearch  : geo filter + full-text + ranking
         ↓
    SearchResult (avec distance calculée)
```

### Règles

* `locations` dans PostgreSQL est la source de vérité pour les coordonnées et la hiérarchie administrative.
* L'index OpenSearch contient une copie dénormalisée des coordonnées pour les requêtes géospatiales rapides.
* `UserLocation` n'est JAMAIS exposé à Search. Il reste dans PostgreSQL.
* `SearchLocation` est fourni à chaque requête ; il n'est pas stocké par défaut.
* `ListingLocation` est la seule source de position pour les listings dans l'index de recherche.
* `DeliveryArea` est un concept réservé pour le futur ; il ne sera pas implémenté avant que Logistics soit disponible.

### Coordonnées

* Stockées en PostgreSQL dans `locations.latitude` et `locations.longitude` (`DECIMAL(10,8)` et `DECIMAL(11,8)`).
* Copiées dans OpenSearch en tant que `geo_point`.
* La précision est suffisante pour des recherches de quartier.

### Radius Search

* Rayon en mètres, converti en unité OpenSearch.
* Le rayon maximum est configurable mais recommandé à 50 km par défaut.
* Au-delà, la requête passe par un filtrage par ville/quartier plutôt que par rayon strict.

### Distance Ranking

* La distance participe au ranking mais ne domine pas automatiquement tous les autres signaux.
* Le ranking permet de favoriser la pertinence tout en pénalisant les listings très éloignés.

## Conséquences

* Deux systèmes à interroger pour les requêtes géospatiales.
* La synchronisation des coordonnées passe par les événements `ListingLocationUpdated`.
* `UserLocation` n'est jamais exposé à Search.
* `DeliveryArea` reste en réserve.
