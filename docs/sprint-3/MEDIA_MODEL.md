# Media Model

## Concept

Un `Listing` peut contenir plusieurs médias :

* images
* vidéos
* documents
* audio

Le modèle est conçu pour un stockage objet (MinIO / S3), pas pour du stockage BLOB PostgreSQL.

## Entité

### Media

```text
id
listing_id
type
url
object_key
mime_type
size
width
height
duration
ordering
primary
metadata
created_at
updated_at
```

Règles :

* `object_key` est la clé dans le stockage objet.
* `url` est l’URL publique ou signée.
* `mime_type` permet au client de choisir le renderer.
* `ordering` permet de trier les médias.
* `primary` indique le média principal.
* `metadata` contient des informations supplémentaires (ex: `alt`, `caption`, `exif`).

## Types

```text
IMAGE
VIDEO
DOCUMENT
AUDIO
```

## Champs spécifiques

### IMAGE

* `width`
* `height`
* `size`
* `mime_type` ex: `image/jpeg`, `image/png`, `image/webp`

### VIDEO

* `duration`
* `width`
* `height`
* `size`
* `mime_type` ex: `video/mp4`

### DOCUMENT

* `size`
* `mime_type` ex: `application/pdf`

### AUDIO

* `duration`
* `size`
* `mime_type` ex: `audio/mpeg`

## Storage

Principe :

```text
Client / Service
    ↓
Object Storage (MinIO / S3)
    ↓
Media ( référence object_key )
    ↓
Listing
```

Règles :

* Le stockage objet est géré par l’infrastructure, pas par le domaine.
* Le domaine ne connaît que `object_key` et `url`.
* La suppression d’un `Listing` ne supprime pas automatiquement les médias du stockage objet ; cela doit être géré par un processus séparé.

## Primary media

* Un seul `Media` par `Listing` peut avoir `primary = true`.
* Si aucun média n’est marqué `primary`, le premier par `ordering` est considéré comme principal par l’UI.

## Ordering

* `ordering` est un entier.
* Plus la valeur est basse, plus le média apparaît tôt.
* Valeur par défaut : `0`.

## Métadonnées

```text
metadata : JSONB
```

Utilisé pour :

* `alt` text pour l’accessibilité
* `caption`
* `exif`
* extensions futures

## Règles métier

* Un `Listing` peut avoir zéro ou plusieurs `Media`s.
* Un `Media` appartient à un seul `Listing`.
* La suppression d’un `Listing` entraîne la suppression des `Media`s en base, mais pas dans le stockage objet.
* Les opérations sur `Media` doivent vérifier l’isolation par `organization_id` via le `Listing` / `Store`.
