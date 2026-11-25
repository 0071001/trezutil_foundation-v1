import { useEffect, useState } from 'react'
import { searchApi } from '@/api/search'
import { toOffer, type Offer } from '@/commerce/offers'
import { safeImage } from '@/lib/format'
import { t, tx } from '@/lib/i18n'

/*
 * What the mobile home and Discover show: the real catalogue. Each kind of offer with how many are published and
 * one picture, and the offers themselves, all from the search API. Nothing is invented: a kind with no offer is not
 * shown, and an offer carries no rating, distance or opening hours the backend does not send.
 */

export interface Kind {
  vertical: string
  label: string
  /** Shown in a collection tile ("15 restaurants"). */
  unit: [string, string]
  /** Asked to Trezutil when the person picks the collection. */
  ask: string
  tone: 'blue' | 'coral' | 'green' | 'violet'
}

export const KINDS: Kind[] = [
  { vertical: 'REAL_ESTATE', label: tx('Immobilier'), unit: [tx('{n} annonce'), tx('{n} annonces')], ask: tx('Je cherche un logement à Kinshasa'), tone: 'blue' },
  { vertical: 'RESTAURANT', label: tx('Restaurants'), unit: [tx('{n} établissement'), tx('{n} établissements')], ask: tx('Je cherche un bon restaurant à Kinshasa'), tone: 'coral' },
  { vertical: 'RETAIL', label: tx('Produits'), unit: [tx('{n} produit'), tx('{n} produits')], ask: tx('Je cherche des produits à acheter à Kinshasa'), tone: 'blue' },
  { vertical: 'SERVICES', label: tx('Services'), unit: [tx('{n} prestataire'), tx('{n} prestataires')], ask: tx('Je cherche un prestataire de services à Kinshasa'), tone: 'green' },
  { vertical: 'HOTEL', label: tx('Hôtels'), unit: [tx('{n} hôtel'), tx('{n} hôtels')], ask: tx('Je cherche un hôtel à Kinshasa'), tone: 'violet' },
  { vertical: 'EVENTS', label: tx('Expériences'), unit: [tx('{n} expérience'), tx('{n} expériences')], ask: tx('Je cherche une expérience à vivre à Kinshasa'), tone: 'coral' },
]

export interface KindCount { kind: Kind; total: number; image: string | null }

let counts: Promise<KindCount[]> | null = null

/** How many offers of each kind are published, with a picture of one (read once per visit). */
export function kindCounts(): Promise<KindCount[]> {
  counts ??= Promise.all(KINDS.map(async (kind) => {
    try {
      const r = await searchApi.search({ text: '', vertical: kind.vertical, page_size: 1 })
      return { kind, total: r.total, image: safeImage(r.items[0]?.primary_media_url) }
    } catch {
      return { kind, total: 0, image: null }
    }
  })).then((list) => {
    const known = list.filter((k) => k.total > 0)
    if (!known.length) counts = null // try again next time: the search may have been down
    return known
  })
  return counts
}

export function useKindCounts(): KindCount[] | null {
  const [list, setList] = useState<KindCount[] | null>(null)
  useEffect(() => {
    let alive = true
    void kindCounts().then((l) => { if (alive) setList(l) })
    return () => { alive = false }
  }, [])
  return list
}

/** Published offers, of one kind or all, as the conversation's cards read them. */
export function useOffers(text: string, vertical: string, size: number, enabled = true): { offers: Offer[] | null; failed: boolean } {
  const [state, setState] = useState<{ offers: Offer[] | null; failed: boolean }>({ offers: null, failed: false })
  useEffect(() => {
    if (!enabled) { setState({ offers: [], failed: false }); return }
    let alive = true
    setState((s) => ({ offers: s.offers, failed: false }))
    searchApi.search({ text, vertical: vertical || undefined, page_size: size })
      .then((r) => { if (alive) setState({ offers: r.items.map(toOffer), failed: false }) })
      .catch(() => { if (alive) setState({ offers: [], failed: true }) })
    return () => { alive = false }
  }, [text, vertical, size, enabled])
  return state
}

export const kindLabel = (vertical: string): string => {
  const kind = KINDS.find((k) => k.vertical === vertical)
  return kind ? t(kind.label) : ''
}

/** For tests: forget what was read. */
export function resetCatalog() { counts = null }
