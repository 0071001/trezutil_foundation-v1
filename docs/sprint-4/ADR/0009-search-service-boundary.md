# ADR 0009 — Search Service Boundary

## Contexte

Trezutil doit supporter la recherche et la découverte de listings.

Le système actuel contient plusieurs services : identity-service, catalog-service, seller-service, etc.

La question est : Search doit-il être un service dédié ou intégré à un service existant ?

## Problème

Où placer la logique de Search & Discovery dans l'architecture ?

## Options

### Option A — Service dédié `search-service`

* Service indépendant avec sa propre base de code, déploiement et scaling.
* Avantages : isolation des responsabilités, scaling indépendant, contrat API clair.
* Inconvénients : overhead opérationnel, duplication de certaines connexions.

### Option B — Intégré dans `catalog-service`

* La logique de search vit dans le même service que le catalog.
* Avantages : pas de nouveau service, partage de connexions DB.
* Inconvénients : coupling, scaling couplé, mélange de responsabilités transactionnelles et de recherche.

### Option C — Module dans `api-gateway`

* Le gateway fait lui-même la recherche.
* Avantages : pas de service supplémentaire.
* Inconvénients : gateway devient un singleton intelligent, pas de scaling indépendant, violation des responsabilités.

## Décision

**Option A — Service dédié `search-service`**

Search est un service dédié pour les raisons suivantes :

1. Search a des exigences de scaling, de latence et de disponibilité distinctes des opérations transactionnelles du catalog.
2. L'indexation search est asynchrone et éventuellement consistante ; la mélanger avec les handlers transactionnels ajoute de la surface de panne.
3. Search a ses propres besoins : parsing de requête, calcul géo, ranking, cache.
4. La Conversation Engine et la couche AI dépendront de Search ; Search ne doit pas dépendre d'eux.
5. Une extraction future vers un service indépendamment déployable est plus facile si la frontière est explicite dès le début.

## Conséquences

* `search-service` est ajouté à la liste des services.
* Search expose des APIs REST et gRPC internes.
* Search consomme des événements Kafka pour maintenir son index.
* PostgreSQL reste la source de vérité transactionnelle.
* OpenSearch (futur) est le moteur de recherche ; Search Service en est l'orchestrateur.
