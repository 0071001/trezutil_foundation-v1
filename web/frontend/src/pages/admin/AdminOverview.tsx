import { useState } from 'react'
import { adminApi, type AdminOverview } from '@/api/admin'
import { analyticsApi, type Analytics, type RankedEntry } from '@/api/analytics'
import { BreakdownBars, Donut, statusTone } from '@/components/charts/Breakdown'
import { ChartCard } from '@/components/charts/ChartCard'
import { KpiCard } from '@/components/charts/KpiCard'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { integer } from '@/components/charts/scale'
import { CountCard, RevenueCard, counts, currenciesOf, labelOf, revenueList, revenueParts, revenueSeries, verticalLabel } from '@/components/dashboard/shared'
import { ErrorState, Loading } from '@/components/states'
import { Button, Heading } from '@/components/ui'
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
    <div className="admin-stat">
      <span>{label}</span>
      <strong>{typeof value === 'number' ? number(value) : value}</strong>
      {hint && <small>{hint}</small>}
    </div>
  )
}

function Breakdown({ title, rows, empty }: { title: string; rows: [string, number][]; empty: string }) {
  return (
    <section className="admin-card" aria-label={title}>
      <h2>{title}</h2>
      {rows.length === 0 ? <p className="admin-empty">{empty}</p> : (
        <ul className="admin-breakdown">
          {rows.map(([name, n]) => <li key={name}><span>{name}</span><strong>{number(n)}</strong></li>)}
        </ul>
      )}
    </section>
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
      <div className="admin-stats">
        <Stat label={t('Accounts')} value={data.users.total} hint={t('{n} new in the last 7 days', { n: number(data.users.newLast7Days) })} />
        <Stat label={t('Active accounts')} value={data.users.active} hint={t('{n} suspended', { n: number(data.users.suspended) })} />
        <Stat label={t('Listings')} value={totalListings} hint={t('{n} organizations', { n: number(data.organizations) })} />
        <Stat label={t('Orders')} value={data.orders.total} hint={t('{n} in the last 7 days', { n: number(data.orders.last7Days) })} />
        <Stat label={t('Reservations')} value={data.reservations} />
        <Stat label={t('Conversations')} value={data.conversationsLast7Days} hint={t('in the last 7 days')} />
      </div>
      <div className="admin-columns">
        <Breakdown title={t('Listings by status')} rows={listings.map(([s, n]) => [t(statusLabel[s] ?? s), n])} empty={t('No listings yet.')} />
        <Breakdown title={t('Accounts by role')} rows={roles.map(([r, n]) => [t(roleLabel[r] ?? r), n])} empty={t('No accounts yet.')} />
      </div>
    </>
  )
}

function TopBusinesses({ entries }: { entries: RankedEntry[] }) {
  if (entries.length === 0) return <p className="tzc-empty">{t('No orders in this period.')}</p>
  return (
    <ol className="tz-rank" aria-label={t('Businesses with the most orders')}>
      {entries.map((e, i) => {
        const earned = revenueList(e.revenue)
        return (
          <li key={e.id}>
            <span className="pos" aria-hidden="true">{i + 1}</span>
            <span className="name">{e.name}<small>{tn(e.orders, '{n} order', '{n} orders')}</small></span>
            <span className="amount">{earned || '—'}</span>
          </li>
        )
      })}
    </ol>
  )
}

/** What happened over the period, against the period before. */
export function PlatformActivity({ data }: { data: Analytics }) {
  const { current: now, previous: before, days } = data
  const currencies = currenciesOf(data)
  const main = currencies[0]!
  const revenue = revenueParts(now.revenue, currencies)
  return (
    <>
      <div className="tzc-grid-kpi" style={{ ['--kpi-cols' as string]: 3 }}>
        <KpiCard label={t('Revenue')} icon="wallet" value={revenue.value} current={now.revenue[main] ?? 0} previous={before.revenue[main] ?? 0} days={days} spark={revenueSeries(data, main)} note={revenue.note} />
        <KpiCard label={t('Orders placed')} icon="orders" value={integer(now.orders)} current={now.orders} previous={before.orders} days={days} spark={counts(data.orders)} />
        <KpiCard label={t('Bookings')} icon="calendar" value={integer(now.reservations)} current={now.reservations} previous={before.reservations} days={days} spark={counts(data.reservations)} />
        <KpiCard label={t('Buyers')} icon="users" value={integer(now.customers)} current={now.customers} previous={before.customers} days={days} note={t('Returning buyers: {n}', { n: integer(now.returningCustomers) })} />
        <KpiCard label={t('New accounts')} icon="user" value={integer(now.newUsers)} current={now.newUsers} previous={before.newUsers} days={days} spark={counts(data.newUsers)} />
        <KpiCard label={t('Conversations')} icon="inbox" value={integer(now.conversations)} current={now.conversations} previous={before.conversations} days={days} spark={counts(data.conversations)} />
      </div>

      <div className="tzc-grid-cards tz-dash-section">
        <RevenueCard data={data} />
        <CountCard title={t('Orders per day')} subtitle={t('Orders placed (carts excluded)')} series={data.orders} unit={t('Orders')} />
        <CountCard title={t('Bookings per day')} subtitle={t('Reservations created')} series={data.reservations} unit={t('Bookings')} />
        <CountCard title={t('New accounts per day')} subtitle={t('People who signed up')} series={data.newUsers} kind="area" unit={t('Accounts')} />
        <CountCard title={t('Conversations per day')} subtitle={t('Conversations with the assistant')} series={data.conversations} kind="area" unit={t('Conversations')} />
        <CountCard title={t('New listings per day')} subtitle={t('Listings created, any status')} series={data.newListings} unit={t('Listings')} wide />
      </div>

      <div className="tzc-grid-cards three tz-dash-section">
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

      <div className="tzc-grid-cards tz-dash-section">
        <ChartCard title={t('Top businesses')} subtitle={t('By orders placed in this period')} wide>
          <TopBusinesses entries={data.topOrganizations} />
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
    <section className="admin-page">
      <div className="tz-dash-head">
        <div>
          <Heading level={1}>{t('Overview')}</Heading>
          {stamp && <p className="tz-asof">{t('Updated {time}', { time: new Date(stamp).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) })}</p>}
        </div>
        <div className="tz-dash-controls">
          <PeriodTabs value={period} onChange={setPeriod} />
          <Button className="outline-button" onClick={reload}>{t('Refresh')}</Button>
        </div>
      </div>

      <div className="tz-dash-section" style={{ marginTop: 0 }}>
        <h2>{t('Platform today')}</h2>
        {overview.loading && !overview.data && <Loading label={t('Loading the overview…')} />}
        {overview.error && <ErrorState error={overview.error} onRetry={overview.reload} />}
        {overview.data && <OverviewContent data={overview.data} />}
      </div>

      <div className="tz-dash-section">
        <h2>{tn(period, 'Activity over the last day', 'Activity over the last {n} days')}</h2>
        {activity.loading && !activity.data && <Loading label={t('Loading the activity…')} />}
        {activity.error && <ErrorState error={activity.error} onRetry={activity.reload} />}
        {activity.data && <PlatformActivity data={activity.data} />}
      </div>
    </section>
  )
}
