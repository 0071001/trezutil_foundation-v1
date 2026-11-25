import type { ApiError } from '@/api/client'
import type { Negotiation, NegotiationEvent, NegotiationStatus, ParticipantStatus } from '@/api/negotiations'
import { formatMinor } from '@/lib/format'
import { Icon } from '@/ui/Icon'
import { errorText, type Tone } from '@/ui/kit'
import { t } from '@/lib/i18n'

/* Words of the negotiation screens, shared by the buyer's and the supplier's views. */

export const STATUS: Record<NegotiationStatus, [string, Tone]> = {
  soliciting: [t('Demandes envoyées'), 'blue'],
  negotiating: [t('En négociation'), 'blue'],
  awaiting_decision: [t('À toi de décider'), 'amber'],
  accepted: [t('Offre acceptée'), 'green'],
  rejected: [t('Refusée'), 'neutral'],
  expired: [t('Expirée'), 'neutral'],
  cancelled: [t('Annulée'), 'neutral'],
}

export const PARTICIPANT: Record<ParticipantStatus, [string, Tone]> = {
  invited: [t('Demande envoyée'), 'blue'],
  responded: [t('A répondu'), 'green'],
  countered: [t('Contre-proposition envoyée'), 'blue'],
  declined: [t('A décliné'), 'neutral'],
  accepted: [t('Retenu'), 'green'],
  rejected: [t('Non retenu'), 'neutral'],
  expired: [t('Sans réponse'), 'neutral'],
}

/** A line of the journal, in words. */
export function eventText(n: Negotiation, e: NegotiationEvent, supplierView = false): string {
  const supplier = String(e.data.supplier ?? '') || t('le fournisseur')
  const price = typeof e.data.unit_minor === 'number' ? formatMinor(e.data.unit_minor, n.currency) : ''
  switch (e.kind) {
    case 'negotiation.started': return t('Négociation lancée')
    case 'request.sent': return supplierView ? t('Demande de prix reçue') : t('Demande de prix envoyée à {supplier}', { supplier })
    case 'supplier.reminded': return supplierView ? t('Rappel envoyé') : t('Rappel envoyé à {supplier}', { supplier })
    case 'supplier.offer': return supplierView ? t('Ton offre : {price} l’unité', { price }) : t('{supplier} propose {price} l’unité', { supplier, price })
    case 'supplier.accepted_counter': return supplierView ? t('Tu as accepté la contre-proposition à {price}', { price }) : t('{supplier} accepte {price} l’unité', { supplier, price })
    case 'counter.sent': return e.data.by === 'strategy'
      ? (supplierView ? t('Contre-proposition de l’acheteur : {price} l’unité', { price }) : t('Trezutil contre-propose {price} l’unité à {supplier}', { price, supplier }))
      : (supplierView ? t('Contre-proposition de l’acheteur : {price} l’unité', { price }) : t('Tu contre-proposes {price} l’unité à {supplier}', { price, supplier }))
    case 'supplier.declined': return supplierView ? t('Tu as décliné la demande') : t('{supplier} décline', { supplier })
    case 'offer.accepted': return supplierView ? t('L’acheteur a retenu ton offre : commande créée') : t('Offre de {supplier} acceptée : commande créée', { supplier })
    case 'offer.rejected': return supplierView ? t('L’acheteur n’a pas retenu ton offre') : t('Offre de {supplier} refusée', { supplier })
    case 'negotiation.cancelled': return t('Demande retirée par l’acheteur')
    case 'deadline.reached': return t('Échéance atteinte')
    default: return e.kind
  }
}

const ERRORS: Record<string, string> = {
  DEMO_SUPPLIER: t('Les fournisseurs de démonstration ne négocient pas : choisis l’offre d’un vrai fournisseur.'),
  OWN_OFFER: t('Tu ne peux pas négocier avec ta propre entreprise.'),
  MIXED_CURRENCY: t('Les offres d’une même négociation doivent être dans la même devise.'),
  LISTING_UNAVAILABLE: t('Cette offre n’est plus publiée.'),
  OFFER_EXPIRED: t('Cette offre n’est plus valable : demande au fournisseur de la renouveler.'),
  CLOSED: t('Cette négociation est close.'),
  CONFLICT: t('La négociation vient de changer. Réessaie.'),
  MANDATE: t('Cette contre-proposition n’est pas possible : elle doit être inférieure à l’offre du fournisseur, avant l’échéance et dans le nombre de tours prévu.'),
}

export function negotiationErrorText(error: ApiError): string {
  if (error.code === 'INVALID_FIELD') return t('Vérifie les montants et la quantité.')
  return ERRORS[error.code] ?? errorText(error)
}

export function NegotiationError({ error }: { error: ApiError }) {
  return <div className="state-error" role="alert"><Icon name="x" size={16} /><span>{negotiationErrorText(error)}</span></div>
}

export function deadlineText(deadline: number): string {
  const left = deadline - Date.now() / 1000
  if (left <= 0) return t('échéance passée')
  if (left < 3600) return t('moins d’une heure')
  if (left < 86400) return t('{n} h restantes', { n: Math.floor(left / 3600) })
  return t('{n} j restants', { n: Math.floor(left / 86400) })
}
