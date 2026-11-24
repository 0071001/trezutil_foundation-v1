import { describe, expect, it } from 'vitest'
import { approx } from '@/lib/exchangeRate'

describe('indicative conversion', () => {
  const rate = { rate: 2850, setAt: '2026-11-24T10:00:00Z' }

  it('shows the other currency at the administrator’s rate', () => {
    expect(approx(10, 'USD', rate)).toMatch(/^≈ 28\s?500\s?CDF$/)
    expect(approx(28500, 'CDF', rate)).toMatch(/^≈ 10\s?\$US$|^≈ 10,00\s?\$US$/)
    expect(approx(1000, 'cdf', rate).startsWith('≈ 0,35')).toBe(true)
  })

  it('converts nothing without a rate, a price, or for another currency', () => {
    expect(approx(10, 'USD', null)).toBe('')
    expect(approx(0, 'USD', rate)).toBe('')
    expect(approx(10, 'EUR', rate)).toBe('')
  })
})
