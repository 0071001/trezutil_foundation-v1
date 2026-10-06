import type { Analytics } from '@/api/analytics'
import { t } from '@/lib/i18n'

/*
 * "Export" on a dashboard: the day-by-day figures the server sent, as a CSV file
 * that opens in a spreadsheet. Nothing is computed here: every cell is a number
 * from the response (the totals row is the server's own totals for the period),
 * and amounts stay in their own currency, one column each, never added together.
 */

const cell = (value: string | number): string => {
  const text = String(value)
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

/** Rows as a CSV file's text: a byte-order mark so a spreadsheet reads accents as UTF-8, CRLF as the format says. */
export function toCsv(header: (string | number)[], rows: (string | number)[][]): string {
  return '\ufeff' + [header, ...rows].map((r) => r.map(cell).join(',')).join('\r\n') + '\r\n'
}

const money = (minor: number | undefined): string => ((minor ?? 0) / 100).toFixed(2)

export function analyticsCsv(a: Analytics): string {
  const platform = a.scope === 'platform'
  const currencies = a.revenueCurrencies.length > 0 ? a.revenueCurrencies : ['USD']
  const header = [
    t('Day'), t('Orders'), t('Bookings'), t('New listings'),
    ...(platform ? [t('New accounts'), t('Conversations')] : []),
    ...currencies.map((c) => `${t('Revenue')} (${c})`),
  ]
  const rows = a.orders.map((day, i) => [
    day.date, day.count, a.reservations[i]?.count ?? 0, a.newListings[i]?.count ?? 0,
    ...(platform ? [a.newUsers[i]?.count ?? 0, a.conversations[i]?.count ?? 0] : []),
    ...currencies.map((c) => money(a.revenue[i]?.amounts[c])),
  ])
  const totals = [
    t('Total'), a.current.orders, a.current.reservations, a.current.newListings,
    ...(platform ? [a.current.newUsers, a.current.conversations] : []),
    ...currencies.map((c) => money(a.current.revenue[c])),
  ]
  // A byte-order mark so a spreadsheet reads the accents as UTF-8; CRLF as the format says.
  return '﻿' + [header, ...rows, totals].map((r) => r.map(cell).join(',')).join('\r\n') + '\r\n'
}

const slug = (name: string): string => name.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40)

/** trezutil-platform-2026-09-06-to-2026-10-05.csv, or the business's name in its place. */
export function analyticsFilename(a: Analytics, name?: string): string {
  const who = a.scope === 'platform' ? 'platform' : slug(name ?? '') || 'business'
  return `trezutil-${who}-${a.from}-to-${a.to}.csv`
}

export function downloadText(filename: string, text: string, type = 'text/csv;charset=utf-8'): void {
  const url = URL.createObjectURL(new Blob([text], { type }))
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

export function exportAnalytics(a: Analytics, name?: string): void {
  downloadText(analyticsFilename(a, name), analyticsCsv(a))
}
