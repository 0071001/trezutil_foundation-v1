import { useState } from 'react'
import { adminApi, type AdminOverview } from '@/api/admin'
import { analyticsApi, type Analytics } from '@/api/analytics'
import { BreakdownBars, Donut, statusTone } from '@/components/charts/Breakdown'
import { ChartCard } from '@/components/charts/ChartCard'
import { integer } from '@/components/charts/scale'
import { ActivityBoard, type BoardMetric } from '@/components/dashboard/ActivityBoard'
import { DashSkeleton, DashToolbar } from '@/components/dashboard/DashToolbar'
import { CountCard, RankedList, RevenueCard, counts, currenciesOf, labelOf, revenueParts, revenueSeries, verticalLabel } from '@/components/dashboard/shared'
import { ErrorState, Loading } from '@/components/states'
import { Heading } from '@/components/ui'
import { locale, t, tn, tx } from '@/lib/i18n'
import { useAsync } from '@/lib/useAsync'

/*
 * The platform dashboard. Two reads, each with its own loading and error state:
 *   - the snapshot (GET /admin/overview): what the platform holds right now;
 *   - the activity (GET /admin/analytics): what happened over the last 7, 30
 *     or 90 days, by day, against the period before.
 * Every figure is computed by the server from its own tables.
 */

const number = (n: number) => new Intl.NumberFormat(locale()).format(n)

function Stat({ label, value, hint }: { label: string; value: number | string; hint?: string }) {
  return (
    <div className="tz-stat">
      <span>{label}</span>
      <strong>{typeof value === 'number' ? number(value) : value}</strong>
      {hint && <small>{hint}</small>}
    </div>
  )
}

const statusLabel: Record<string, string> = {
  published: tx('Published'), draft: tx('Draft'), paused: tx('Paused'), closed: tx('Closed'), pending_review: tx('Pending review'), sold: tx('Sold'),
}
const roleLabel: Record<string, string> = {
  customer: tx('Customer'), seller: tx('Seller'), staff: tx('Staff'), admin: tx('Administrator'), operator: tx('Operator'), moderator: tx('Moderator'), delivery_partner: tx('Delivery partner'),
}

/** What the platform holds today: accounts, listings, orders, conversations. */
export function OverviewContent({ data }: { data: AdminOverview }) {
  const listings = Object.entries(data.listingsByStatus).sort((a, b) => b[1] - a[1])
  const roles = Object.entries(data.users.byRole).sort((a, b) => b[1] - a[1])
  const totalListings = listings.reduce((sum, [, n]) => sum + n, 0)
  return (
    <>
      <div className="tz-statbar">
        <Stat label={t('Accounts')} value={data.users.total} hint={t('{n} new in the last 7 days', { n: number(data.users.newLast7Days) })} />
        <Stat label={t('Active accounts')} value={data.users.active} hint={t('{n} suspended', { n: number(data.users.suspended) })} />
        <Stat label={t('Listings')} value={totalListings} hint={t('{n} organizations', { n: number(data.organizations) })} />
        <Stat label={t('Orders')} value={data.orders.total} hint={t('{n} in the last 7 days', { n: number(data.orders.last7Days) })} />
        <Stat label={t('Reservations')} value={data.reservations} />
        <Stat label={t('Conversations')} value={data.conversationsLast7Days} hint={t('in the last 7 days')} />
      </div>
      <div className="tzc-grid-cards tz-dash-gap">
        <ChartCard title={t('Listings by status')} subtitle={t('Everything on the platform today')}>
          <BreakdownBars label={t('Listings by status')} items={listings.map(([key, value]) => ({ key, label: t(statusLabel[key] ?? key), value, tone: statusTone(key) }))} />
        </ChartCard>
        <ChartCard title={t('Accounts by role')} subtitle={t('People who can sign in')}>
          <BreakdownBars label={t('Accounts by role')} items={roles.map(([key, value], i) => ({ key, label: t(roleLabel[key] ?? key), value, tone: (i % 6) + 1 }))} />
        </ChartCard>
      </div>
    </>
  )
}

/** The key figures of the period, each opening its own chart, and what the activity is made of. */
export function PlatformActivity({ data }: { data: Analytics }) {
  const { current: now, previous: before, days } = data
  const currencies = currenciesOf(data)
  const main = currencies[0]!
  const revenue = revenueParts(now.revenue, currencies)
  const metrics: BoardMetric[] = [
    { id: 'revenue', tile: { label: t('Revenue'), icon: 'wallet', value: revenue.value, current: now.revenue[main] ?? 0, previous: before.revenue[main] ?? 0, days, spark: revenueSeries(data, main), note: revenue.note, hero: true }, panel: <RevenueCard data={data} wide={false} /> },
    { id: 'orders', tile: { label: t('Orders placed'), icon: 'orders', value: integer(now.orders), current: now.orders, previous: before.orders, days, spark: counts(data.orders) }, panel: <CountCard title={t('Orders per day')} subtitle={t('Orders placed (carts excluded)')} series={data.orders} unit={t('Orders')} /> },
    { id: 'bookings', tile: { label: t('Bookings'), icon: 'calendar', value: integer(now.reservations), current: now.reservations, previous: before.reservations, days, spark: counts(data.reservations) }, panel: <CountCard title={t('Bookings per day')} subtitle={t('Reservations created')} series={data.reservations} unit={t('Bookings')} /> },
    { id: 'buyers', tile: { label: t('Buyers'), icon: 'users', value: integer(now.customers), current: now.customers, previous: before.customers, days, note: t('Returning buyers: {n}', { n: integer(now.returningCustomers) }) } },
    { id: 'accounts', tile: { label: t('New accounts'), icon: 'user', value: integer(now.newUsers), current: now.newUsers, previous: before.newUsers, days, spark: counts(data.newUsers) }, panel: <CountCard title={t('New accounts per day')} subtitle={t('People who signed up')} series={data.newUsers} kind="area" unit={t('Accounts')} /> },
    { id: 'conversations', tile: { label: t('Conversations'), icon: 'inbox', value: integer(now.conversations), current: now.conversations, previous: before.conversations, days, spark: counts(data.conversations) }, panel: <CountCard title={t('Conversations per day')} subtitle={t('Conversations with the assistant')} series={data.conversations} kind="area" unit={t('Conversations')} /> },
    { id: 'listings', tile: { label: t('New listings'), icon: 'shop', value: integer(now.newListings), current: now.newListings, previous: before.newListings, days, spark: counts(data.newListings) }, panel: <CountCard title={t('New listings per day')} subtitle={t('Listings created, any status')} series={data.newListings} unit={t('Listings')} /> },
  ]
  return (
    <>
      <ActivityBoard label={t('Key figures')} metrics={metrics} />

      <div className="tzc-grid-cards three tz-dash-gap">
        <ChartCard title={t('Orders by status')} subtitle={t('Placed in this period')}>
          <Donut label={t('Orders by status')} centre={t('orders')} items={Object.entries(data.ordersByStatus).map(([key, value]) => ({ key, label: labelOf(key), value, tone: statusTone(key) }))} />
        </ChartCard>
        <ChartCard title={t('Bookings by status')} subtitle={t('Created in this period')}>
          <Donut label={t('Bookings by status')} centre={t('bookings')} items={Object.entries(data.reservationsByStatus).map(([key, value]) => ({ key, label: labelOf(key), value, tone: statusTone(key) }))} />
        </ChartCard>
        <ChartCard title={t('Live listings by sector')} subtitle={t('Published today')}>
          <BreakdownBars label={t('Live listings by sector')} items={Object.entries(data.publishedByVertical).map(([key, value], i) => ({ key, label: verticalLabel(key), value, tone: (i % 6) + 1 }))} />
        </ChartCard>
      </div>

      <div className="tz-dash-gap">
        <ChartCard title={t('Top businesses')} subtitle={t('By orders placed in this period')} wide>
          <RankedList label={t('Businesses with the most orders')} entries={data.topOrganizations} empty={t('No orders in this period.')} meta={(e) => tn(e.orders, '{n} order', '{n} orders')} weight={(e) => e.orders} />
        </ChartCard>
      </div>
    </>
  )
}

export default function AdminOverviewPage() {
  const [period, setPeriod] = useState(30)
  const overview = useAsync(() => adminApi.overview(), [])
  const activity = useAsync(() => analyticsApi.platform(period), [period])
  const reload = () => { overview.reload(); activity.reload() }
  const stamp = activity.data?.generatedAt || overview.data?.generatedAt
  return (
    <section className="admin-page tz-dash">
      <div className="tz-dash-head"><Heading level={1}>{t('Overview')}</Heading></div>
      <DashToolbar
        asOf={stamp ? t('Updated {time}', { time: new Date(stamp).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) }) : undefined}
        period={period} onPeriod={setPeriod} onRefresh={reload} exportData={activity.data}
      />

      <div className="tz-dash-section">
        <h2>{t('Platform today')}</h2>
        {overview.loading && !overview.data && <Loading label={t('Loading the overview…')} />}
        {overview.error && <ErrorState error={overview.error} onRetry={overview.reload} />}
        {overview.data && <OverviewContent data={overview.data} />}
      </div>

      <div className="tz-dash-section">
        <h2>{tn(period, 'Activity over the last day', 'Activity over the last {n} days')}</h2>
        {activity.loading && !activity.data && <DashSkeleton label={t('Loading the activity…')} />}
        {activity.error && <ErrorState error={activity.error} onRetry={activity.reload} />}
        {activity.data && <PlatformActivity data={activity.data} />}
      </div>
    </section>
  )
}
