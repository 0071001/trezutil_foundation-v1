# Location Model

## Principe

La localisation est un composant stratégique de Trezutil.

Elle ne se réduit pas à trois champs :

```text
city
latitude
longitude
```

Le modèle doit supporter :

* une hiérarchie administrative
* une position géographique optionnelle
* une précision variable
* plusieurs usages : Listing, User, Discovery

## Hiérarchie

```text
Country
    ↓
Province
    ↓
City
    ↓
Municipality / Commune
    ↓
Neighborhood / Quartier
    ↓
Zone
```

Règles :

* Chaque niveau est optionnel.
* Un `Listing` peut être identifié par :
  * `Country` + `City` seulement
  * `Country` + `Province` + `City` + `Neighborhood`
  * `latitude` + `longitude` seuls
  * une combinaison des deux
* `latitude` / `longitude` sont optionnels.
* `accuracy` reflète la précision de la position.

## Entités

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

Règles :

* Un `Listing` peut avoir plusieurs `ListingLocation`.
* Une seule `ListingLocation` peut être `is_primary = true`.
* `label` est utilisé pour :
  * “Point de retrait”
  * “Lieu de prestation”
  * “Entrepôt”
  * “Adresse du bien”

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

Règles :

* `UserLocation` ne doit jamais être confondue avec `ListingLocation`.
* `source` peut être :
  * `GPS`
  * `USER_INPUT`
  * `SEARCH_CONTEXT`
  * `DERIVED`
* `accuracy` indique la précision.
* `captured_at` indique quand la position a été capturée.
* La confidentialité doit être prise en compte :
  * ne pas stocker de position précise en permanence sans consentement
  * permettre la suppression
  * ne pas exposer `UserLocation` dans les réponses publiques

## Distinctions obligatoires

| Concept | Usage | Exemple |
|---|---|---|
| `ListingLocation` | Où se trouve l’offre | Adresse d’un restaurant |
| `UserLocation` | Où se trouve l’utilisateur | Position GPS de l’acheteur |
| `DiscoveryQuery.location` | Où l’utilisateur veut chercher | “Trouve-moi un restaurant à Gombe” |
| `DeliveryArea` | Où le vendeur livre | Zones de livraison d’un restaurant |

Ces concepts ne doivent pas être fusionnés dans le domaine.

## Cas d’usage

### Restaurant

```text
ListingLocation 1 (primary)
    label = "Restaurant"
    location = exact address

DeliveryArea
    cities = [Gombe, Ngaliema, Bandalungwa]
```

### Service de plomberie

```text
ListingLocation 1 (primary)
    label = "Base"
    location = exact address

ListingLocation 2
    label = "Service area"
    location = city-level only
```

### Maison à vendre

```text
ListingLocation 1 (primary)
    label = "Property"
    location = exact address + latitude/longitude
```

### Produit en entrepôt

```text
ListingLocation 1 (primary)
    label = "Warehouse"
    location = city + province
```

## Recommandations

1. Ne pas contraindre `Listing` à une seule `ListingLocation`.
2. Ne pas stocker `latitude` / `longitude` obligatoirement.
3. Ne jamais exposer `UserLocation` dans les réponses publiques.
4. Prévoir `source` et `accuracy` pour toute localisation capturée.
5. Prévoir un mécanisme de suppression/oubli pour `UserLocation`.
