import { useEffect, useState } from 'react'
import { searchApi } from '@/api/search'
import { isBookable } from '@/lib/format'
import type { IconName } from '@/components/ui'
import type { CustomerFlow, ListingType, VerticalInfo } from '@/types'

/**
 * The vertical catalogue from the backend (GET /verticals) decides whether a
 * listing is booked or bought. It is loaded once per page load and shared.
 * If it cannot be loaded, flows fall back to the legacy listing-type guess
 * so the page still works (degraded, never blocking).
 */

let cached: VerticalInfo[] | null = null
let pending: Promise<VerticalInfo[]> | null = null

export function loadVerticals(): Promise<VerticalInfo[]> {
  if (cached) return Promise.resolve(cached)
  pending ??= searchApi.verticals()
    .then((items) => { cached = items; return items })
    .finally(() => { pending = null })
  return pending
}

/** Test hook: forget the cached catalogue. */
export function resetVerticals() { cached = null; pending = null }

export function useVerticals(): { verticals: VerticalInfo[]; byCode: Record<string, VerticalInfo>; ready: boolean } {
  const [items, setItems] = useState<VerticalInfo[] | null>(cached)
  const [ready, setReady] = useState(cached !== null)
  useEffect(() => {
    if (cached) return
    let live = true
    loadVerticals().then((v) => { if (live) { setItems(v); setReady(true) } }).catch(() => { if (live) setReady(true) })
    return () => { live = false }
  }, [])
  const list = items ?? []
  return { verticals: list, byCode: Object.fromEntries(list.map((v) => [v.code, v])), ready }
}

/** Customer flows for a listing, most prominent first. */
export function flowsFor(vertical: VerticalInfo | undefined, listingType?: string): CustomerFlow[] {
  if (vertical && vertical.flows.length) return vertical.flows
  return isBookable(listingType) ? ['BOOK'] : ['BUY']
}

/** Old `?type=` search links map onto verticals. */
export const legacyTypeVertical: Record<string, string> = {
  products: 'RETAIL', services: 'SERVICES', property: 'REAL_ESTATE', food: 'RESTAURANT', vehicles: 'TRANSPORT', hotels: 'HOTEL',
}

/** Presentation only: the Figma glyph for a vertical. */
export function verticalIcon(code: string): IconName {
  const icons: Record<string, IconName> = {
    RETAIL: 'shop', SERVICES: 'service', HOTEL: 'building', RESTAURANT: 'food', REAL_ESTATE: 'home',
    VEHICLE_RENTAL: 'car', TRANSPORT: 'car', EVENTS: 'calendar', TRAVEL: 'location', OTHER: 'spark',
  }
  return icons[code] ?? 'spark'
}

/** Seller-facing names of listing kinds. */
export const listingTypeLabel: Record<ListingType, string> = {
  physical_product: 'Product', service: 'Service', food: 'Food & drink', real_estate: 'Property or stay', vehicle: 'Vehicle', other: 'Other',
}
