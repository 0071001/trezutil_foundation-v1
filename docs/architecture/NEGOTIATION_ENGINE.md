# Moteur de négociation — audit et architecture

État : **proposition**, aucune ligne de code du moteur n'est écrite. Audit fait le 2026-11-22 sur la branche
`feature/commerce-os-web` (commit `deb9059`) et sur la base de production.

## 1. Verdict

| | |
|---|---|
| Développement de la phase 1 (cœur métier + portail fournisseur) | **GO** : tout ce qu'il faut existe dans le code, rien n'attend un tiers. |
| Négociation réelle en production | **NO-GO** : aucun fournisseur n'existe en base, aucun canal ne peut aujourd'hui joindre un fournisseur. |

Le moteur n'aurait aujourd'hui **personne à qui parler** : la base de production contient 0 organisation, 0 profil
vendeur, 0 offre et 0 utilisateur. Construire le moteur reste utile (il sera prêt quand les fournisseurs arrivent), mais
la première valeur réelle viendra de l'inscription des fournisseurs (lot 1, brique 3), qui est le prérequis du canal
« portail fournisseur ».

## 2. Audit : ce qui existe et se réutilise

| Composant existant | Où | Réutilisation dans la négociation |
|---|---|---|
| Conversation + confirmation explicite (jeton à usage unique, expiration) | `internal/application/conversation`, `conversation_confirmations` | Point d'entrée du besoin ; même mécanisme pour « accepter l'offre retenue ». Rien n'est engagé sans ce jeton. |
| Commerce Action Runtime (identité, validation, crédits, idempotence, confirmation) | `internal/application/commerce` | Seule voie vers une commande une fois l'offre acceptée. Le moteur ne crée **jamais** de commande lui-même. |
| Missions (Organiser) : plan, étapes, verrou d'exécution, reprise, exécutions durables | `missionexec`, `missionorch`, migrations 27–30 | Modèle à suivre (verrou par bail, compare-and-set de statut, journal durable des tentatives). Une négociation peut être une étape de mission (`NEGOTIATE`), mais son cycle (réponses asynchrones, tours, échéances) ne tient pas dans une étape synchrone : contexte propre, relié par référence. |
| Passerelle IA multi-fournisseurs, sorties structurées (schéma JSON), bascule, comptage | `internal/application/ai`, `infrastructure/ai` | Extraction des offres et rédaction des contre-propositions. Jamais de décision. |
| Crédits (politiques par action, réservation, consommation, grand livre) | migration 15, `usage.go` | Chaque sollicitation et contre-proposition est une action comptée par les politiques existantes. Pas de seconde facturation. |
| Journal d'audit (append-only) | `audit_events` (migration 25) | Actions sensibles : mandat accordé ou modifié, envoi, acceptation, refus. |
| Offres de dispatch (PENDING → ACCEPTED/DECLINED/EXPIRED, une seule en attente) | migration 22 | Modèle d'états et d'expiration pour les demandes de prix. |
| Boucles périodiques (ticker) | `conversation-service/main.go:476`, `dispatch/service.go` | Échéances, relances, expiration des demandes : même pattern, avec verrou. |
| Mémoire commerciale (serveur) | `commercial_memory_*` | Préférences de l'acheteur (marques, budgets habituels) proposées comme valeurs par défaut du mandat, jamais imposées. |

## 3. Audit : ce qui manque

| Manque | Conséquence |
|---|---|
| **Aucun fournisseur en base**, pas d'inscription libre (POST `/organizations` réservé aux admins) | Rien à solliciter. Prérequis : lot 1, brique 3. |
| Aucune notion de négociation, de demande de prix (RFQ), d'offre fournisseur structurée, de contre-proposition | À créer (§ 6). |
| Aucun canal sortant vers un fournisseur | § 4. |
| Pas de rédaction par l'IA ni de streaming (en cours, lot 1) | L'affichage progressif de la négociation réutilisera le flux SSE prévu pour le streaming. |
| Crédits rattachés à une organisation (`credit_accounts.tenant_id NOT NULL`) | Un acheteur particulier n'a pas de solde (lot 1, brique 2). |
| Claude sans crédit sur le compte Anthropic | Extraction et rédaction IA indisponibles ; le moteur doit fonctionner sans IA (mode dégradé déterministe). |

## 4. Connecteurs : état réel (testé ou lu, pas supposé)

| Canal | État | Preuve | Ce qu'il faut pour qu'il serve à négocier |
|---|---|---|---|
| Portail fournisseur | **Inexistant** (console vendeur en lecture : `/sellers/me`, `/finance`, `/customers`) | code | Inscription fournisseur + boîte de réception des demandes + formulaire d'offre structuré. Entièrement sous notre contrôle : **canal de la phase 1**. |
| WhatsApp sortant | **Inutilisable pour des fournisseurs** | numéro de test Meta (+1 555-173-9351) : n'envoie qu'aux numéros de la liste autorisée (erreur 131030 constatée) ; seul modèle connu : `hello_world`. Le modèle `trezutil_code` n'existe pas sur ce compte (connexion par OTP en panne, 2026-11-22). | Numéro de production vérifié ; modèles « utilitaires » approuvés pour la demande de prix et la contre-proposition (hors fenêtre de 24 h, Meta n'autorise que des modèles). |
| WhatsApp entrant | Fonctionne, **mais pour les acheteurs** : tout message entrant devient une conversation d'achat (`whatsapp/bridge.go`) | code | Routage : un expéditeur reconnu comme contact d'un fournisseur ayant une demande ouverte va vers la négociation, pas vers une nouvelle conversation d'achat. |
| E-mail | **Inexistant** (aucun code SMTP ni fournisseur d'envoi) | recherche dans le dépôt | Choix d'un fournisseur (envoi + réception entrante par webhook), domaine vérifié (SPF/DKIM). |
| notification-service | **Coquille vide** (9 lignes, `log.Printf`) | code | Non bloquant : le moteur a sa propre boîte d'envoi (outbox). |

## 5. Architecture proposée

Un nouveau contexte `negotiation` dans **conversation-service** (là où vivent les missions et la conversation), qui ne
remplace rien : il s'appuie sur le runtime, la passerelle IA, les crédits et l'audit existants.

```text
Conversation ──besoin──▶ NegotiationManager ──▶ MandateGuard (déterministe, autorité finale)
                              │   ▲                     ▲
                              │   │ état, transitions   │ chaque envoi et chaque décision passent ici
                              ▼   │                     │
                         StrategyEngine (règles) ──▶ AINegotiator (propose : extraction, brouillon)
                              │
                              ▼
                     SupplierGateway ── Outbox durable ──▶ ChannelAdapter : Portal | WhatsApp | Email | …
                              ▲                                   │
                              └── Inbox (dédoublonnage) ◀──────────┘  réponses, quel que soit le canal
                              │
                              ▼
                     OfferAnalyzer ──▶ Evaluator ──▶ recommandation ──▶ confirmation explicite de l'acheteur
                                                                          └─▶ Commerce Action Runtime (commande)
```

| Composant | Rôle | Décide ? |
|---|---|---|
| **NegotiationManager** | Cycle de vie, machine à états, verrou d'exécution, reprise. Seul à écrire l'état. | Oui (transitions) |
| **MandateGuard** | Vérifie toute action contre le mandat : plafond, quantité, délais, nombre de tours, durée, canaux permis, confidentialité. Code pur, testé exhaustivement. | Oui (veto) |
| **StrategyEngine** | Phase A (qui solliciter, combien) puis phase B (qui relancer, quel point négocier, quel prix proposer dans la marge autorisée), arrêt. Règles déterministes. | Oui (dans le mandat) |
| **AINegotiator** | Extrait une offre d'un message libre (sortie structurée), rédige le texte d'une demande ou d'une contre-proposition à partir de chiffres **fixés par la stratégie**. | Non : propose, le Guard valide |
| **SupplierGateway** | Résout le contact d'un fournisseur et le canal permis, envoie via une **outbox** (exactement une fois par clé), reçoit via une **inbox** (dédoublonnage par canal + identifiant externe). | Non |
| **OfferAnalyzer** | Normalise les offres (prix unitaire, total, livraison incluse ou non, délai, paiement), compare à périmètre égal, signale ce qui manque. | Non |
| **Evaluator** | Classe les offres réelles, explique, recommande ; constate quand plus rien n'est raisonnablement améliorable. | Recommande seulement |

### Règles du mandat appliquées par le code, jamais par l'IA

- Le budget maximal n'apparaît dans **aucun** message sortant : le Guard refuse tout texte contenant ce montant (ou un
  montant supérieur au prix proposé par la stratégie). L'IA ne le reçoit pas dans ses entrées de rédaction.
- Un prix proposé vient de la stratégie, borné par `[prix cible, plafond autorisé]` ; l'IA ne choisit aucun chiffre.
- Aucune offre concurrente n'est citée si elle n'existe pas en base comme offre reçue ; citer une offre réelle d'un
  concurrent est lui-même une permission du mandat (désactivée par défaut).
- Une offre extraite d'un texte libre est `unverified` jusqu'à confirmation par le fournisseur (formulaire du portail,
  ou réponse explicite à un récapitulatif) ; seule une offre `confirmed` peut être recommandée comme réelle.
- Hors limites → refus automatique côté acheteur, jamais d'acceptation ; dépassement des tours ou de la durée → arrêt.
- Aucune réponse fournisseur ne déclenche de commande ni de paiement : l'acceptation finale est une confirmation de
  l'acheteur (jeton), puis une action du runtime, comptée et idempotente comme les autres.

## 6. Modèle de données (proposé, migration 33+)

| Table | Contenu |
|---|---|
| `negotiations` | acheteur (`user_id`, `organization_id` facultatif), conversation et mission d'origine, besoin normalisé, statut, version (compare-and-set), échéance |
| `negotiation_mandates` | versions successives du mandat (prix cible, plafond, quantité, qualité minimale, livraison, délai, paiement, marge, tours max, durée max, canaux permis, divulgations permises) ; qui l'a accordé et quand |
| `negotiation_participants` | un fournisseur par négociation : contact et canal retenus, statut (`invited`, `responded`, `shortlisted`, `countered`, `declined`, `withdrawn`, `expired`), tour courant |
| `negotiation_messages` | chaque message entrant et sortant : canal, sens, texte, pièce, identifiant externe (**UNIQUE (canal, external_id)** pour les doublons), clé d'idempotence d'envoi, état d'envoi (outbox) |
| `negotiation_offers` | offres versionnées par participant : conditions structurées, message source, `unverified`/`confirmed`/`withdrawn`, validité |
| `negotiation_events` | journal chronologique append-only : qui (acheteur, système, fournisseur, IA), quoi, quand, canal, version du mandat appliquée |

Statuts d'une négociation : `draft → soliciting → collecting → negotiating → awaiting_decision → accepted | rejected |
expired | cancelled`. Les transitions sont celles du Manager, chacune vérifiée par le Guard et écrite dans
`negotiation_events` dans la même transaction.

Cas difficiles prévus : message reçu deux fois (contrainte unique) ; réponse tardive après expiration (enregistrée,
marquée tardive, sans effet sur l'état) ; fournisseur qui répond par un autre canal (rattaché par ses points de contact
connus, sinon mis en attente de rapprochement par l'acheteur) ; deux canaux qui se contredisent (la dernière offre
confirmée fait foi, l'historique garde les deux) ; interruption du service (verrou à bail + outbox rejouée).

## 7. Expérience dans la conversation

La négociation est une carte vivante dans le fil : étapes réelles (fournisseurs trouvés, demandes envoyées, réponses
reçues, contre-propositions, nouvelles conditions, recommandation), alimentée par le flux SSE du streaming. Une vue
détaillée (offres côte à côte, chronologie) s'ouvre depuis la carte, sans quitter la conversation. L'acceptation finale
réutilise la confirmation explicite existante.

## 8. Plan par phases

| Phase | Contenu | Dépend de |
|---|---|---|
| 0 — Prérequis | Inscription fournisseurs (lot 1, brique 3) ; fin du streaming (lot 1, brique 1) | — |
| 1 — Cœur | Tables, Manager, Guard, Strategy (règles), Analyzer, Evaluator, journal ; canal **portail** (boîte de réception fournisseur, formulaire d'offre) ; API acheteur et fournisseur ; carte dans la conversation ; tests (mandat, doublons, retards, reprise, isolation) | Phase 0 |
| 2 — IA | Extraction d'offres et rédaction des messages via la passerelle IA, contrôlées par le Guard ; mode dégradé sans IA | Crédits Anthropic |
| 3 — WhatsApp fournisseur | Adaptateur sortant (modèles utilitaires), routage entrant fournisseur | Numéro de production + modèles approuvés |
| 4 — E-mail | Adaptateur sortant et entrant | Choix du fournisseur e-mail + domaine |

## 9. Risques

- **Aucun fournisseur** : le moteur est inutile en production tant que l'inscription n'est pas ouverte et peuplée.
- Dépendance Meta (numéro de production, modèles) et Anthropic (crédits) hors de notre contrôle.
- Risque juridique d'une négociation automatisée « au nom de » l'acheteur : le mandat doit être explicite, visible et
  révocable ; chaque message sortant doit dire qu'il est envoyé par Trezutil pour le compte d'un acheteur.
- Charge IA : extraction + rédaction à chaque tour, à compter dans les politiques de crédit existantes.
