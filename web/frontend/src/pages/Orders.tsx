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

const active = new Set(['draft', 'pending_payment', 'paid', 'confirmed', 'processing', 'ready_for_fulfillment'])
const done = new Set(['fulfilled', 'completed'])
type OrderTab = 'Active' | 'Completed' | 'Cancelled'

function OperationCard({ order }: { order: OrderSummary }) {
  return (
    <article className="operation-card">
      <div className="operation-thumb"><ListingImage src={null} alt="" type="physical_product" /></div>
      <div className="operation-copy">
        <div><span>#{order.id.slice(0, 8)} · {timeAgo(order.created_at)}</span><StatusBadge status={order.status} /></div>
        <Heading level={3}>{order.title || 'Order'}</Heading>
        <p>{order.quantity ? `Quantity ${order.quantity}` : ''}</p>
      </div>
      <strong className="operation-price">{formatMinor(order.total_minor, order.currency)}</strong>
      <ActionLink className="outline-button" href={`/orders/${encodeURIComponent(order.id)}`}>View details <Icon name="arrow" size={16} /></ActionLink>
    </article>
  )
}

export function OrdersPage() {
  const orders = useAsync(() => ordersApi.list(), [])
  const [tab, setTab] = useState<OrderTab>('Active')
  useEffect(() => { document.title = 'Your orders — Trezutil' }, [])
  const list = (orders.data ?? []).filter((o) => tab === 'Active' ? active.has(o.status) : tab === 'Completed' ? done.has(o.status) : !active.has(o.status) && !done.has(o.status))
  const count = (t: OrderTab) => (orders.data ?? []).filter((o) => t === 'Active' ? active.has(o.status) : t === 'Completed' ? done.has(o.status) : !active.has(o.status) && !done.has(o.status)).length
  return (
    <AppShell>
      <section className="page-section">
        <PageIntro eyebrow="Your purchases" title="Orders" copy="Follow active orders and revisit what you’ve bought." />
        <Tabs label="Order status" items={(['Active', 'Completed', 'Cancelled'] as OrderTab[]).map((t) => ({ value: t, label: t, count: orders.data ? count(t) : undefined }))} value={tab} onChange={setTab} />
        {orders.loading && <SkeletonGrid count={3} layout="list" />}
        {orders.error && <ErrorState error={orders.error} onRetry={orders.reload} />}
        {orders.data && list.length === 0 && <EmptyState icon="orders" title={tab === 'Active' ? 'No active orders' : `No ${tab.toLowerCase()} orders`} copy="Orders you place on Trezutil — on the web or in a conversation — appear here." action="Start discovering" href="/discover" />}
        {list.length > 0 && <div className="operation-list">{list.map((o) => <OperationCard key={o.id} order={o} />)}</div>}
      </section>
    </AppShell>
  )
}

const timelineSteps: { key: string; label: string; reached: (s: string) => boolean }[] = [
  { key: 'placed', label: 'Order placed', reached: () => true },
  { key: 'paid', label: 'Paid', reached: (s) => !['draft', 'pending_payment', 'cancelled', 'expired'].includes(s) },
  { key: 'confirmed', label: 'Confirmed by the seller', reached: (s) => ['confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled', 'completed'].includes(s) },
  { key: 'ready', label: 'Ready for pickup or delivery', reached: (s) => ['ready_for_fulfillment', 'fulfilled', 'completed'].includes(s) },
  { key: 'completed', label: 'Completed', reached: (s) => ['fulfilled', 'completed'].includes(s) },
]

export function OrderDetailPage() {
  const { id = '' } = useParams()
  const notify = useToast()
  const order = useAsync(() => ordersApi.get(id), [id])
  const o = order.data
  const listing = useAsync(() => listingsApi.get(o!.listing_id), [o?.listing_id], !!o?.listing_id)
  const [cancelling, setCancelling] = useState(false)
  const onUpdated = useCallback((updated: OrderSummary) => order.setData(updated), [order.setData]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { document.title = 'Order — Trezutil' }, [])

  if (order.loading) return <AppShell><Loading label="Loading order…" /></AppShell>
  if (order.error || !o) return <AppShell><section className="page-section narrow-page"><ErrorState error={order.error ?? 'Order not found'} onRetry={order.reload} /></section></AppShell>

  const cancelled = ['cancelled', 'refunded', 'expired'].includes(o.status)
  const cancel = async () => {
    setCancelling(true)
    try {
      await ordersApi.cancel(o.id)
      order.setData({ ...o, status: 'cancelled' })
      notify('Order cancelled.', 'success')
      order.reload()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setCancelling(false)
    }
  }

  return (
    <AppShell dockPrompt={`I have a question about order ${o.id.slice(0, 8)}`}>
      <section className="page-section narrow-page">
        <div className="back-link"><ActionLink href="/orders">← Back to orders</ActionLink></div>
        <PageIntro eyebrow={`Order #${o.id.slice(0, 8)}`} title={statusLabel(o.status)} copy={`${o.title || 'Your order'}${o.quantity ? ` · quantity ${o.quantity}` : ''} · ${formatMinor(o.total_minor, o.currency)}`} actions={<StatusBadge status={o.status} />} />
        <div className="order-detail-grid">
          <div className="timeline-card">
            <Heading level={2}>Order progress</Heading>
            {cancelled ? (
              <div className="timeline"><div className="complete"><span>✓</span><div><strong>Order placed</strong><small>{formatDateTime(o.created_at)}</small></div></div><div className="cancelled-step"><span>×</span><div><strong>{statusLabel(o.status)}</strong><small>{formatDateTime(o.updated_at)}</small></div></div></div>
            ) : (
              <div className="timeline" role="list">
                {timelineSteps.map((step, index) => {
                  const reached = step.reached(o.status)
                  return <div role="listitem" className={reached ? 'complete' : ''} key={step.key}><span aria-hidden="true">{reached ? '✓' : index + 1}</span><div><strong>{step.label}</strong><small>{index === 0 ? formatDateTime(o.created_at) : reached ? 'Done' : 'Pending'}</small></div></div>
                })}
              </div>
            )}
            {o.tracking_no && <p className="tracking">Tracking number: <strong>{o.tracking_no}</strong></p>}
          </div>
          <aside className="pickup-card">
            <span className="float-icon blue"><Icon name="shop" /></span>
            <Heading level={3}>{o.title || 'Your item'}</Heading>
            {listing.data && <div className="pickup-image"><ListingImage src={listing.data.media[0]?.url} alt="" type={listing.data.listing_type} /></div>}
            <small>{isAwaitingPayment(o) ? 'Pay to confirm this order with the seller.' : cancelled ? 'This order is closed.' : 'The seller will arrange pickup or delivery with you.'}</small>
            <PaymentPanel order={o} onUpdated={onUpdated} />
            {isAwaitingPayment(o) && <Button className="text-danger" onClick={() => void cancel()} busy={cancelling}>Cancel order</Button>}
            {o.listing_id && <ActionLink className="outline-button" href={`/products/${encodeURIComponent(o.listing_id)}`}>View listing</ActionLink>}
          </aside>
        </div>
        <div className="detail-ask wide-ask"><span className="dock-logo"><Icon name="spark" /></span><span><strong>Ask Trezutil about this order</strong><small>Get an update or help with anything</small></span><ActionLink className="primary-button" href={`/conversation?q=${encodeURIComponent(`Show my order ${o.id}`)}`}>Start conversation <Icon name="arrow" /></ActionLink></div>
      </section>
    </AppShell>
  )
}
