# Discovery Model

## Concept

Le Discovery Engine permet à un utilisateur de trouver des `Listing`s selon :

* un texte de recherche
* une catégorie
* des attributs
* un prix
* une localisation
* une disponibilité

Il ne doit pas être limité à `ORDER BY distance`.

## Entités conceptuelles

### DiscoveryQuery

```text
query_text
category_id
attributes
price_range
currency
location
radius
availability
seller_trust
delivery_required
sort
page
page_size
```

Champs :

* `query_text` : texte libre.
* `category_id` : filtre par catégorie.
* `attributes` : filtres par attributs dynamiques.
* `price_range` : `{ min, max }`.
* `currency` : devise.
* `location` : `DiscoveryLocation`.
* `radius` : rayon en mètres.
* `availability` : statut de disponibilité.
* `seller_trust` : seuil de confiance vendeur.
* `delivery_required` : filtre sur la livraison.
* `sort` : `RELEVANCE`, `DISTANCE`, `PRICE_ASC`, `PRICE_DESC`, `NEWEST`.
* `page`, `page_size` : pagination.

### DiscoveryLocation

```text
type

# CITY, NEIGHBORHOOD, EXACT, USER
value
radius
```

Règles :

* `type = CITY` : recherche par ville.
* `type = NEIGHBORHOOD` : recherche par quartier.
* `type = EXACT` : recherche par coordonnées exactes.
* `type = USER` : utilise la `UserLocation` de l’utilisateur.

### DiscoveryResult

```text
listing_id
title
price_amount
price_currency
pricing_model
distance
score
media
availability
seller
store
location
```

## Ranking

Le `DiscoveryScore` combine :

* `Relevance` : pertinence texte / attributs
* `Distance` : distance géographique
* `Price` : proximité du prix souhaité
* `Availability` : disponibilité
* `SellerTrust` : confiance vendeur
* `Delivery` : capacité de livraison
* `Freshness` : ancienneté du Listing

Règle :

* Le scoring exact est hors scope Sprint 3.
* Le modèle doit prévoir un `score` et un `sort` suffisants pour le MVP.

## Géodécouverte

Requêtes supportées :

```text
"Trouve-moi un téléphone près de moi."
"Trouve-moi une maison à Gombe."
"Trouve-moi un restaurant à moins de 5 km."
"Trouve-moi une voiture à Kinshasa."
"Trouve-moi un plombier disponible aujourd'hui près de moi."
```

Chaque requête est transformée en `DiscoveryQuery` par l’AI Commerce Agent.

Exemple :

```json
{
  "query_text": "téléphone",
  "category_id": "smartphones",
  "location": {
    "type": "USER",
    "radius": 5000
  },
  "sort": "DISTANCE"
}
```

## Architecture cible

```text
Client
    ↓
API Gateway
    ↓
Discovery Service
    ↓
PostgreSQL + OpenSearch
```

Responsabilités :

* PostgreSQL : données transactionnelles, filtres exacts, contraintes.
* OpenSearch : recherche full-text, filtres dynamiques, géo-requêtes.

Voir `SEARCH_ARCHITECTURE.md`.

## Règles métier

* Un `Listing` doit être `PUBLISHED` pour apparaître dans Discovery.
* Un `Listing` expiré ou `SOLD` ne doit pas apparaître.
* Les attributs `searchable` et `filterable` définissent la participation au Discovery.
* La localisation du `Listing` détermine la distance.
* `UserLocation` n’est jamais exposée publiquement.
