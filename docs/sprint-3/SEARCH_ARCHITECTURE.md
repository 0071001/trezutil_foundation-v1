# Search Architecture

## Principe

La recherche et le filtrage doivent être séparés en deux couches :

* **PostgreSQL** : données transactionnelles, filtres exacts, contraintes d’intégrité.
* **OpenSearch** : recherche full-text, filtres dynamiques, géo-requêtes, ranking.

## PostgreSQL

Responsabilités :

* stockage des entités métier
* contraintes d’intégrité
* filtres exacts sur champs indexés
* jointures et agrégations simples
* pagination base

Tables concernées :

```text
listings
listing_attributes
attribute_values
categories
listing_media
listing_availability
locations
listing_locations
```

Index suggérés :

```text
listings.status
listings.listing_type
listings.category_id
listings.organization_id
listings.seller_id
listings.store_id
listings.price_amount
listings.price_currency
listings.published_at
listings.created_at
listing_attributes.attribute_definition_id
listing_attributes.value_text
listing_attributes.value_number
listing_attributes.value_boolean
listing_attributes.value_date
categories.parent_id
categories.slug
categories.status
locations.country
locations.city
locations.neighborhood
```

## OpenSearch

Responsabilités :

* recherche full-text sur `title`, `description`, attributs
* filtres dynamiques sur attributs
* géo-requêtes par rayon, ville, quartier
* tri par pertinence, distance, prix, date
* agrégations pour facettes

Mapping conceptuel :

```json
{
  "listing": {
    "properties": {
      "id": { "type": "keyword" },
      "title": { "type": "text" },
      "description": { "type": "text" },
      "listing_type": { "type": "keyword" },
      "category_id": { "type": "keyword" },
      "price_amount": { "type": "scaled_float", "scaling_factor": 100 },
      "price_currency": { "type": "keyword" },
      "status": { "type": "keyword" },
      "availability_status": { "type": "keyword" },
      "location": { "type": "geo_point" },
      "attributes": {
        "properties": {
          "brand": { "type": "keyword" },
          "model": { "type": "text" },
          "ram": { "type": "integer" }
        }
      },
      "seller": {
        "properties": {
          "id": { "type": "keyword" },
          "trust_score": { "type": "scaled_float", "scaling_factor": 100 }
        }
      },
      "store": {
        "properties": {
          "id": { "type": "keyword" },
          "name": { "type": "text" }
        }
      },
      "published_at": { "type": "date" }
    }
  }
}
```

## Géorequêtes

Exemples :

```text
distance(location, POINT(lon, lat)) < 5000
location within city = "Kinshasa"
location within neighborhood = "Gombe"
```

Options :

* `geo_distance`
* `geo_bounding_box`
* `geo_polygon`

## Synchronisation

Principe :

```text
Listing
    ↓
Domain Event
    ↓
Indexer
    ↓
OpenSearch
```

Règles :

* PostgreSQL est la source de vérité.
* OpenSearch est un index de recherche.
* La synchronisation est asynchrone via événements.
* En cas d’incohérence, PostgreSQL gagne.

## Recommandation

* Utiliser PostgreSQL + OpenSearch.
* PostGIS n’est pas requis pour le MVP si OpenSearch gère le geo.
* Si la recherche géospatiale devient trop complexe, ajouter PostGIS plus tard.

## Non-objectifs Sprint 3

* implémenter OpenSearch
* implémenter l’indexation
* implémenter le geo queries
