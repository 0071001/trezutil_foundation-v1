import { sellerApi } from '@/api/seller'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Heading, Icon, StatusBadge } from '@/components/ui'
import { formatDate, formatMinor, timeAgo } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { Metric, SellerHead, useSeller } from '@/pages/seller/SellerShell'

const earning = new Set(['paid', 'confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed'])

function greeting(): string {
  const h = new Date().getHours()
  return h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
}

/** Figma SellerHomePage with metrics computed from the business's real orders, bookings and listings. */
export function SellerOverviewPage() {
  const { org, canManage } = useSeller()
  const data = useAsync(async () => {
    const [listings, orders, reservations] = await Promise.all([
      sellerApi.listings(org.organization_id),
      canManage ? sellerApi.incomingOrders(org.organization_id) : Promise.resolve([]),
      canManage ? sellerApi.incomingReservations(org.organization_id) : Promise.resolve([]),
    ])
    return { listings, orders, reservations }
  }, [org.organization_id, canManage])

  if (data.loading) return <Loading label="Loading your business…" />
  if (data.error || !data.data) return <ErrorState error={data.error ?? 'Could not load'} onRetry={data.reload} />

  const { listings, orders, reservations } = data.data
  const now = Date.now() / 1000
  const week = now - 7 * 86400
  const weekSales = orders.filter((o) => earning.has(o.status) && o.created_at >= week)
  const currency = weekSales[0]?.currency || orders[0]?.currency || 'USD'
  const salesMinor = weekSales.filter((o) => o.currency === currency).reduce((n, o) => n + o.total_minor, 0)
  const toConfirm = orders.filter((o) => o.status === 'paid')
  const awaitingPayment = orders.filter((o) => o.status === 'pending_payment' || o.status === 'draft')
  const upcoming = reservations.filter((r) => r.end_at > now && (r.status === 'pending' || r.status === 'confirmed'))
  const today = upcoming.filter((r) => new Date(r.start_at * 1000).toDateString() === new Date().toDateString())
  const live = listings.filter((l) => l.status === 'published').length
  const drafts = listings.filter((l) => l.status === 'draft').length
  const activity = [
    ...orders.map((o) => ({ id: `o${o.id}`, icon: 'orders' as const, title: `Order ${o.title ? `for ${o.title}` : `#${o.id.slice(0, 8)}`}`, status: o.status, at: o.updated_at || o.created_at, href: '/seller/orders' })),
    ...reservations.map((r) => ({ id: `r${r.id}`, icon: 'calendar' as const, title: `Booking ${formatDate(r.start_at)}`, status: r.status, at: r.updated_at || r.created_at, href: '/seller/reservations' })),
  ].sort((a, b) => b.at - a.at).slice(0, 6)

  const needsSetup = !org.seller_profiles.some((p) => p.mine) || org.stores.length === 0

  return (
    <>
      <SellerHead eyebrow="Seller workspace" title={`${greeting()}, ${org.display_name}.`} copy="Here’s what’s happening with your business." action={canManage ? <ActionLink className="primary-button" href="/seller/listings/new">Create listing <Icon name="arrow" /></ActionLink> : undefined} />
      {needsSetup && <p className="inline-notice info" role="note"><Icon name="bell" size={16} /><span>Your workspace isn’t fully set up yet (seller profile or store missing). The Trezutil team completes this during onboarding — see <ActionLink href="/seller/business">Business</ActionLink>.</span></p>}
      <div className="seller-metrics">
        <Metric label="Sales this week" value={canManage ? formatMinor(salesMinor, currency) : '—'} note={`${weekSales.length} paid order${weekSales.length === 1 ? '' : 's'}`} icon="orders" />
        <Metric label="Orders to confirm" value={canManage ? String(toConfirm.length) : '—'} note={`${awaitingPayment.length} awaiting payment`} icon="shop" />
        <Metric label="Upcoming bookings" value={canManage ? String(upcoming.length) : '—'} note={`${today.length} today`} icon="calendar" />
        <Metric label="Live listings" value={String(live)} note={`${drafts} draft${drafts === 1 ? '' : 's'}`} icon="spark" />
      </div>
      <div className="seller-main-grid">
        <section className="seller-panel" aria-labelledby="attention-title">
          <div className="panel-head"><Heading level={2} id="attention-title">Needs your attention</Heading><ActionLink href="/seller/orders">View all</ActionLink></div>
          {toConfirm.length === 0 && upcoming.filter((r) => r.status === 'pending').length === 0 && <p className="panel-empty">You’re all caught up.</p>}
          {toConfirm.slice(0, 4).map((o) => (
            <ActionLink className="activity-row" href="/seller/orders" key={o.id}><span className="activity-icon"><Icon name="orders" size={17} /></span><div><strong>Confirm {o.title || `order #${o.id.slice(0, 8)}`}</strong><small>Paid {formatMinor(o.total_minor, o.currency)} · {timeAgo(o.created_at)}</small></div></ActionLink>
          ))}
          {upcoming.filter((r) => r.status === 'pending').slice(0, 3).map((r) => (
            <ActionLink className="activity-row" href="/seller/reservations" key={r.id}><span className="activity-icon mint"><Icon name="calendar" size={17} /></span><div><strong>Booking request for {formatDate(r.start_at)}</strong><small>{r.party_size} guest{r.party_size === 1 ? '' : 's'}</small></div></ActionLink>
          ))}
        </section>
        <section className="seller-panel activity-panel" aria-labelledby="activity-title">
          <div className="panel-head"><Heading level={2} id="activity-title">Recent activity</Heading></div>
          {activity.length === 0 && <p className="panel-empty">{canManage ? 'Orders and bookings will appear here.' : 'Only managers see orders and bookings.'}</p>}
          {activity.map((a, i) => (
            <ActionLink className="activity-row" href={a.href} key={a.id}><span className={`activity-icon ${i % 2 ? 'mint' : ''}`}><Icon name={a.icon} size={17} /></span><div><strong>{a.title}</strong><small>{timeAgo(a.at)}</small></div><StatusBadge status={a.status} /></ActionLink>
          ))}
        </section>
      </div>
    </>
  )
}
