# Attribute Model

## Concept

Le système d’attributs permet de décrire un `Listing` avec des champs spécifiques à sa catégorie, sans créer un modèle EAV anarchique.

Il repose sur :

```text
AttributeDefinition
    ↓
AttributeValue
    ↓
ListingAttribute
```

## Entités

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

Règles :

* `key` est unique par catégorie.
* `data_type` définit le type de valeur.
* `unit` est optionnel (ex: `cm`, `kg`, `km`).
* `validation_json` contient les règles de validation.
* `searchable` indique si l’attribut participe à la recherche.
* `filterable` indique si l’attribut peut être utilisé dans un filtre.

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

Règles :

* Un seul `value_*` est rempli selon `data_type`.
* `value_json` est utilisé pour :
  * les enum
  * les valeurs complexes
  * les extensions futures

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

Règles :

* `ListingAttribute` peut dénormaliser la valeur pour éviter des jointures coûteuses.
* `attribute_value_id` permet de réutiliser une `AttributeValue` existante.
* `value_*` permet de stocker la valeur directement.

## Data types

```text
TEXT
NUMBER
BOOLEAN
DATE
ENUM
JSON
```

### TEXT

* chaîne de caractères
* ex: `Samsung`, `Black`, `Toyota`

### NUMBER

* nombre
* ex: `12`, `256`, `180`

### BOOLEAN

* vrai / faux
* ex: `Furnished = true`

### DATE

* date
* ex: `2023-01-01`

### ENUM

* choix parmi valeurs prédéfinies
* ex: `Fuel = Diesel | Petrol | Electric`
* stocké dans `value_json` sous forme de tableau ou de clé

### JSON

* objet ou tableau libre
* utilisé pour des cas complexes
* ex: `{ "brand": "Samsung", "model": "S25", "colors": ["Black", "White"] }`

## Validation

`validation_json` est un objet JSON dont la structure dépend de `data_type`.

Exemples :

```json
{
  "type": "number",
  "min": 0,
  "max": 1000000,
  "unit": "USD"
}
```

```json
{
  "type": "enum",
  "values": ["Diesel", "Petrol", "Electric", "Hybrid"]
}
```

```json
{
  "type": "text",
  "maxLength": 255,
  "pattern": "^[A-Za-z]+$"
}
```

Règles :

* La validation est effectuée au niveau application, pas stockée pour exécution.
* `validation_json` est utilisé par l’UI et les tests.

## Search et filtre

* `searchable = true` : l’attribut est indexé pour la recherche full-text.
* `filterable = true` : l’attribut peut être utilisé dans un filtre de Discovery.

Exemples :

```text
Category: Smartphone
  Attribute: Brand       → searchable, filterable
  Attribute: Model       → searchable, not filterable
  Attribute: RAM         → filterable, not searchable
  Attribute: Color       → filterable, not searchable
```

## Attributs par catégorie

Exemples :

### Smartphone

```text
Brand      TEXT       searchable, filterable
Model      TEXT       searchable
RAM        NUMBER     filterable, unit GB
Storage    NUMBER     filterable, unit GB
Color      TEXT       filterable
Screen     NUMBER     filterable, unit inches
```

### Vehicle

```text
Brand       TEXT       searchable, filterable
Model       TEXT       searchable
Year        NUMBER     filterable
Fuel        ENUM       filterable
Transmission ENUM      filterable
Mileage     NUMBER     filterable, unit km
```

### Property

```text
Bedrooms    NUMBER     filterable
Bathrooms   NUMBER     filterable
Surface     NUMBER     filterable, unit m²
Furnished   BOOLEAN    filterable
Type        ENUM       filterable
```

## Règles métier

* Un `Listing` ne peut avoir qu’une seule valeur par `AttributeDefinition`.
* Un `AttributeDefinition` appartient à une `Category`.
* La suppression d’un `AttributeDefinition` est interdite si des `ListingAttribute`s l’utilisent.
* Les opérations sur `AttributeDefinition` doivent vérifier l’isolation par `organization_id` via la `Category`.
