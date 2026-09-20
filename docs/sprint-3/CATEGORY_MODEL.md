# Category Model

## Concept

Le système de catégories permet de classer les `Listing`s.

Il doit être :

* hiérarchique
* extensible
* compatible avec le multi-tenant
* utilisable pour la recherche et les filtres

## Entité

### Category

```text
id
parent_id
name
slug
description
status
display_order
is_system
organization_id
created_at
updated_at
```

Règles :

* `parent_id` permet la hiérarchie.
* `slug` est unique au sein d’un scope.
* `is_system` indique une catégorie système, non supprimable par les utilisateurs.
* `organization_id` permet des catégories spécifiques à une organisation.
* `status` permet de désactiver une catégorie sans la supprimer.

## Scope

### Catégories globales

```text
organization_id = null
```

Utilisées pour :

* catégories système
* catégories partagées entre organisations

Exemples :

```text
Électronique
Immobilier
Véhicules
Alimentation
Services
```

### Catégories par organisation

```text
organization_id = <org id>
```

Utilisées pour :

* catégories spécifiques à une organisation
* sous-catégories privées

Règle :

* Une catégorie enfant hérite du scope de son parent.
* Une catégorie globale ne peut pas avoir un parent spécifique à une organisation.

## Hiérarchie

Exemples :

```text
Électronique
└── Téléphones
    └── Smartphones
        └── Android

Immobilier
└── Location
    └── Appartement
    └── Maison

Services
└── Réparation
    └── Électricité
    └── Plomberie
```

Profondeur :

* variable
* pas de limite fixe imposée au domaine
* l’UI peut imposer une limite si nécessaire

## Slug

* unique par scope (`organization_id`, `parent_id`)
* lisible dans les URLs
* stable dans le temps

## Status

```text
ACTIVE
INACTIVE
```

* `INACTIVE` : la catégorie existe mais n’est plus utilisable pour de nouveaux Listings.
* Les Listings existants ne sont pas supprimés.

## Display order

* `display_order` permet de trier les catégories dans l’UI.
* Valeur par défaut : `0`.

## Catégories système

* `is_system = true`
* non supprimables par les utilisateurs
* modifiables uniquement par des opérateurs de plateforme
* utilisées comme base pour la classification

## Attributs par catégorie

Voir `ATTRIBUTE_MODEL.md`.

Une `Category` est reliée à des `AttributeDefinition`s :

```text
Category
    ↓
CategoryAttribute
    ↓
AttributeDefinition
```

Cette relation définit :

* quels attributs sont disponibles pour une catégorie
* l’ordre d’affichage
* si l’attribut est requis

## Relations

```text
Listing
    ↓
Category

Category
    ↓
CategoryAttribute
    ↓
AttributeDefinition

AttributeDefinition
    ↓
AttributeValue
```

## Règles métier

* Un `Listing` doit appartenir à une `Category` active.
* Une `Category` inactive ne peut pas recevoir de nouveaux `Listing`s.
* La suppression d’une `Category` est interdite si elle contient des `Listing`s actifs.
* Les opérations sur `Category` doivent vérifier l’isolation par `organization_id`.
