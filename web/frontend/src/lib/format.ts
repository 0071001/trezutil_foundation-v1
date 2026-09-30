import type { ListingType, SearchResult } from '@/types'

export function formatMoney(amount: number, currency: string): string {
  if (!currency) return amount ? amount.toLocaleString() : ''
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: amount % 1 ? 2 : 0 }).format(amount)
  } catch {
    return `${currency} ${amount.toFixed(2)}`
  }
}

export function formatMinor(amountMinor: number, currency: string): string {
  return formatMoney(amountMinor / 100, currency)
}

export function formatDateTime(ts: number): string {
  return ts ? new Date(ts * 1000).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : '—'
}

export function formatDate(ts: number): string {
  return ts ? new Date(ts * 1000).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' }) : '—'
}

export function timeAgo(ts: number): string {
  if (!ts) return ''
  const seconds = Math.max(0, Date.now() / 1000 - ts)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`
  if (seconds < 7 * 86400) return `${Math.floor(seconds / 86400)} d ago`
  return formatDate(ts)
}

export function formatDistance(meters: number | null): string {
  if (meters === null || meters < 0) return ''
  return meters < 1000 ? `${Math.round(meters)} m` : `${(meters / 1000).toFixed(1)} km`
}

export function place(result: Pick<SearchResult, 'location'>): string {
  const parts = [result.location?.neighborhood, result.location?.city].filter(Boolean)
  return parts.join(', ')
}

/** Only http(s) URLs are rendered (listing media is seller-supplied). */
export function safeImage(url: string | null | undefined): string | null {
  if (!url) return null
  try {
    const parsed = new URL(url, window.location.origin)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : null
  } catch {
    return null
  }
}

export function sentence(value: string): string {
  const s = value.replace(/_/g, ' ')
  return s.charAt(0).toUpperCase() + s.slice(1)
}

/** Listing detail route (the Figma detail variants), by vertical, else by listing type. */
export function detailPath(id: string, type?: string, vertical?: string): string {
  const byVertical: Record<string, string> = { HOTEL: 'hotels', RESTAURANT: 'restaurants', SERVICES: 'services', REAL_ESTATE: 'properties' }
  const byType: Record<string, string> = { service: 'services', real_estate: 'properties', food: 'restaurants' }
  return `/${byVertical[vertical ?? ''] ?? byType[type ?? ''] ?? 'products'}/${encodeURIComponent(id)}`
}

/**
 * Legacy guess from the listing type, used only when the vertical catalogue
 * cannot be loaded (see lib/verticals.ts).
 */
export function isBookable(type: ListingType | string | undefined): boolean {
  return type === 'real_estate' || type === 'service' || type === 'food'
}
