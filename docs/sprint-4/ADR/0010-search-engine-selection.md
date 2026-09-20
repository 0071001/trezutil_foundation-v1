# ADR 0010 — Search Engine Selection

## Contexte

Trezutil doit supporter la recherche plein-texte, les filtres dynamiques, les requêtes géospatiales et le ranking.

Les options envisagées sont :

1. PostgreSQL uniquement
2. OpenSearch
3. Hybride : PostgreSQL + OpenSearch

## Problème

Quelle architecture de moteur de recherche choisir pour supporter les exigences de Search & Discovery ?

## Options

### Option A — PostgreSQL uniquement

* Utilise PostgreSQL pour tout : filtres exacts, full-text search (`tsvector`), géo-requêtes.
* Avantages : pas de dépendance supplémentaire, opérationnellement simple, données cohérentes.
* Inconvénients : `tsvector` limité pour le ranking, pas de facettes natives, géo-requêtes complexes coûteuses, scaling limité, pas de recherche sémantique.

### Option B — OpenSearch uniquement

* OpenSearch comme moteur principal ; PostgreSQL reste la source de vérité mais n'est pas interrogé pour les recherches.
* Avantages : recherche plein-texte avancée, géo-requêtes performantes, facettes, scaling horizontal.
* Inconvénients : complexité opérationnelle, synchronisation des données, cohérence éventuelle, surcharge pour des filtres exacts simples.

### Option C — Hybride PostgreSQL + OpenSearch

* PostgreSQL pour les filtres exacts et la source de vérité.
* OpenSearch pour la recherche plein-texte, géo et ranking.
* Search Service orchestre les deux.
* Avantages : meilleur de chaque monde, filtres exacts rapides dans PG, recherche avancée dans OpenSearch.
* Inconvénients : deux systèmes à maintenir, complexité de synchronisation.

## Décision

**Option C — Hybride PostgreSQL + OpenSearch**

Le Search Service utilise :

* **PostgreSQL** pour :
  * filtres exacts (category, listing_type, price, currency, availability, tenant)
  * validation des droits d'accès
  * données transactionnelles de référence

* **OpenSearch** pour :
  * recherche plein-texte (title, description, category path, attributes)
  * requêtes géospatiales (rayon, distance)
  * facettes et agrégations
  * ranking et pertinence

### Justification

* PostgreSQL est déjà déployé et maîtrisé.
* OpenSearch apporte les capacités de recherche avancée que PostgreSQL ne peut pas fournir à grande échelle.
* La séparation permet de choisir le bon outil pour chaque type de requête.
* L'architecture hybride est cohérente avec la stratégie documentée dans `ARCHITECTURE.md` (PostgreSQL = source de vérité, OpenSearch = index de recherche dérivé).

### Non-objectifs

* Implémenter OpenSearch dans ce sprint.
* Implémenter le client OpenSearch.
* Implémenter la synchronisation.

## Conséquences

* Le Search Service doit maintenir deux connexions : PostgreSQL et OpenSearch.
* La synchronisation des index passe par des événements de domaine.
* Les requêtes hybrides nécessitent un orchestrateur dans Search Service.
* La cohérence est éventuelle pour le côté OpenSearch.
