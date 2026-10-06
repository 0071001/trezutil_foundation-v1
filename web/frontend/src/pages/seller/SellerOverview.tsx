import { useState } from 'react'
import { analyticsApi } from '@/api/analytics'
import { sellerApi } from '@/api/seller'
import { integer, shortDay } from '@/components/charts/scale'
import type { BoardMetric } from '@/components/dashboard/ActivityBoard'
import { DashSkeleton, DashToolbar } from '@/components/dashboard/DashToolbar'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Heading, Icon, StatusBadge } from '@/components/ui'
import { formatDate, formatMinor, timeAgo } from '@/lib/format'
import { t, plural, tn } from '@/lib/i18n'
import { useAsync } from '@/lib/useAsync'
import { SellerActivity } from '@/pages/seller/SellerActivity'
import { Metric, SellerHead, useSeller } from '@/pages/seller/SellerShell'

function greeting(): string {
  const h = new Date().getHours()
  return h < 12 ? t('Good morning') : h < 18 ? t('Good afternoon') : t('Good evening')
}

/**
 * The business dashboard. For owners, admins and managers the figures and charts
 * come from the platform's own counts of this business's orders, bookings and
 * listings (GET /sellers/analytics: complete, not limited to the latest rows).
 * Below them, what needs doing now and the latest activity are read from the
 * most recent orders and bookings. Other members see the catalogue only.
 */
export function SellerOverviewPage() {
  const { org, canManage } = useSeller()
  const [period, setPeriod] = useState(30)

  const lists = useAsync(async () => {
    const [listings, orders, reservations] = await Promise.all([
      sellerApi.listings(org.organization_id),
      canManage ? sellerApi.incomingOrders(org.organization_id) : Promise.resolve([]),
      canManage ? sellerApi.incomingReservations(org.organization_id) : Promise.resolve([]),
    ])
    return { listings, orders, reservations }
  }, [org.organization_id, canManage])
  const activity = useAsync(() => analyticsApi.organization(org.organization_id, period), [org.organization_id, period], canManage)

  if (lists.loading) return <Loading label={t('Loading your business…')} />
  if (lists.error || !lists.data) return <ErrorState error={lists.error ?? t('Could not load')} onRetry={lists.reload} />

  const { listings, orders, reservations } = lists.data
  const now = Date.now() / 1000
  const toConfirm = orders.filter((o) => o.status === 'paid')
  const upcoming = reservations.filter((r) => r.end_at > now && (r.status === 'pending' || r.status === 'confirmed'))
  const live = listings.filter((l) => l.status === 'published').length
  const drafts = listings.filter((l) => l.status === 'draft').length
  const activityRows = [
    ...orders.map((o) => ({ id: `o${o.id}`, icon: 'orders' as const, title: o.title ? t('Order for {title}', { title: o.title }) : t('Order #{id}', { id: o.id.slice(0, 8) }), status: o.status, at: o.updated_at || o.created_at, href: '/seller/orders' })),
    ...reservations.map((r) => ({ id: `r${r.id}`, icon: 'calendar' as const, title: t('Booking {formatDate}', { formatDate: formatDate(r.start_at) }), status: r.status, at: r.updated_at || r.created_at, href: '/seller/reservations' })),
  ].sort((a, b) => b.at - a.at).slice(0, 6)

  const needsSetup = !org.seller_profiles.some((p) => p.mine) || org.stores.length === 0
  // What is waiting for the business, from the latest orders and bookings: a tile beside the figures of the period.
  const pendingBookings = upcoming.filter((r) => r.status === 'pending').length
  const toDo: BoardMetric = { id: 'todo', tile: { label: t('To confirm'), icon: 'clock', value: integer(toConfirm.length), note: tn(pendingBookings, '{n} booking request', '{n} booking requests') } }

  return (
    <>
      <SellerHead eyebrow={t('Seller workspace')} title={`${greeting()}, ${org.display_name}.`} copy={t('Here’s what’s happening with your business.')} action={canManage ? <ActionLink className="primary-button" href="/seller/listings/new">{t('Create listing')} <Icon name="arrow" /></ActionLink> : undefined} />
      {needsSetup && <p className="inline-notice info" role="note"><Icon name="bell" size={16} /><span>{t('Your workspace isn’t fully set up yet (seller profile or store missing). The Trezutil team completes this during onboarding — see')} <ActionLink href="/seller/business">{t('Business')}</ActionLink>.</span></p>}

      {canManage ? (
        <>
          <DashToolbar
            asOf={activity.data ? t('Counted by the platform from your orders, bookings and listings, {from} to {to}.', { from: shortDay(activity.data.from), to: shortDay(activity.data.to) }).replace(/\.\.$/, '.') : undefined}
            period={period} onPeriod={setPeriod} onRefresh={() => { lists.reload(); activity.reload() }} exportData={activity.data} exportName={org.display_name}
          >
            <ActionLink href="/seller/analytics" className="tz-link-action">{t('All analytics')} <Icon name="arrow" size={14} /></ActionLink>
          </DashToolbar>
          {activity.loading && !activity.data && <DashSkeleton label={t('Loading your figures…')} />}
          {activity.error && <ErrorState error={activity.error} onRetry={activity.reload} />}
          {activity.data && <SellerActivity data={activity.data} extra={[toDo]} />}
        </>
      ) : (
        <div className="seller-metrics">
          <Metric label={t('Live listings')} value={String(live)} note={plural(drafts, 'draft')} icon="spark" source={t('Your listings')} />
        </div>
      )}

      <div className="seller-main-grid tz-dash-section">
        <section className="seller-panel" aria-labelledby="attention-title">
          <div className="panel-head"><Heading level={2} id="attention-title">{t('Needs your attention')}</Heading><ActionLink href="/seller/orders">{t('View all')}</ActionLink></div>
          {toConfirm.length === 0 && upcoming.filter((r) => r.status === 'pending').length === 0 && <p className="panel-empty">{t('You’re all caught up.')}</p>}
          {toConfirm.slice(0, 4).map((o) => (
            <ActionLink className="activity-row" href="/seller/orders" key={o.id}><span className="activity-icon"><Icon name="orders" size={17} /></span><div><strong>{t('Confirm')} {o.title || t('order #{slice}', { slice: o.id.slice(0, 8) })}</strong><small>{t('Paid')} {formatMinor(o.total_minor, o.currency)} · {timeAgo(o.created_at)}</small></div></ActionLink>
          ))}
          {upcoming.filter((r) => r.status === 'pending').slice(0, 3).map((r) => (
            <ActionLink className="activity-row" href="/seller/reservations" key={r.id}><span className="activity-icon mint"><Icon name="calendar" size={17} /></span><div><strong>{t('Booking request for')} {formatDate(r.start_at)}</strong><small>{plural(r.party_size, 'guest')}</small></div></ActionLink>
          ))}
        </section>
        <section className="seller-panel activity-panel" aria-labelledby="activity-title">
          <div className="panel-head"><Heading level={2} id="activity-title">{t('Recent activity')}</Heading></div>
          {activityRows.length === 0 && <p className="panel-empty">{canManage ? t('Orders and bookings will appear here.') : t('Only managers see orders and bookings.')}</p>}
          {activityRows.map((a, i) => (
            <ActionLink className="activity-row" href={a.href} key={a.id}><span className={`activity-icon ${i % 2 ? 'mint' : ''}`}><Icon name={a.icon} size={17} /></span><div><strong>{a.title}</strong><small>{timeAgo(a.at)}</small></div><StatusBadge status={a.status} /></ActionLink>
          ))}
        </section>
      </div>
    </>
  )
}
