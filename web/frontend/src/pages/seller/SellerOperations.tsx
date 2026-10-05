import { useState } from 'react'
import { toApiError } from '@/api/client'
import { sellerApi } from '@/api/seller'
import { ListingImage } from '@/components/cards'
import { EmptyState, ErrorState, SkeletonGrid } from '@/components/states'
import { Button, Heading, Icon, StatusBadge, Tabs } from '@/components/ui'
import { formatDate, formatDateTime, formatMinor, sentence, timeAgo } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { SellerHead, useSeller } from '@/pages/seller/SellerShell'
import { t, plural } from '@/lib/i18n'

type OrderTab = 'To confirm' | 'Awaiting payment' | 'In progress' | 'Completed' | 'Cancelled'
const orderTab = (s: string): OrderTab =>
  s === 'paid' ? 'To confirm'
    : s === 'draft' || s === 'pending_payment' ? 'Awaiting payment'
      : ['confirmed', 'processing', 'ready_for_fulfillment'].includes(s) ? 'In progress'
        : ['fulfilled', 'completed'].includes(s) ? 'Completed' : 'Cancelled'

/** Figma SellerOperationsPage (orders). Only paid orders can be confirmed (server rule). */
export function SellerOrdersPage() {
  const { org, canManage } = useSeller()
  const notify = useToast()
  const orders = useAsync(() => sellerApi.incomingOrders(org.organization_id), [org.organization_id], canManage)
  const [tab, setTab] = useState<OrderTab>('To confirm')
  const [busy, setBusy] = useState<string | null>(null)
  if (!canManage) return <><SellerHead eyebrow={t('Operations')} title={t('Orders')} copy={t('Move every order forward with clear next actions.')} /><EmptyState icon="user" title={t('Managers only')} copy={t('Orders are visible to the business owner, admins and managers.')} /></>
  const all = orders.data ?? []
  const rows = all.filter((o) => orderTab(o.status) === tab)
  const confirm = async (id: string) => {
    setBusy(id)
    try {
      await sellerApi.confirmOrder(id)
      notify(t('Order confirmed — the customer has been updated.'), 'success')
      orders.reload()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setBusy(null)
    }
  }
  const tabs: OrderTab[] = ['To confirm', 'Awaiting payment', 'In progress', 'Completed', 'Cancelled']
  return (
    <>
      <SellerHead eyebrow={t('Operations')} title={t('Orders')} copy={t('Move every order forward with clear next actions.')} />
      <Tabs label={t('Order status')} items={tabs.map((t) => ({ value: t, label: t, count: orders.data ? all.filter((o) => orderTab(o.status) === t).length : undefined }))} value={tab} onChange={setTab} />
      {orders.loading && <SkeletonGrid count={3} layout="list" />}
      {orders.error && <ErrorState error={orders.error} onRetry={orders.reload} />}
      {orders.data && rows.length === 0 && <EmptyState icon="orders" title={({ 'To confirm': t('No orders to confirm'), 'Awaiting payment': t('No orders awaiting payment'), 'In progress': t('No orders in progress'), Completed: t('No completed orders'), Cancelled: t('No cancelled orders') } as Record<string, string>)[tab]} copy={tab === 'To confirm' ? t('Paid orders waiting for your confirmation appear here.') : t('Orders move between these tabs as they progress.')} />}
      {rows.length > 0 && (
        <div className="operations-table">
          {rows.map((o) => (
            <article className="operation-card" key={o.id}>
              <div className="operation-thumb"><ListingImage src={null} alt="" type="physical_product" /></div>
              <div className="operation-copy">
                <div><span>#{o.id.slice(0, 8)} · {timeAgo(o.created_at)}</span><StatusBadge status={o.status} /></div>
                <Heading level={3}>{o.title || t('Order')}</Heading>
                <p>{t('Quantity')} {o.quantity || 1}{o.delivery_status ? t(' · Delivery {sentence}', { sentence: sentence(o.delivery_status) }) : ''}</p>
              </div>
              <strong className="operation-price">{formatMinor(o.total_minor, o.currency)}</strong>
              {o.status === 'paid'
                ? <Button className="primary-button" busy={busy === o.id} onClick={() => void confirm(o.id)}>{t('Confirm')} <Icon name="check" size={16} /></Button>
                : <small className="operation-note">{formatDateTime(o.updated_at || o.created_at)}</small>}
            </article>
          ))}
        </div>
      )}
      <p className="field-hint">{t('Fulfilment (pickup and delivery tracking) is arranged directly with the customer; delivery updates from logistics partners appear on the order when available.')}</p>
    </>
  )
}

type ResTab = 'Pending' | 'Confirmed' | 'Past' | 'Cancelled'
const resTab = (status: string, endAt: number): ResTab =>
  status === 'cancelled' ? 'Cancelled' : status === 'completed' || endAt < Date.now() / 1000 ? 'Past' : status === 'pending' ? 'Pending' : 'Confirmed'

export function SellerReservationsPage() {
  const { org, canManage } = useSeller()
  const data = useAsync(async () => {
    const [reservations, listings] = await Promise.all([sellerApi.incomingReservations(org.organization_id), sellerApi.listings(org.organization_id)])
    return { reservations, titles: Object.fromEntries(listings.map((l) => [l.id, l])) }
  }, [org.organization_id], canManage)
  const [tab, setTab] = useState<ResTab>('Confirmed')
  if (!canManage) return <><SellerHead eyebrow={t('Operations')} title={t('Reservations')} copy={t('Keep dates, guests and confirmations under control.')} /><EmptyState icon="user" title={t('Managers only')} copy={t('Bookings are visible to the business owner, admins and managers.')} /></>
  const all = data.data?.reservations ?? []
  const rows = all.filter((r) => resTab(r.status, r.end_at) === tab).sort((a, b) => a.start_at - b.start_at)
  return (
    <>
      <SellerHead eyebrow={t('Operations')} title={t('Reservations')} copy={t('Keep dates, guests and confirmations under control.')} />
      <Tabs label={t('Reservation status')} items={(['Pending', 'Confirmed', 'Past', 'Cancelled'] as ResTab[]).map((t) => ({ value: t, label: t, count: data.data ? all.filter((r) => resTab(r.status, r.end_at) === t).length : undefined }))} value={tab} onChange={setTab} />
      {data.loading && <SkeletonGrid count={3} layout="list" />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && rows.length === 0 && <EmptyState icon="calendar" title={({ Pending: t('No pending bookings'), Confirmed: t('No confirmed bookings'), Past: t('No past bookings'), Cancelled: t('No cancelled bookings') } as Record<string, string>)[tab]} copy={t('Reservations for your listings appear here, with dates and guests.')} />}
      {rows.length > 0 && (
        <div className="operations-table">
          {rows.map((r) => {
            const l = data.data!.titles[r.listing_id]
            return (
              <article className="operation-card" key={r.id}>
                <div className="operation-thumb"><ListingImage src={l?.media[0]?.url} alt="" type={l?.listing_type ?? 'real_estate'} /></div>
                <div className="operation-copy"><span>#{r.id.slice(0, 8)}</span><Heading level={3}>{l?.title ?? t('Reservation')}</Heading><p>{formatDate(r.start_at)} → {formatDate(r.end_at)} · {plural(r.party_size, 'guest')}</p></div>
                <StatusBadge status={r.status} />
                <small className="operation-note">{t('Booked')} {timeAgo(r.created_at)}</small>
              </article>
            )
          })}
        </div>
      )}
      <p className="field-hint">{t('Bookings are confirmed automatically when the dates are available; customers can cancel from their reservations.')}</p>
    </>
  )
}

export function SellerInventoryPage() {
  const { org, canManage } = useSeller()
  const data = useAsync(async () => {
    const [stock, listings] = await Promise.all([sellerApi.inventory(org.organization_id), sellerApi.listings(org.organization_id)])
    return { stock, byId: Object.fromEntries(listings.map((l) => [l.id, l])) }
  }, [org.organization_id], canManage)
  if (!canManage) return <><SellerHead eyebrow={t('Operations')} title={t('Inventory')} copy={t('Stock and capacity for your listings.')} /><EmptyState icon="user" title={t('Managers only')} copy={t('Inventory is visible to the business owner, admins and managers.')} /></>
  return (
    <>
      <SellerHead eyebrow={t('Operations')} title={t('Inventory')} copy={t('Stock and capacity for your listings. Reservations and orders update it automatically.')} />
      {data.loading && <SkeletonGrid count={3} layout="list" />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && data.data.stock.length === 0 && <EmptyState icon="briefcase" title={t('No stock tracked yet')} copy={t('Stock is tracked for listings created with a quantity. Listings without stock are always available.')} />}
      {data.data && data.data.stock.length > 0 && (
        <div className="listing-table" role="table" aria-label={t('Inventory')}>
          <div className="table-head inventory-head" role="row"><span role="columnheader">{t('Listing')}</span><span role="columnheader">{t('Available')}</span><span role="columnheader">{t('Reserved')}</span><span role="columnheader">{t('Sold')}</span><span role="columnheader">{t('Total')}</span></div>
          {data.data.stock.map((s) => {
            const l = data.data!.byId[s.listing_id]
            return (
              <div className="listing-row inventory-row" role="row" key={s.id}>
                <div role="cell"><ListingImage src={l?.media[0]?.url} alt="" type={l?.listing_type} /><span><strong>{l?.title ?? t('Listing {slice}', { slice: s.listing_id.slice(0, 8) })}</strong><small>{s.available === 0 ? t('Sold out') : s.available <= 2 ? t('Running low') : t('In stock')}</small></span></div>
                <b role="cell" className={s.available === 0 ? 'text-danger' : ''}>{s.available}</b><span role="cell">{s.reserved}</span><span role="cell">{s.consumed}</span><span role="cell">{s.total}</span>
              </div>
            )
          })}
        </div>
      )}
      <p className="field-hint">{t('Adjusting stock from the workspace isn’t available yet; contact the Trezutil team to restock.')}</p>
    </>
  )
}

export function SellerBusinessPage() {
  const { org } = useSeller()
  return (
    <>
      <SellerHead eyebrow={t('Business')} title={org.display_name} copy={t('Your business details on Trezutil.')} />
      <div className="reservation-detail-grid">
        <div>
          <Heading level={2}>{t('Organization')}</Heading>
          <div className="info-rows">
            <div><span>{t('Status')}</span><strong><StatusBadge status={org.status || 'pending'} label={sentence(org.status || 'pending')} /></strong></div>
            <div><span>{t('Verification')}</span><strong>{sentence(org.verification_status || 'unverified')}</strong></div>
            <div><span>{t('Your role')}</span><strong>{sentence(org.role)}</strong></div>
          </div>
          <Heading level={2}>{t('Stores')}</Heading>
          <div className="info-rows">{org.stores.length ? org.stores.map((s) => <div key={s.id}><span>{s.name}</span><strong>{sentence(s.status || 'active')}</strong></div>) : <div><span>{t('No store yet')}</span><strong>{t('Created during onboarding')}</strong></div>}</div>
          <Heading level={2}>{t('Seller profiles')}</Heading>
          <div className="info-rows">{org.seller_profiles.length ? org.seller_profiles.map((p) => <div key={p.id}><span>{p.display_name}{p.mine ? t(' (you)') : ''}</span><strong>{sentence(p.verification_status || p.status)}</strong></div>) : <div><span>{t('No seller profile yet')}</span><strong>{t('Created during onboarding')}</strong></div>}</div>
        </div>
        <aside className="summary-card">
          <Heading level={3}>{t('Need a change?')}</Heading>
          <p>{t('Business details, verification and team members are managed with the Trezutil team during this phase.')}</p>
        </aside>
      </div>
    </>
  )
}
