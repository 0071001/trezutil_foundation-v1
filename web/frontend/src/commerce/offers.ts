import { mapSearchResult } from '@/api/search'
import { formatMoney, safeImage } from '@/lib/format'
import type { SearchResult } from '@/types'
import { initials } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * The offers a search of the runtime returned (result_items of a reply), in the shape the cards and the comparison
 * read. Every value comes from the backend; what it does not send (delivery time, warranty, payment terms) is shown as
 * "not provided", never estimated.
 */

export interface Offer {
  n: number
  id: string
  title: string
  supplier: string
  initials: string
  color: string
  image: string | null
  price: string
  priceValue: number
  currency: string
  wasPrice: string
  discount: number
  availability: string
  place: string
  verified: boolean
  trustLevel: number
  vertical: string
}

const SHADES = ['#183c34', '#303b53', '#654a27', '#3d4b45', '#4a3552', '#27505c']

function shade(name: string): string {
  let h = 0
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  return SHADES[h % SHADES.length]!
}

const AVAILABILITY: Record<string, string> = {
  available: t('Disponible'), in_stock: t('En stock'), out_of_stock: t('Rupture de stock'), limited: t('Stock limité'),
  on_request: t('Sur demande'), preorder: t('Sur commande'), unavailable: t('Indisponible'),
}

export function availabilityText(status: string): string {
  if (!status) return ''
  return AVAILABILITY[status.toLowerCase()] ?? status.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase())
}

export function toOffer(r: SearchResult, index: number): Offer {
  const supplier = r.seller?.display_name || r.store?.name || t('Fournisseur')
  const where = [r.location?.neighborhood, r.location?.city].filter(Boolean).join(', ')
  return {
    n: index + 1,
    id: r.listing_id,
    title: r.title,
    supplier,
    initials: initials(supplier),
    color: shade(supplier),
    image: safeImage(r.primary_media_url),
    price: r.price_amount > 0 ? formatMoney(r.price_amount, r.price_currency) : t('Prix sur demande'),
    priceValue: r.price_amount,
    currency: r.price_currency,
    wasPrice: r.compare_at_amount ? formatMoney(r.compare_at_amount, r.price_currency) : '',
    discount: r.discount_percent,
    availability: availabilityText(r.availability_status),
    place: where,
    verified: (r.seller?.trust_level ?? 0) > 0,
    trustLevel: r.seller?.trust_level ?? 0,
    vertical: r.vertical,
  }
}

export function offersOf(items: Record<string, unknown>[]): Offer[] {
  return items.map(mapSearchResult).filter((r) => r.listing_id).map(toOffer)
}

/** The four trust levels of a supplier. A new supplier is "registered", which is not a fault. */
export const TRUST_LEVELS = [
  { level: 0, label: t('Inscrit'), text: t('Compte créé sur Trezutil. Aucun document examiné pour l’instant.') },
  { level: 1, label: t('Informations vérifiées'), text: t('Identité et coordonnées contrôlées.') },
  { level: 2, label: t('Vérifié'), text: t('Documents de l’entreprise examinés par Trezutil.') },
  { level: 3, label: t('Fiabilité établie'), text: t('Historique de transactions menées à bien.') },
]

export const trustLabel = (level: number) => TRUST_LEVELS[Math.max(0, Math.min(3, level))]!.label
