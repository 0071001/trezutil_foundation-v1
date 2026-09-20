# ADR 0012 — Ranking Architecture

## Contexte

Search & Discovery doit retourner des résultats classés par pertinence.

Les signaux potentiels incluent :

* Pertinence textuelle (title, description, catégorie, attributs)
* Correspondance d'attributs
* Distance géographique
* Prix
* Fraîcheur (date de publication)
* Confiance du vendeur
* Disponibilité
* Popularité (futur)

## Problème

Comment concevoir un système de ranking qui reste déterministe, testable, et évolutif sans hardcoder de poids arbitraires ?

## Options

### Option A — Scoring black-box

* Un algorithme de scoring opaque calcule un score unique.
* Avantages : simplicité d'implémentation.
* Inconvénients : difficile à tester, difficile à expliquer, difficile à ajuster.

### Option B — Scoring déterministe par signaux pondérés

* Chaque signal est calculé séparément.
* Les signaux sont combinés par une formule documentée.
* Avantages : testable, explicable, ajustable.
* Inconvénients : nécessite de maintenir les poids.

### Option C — Scoring multi-phase

* Phase 1 : filtres exacts (élimination)
* Phase 2 : scoring de pertinence (texte + catégorie + attributs)
* Phase 3 : ranking par signaux (distance, prix, fraîcheur, confiance)
* Avantages : séparation claire, optimisation par phase.
* Inconvénients : plus complexe.

## Décision

**Option C — Scoring multi-phase**

### Phase 1 — Filtrage (Filtering)

Les filtres sont déterministes et binaires. Un listing correspond ou ne correspond pas.

Filtres appliqués :
* `status = PUBLISHED`
* `tenant_id` (si recherche interne)
* `category` (si spécifié)
* `listing_type` (si spécifié)
* `price_range` (si spécifié)
* `currency` (si spécifié)
* `availability` (si spécifié)
* `seller_id` / `store_id` (si spécifié)
* `attributes` (si spécifié)
* `location` + `radius` (si spécifié)

### Phase 2 — Pertinence (Relevance Scoring)

La pertinence textuelle est calculée par OpenSearch (BM25 ou similaire).

La correspondance de catégorie et d'attributs est incluse dans le score de pertinence.

### Phase 3 — Ranking Signals (Ranking)

Les signaux de ranking sont appliqués après le filtrage et la pertinence :

| Signal | Type | Direction | Weight (initial) |
|--------|------|-----------|------------------|
| Text relevance | computed | higher = better | base |
| Category match | boolean | exact = better | +10% |
| Attribute match | count | more matches = better | +5% per match |
| Distance | computed | closer = better | distance penalty |
| Price | relative | competitive = better | configurable |
| Freshness | computed | newer = better | time decay |
| Seller trust | integer | higher = better | +trust_level * 2% |
| Availability | boolean | in_stock = better | +5% |
| Media richness | integer | more media = better | +1% per item |

### Distance Penalty

La distance ne doit pas automatiquement dominer tous les autres signaux.

Formule proposée :

```
distance_penalty = max(0, 1 - (distance_meters / max_radius_meters))
final_score = relevance_score * (1 + distance_penalty * 0.3 + ...)
```

Cela signifie qu'un listing à distance maximale perd 30% de son score de pertinence, mais peut toujours gagner s'il est beaucoup plus pertinent.

### Tie Breaking

En cas d'égalité de score :

1. `published_at` décroissant (plus récent d'abord)
2. `listing_id` croissant (déterministe)

### Explicabilité

Le système doit pouvoir expliquer pourquoi un résultat est classé ainsi :

* Score de pertinence textuelle
* Signaux de ranking appliqués
* Distance

Cela est nécessaire pour :
* le debugging
* la confiance utilisateur
* l'ajustement des poids

### Non-Goals

* Pas de personnalisation basée sur l'historique utilisateur dans ce sprint.
* Pas de "meilleure vente" sans règle métier explicite.
* Pas de boosting promotionnel sans règle métier explicite.

## Conséquences

* Le ranking est déterministe et testable.
* Les poids sont configurables mais doivent être versionnés.
* Le ranking reste explicable.
* L'ajout de nouveaux signaux ne nécessite pas de refonte du système.
