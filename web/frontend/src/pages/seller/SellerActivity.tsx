import type { Analytics } from '@/api/analytics'
import { BreakdownBars, Donut, statusTone } from '@/components/charts/Breakdown'
import { ChartCard } from '@/components/charts/ChartCard'
import { integer } from '@/components/charts/scale'
import { ActivityBoard, type BoardMetric } from '@/components/dashboard/ActivityBoard'
import { CountCard, RankedList, RevenueCard, counts, currenciesOf, labelOf, revenueParts, revenueSeries, verticalLabel } from '@/components/dashboard/shared'
import { t, tn } from '@/lib/i18n'

/*
 * A business's activity over a period, from the platform's own orders,
 * bookings and listings (GET /sellers/analytics). Used by the dashboard (a
 * summary) and by the Analytics module (everything). Only owners, admins and
 * managers can read it; the server refuses anyone else.
 */

export function SellerActivity({ data, full = false, extra = [] }: { data: Analytics; full?: boolean; extra?: BoardMetric[] }) {
  const { current: now, previous: before, days } = data
  const currencies = currenciesOf(data)
  const main = currencies[0]!
  const revenue = revenueParts(now.revenue, currencies)
  const live = data.listingsByStatus.published ?? 0
  const drafts = data.listingsByStatus.draft ?? 0
  const metrics: BoardMetric[] = [
    { id: 'sales', tile: { label: t('Sales'), icon: 'wallet', value: revenue.value, current: now.revenue[main] ?? 0, previous: before.revenue[main] ?? 0, days, spark: revenueSeries(data, main), note: revenue.note, hero: true }, panel: <RevenueCard data={data} wide={false} /> },
    { id: 'orders', tile: { label: t('Orders placed'), icon: 'orders', value: integer(now.orders), current: now.orders, previous: before.orders, days, spark: counts(data.orders) }, panel: <CountCard title={t('Orders per day')} subtitle={t('Orders placed (carts excluded)')} series={data.orders} unit={t('Orders')} /> },
    { id: 'bookings', tile: { label: t('Bookings'), icon: 'calendar', value: integer(now.reservations), current: now.reservations, previous: before.reservations, days, spark: counts(data.reservations) }, panel: <CountCard title={t('Bookings per day')} subtitle={t('Reservations created')} series={data.reservations} unit={t('Bookings')} /> },
    { id: 'customers', tile: { label: t('Customers'), icon: 'users', value: integer(now.customers), current: now.customers, previous: before.customers, days, note: t('Returning buyers: {n}', { n: integer(now.returningCustomers) }) } },
    { id: 'live', tile: { label: t('Live listings'), icon: 'shop', value: integer(live), note: tn(drafts, '{n} draft', '{n} drafts') } },
    { id: 'new-listings', tile: { label: t('New listings'), icon: 'spark', value: integer(now.newListings), current: now.newListings, previous: before.newListings, days, spark: counts(data.newListings) }, panel: <CountCard title={t('New listings per day')} subtitle={t('Listings you created, any status')} series={data.newListings} unit={t('Listings')} /> },
    ...extra,
  ]
  return (
    <>
      <ActivityBoard label={t('Key figures')} metrics={metrics} />

      <div className={`tzc-grid-cards ${full ? 'three' : ''} tz-dash-gap`}>
        <ChartCard title={t('Orders by status')} subtitle={t('Placed in this period')}>
          <Donut label={t('Orders by status')} centre={t('orders')} items={Object.entries(data.ordersByStatus).map(([key, value]) => ({ key, label: labelOf(key), value, tone: statusTone(key) }))} />
        </ChartCard>
        {full && (
          <ChartCard title={t('Bookings by status')} subtitle={t('Created in this period')}>
            <Donut label={t('Bookings by status')} centre={t('bookings')} items={Object.entries(data.reservationsByStatus).map(([key, value]) => ({ key, label: labelOf(key), value, tone: statusTone(key) }))} />
          </ChartCard>
        )}
        <ChartCard title={t('Best sellers')} subtitle={t('Listings that sold the most, paid orders only')}>
          <RankedList label={t('Best sellers')} entries={data.topListings} empty={t('No itemised sales in this period yet.')} meta={(e) => tn(e.units, '{n} unit sold', '{n} units sold')} weight={(e) => e.units} />
        </ChartCard>
        {full && (
          <ChartCard title={t('Listings by status')} subtitle={t('Your catalogue today')}>
            <BreakdownBars label={t('Listings by status')} items={Object.entries(data.listingsByStatus).map(([key, value]) => ({ key, label: labelOf(key), value, tone: statusTone(key) }))} />
          </ChartCard>
        )}
        {full && (
          <ChartCard title={t('Live listings by sector')} subtitle={t('Published today')}>
            <BreakdownBars label={t('Live listings by sector')} items={Object.entries(data.publishedByVertical).map(([key, value], i) => ({ key, label: verticalLabel(key), value, tone: (i % 6) + 1 }))} />
          </ChartCard>
        )}
      </div>
    </>
  )
}
