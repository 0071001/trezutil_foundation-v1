import type { ListingDetail, SearchResult } from '@/types'

/**
 * A sale: the listing sells at its current price (what orders and payments
 * charge); compareAt is the regular price it replaced. Sellers manage it
 * through the seller console; the server computes the same figures.
 */
export interface Sale {
  compareAt: number
  percent: number
}

/**
 * The saving from compareAt to price as a whole percentage, rounded down so a
 * discount is never overstated (mirrors domain.DiscountPercent on the server).
 * 0 when there is no saving or the inputs are invalid.
 */
export function discountPercent(compareAt: number, price: number): number {
  if (!Number.isFinite(compareAt) || !Number.isFinite(price) || compareAt <= 0 || price < 0 || price >= compareAt) return 0
  // The epsilon keeps exact results (20%) from floating-point noise (19.999999999999996).
  return Math.floor((compareAt - price) * 100 / compareAt + 1e-9)
}

/** The sale on a search result, or null when it is not on promotion. */
export function saleOf(item: Pick<SearchResult, 'price_amount' | 'compare_at_amount'>): Sale | null {
  if (item.compare_at_amount == null) return null
  const percent = discountPercent(item.compare_at_amount, item.price_amount)
  return percent > 0 ? { compareAt: item.compare_at_amount, percent } : null
}

/** The sale on a full listing (the server keeps it under metadata.promotion). */
export function listingSale(listing: Pick<ListingDetail, 'price_amount' | 'metadata'>): Sale | null {
  const promotion = listing.metadata?.promotion
  if (!promotion || typeof promotion !== 'object') return null
  const compareAt = (promotion as Record<string, unknown>).compare_at_amount
  return typeof compareAt === 'number' ? saleOf({ price_amount: listing.price_amount, compare_at_amount: compareAt }) : null
}

