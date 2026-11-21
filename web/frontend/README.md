# Trezutil — interface web

Cette interface est le côté client du Commerce OS. Elle s'ouvre sur une conversation : « Qu'est-ce que tu souhaites
acheter, vendre ou faire réaliser ? ». Le design vient de la maquette `Redesign Trezutil/` (Figma Make) : sa feuille de
style est reprise telle quelle dans `src/styles/app.css`. Chaque écran est branché sur les vraies API de la passerelle Go.

## Lancer

Il faut d'abord démarrer la pile backend, depuis la racine du dépôt. Sous Windows sans `make` :

```powershell
docker compose -f deployments/docker/docker-compose.app.yml --env-file deployments/docker/app.env up -d
```

Puis l'interface :

```bash
cd web/frontend
pnpm install            # ou : npx pnpm@10 install --frozen-lockfile
pnpm dev                # http://127.0.0.1:5173/ (relaie /api vers http://127.0.0.1:8080)
pnpm test:run           # tests
pnpm typecheck && pnpm build
```

En local (`APP_ENV=development`), le code de connexion n'est pas envoyé sur WhatsApp. Il apparaît dans les logs :
`docker compose -f deployments/docker/docker-compose.app.yml --env-file deployments/docker/app.env logs identity-service`.

## Organisation

| Dossier | Rôle |
|---|---|
| `api/`, `lib/` | Le socle conservé : client HTTP, session et connexion OTP, i18n, formats, dictée. |
| `shell/` | Le cadre : sidebar (conversations, espace commercial), en-tête, profil, données partagées du compte. |
| `conversation/` | Le store des fils, l'accueil, le fil, le composer (fichiers, photo, dictée) et les messages. |
| `commerce/` | Les offres renvoyées par le runtime : cartes fournisseurs, comparateur, panneau de négociation. |
| `operations/` | Les opérations (missions) : plan, exécution, pause, reprise, arrêt, instructions ; chronologie et Commerce Flow. |
| `voice/` | Le mode vocal. |
| `memory/` | La mémoire commerciale. |
| `notifications/` | Le centre de notifications et ses préférences. |
| `settings/`, `pages/`, `auth/` | Les paramètres, les listes (opérations, achats, fournisseurs, vue d'ensemble) et la connexion. |
| `services.ts` | Le registre de ce qui est branché ou non, affiché dans Paramètres → Services connectés. |

## Ce qui fonctionne avec le backend

- **Conversation et actions du Commerce Action Runtime.** Le message passe par `POST /conversations/message`, avec un
  `sender_external_id` propre à chaque fil. La confirmation explicite passe par `/conversations/{id}/confirm` et le
  refus par `/reject`. Un message renvoyé après un échec garde sa clé d'idempotence.
- **Opérations.** `/missions` permet de créer, préparer le plan, lancer, reprendre, exécuter une étape, mettre en pause,
  reprendre, arrêter (`PATCH status`) et modifier les instructions.
- **Historique des conversations.** `GET /conversations` (la liste du compte) et `GET /conversations/{id}/messages`
  (le texte des échanges). Les fils d'un autre appareil apparaissent dans la barre latérale et se relisent à
  l'ouverture. Titres, archives et liens d'opération restent sur l'appareil.
- **Offres.** Leur détail est lu dans le catalogue (`GET /listings?listing_id=`) : photos, description et
  caractéristiques.
- **Commandes et réservations.** Elles sont listées, une commande peut être consultée, payée (`POST /payments`, Stripe)
  et annulée, et une réservation à venir peut être annulée.
- **Compte.** Connexion par code, nom et e-mail (`PATCH /auth/users/me`), organisations (`GET /sellers/me`) et crédits
  d'une organisation gérée (`GET /sellers/usage`).
- **Polices.** DM Sans et Manrope sont servies avec l'application (`@fontsource`) : aucun appel à un hôte tiers.

## Ce qui attend un service backend

Ces fonctions ne sont jamais simulées. L'interface les présente comme absentes ou partielles :

- **Streaming.** La réponse arrive entière. Elle s'affiche ensuite progressivement, mais c'est seulement de la
  présentation.
- **Renommer, archiver ou supprimer côté serveur.** Ces actions restent locales. Une conversation retirée de la liste
  reste sur le serveur, et la réponse structurée (offres, cartes) n'y est pas conservée.
- **Pièces jointes et analyse de documents.** Les fichiers sont choisis et prévisualisés, mais ne quittent pas l'appareil.
- **Conversation vocale en temps réel.** Le mode vocal utilise la reconnaissance et la synthèse vocales du navigateur,
  tour par tour. Une confirmation ne se donne jamais à la voix.
- **Négociation avec les fournisseurs.** Elle n'existe pas encore : sollicitation, propositions et contre-propositions
  manquent.
- **Paiement effectif.** Il faut les clés Stripe (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`). Sans elles,
  l'interface indique que le paiement en ligne n'est pas activé. Aucun adaptateur Mobile Money n'existe.
- **Inscription fournisseur en libre-service.** `POST /organizations` est réservé aux administrateurs.
- **Mémoire lue par l'assistant.** Elle est gardée sur l'appareil et transmise aux opérations comme préférences.
- **Notifications push et surveillance des prix.** `notification-service` n'est encore qu'un squelette.

## Règles tenues

- Rien n'est commandé, réservé ou payé sans la confirmation explicite que demande le runtime. Un invité doit se
  connecter pour approuver, et le mode vocal renvoie vers l'écrit.
- Lancer des étapes d'une opération (ce qui peut consommer des crédits) se fait toujours après un avertissement.
- Aucune donnée fictive n'apparaît dans l'interface. Les tests utilisent des réponses simulées, présentées comme telles.
