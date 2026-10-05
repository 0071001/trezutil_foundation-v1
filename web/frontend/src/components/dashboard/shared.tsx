import { useState } from 'react'
import type { Analytics, DayCount } from '@/api/analytics'
import { ChartCard } from '@/components/charts/ChartCard'
import { compact } from '@/components/charts/scale'
import { TimeChart, seriesTable } from '@/components/charts/TimeChart'
import { statusLabel } from '@/components/ui'
import { formatMoney, formatWholeMinor } from '@/lib/format'
import { t, tx } from '@/lib/i18n'

/* Pieces both dashboards (administration and seller) are built from. */

export const counts = (series: DayCount[]): number[] => series.map((d) => d.count)
export const dates = (series: { date: string }[]): string[] => series.map((d) => d.date)

/** USD first, then the other currencies of the period: amounts in different currencies are never added. */
export function currenciesOf(a: Analytics): string[] {
  return a.revenueCurrencies.length > 0 ? a.revenueCurrencies : ['USD']
}

/** The revenue of one currency per day, in major units. */
export const revenueSeries = (a: Analytics, currency: string): number[] => a.revenue.map((d) => (d.amounts[currency] ?? 0) / 100)

/**
 * The headline revenue: the main currency in whole units, and a note naming the
 * others when the period earned in several. Currencies are never added together.
 */
export function revenueParts(revenue: Record<string, number>, currencies: string[]): { value: string; note?: string } {
  const main = currencies[0] ?? 'USD'
  const others = currencies.slice(1).filter((c) => (revenue[c] ?? 0) > 0).map((c) => formatWholeMinor(revenue[c]!, c))
  return { value: formatWholeMinor(revenue[main] ?? 0, main), note: others.length > 0 ? t('Also {amount}, counted separately', { amount: others.join(' + ') }) : undefined }
}

/** Every currency of a total, each in whole units: "$3,500 + CDF 50,000". */
export function revenueList(revenue: Record<string, number>): string {
  return Object.entries(revenue).filter(([, v]) => v > 0).map(([c, v]) => formatWholeMinor(v, c)).join(' + ')
}

const verticalLabels: Record<string, string> = {
  RETAIL: tx('Retail'), RESTAURANT: tx('Restaurants'), HOTEL: tx('Hotels'), SERVICES: tx('Services'), REAL_ESTATE: tx('Real estate'), OTHER: tx('Other'),
}
export const verticalLabel = (code: string): string => (verticalLabels[code] ? t(verticalLabels[code]!) : code.replace(/_/g, ' ').toLowerCase())

/** Orders, bookings and listings statuses read the same wherever they appear. */
export const labelOf = (status: string): string => statusLabel(status)

/** Revenue per day, with a currency switch when the period earned in more than one. */
export function RevenueCard({ data, wide = true }: { data: Analytics; wide?: boolean }) {
  const currencies = currenciesOf(data)
  const [chosen, setChosen] = useState<string | null>(null)
  const currency = chosen && currencies.includes(chosen) ? chosen : currencies[0]!
  const values = revenueSeries(data, currency)
  const money = (v: number) => formatMoney(v, currency)
  const label = t('Revenue per day')
  const series = [{ id: 'revenue', label: t('Revenue'), values }]
  return (
    <ChartCard
      title={label}
      subtitle={t('Paid orders, not cancelled or awaiting payment, in {currency}', { currency })}
      wide={wide}
      empty={values.every((v) => v === 0) ? t('No revenue in this period.') : undefined}
      table={seriesTable(dates(data.revenue), series, money)}
      actions={currencies.length > 1 ? (
        <div className="tzc-period" role="group" aria-label={t('Currency')}>
          {currencies.map((c) => <button key={c} type="button" aria-pressed={c === currency} onClick={() => setChosen(c)}>{c}</button>)}
        </div>
      ) : undefined}
    >
      <TimeChart label={label} dates={dates(data.revenue)} series={series} kind="area" format={money} axisFormat={(v) => compact(v)} />
    </ChartCard>
  )
}

/** A per-day count as a chart card (bars or area), with an honest empty state. */
export function CountCard({ title, subtitle, series, kind = 'bars', unit, wide }: { title: string; subtitle: string; series: DayCount[]; kind?: 'bars' | 'area'; unit: string; wide?: boolean }) {
  const values = counts(series)
  const chart = [{ id: 'count', label: unit, values }]
  return (
    <ChartCard title={title} subtitle={subtitle} wide={wide} empty={values.every((v) => v === 0) ? t('Nothing in this period.') : undefined} table={seriesTable(dates(series), chart)}>
      <TimeChart label={title} dates={dates(series)} series={chart} kind={kind} />
    </ChartCard>
  )
}
