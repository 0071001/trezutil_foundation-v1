# Product Specification — Catalog, Listing & Location Discovery

## Vision

Trezutil doit permettre à un vendeur de publier une offre, et à un acheteur de la découvrir.

Le concept central n’est pas `Product`. C’est `Listing`.

Un `Listing` est une offre publiée par un `Seller` dans un `Store`, éventuellement rattachée à une `Organization`, avec :

* une localisation
* un prix
* une disponibilité
* des attributs
* des médias
* un cycle de vie

Le système doit être générique :

* produit physique
* immobilier
* véhicule
* alimentation
* service
* autre offre

## Principes

1. **NO CODE BEFORE CONTRACTS** : ce sprint est purement contractuel et architectural.
2. **Listing first** : le modèle métier est centré sur `Listing`, pas sur `Product`.
3. **Multi-tenant** : Organization / Seller / Store gardent leurs frontières.
4. **Généricité** : un même noyau doit servir plusieurs types d’offres.
5. **Discovery first** : la localisation et la recherche sont des citoyens de première classe.
6. **Sécurité** : ownership et isolation vérifiés côté serveur.
7. **Événements** : les changements d’état sont exprimés en events.

## Livrables attendus

* Modèle de domaine Listing / Category / Attribute / Media / Availability / Location
* Contrats API
* Contrats events
* Architecture de recherche
* Stratégie de tests
* ADRs

## Non-objectifs Sprint 3

* implémenter les handlers
* implémenter les repositories
* créer les migrations
* coder le Discovery Engine
* coder le Search Engine
* intégrer Payment / Logistics / WhatsApp / AI
