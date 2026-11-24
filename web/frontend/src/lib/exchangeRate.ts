import { useSyncExternalStore } from 'react'
import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'
import { formatMoney } from '@/lib/format'

/*
 * The CDF / USD rate a Trezutil administrator entered (GET /exchange-rate). It only adds an indicative "≈" beside a
 * price in the other currency: the price, the order and the payment stay in the currency the supplier set. With no
 * rate entered, nothing is converted.
 */

export interface ExchangeRate {
  /** Congolese francs for one US dollar. */
  rate: number
  setAt: string
  note?: string
}

let current: ExchangeRate | null = null
let loading: Promise<void> | null = null
const listeners = new Set<() => void>()
const emit = () => listeners.forEach((l) => l())

function parse(raw: unknown): ExchangeRate | null {
  const o = obj(raw)
  const rate = num(o.rate)
  return rate > 0 ? { rate, setAt: str(o.set_at), note: str(o.note) || undefined } : null
}

/** Reads the rate once per visit (again after an administrator changes it). */
export function loadExchangeRate(force = false): Promise<void> {
  if (loading && !force) return loading
  loading = http.get<unknown>('/exchange-rate')
    .then((raw) => { current = parse(raw); emit() })
    .catch(() => { /* no conversion shown */ })
  return loading
}

export function useExchangeRate(): ExchangeRate | null {
  return useSyncExternalStore(
    (l) => { listeners.add(l); void loadExchangeRate(); return () => { listeners.delete(l) } },
    () => current,
    () => current,
  )
}

/** "≈ 41 000 CDF" for a price in USD, "≈ 14,64 $US" for one in CDF; '' when there is no rate or nothing to convert. */
export function approx(amount: number, currency: string, rate: ExchangeRate | null): string {
  if (!rate || !(amount > 0)) return ''
  const c = currency.toUpperCase()
  if (c === 'USD') return '≈ ' + formatMoney(Math.round(amount * rate.rate), 'CDF')
  if (c === 'CDF') return '≈ ' + formatMoney(Math.round((amount / rate.rate) * 100) / 100, 'USD')
  return ''
}

export const exchangeRateApi = {
  /** Administrator only: records a new rate. */
  async set(rate: number, note: string): Promise<ExchangeRate> {
    const r = parse(await http.put<unknown>('/admin/exchange-rate', { rate, note }))
    if (!r) throw new Error('invalid rate')
    current = r
    emit()
    return r
  },
  /** Staff: the rates entered, most recent first. */
  async history(): Promise<ExchangeRate[]> {
    const raw = obj(await http.get<unknown>('/admin/exchange-rate/history'))
    return arr(raw.items).map(parse).filter((r): r is ExchangeRate => r !== null)
  },
}
