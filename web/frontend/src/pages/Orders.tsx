import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { toApiError } from '@/api/client'
import { ordersApi, type OrderSummary } from '@/api/commerce'
import { listingsApi } from '@/api/listings'
import { ListingImage } from '@/components/cards'
import { PaymentPanel, isAwaitingPayment } from '@/components/PaymentPanel'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, Loading, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Heading, Icon, PageIntro, StatusBadge, Tabs, statusLabel } from '@/components/ui'
import { formatDateTime, formatMinor, timeAgo } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { t, tx } from '@/lib/i18n'

const active = new Set(['draft', 'pending_payment', 'paid', 'confirmed', 'processing', 'ready_for_fulfillment'])
const done = new Set(['fulfilled', 'completed'])
type OrderTab = 'Active' | 'Completed' | 'Cancelled'

function OperationCard({ order }: { order: OrderSummary }) {
  // Identical listing lookups are de-duplicated by the HTTP client.
  const listing = useAsync(() => listingsApi.get(order.listing_id), [order.listing_id], !!order.listing_id)
  return (
    <article className="operation-card">
      <div className="operation-thumb"><ListingImage src={listing.data?.media[0]?.url} alt="" type={listing.data?.listing_type ?? 'physical_product'} /></div>
      <div className="operation-copy">
        <div><span>#{order.id.slice(0, 8)} · {timeAgo(order.created_at)}</span><StatusBadge status={order.status} /></div>
        <Heading level={3}>{order.title || t('Order')}</Heading>
        <p>{order.quantity ? t('Quantity {quantity}', { quantity: order.quantity }) : ''}</p>
      </div>
      <strong className="operation-price">{formatMinor(order.total_minor, order.currency)}</strong>
      <ActionLink className="outline-button" href={`/orders/${encodeURIComponent(order.id)}`}>{t('View details')} <Icon name="arrow" size={16} /></ActionLink>
    </article>
  )
}

export function OrdersPage() {
  const orders = useAsync(() => ordersApi.list(), [])
  const [tab, setTab] = useState<OrderTab>('Active')
  useEffect(() => { document.title = t('Your orders — Trezutil') }, [])
  const list = (orders.data ?? []).filter((o) => tab === 'Active' ? active.has(o.status) : tab === 'Completed' ? done.has(o.status) : !active.has(o.status) && !done.has(o.status))
  const count = (t: OrderTab) => (orders.data ?? []).filter((o) => t === 'Active' ? active.has(o.status) : t === 'Completed' ? done.has(o.status) : !active.has(o.status) && !done.has(o.status)).length
  return (
    <AppShell>
      <section className="page-section">
        <PageIntro eyebrow={t('Your purchases')} title={t('Orders')} copy={t('Follow active orders and revisit what you’ve bought.')} />
        <Tabs label={t('Order status')} items={(['Active', 'Completed', 'Cancelled'] as OrderTab[]).map((t) => ({ value: t, label: t, count: orders.data ? count(t) : undefined }))} value={tab} onChange={setTab} />
        {orders.loading && <SkeletonGrid count={3} layout="list" />}
        {orders.error && <ErrorState error={orders.error} onRetry={orders.reload} />}
        {orders.data && list.length === 0 && <EmptyState icon="orders" title={tab === 'Active' ? t('No active orders') : (tab === 'Completed' ? t('No completed orders') : t('No cancelled orders'))} copy={t('Orders you place on Trezutil — on the web or in a conversation — appear here.')} action={t('Start discovering')} href="/discover" />}
        {list.length > 0 && <div className="operation-list">{list.map((o) => <OperationCard key={o.id} order={o} />)}</div>}
      </section>
    </AppShell>
  )
}

const timelineSteps: { key: string; label: string; reached: (s: string) => boolean }[] = [
  { key: 'placed', label: tx('Order placed'), reached: () => true },
  { key: 'paid', label: tx('Paid'), reached: (s) => !['draft', 'pending_payment', 'cancelled', 'expired'].includes(s) },
  { key: 'confirmed', label: tx('Confirmed by the seller'), reached: (s) => ['confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed'].includes(s) },
  { key: 'ready', label: tx('Ready for pickup or delivery'), reached: (s) => ['ready_for_fulfillment', 'fulfilled', 'completed'].includes(s) },
  { key: 'completed', label: tx('Completed'), reached: (s) => ['fulfilled', 'completed'].includes(s) },
]

export function OrderDetailPage() {
  const { id = '' } = useParams()
  const notify = useToast()
  const order = useAsync(() => ordersApi.get(id), [id])
  const o = order.data
  const listing = useAsync(() => listingsApi.get(o!.listing_id), [o?.listing_id], !!o?.listing_id)
  const [cancelling, setCancelling] = useState(false)
  const onUpdated = useCallback((updated: OrderSummary) => order.setData(updated), [order.setData]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { document.title = t('Order — Trezutil') }, [])

  if (order.loading) return <AppShell><Loading label={t('Loading order…')} /></AppShell>
  if (order.error || !o) return <AppShell><section className="page-section narrow-page"><ErrorState error={order.error ?? t('Order not found')} onRetry={order.reload} /></section></AppShell>

  const cancelled = ['cancelled', 'refunded', 'expired'].includes(o.status)
  const cancel = async () => {
    setCancelling(true)
    try {
      await ordersApi.cancel(o.id)
      order.setData({ ...o, status: 'cancelled' })
      notify(t('Order cancelled.'), 'success')
      order.reload()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setCancelling(false)
    }
  }

  return (
    <AppShell dockPrompt={t('I have a question about order {slice}', { slice: o.id.slice(0, 8) })}>
      <section className="page-section narrow-page">
        <div className="back-link"><ActionLink href="/orders">{t('← Back to orders')}</ActionLink></div>
        <PageIntro eyebrow={t('Order #{slice}', { slice: o.id.slice(0, 8) })} title={statusLabel(o.status)} copy={`${o.title || t('Your order')}${o.quantity ? t(' · quantity {quantity}', { quantity: o.quantity }) : ''} · ${formatMinor(o.total_minor, o.currency)}`} actions={<StatusBadge status={o.status} />} />
        <div className="order-detail-grid">
          <div className="timeline-card">
            <Heading level={2}>{t('Order progress')}</Heading>
            {cancelled ? (
              <div className="timeline"><div className="complete"><span>✓</span><div><strong>{t('Order placed')}</strong><small>{formatDateTime(o.created_at)}</small></div></div><div className="cancelled-step"><span>×</span><div><strong>{statusLabel(o.status)}</strong><small>{formatDateTime(o.updated_at)}</small></div></div></div>
            ) : (
              <div className="timeline" role="list">
                {timelineSteps.map((step, index) => {
                  const reached = step.reached(o.status)
                  return <div role="listitem" className={reached ? 'complete' : ''} key={step.key}><span aria-hidden="true">{reached ? '✓' : index + 1}</span><div><strong>{t(step.label)}</strong><small>{index === 0 ? formatDateTime(o.created_at) : reached ? t('Done') : t('Pending')}</small></div></div>
                })}
              </div>
            )}
            {o.tracking_no && <p className="tracking">{t('Tracking number:')} <strong>{o.tracking_no}</strong></p>}
          </div>
          <aside className="pickup-card">
            <span className="float-icon blue"><Icon name="shop" /></span>
            <Heading level={3}>{o.title || t('Your item')}</Heading>
            {listing.data && <div className="pickup-image"><ListingImage src={listing.data.media[0]?.url} alt="" type={listing.data.listing_type} /></div>}
            <small>{isAwaitingPayment(o) ? t('Pay to confirm this order with the seller.') : cancelled ? t('This order is closed.') : t('The seller will arrange pickup or delivery with you.')}</small>
            <PaymentPanel order={o} onUpdated={onUpdated} />
            {isAwaitingPayment(o) && <Button className="text-danger" onClick={() => void cancel()} busy={cancelling}>{t('Cancel order')}</Button>}
            {o.listing_id && <ActionLink className="outline-button" href={`/products/${encodeURIComponent(o.listing_id)}`}>{t('View listing')}</ActionLink>}
          </aside>
        </div>
        <div className="detail-ask wide-ask"><span className="dock-logo"><Icon name="spark" /></span><span><strong>{t('Ask Trezutil about this order')}</strong><small>{t('Get an update or help with anything')}</small></span><ActionLink className="primary-button" href={`/conversation?q=${encodeURIComponent(`Show my order ${o.id}`)}`}>{t('Start conversation')} <Icon name="arrow" /></ActionLink></div>
      </section>
    </AppShell>
  )
}
