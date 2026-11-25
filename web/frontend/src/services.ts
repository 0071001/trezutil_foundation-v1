import { t } from '@/lib/i18n'
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
  { id: 'conversation', label: t('Conversation et actions commerciales'), status: 'connected', uses: t('POST /conversations/message, /conversations/{id}/confirm, /reject (Commerce Action Runtime)') },
  { id: 'streaming', label: t('Réponses en continu (streaming)'), status: 'connected', uses: t('POST /conversations/message/stream (SSE) : les étapes réelles, puis le texte à mesure que Claude le rédige ; arrêt possible ; reprise avec la même clé après une coupure. Sans Claude (crédit épuisé, panne), la réponse du serveur arrive en un bloc.') },
  { id: 'history', label: t('Historique des conversations'), status: 'connected', uses: t('GET /conversations, GET /conversations/{id}/messages (avec les offres et cartes de chaque réponse), PATCH et DELETE /conversations/{id} : renommage, archivage et suppression suivent le compte sur tous ses appareils.') },
  { id: 'attachments', label: t('Pièces jointes et analyse de documents'), status: 'pending', uses: t('Les fichiers sont choisis et prévisualisés, mais ne quittent pas l’appareil.'), missing: t('POST /conversations/{id}/attachments et un service d’analyse (images, PDF, Excel)') },
  { id: 'dictation', label: t('Dictée vocale'), status: 'connected', uses: t('Reconnaissance vocale du navigateur (Chrome, Edge, Safari).') },
  { id: 'voice', label: t('Mode vocal'), status: 'partial', uses: t('Reconnaissance et synthèse vocales du navigateur, tour par tour, sur la vraie conversation.'), missing: t('Un service audio temps réel (flux bidirectionnel, interruption côté serveur)') },
  { id: 'operations', label: t('Opérations (missions)'), status: 'connected', uses: t('/missions : création, plan, exécution, reprise, pause, arrêt, instructions') },
  { id: 'negotiation', label: t('Négociation avec les fournisseurs'), status: 'pending', uses: t('Aucune négociation n’est simulée.'), missing: t('Un service de sollicitation, de propositions et de contre-propositions') },
  { id: 'orders', label: t('Commandes et réservations'), status: 'connected', uses: 'GET /orders, GET /orders/{id}, POST /orders/{id}/cancel, GET /reservations, POST /reservations/{id}/cancel' },
  { id: 'catalogue', label: t('Détail des offres'), status: 'connected', uses: t('GET /listings?listing_id= : photos, description et caractéristiques d’une offre') },
  { id: 'suppliers', label: t('Inscription des fournisseurs'), status: 'connected', uses: t('/supplier/applications : inscription en libre-service (l’inscrit devient propriétaire), profil, informations légales, secteurs, capacités, zones desservies, documents (PDF, images ≤ 5 Mo), soumission. Vérification par l’équipe Trezutil uniquement (/admin/supplier-applications) : inscrire ne vaut jamais vérifier.') },
  { id: 'payment', label: t('Paiement des commandes'), status: 'partial', uses: t('POST /payments : SasPay (compte Trezutil) pour le mobile money en RDC (M-Pesa, Airtel Money, Orange Money) et les cartes en USD, ou le Stripe d’un vendeur ; le paiement est relu auprès du fournisseur jusqu’à sa confirmation, et le webhook signé le confirme aussi.'), missing: t('Les remboursements SasPay (aucune API : ils se font depuis le tableau de bord SasPay)') },
  { id: 'memory', label: t('Mémoire commerciale'), status: 'connected', uses: t('GET, POST, PATCH, DELETE /memory et PUT /memory/settings : conservée sur le serveur, sur tous les appareils du compte ; l’assistant la lit comme indications, et les opérations la reçoivent comme préférences.') },
  { id: 'notifications', label: t('Notifications et suivi proactif'), status: 'partial', uses: t('Le centre de notifications montre ce qui attend réellement ta décision.'), missing: t('Le service notification-service (aujourd’hui un squelette), la surveillance des prix et l’envoi push') },
  { id: 'credits', label: t('Crédits d’utilisation'), status: 'connected', uses: t('GET /me/credits : le solde personnel réel (espace personnel du compte), distinct du solde des organisations que tu gères (GET /sellers/usage) ; indique si des actions sont facturées.') },
]

export const service = (id: string) => SERVICES.find((s) => s.id === id)!
