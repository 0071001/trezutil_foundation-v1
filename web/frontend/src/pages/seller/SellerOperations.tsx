import { useEffect, useState } from 'react'
import { toApiError } from '@/api/client'
import { consoleApi, type BusinessDetails, type StoreDetails } from '@/api/console'
import { sellerApi, type OrderStep } from '@/api/seller'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import { TextField } from '@/components/portal/ConsoleBits'
import { ListingImage } from '@/components/cards'
import { EmptyState, ErrorState, SkeletonGrid } from '@/components/states'
import { Button, Heading, Icon, StatusBadge, Tabs } from '@/components/ui'
import { formatDate, formatDateTime, formatMinor, sentence, timeAgo } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { SellerHead, useSeller } from '@/pages/seller/SellerShell'
import { t, tx, plural } from '@/lib/i18n'

type OrderTab = 'To confirm' | 'Awaiting payment' | 'In progress' | 'Completed' | 'Cancelled'
// A delivered order stays "In progress" until the seller closes it: closing is the last step of the lifecycle.
const orderTab = (s: string): OrderTab =>
  s === 'paid' ? 'To confirm'
    : s === 'draft' || s === 'pending_payment' ? 'Awaiting payment'
      : ['confirmed', 'processing', 'ready_for_fulfillment', 'fulfilled'].includes(s) ? 'In progress'
        : s === 'completed' ? 'Completed' : 'Cancelled'

/**
 * The one step each status allows (the server enforces the same order: paid → confirmed → processing →
 * ready → fulfilled → completed, seller side only). A status with no step is waiting for someone else.
 */
const nextOrderStep: Record<string, { step: OrderStep; label: string; done: string }> = {
  paid: { step: 'confirm', label: tx('Confirm'), done: tx('Order confirmed — the customer has been updated.') },
  confirmed: { step: 'process', label: tx('Start preparing'), done: tx('Order is being prepared — the customer has been updated.') },
  processing: { step: 'ready', label: tx('Mark ready'), done: tx('Order marked ready — the customer has been updated.') },
  ready_for_fulfillment: { step: 'fulfil', label: tx('Mark delivered'), done: tx('Order marked delivered — the customer has been updated.') },
  fulfilled: { step: 'complete', label: tx('Complete order'), done: tx('Order completed.') },
}

/** Figma SellerOperationsPage (orders). Each order offers the one next step its status allows. */
export function SellerOrdersPage() {
  const { org, canManage } = useSeller()
  const notify = useToast()
  const orders = useAsync(() => sellerApi.incomingOrders(org.organization_id), [org.organization_id], canManage)
  const [tab, setTab] = useState<OrderTab>('To confirm')
  const [busy, setBusy] = useState<string | null>(null)
  if (!canManage) return <><SellerHead eyebrow={t('Operations')} title={t('Orders')} copy={t('Move every order forward with clear next actions.')} /><EmptyState icon="user" title={t('Managers only')} copy={t('Orders are visible to the business owner, admins and managers.')} /></>
  const all = orders.data ?? []
  const rows = all.filter((o) => orderTab(o.status) === tab)
  const advance = async (id: string, status: string) => {
    const next = nextOrderStep[status]
    if (!next) return
    setBusy(id)
    try {
      await sellerApi.advanceOrder(id, next.step)
      notify(t(next.done), 'success')
    } catch (err) {
      // a 409 means the order is no longer where this page thought it was: show the truth, not the stale button
      notify(toApiError(err).message, 'error')
    } finally {
      setBusy(null)
      orders.reload()
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
              {nextOrderStep[o.status]
                ? <Button className="primary-button" busy={busy === o.id} onClick={() => void advance(o.id, o.status)}>{t(nextOrderStep[o.status].label)} <Icon name="check" size={16} /></Button>
                : <small className="operation-note">{formatDateTime(o.updated_at || o.created_at)}</small>}
            </article>
          ))}
        </div>
      )}
      <p className="field-hint">{t('Move each order forward as you prepare and hand it over: the customer follows every step on their order. Assigning a courier and tracking a delivery are not available yet; arrange them directly with the customer.')}</p>
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
  const notify = useToast()
  const [adjusting, setAdjusting] = useState<{ listingId: string; title: string; available: number } | null>(null)
  const [delta, setDelta] = useState('')
  const [why, setWhy] = useState('')
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
                <b role="cell" className={s.available === 0 ? 'text-danger' : ''}>{s.available}</b><span role="cell">{s.reserved}</span><span role="cell">{s.consumed}</span><span role="cell">{s.total} <button type="button" className="tz-small-btn" onClick={() => { setDelta(''); setWhy(''); setAdjusting({ listingId: s.listing_id, title: l?.title ?? s.listing_id.slice(0, 8), available: s.available }) }}>{t('Adjust')}<span className="sr-only"> {l?.title}</span></button></span>
              </div>
            )
          })}
        </div>
      )}
      <p className="field-hint">{t('Restock or write off from here: only the total changes, what is reserved and sold follows orders and bookings. Every change is recorded with its reason.')}</p>
      <ConfirmationDialog open={!!adjusting} title={t('Adjust stock of {title}', { title: adjusting?.title ?? '' })} confirmLabel={t('Save change')}
        onConfirm={async () => {
          if (!adjusting) return
          const n = Number(delta)
          if (!Number.isInteger(n) || n === 0) throw new Error(t('Enter a whole number other than zero: 5 adds five, -2 removes two.'))
          if (why.trim().length < 3) throw new Error(t('Give a reason of at least 3 characters.'))
          await consoleApi.adjustStock(org.organization_id, adjusting.listingId, n, why.trim())
          notify(t('Stock updated.'), 'success')
          data.reload()
        }}
        onClose={() => setAdjusting(null)}>
        <p>{t('Available now: {n}. The total cannot go below what is reserved and sold.', { n: adjusting?.available ?? 0 })}</p>
        <TextField label={t('Change')} value={delta} onChange={setDelta} inputMode="numeric" hint={t('5 adds five, -2 removes two.')} required />
        <TextField label={t('Reason')} value={why} onChange={setWhy} maxLength={200} hint={t('Delivery received, damaged, counted again…')} required />
      </ConfirmationDialog>
    </>
  )
}

export function SellerBusinessPage() {
  const { org, canManage, reload } = useSeller()
  const notify = useToast()
  // Owners and admins edit the business; owners, admins and managers edit its stores.
  const canEditBusiness = org.role === 'owner' || org.role === 'admin'
  const details = useAsync(() => consoleApi.business(org.organization_id), [org.organization_id])
  const [form, setForm] = useState<BusinessDetails | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [storeEdit, setStoreEdit] = useState<StoreDetails | null>(null)
  useEffect(() => { if (details.data) setForm(details.data) }, [details.data])
  const set = (k: keyof BusinessDetails) => (v: string) => setForm((f) => (f ? { ...f, [k]: v } : f))
  const changes = (): Partial<Record<keyof BusinessDetails, string>> => {
    if (!form || !details.data) return {}
    const out: Partial<Record<keyof BusinessDetails, string>> = {}
    for (const k of ['displayName', 'legalName', 'description', 'phone', 'email', 'province', 'city', 'address'] as const) if (form[k].trim() !== details.data[k]) out[k] = form[k].trim()
    return out
  }
  const save = async () => {
    setSaving(true)
    setError('')
    try {
      await consoleApi.updateBusiness(org.organization_id, changes())
      notify(t('Business details saved.'), 'success')
      details.reload()
      reload()
    } catch (err) {
      setError(toApiError(err).message)
    } finally {
      setSaving(false)
    }
  }
  const openStore = async (id: string) => {
    try { setStoreEdit(await consoleApi.store(id)) } catch (err) { notify(toApiError(err).message, 'error') }
  }
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
          <Heading level={2}>{t('Details')}</Heading>
          {details.error && <ErrorState error={details.error} onRetry={details.reload} />}
          {form && (
            <form className="tzx-form-grid" onSubmit={(e) => { e.preventDefault(); void save() }}>
              <TextField label={t('Name customers see')} value={form.displayName} onChange={set('displayName')} maxLength={120} required />
              <TextField label={t('Legal name')} value={form.legalName} onChange={set('legalName')} maxLength={200} required />
              <TextField label={t('Phone')} value={form.phone} onChange={set('phone')} type="tel" inputMode="tel" maxLength={32} />
              <TextField label={t('Email address')} value={form.email} onChange={set('email')} type="email" inputMode="email" maxLength={254} />
              <TextField label={t('Province')} value={form.province} onChange={set('province')} maxLength={100} />
              <TextField label={t('City')} value={form.city} onChange={set('city')} maxLength={100} />
              <div className="tzx-wide"><TextField label={t('Address')} value={form.address} onChange={set('address')} maxLength={300} /></div>
              <div className="tzx-wide"><TextField label={t('Description')} value={form.description} onChange={set('description')} multiline maxLength={2000} /></div>
              {error && <p className="tzx-wide inline-notice error" role="alert">{error}</p>}
              {canEditBusiness
                ? <div className="tzx-actions tzx-wide"><Button type="submit" className="primary-button" busy={saving} disabled={Object.keys(changes()).length === 0}>{t('Save details')}</Button></div>
                : <p className="tzx-wide field-hint">{t('Only the owner and admins of this business can change its details.')}</p>}
            </form>
          )}
          <Heading level={2}>{t('Stores')}</Heading>
          <div className="info-rows">{org.stores.length ? org.stores.map((s) => <div key={s.id}><span>{s.name}</span><strong>{sentence(s.status || 'active')} {canManage && <button type="button" className="tz-small-btn" onClick={() => void openStore(s.id)}>{t('Edit')}<span className="sr-only"> {s.name}</span></button>}</strong></div>) : <div><span>{t('No store yet')}</span><strong>{t('Created during onboarding')}</strong></div>}</div>
          <Heading level={2}>{t('Seller profiles')}</Heading>
          <div className="info-rows">{org.seller_profiles.length ? org.seller_profiles.map((p) => <div key={p.id}><span>{p.display_name}{p.mine ? t(' (you)') : ''}</span><strong>{sentence(p.verification_status || p.status)}</strong></div>) : <div><span>{t('No seller profile yet')}</span><strong>{t('Created during onboarding')}</strong></div>}</div>
        </div>
        <aside className="summary-card">
          <Heading level={3}>{t('Need a change?')}</Heading>
          <p>{t('Verification, a new store and a change of owner are handled with the Trezutil team. The team is managed on the Team page.')}</p>
        </aside>
      </div>
      <ConfirmationDialog open={!!storeEdit} title={t('Edit {name}', { name: storeEdit?.name ?? '' })} confirmLabel={t('Save store')}
        onConfirm={async () => {
          if (!storeEdit) return
          await consoleApi.updateStore(storeEdit.id, { name: storeEdit.name.trim(), phone: storeEdit.phone.trim(), email: storeEdit.email.trim(), city: storeEdit.city.trim(), address: storeEdit.address.trim(), description: storeEdit.description.trim() })
          notify(t('Store saved.'), 'success')
          reload()
        }}
        onClose={() => setStoreEdit(null)}>
        {storeEdit && (
          <>
            <TextField label={t('Store name')} value={storeEdit.name} onChange={(v) => setStoreEdit({ ...storeEdit, name: v })} maxLength={120} required />
            <TextField label={t('Phone')} value={storeEdit.phone} onChange={(v) => setStoreEdit({ ...storeEdit, phone: v })} type="tel" maxLength={32} />
            <TextField label={t('Email address')} value={storeEdit.email} onChange={(v) => setStoreEdit({ ...storeEdit, email: v })} type="email" maxLength={254} />
            <TextField label={t('City')} value={storeEdit.city} onChange={(v) => setStoreEdit({ ...storeEdit, city: v })} maxLength={100} />
            <TextField label={t('Address')} value={storeEdit.address} onChange={(v) => setStoreEdit({ ...storeEdit, address: v })} maxLength={300} />
            <TextField label={t('Description')} value={storeEdit.description} onChange={(v) => setStoreEdit({ ...storeEdit, description: v })} multiline maxLength={2000} />
          </>
        )}
      </ConfirmationDialog>
    </>
  )
}
