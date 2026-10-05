import { describe, expect, it } from 'vitest'
import { mapSearchResult } from '@/api/search'
import { discountPercent, listingSale, saleOf } from '@/lib/promo'

describe('discountPercent', () => {
  // The same cases as domain.DiscountPercent on the server: both must agree.
  it.each([
    [100, 80, 20],
    [120, 90, 25],
    [100, 66.5, 33], // 33.5% rounds down: a discount is never overstated
    [100, 99.5, 0],
    [100, 99, 1],
    [100, 100, 0],
    [100, 120, 0],
    [0, 0, 0],
    [-5, 1, 0],
    [100, -1, 0],
    [1000000, 1, 99],
    [19.99, 14.99, 25],
    [29.99, 20.99, 30],
    [299.99, 209.99, 30], // must not drop to 29 through floating-point noise
  ])('from %d to %d saves %d%%', (compareAt, price, expected) => {
    expect(discountPercent(compareAt, price)).toBe(expected)
  })

  it('treats non-numbers as no discount', () => {
    expect(discountPercent(Number.NaN, 5)).toBe(0)
    expect(discountPercent(100, Number.POSITIVE_INFINITY)).toBe(0)
  })
})

describe('saleOf and listingSale', () => {
  it('reports a sale only when the regular price is above the selling price', () => {
    expect(saleOf({ price_amount: 80, compare_at_amount: 100 })).toEqual({ compareAt: 100, percent: 20 })
    expect(saleOf({ price_amount: 100, compare_at_amount: 100 })).toBeNull()
    expect(saleOf({ price_amount: 120, compare_at_amount: 100 })).toBeNull()
    expect(saleOf({ price_amount: 99.7, compare_at_amount: 100 })).toBeNull()
    expect(saleOf({ price_amount: 80, compare_at_amount: null })).toBeNull()
  })

  it('reads the sale of a full listing from its promotion metadata', () => {
    expect(listingSale({ price_amount: 75, metadata: { promotion: { compare_at_amount: 100 } } })).toEqual({ compareAt: 100, percent: 25 })
    expect(listingSale({ price_amount: 75, metadata: {} })).toBeNull()
    expect(listingSale({ price_amount: 75, metadata: { promotion: 'nope' } })).toBeNull()
    expect(listingSale({ price_amount: 75, metadata: { promotion: { compare_at_amount: '100' } } })).toBeNull()
    expect(listingSale({ price_amount: 100, metadata: { promotion: { compare_at_amount: 100 } } })).toBeNull()
  })
})

describe('search result mapping', () => {
  const base = { ListingID: 'l1', Title: 'Phone', PriceAmount: 80, PriceCurrency: 'USD' }

  it('derives the discount from the two prices (PascalCase or snake_case)', () => {
    expect(mapSearchResult({ ...base, CompareAtAmount: 100, DiscountPercent: 99 })).toMatchObject({ compare_at_amount: 100, discount_percent: 20 })
    expect(mapSearchResult({ listing_id: 'l1', price_amount: 50, compare_at_amount: 200 })).toMatchObject({ compare_at_amount: 200, discount_percent: 75 })
  })

  it('ignores a compare-at price that is not a real saving', () => {
    for (const compare of [80, 60, 0, '100', null, undefined]) {
      expect(mapSearchResult({ ...base, CompareAtAmount: compare })).toMatchObject({ compare_at_amount: null, discount_percent: 0 })
    }
  })
})
