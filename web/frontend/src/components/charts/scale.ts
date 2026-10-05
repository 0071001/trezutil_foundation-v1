import { locale } from '@/lib/i18n'

/* Axis maths and number/date formatting shared by the charts. */

/** The smallest "nice" number (1, 2, 2.5, 5 times a power of ten) that is at least `value`. */
export function niceMax(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1
  const exponent = Math.floor(Math.log10(value))
  const base = 10 ** exponent
  for (const step of [1, 2, 2.5, 5, 10]) {
    if (value <= step * base + 1e-9) return step * base
  }
  return 10 * base
}

/**
 * An axis from zero to a round number, with `count` equal steps of 1, 2, 2.5, 5
 * or 10 times a power of ten. With `integer`, a step is never below 1, so a chart
 * of counts is never labelled 1.3 or 2.5.
 */
export function niceScale(max: number, count = 4, integer = false): { max: number; step: number; ticks: number[] } {
  const wanted = Number.isFinite(max) && max > 0 ? max / count : 0.25
  const exponent = Math.floor(Math.log10(wanted))
  const base = 10 ** exponent
  let step = 10 * base
  for (const m of [1, 2, 2.5, 5, 10]) {
    if (wanted <= m * base + 1e-9) { step = m * base; break }
  }
  if (integer) step = Math.max(1, Math.ceil(step - 1e-9))
  return { max: step * count, step, ticks: Array.from({ length: count + 1 }, (_, i) => step * i) }
}

/** `count + 1` evenly spaced tick values from 0 to `max`. */
export function ticks(max: number, count = 4): number[] {
  return Array.from({ length: count + 1 }, (_, i) => (max / count) * i)
}

/** 1 200 → "1.2K" / "1,2 k", for axis labels. */
export function compact(value: number): string {
  return new Intl.NumberFormat(locale(), { notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

/** A whole number with the language's grouping. */
export function integer(value: number): string {
  return new Intl.NumberFormat(locale(), { maximumFractionDigits: 0 }).format(value)
}

function parse(date: string): Date {
  // Noon UTC: a calendar day never slides to its neighbour in any time zone.
  return new Date(`${date}T12:00:00Z`)
}

/** "4 Oct" / "4 oct." */
export function shortDay(date: string): string {
  return parse(date).toLocaleDateString(locale(), { day: 'numeric', month: 'short', timeZone: 'UTC' })
}

/** "Sat, 4 Oct 2026" */
export function longDay(date: string): string {
  return parse(date).toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })
}

/** The change from `previous` to `current`, in whole percent, or null when there is nothing to compare with. */
export function percentChange(current: number, previous: number): number | null {
  if (previous <= 0) return null
  return Math.round(((current - previous) / previous) * 100)
}

/** Indices of about `count` labels spread over `length` points, always including the first and the last. */
export function labelIndices(length: number, count = 5): number[] {
  if (length <= 0) return []
  if (length <= count) return Array.from({ length }, (_, i) => i)
  const out = new Set<number>([0, length - 1])
  for (let i = 1; i < count - 1; i++) out.add(Math.round((i * (length - 1)) / (count - 1)))
  return [...out].sort((a, b) => a - b)
}
