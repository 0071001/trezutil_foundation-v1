# Intégration frontend ↔ backend de Trezutil

## A. État de l'intégration

**Routes.** La gateway et les services enregistrent 139 routes. Il y en avait 137 au départ, plus les deux routes
d'historique ajoutées. Elles se répartissent ainsi :

| Domaine | Routes | Côté client web |
|---|---|---|
| Infrastructure (`/healthz`, `/readyz`, `/metrics`, `/csp-report`, statique) | 10 | santé et déploiement |
| Identité et session | 6 | **5 intégrées** (`staff/login` relève de l'administration) |
| Conversations | 6 | **5 intégrées**, dont les 2 nouvelles (`GET /conversations`, `GET /conversations/{id}/messages`) |
| Missions (Organiser) | 10 | **10 intégrées** (création, lecture, `PATCH` statut et instructions, plan, exécution d'une étape, run, resume, état d'exécution) |
| Commandes | 7 | **3 intégrées** (liste, détail, annulation) ; `incoming` et `confirm` sont côté vendeur |
| Réservations | 6 | **2 intégrées** (liste, annulation) ; la création passe par le runtime de conversation |
| Paiements | 8 | **1 intégrée** (`POST /payments`) ; les webhooks arrivent du fournisseur |
| Catalogue et recherche (`/listings`, `/search`, `/categories`, `/verticals`) | 17 | **2 utilisées** (détail d'une offre ; la recherche passe par le runtime) |
| Vendeurs, organisations, équipes, boutiques, console vendeur | 22 | **2 intégrées** (`/sellers/me`, `/sellers/usage`) |
| Opérations vendeur (stocks, hôtels, livraisons, disponibilité) | 17 | hors périmètre client |
| Administration | 22 | hors périmètre client |
| Webhooks (WhatsApp, paiements) | 4 | appelés par les fournisseurs, pas par l'interface |

Au total, **31 routes** sont appelées par l'interface client. Chaque appel passe par le client HTTP unique
(`web/frontend/src/api/client.ts`) : jeton, rafraîchissement de session, délais, annulation, erreurs typées. Il est
ensuite regroupé dans un module par domaine (`api/conversations.ts`, `missions.ts`, `commerce.ts`, `listings.ts`,
`seller.ts`, `console.ts`, `auth.ts`). Aucun composant n'appelle `fetch` directement.

### Matrice des fonctionnalités

| Domaine | Frontend | API | Service métier | Base | État |
|---|---|---|---|---|---|
| Authentification | Connexion, profil, déconnexion | `/auth/*` | identity | `users`, `otp_challenges` | **Opérationnel**, mais l'envoi des codes demande WhatsApp en production |
| Conversations | Fil, confirmations, relance, modification | `/conversations/*` | conversation (Commerce Action Runtime) | `conversations`, `messages` | **Opérationnel** |
| Historique | Barre latérale, relecture | 2 nouvelles routes | `HistoryService` | idem | **Opérationnel** ; renommer, archiver et supprimer restent locaux |
| Recherche commerciale | Cartes d'offres | via le runtime, `/search` | search (OpenSearch) | projection | **Opérationnel** ; le catalogue local est vide |
| Détail d'une offre | Panneau latéral | `GET /listings` | search | `listings`, médias | **Opérationnel** |
| Comparaison | Tableau, export CSV | runtime (`COMPARE_LISTINGS`) | conversation | — | **Opérationnel**, sur les données renvoyées |
| Fournisseurs | Liste, niveaux de confiance, favoris | `/sellers/me` | identity | `seller_profiles` | **Partiel** : pas d'inscription en libre-service |
| Offres, devis, négociation | Panneau explicatif | — | **absent** | — | **Non implémenté** dans le backend |
| Commandes | Liste, détail, annulation, paiement | `/orders`, `/payments` | order, payment | `orders`, `payments` | **Opérationnel** ; le paiement effectif demande les clés Stripe |
| Réservations | Liste, annulation | `/reservations` | order | `reservations` | **Opérationnel** |
| Missions | Opération, chronologie, Commerce Flow, pilotage | `/missions/*` | conversation (planification, orchestration) | `missions`, `mission_*` | **Opérationnel** |
| Crédits | Solde et mouvements d'une organisation gérée | `/sellers/usage` | usage | `usage_*` | **Partiel** : pas de solde personnel exposé |
| Notifications | Centre alimenté par les vraies données | — | `notification-service` est un squelette | — | **Partiel** |
| Documents | Prévisualisation locale | — | **absent** | — | **Non implémenté** |
| Mémoire commerciale | Paramètres, transmise aux missions | `preferences` des missions | — | — | **Partiel** : stockage sur l'appareil |
| Voix | Dictée, mode vocal tour par tour | — | navigateur | — | **Partiel** : pas de temps réel côté serveur |

## B. Changements apportés au backend

Ces changements sont additifs. Aucun contrat existant n'est modifié et aucune migration n'est ajoutée.

1. **Lecture de l'historique des conversations.** Conversations et messages étaient déjà enregistrés, mais aucune
   route ne permettait de les relire. Les ajouts :
   - `domain.ConversationHistoryReader`, une interface séparée, pour que les stores et les doublures de test
     existants restent inchangés ;
   - `postgres.ConversationStore.ListByCustomer` ;
   - `conversation.HistoryService`, qui applique la même règle de propriété que le processeur ;
   - les handlers et routes `GET /api/v1/conversations` et `GET /api/v1/conversations/{id}/messages`.

   Les messages sont horodatés à la seconde. Chaque réponse est donc replacée après sa question grâce à la clé
   `reply:<clé>`, et l'ordre d'insertion est départagé par `xmin` (dans `ListByCustomer` et
   `MessageStore.FindByConversationID`).
2. **Tests sur base PostgreSQL** (`infrastructure/postgres/integration` et `internal/application/reservations`).
   - Serveur configurable par `TREZUTIL_TEST_PG_*`. Sans ces variables, les tests visaient `localhost:5432`, qui est
     sur ce poste le **PostgreSQL d'Odoo**, alors qu'ils suppriment et recréent une base.
   - Délais de préparation portés de 10 s à 60 s.
3. **Contrat OpenAPI.** Ajout des deux routes, et correction d'une indentation à la ligne 95.

## C. Qualité technique

| Vérification | Résultat |
|---|---|
| Tests frontend (vitest) | **51/51** |
| Typecheck frontend, build de production | OK. JS 438 kB (133 kB gzip), polices servies par l'application |
| Tests Go unitaires (`go test ./...` hors intégration) | **29 paquets OK** ; 6 nouveaux tests de handlers ; tests de réservation sur base jetable : OK |
| Tests d'intégration PostgreSQL (base jetable isolée) | Tous réussissent, **sauf 1 échec antérieur** (voir D) |
| Parcours réel sur la pile Docker (`test/e2e/api-integration.mjs`) | **14/14** |
| Parcours navigateur, build de production servi par la gateway | Historique multi-appareils, opérations, paiement ; 0 erreur console, 0 requête externe |

Le parcours réel couvre :
- l'inscription et la connexion ;
- le profil et le rafraîchissement de session ;
- l'envoi d'un message et le message rejoué (idempotence) ;
- l'historique et son isolation entre comptes ;
- le refus sans session ;
- la recherche ;
- le cycle de vie d'une mission et son isolation ;
- les commandes et réservations ;
- les permissions sur les crédits ;
- les jetons invalides.

**Sécurité vérifiée.**
- La propriété est contrôlée côté serveur : 403 pour la conversation d'un autre compte ou d'un invité non rattaché,
  401 sans session.
- `Cache-Control: no-store` sur les données personnelles.
- Le transcript ne contient que le texte, jamais les données internes du processeur.
- La limitation des demandes de code fonctionne (429 constaté).
- La CSP autorise uniquement Stripe.
- Aucun secret dans Git (`app.env` est ignoré).
- Le masquage d'un bouton n'est jamais utilisé comme protection.

## D. Problèmes antérieurs constatés, non corrigés

- **Le contrat OpenAPI n'est pas un YAML valide.** Plusieurs clés `post:` sont dupliquées sur un même chemin, et un
  niveau d'imbrication superflu apparaît (`get: users-me-roles:`). Aucun outil OpenAPI ne peut le lire. Le corriger
  demande une réécriture.
- **`TestMissionPlans_MigrationDownAndUp` échoue.** Le retour arrière de la migration 000028 est impossible, car
  000029 et 000030 dépendent de `plan_version`. Ce problème précède ce travail : les migrations n'ont pas été
  modifiées.
- **Les parcours navigateur `test/e2e/web/journeys.spec.mjs` sont obsolètes.** Ils pilotaient l'ancienne interface,
  qui a été remplacée. À réécrire pour la nouvelle.

## E. Déploiement

Préparé pour **https://trezutil.com** sur le VPS partagé avec Odoo : voir `docs/deployment/VPS_DEPLOYMENT.md`.

- **Fichiers.**
  - `deployments/docker/docker-compose.vps.yml` : gateway sur `127.0.0.1:18080`, mémoire plafonnée à environ 3,6 Go
    (mesuré au repos : 1,2 Go) ;
  - `deployments/nginx/trezutil-bootstrap.conf` puis `trezutil.conf` : un bloc `server` de plus, www redirigé vers
    l'apex, HSTS ;
  - `scripts/ops/deploy.sh` (vérifications, sauvegarde, build, migration, santé) et `scripts/ops/rollback.sh`.
- **Ports.** Trezutil n'expose que `127.0.0.1:18080`. Il ne prend ni 80/443 (nginx), ni 8069/8072 (Odoo), ni 5432.
- **Variables obligatoires.** `APP_ENV=production`, `POSTGRES_PASSWORD`, `JWT_SECRET`, `OTP_CODE_SECRET`,
  `CORS_ALLOWED_ORIGINS=https://trezutil.com`, `TRUSTED_PROXY_HOPS=1`, `GATEWAY_LOCAL_PORT=18080`, `WHATSAPP_*`.
- **Retour arrière.** `trezutil:previous` est conservée automatiquement. La base se restaure manuellement depuis la
  sauvegarde prise avant migration.

## F. Points bloquants pour une mise en production réelle

1. **Accès SSH au VPS** et **enregistrements DNS** de trezutil.com (A `@`, CNAME `www`).
2. **WhatsApp Business** (`WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_OTP_TEMPLATE` approuvé).
   Sans eux, aucun utilisateur ne peut se connecter en production.
3. **Clés Stripe** pour encaisser. Aucun adaptateur Mobile Money n'existe, ce qui pèse pour le marché congolais.
4. **Catalogue** : aucun fournisseur ni aucune offre en base. Les recherches renverront « rien trouvé » tant que des
   vendeurs n'auront pas publié. L'inscription des fournisseurs est réservée aux administrateurs.
5. **Fonctions absentes du backend** : négociation, propositions et devis, documents et analyse, notifications push,
   mémoire côté serveur, streaming.

## G. Verdict

**READY WITH LIMITATIONS.**
- **Ce qui fonctionne.** L'application se construit, se déploie et fonctionne de bout en bout sur ses fonctions
  réelles : conversation et runtime, historique, missions, commandes, réservations, paiement si Stripe est configuré,
  détail des offres, comptes.
- **Le prérequis non négociable.** La production ne peut pas ouvrir tant que WhatsApp n'est pas configuré, puisque
  personne ne pourrait se connecter.
- **Ce qui reste absent.** Négociation, documents et notifications n'existent pas dans le backend. L'interface les
  présente comme indisponibles et ne simule rien.
