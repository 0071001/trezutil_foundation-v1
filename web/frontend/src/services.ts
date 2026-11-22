/*
 * What this interface is connected to, and what it waits for. One list, shown in Settings → Services connectés, so a
 * missing backend service is never mistaken for a working one. Update it when the backend gains an endpoint.
 */

export type ServiceStatus = 'connected' | 'partial' | 'pending'

export interface Service {
  id: string
  label: string
  status: ServiceStatus
  /** What the interface uses today. */
  uses: string
  /** What is missing on the backend, when something is. */
  missing?: string
}

export const SERVICES: Service[] = [
  { id: 'conversation', label: 'Conversation et actions commerciales', status: 'connected', uses: 'POST /conversations/message, /conversations/{id}/confirm, /reject (Commerce Action Runtime)' },
  { id: 'streaming', label: 'Réponses en continu (streaming)', status: 'pending', uses: 'La réponse arrive entière, puis s’affiche progressivement.', missing: 'Un flux SSE ou WebSocket sur /conversations/message' },
  { id: 'history', label: 'Historique des conversations', status: 'connected', uses: 'GET /conversations, GET /conversations/{id}/messages (avec les offres et cartes de chaque réponse), PATCH et DELETE /conversations/{id} : renommage, archivage et suppression suivent le compte sur tous ses appareils.' },
  { id: 'attachments', label: 'Pièces jointes et analyse de documents', status: 'pending', uses: 'Les fichiers sont choisis et prévisualisés, mais ne quittent pas l’appareil.', missing: 'POST /conversations/{id}/attachments et un service d’analyse (images, PDF, Excel)' },
  { id: 'dictation', label: 'Dictée vocale', status: 'connected', uses: 'Reconnaissance vocale du navigateur (Chrome, Edge, Safari).' },
  { id: 'voice', label: 'Mode vocal', status: 'partial', uses: 'Reconnaissance et synthèse vocales du navigateur, tour par tour, sur la vraie conversation.', missing: 'Un service audio temps réel (flux bidirectionnel, interruption côté serveur)' },
  { id: 'operations', label: 'Opérations (missions)', status: 'connected', uses: '/missions : création, plan, exécution, reprise, pause, arrêt, instructions' },
  { id: 'negotiation', label: 'Négociation avec les fournisseurs', status: 'pending', uses: 'Aucune négociation n’est simulée.', missing: 'Un service de sollicitation, de propositions et de contre-propositions' },
  { id: 'orders', label: 'Commandes et réservations', status: 'connected', uses: 'GET /orders, GET /orders/{id}, POST /orders/{id}/cancel, GET /reservations, POST /reservations/{id}/cancel' },
  { id: 'catalogue', label: 'Détail des offres', status: 'connected', uses: 'GET /listings?listing_id= : photos, description et caractéristiques d’une offre' },
  { id: 'suppliers', label: 'Inscription des fournisseurs', status: 'pending', uses: 'Les organisations dont tu es membre (GET /sellers/me).', missing: 'Une inscription en libre-service : POST /organizations est réservé aux administrateurs' },
  { id: 'payment', label: 'Paiement des commandes', status: 'partial', uses: 'POST /payments et formulaire Stripe ; la commande est relue jusqu’à la confirmation par webhook.', missing: 'Les clés Stripe (STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET) en configuration ; aucun adaptateur Mobile Money (seul Stripe est implémenté)' },
  { id: 'memory', label: 'Mémoire commerciale', status: 'connected', uses: 'GET, POST, PATCH, DELETE /memory et PUT /memory/settings : conservée sur le serveur, sur tous les appareils du compte ; l’assistant la lit comme indications, et les opérations la reçoivent comme préférences.' },
  { id: 'notifications', label: 'Notifications et suivi proactif', status: 'partial', uses: 'Le centre de notifications montre ce qui attend réellement ta décision.', missing: 'Le service notification-service (aujourd’hui un squelette), la surveillance des prix et l’envoi push' },
  { id: 'credits', label: 'Crédits d’utilisation', status: 'partial', uses: 'Solde des organisations que tu gères : GET /sellers/usage', missing: 'Le solde d’un compte personnel' },
]

export const service = (id: string) => SERVICES.find((s) => s.id === id)!
