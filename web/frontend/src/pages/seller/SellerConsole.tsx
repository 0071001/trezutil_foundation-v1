import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { toApiError } from '@/api/client'
import {
  consoleApi, FULFILMENT_NEXT, FULFILMENT_STOP, INVITABLE_ROLES,
  type Customer, type Fulfilment, type Payment, type TeamMember,
} from '@/api/console'
import { searchApi } from '@/api/search'
import { sellerApi } from '@/api/seller'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import {
  count, Fact, isoDate, isoDateTime, ModulePage, Panel, perCurrency, shortId, StatTiles, TextField,
} from '@/components/portal/ConsoleBits'
import { DataTable, type Column } from '@/components/portal/DataTable'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { FilterBar } from '@/components/portal/FilterBar'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { Button, StatusBadge } from '@/components/ui'
import { verticalLabel } from '@/components/dashboard/shared'
import { formatMinor } from '@/lib/format'
import { t, tn, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { useSeller } from '@/pages/seller/SellerShell'

/*
 * The seller workspace modules that read the business's money, customers, deliveries, availability, usage,
 * connections and team, and let the owner change the team and their own details. Every figure is the
 * server's answer for this business; nothing is estimated in the browser.
 */

const PAGE_SIZE = 25

/** Search as the person types, without a request per keystroke. */
function useDebounced(value: string, ms = 300): string {
  const [v, setV] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setV(value.trim()), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return v
}

const roleLabels: Record<string, string> = {
  owner: tx('Owner'), admin: tx('Admin'), manager: tx('Manager'), staff: tx('Staff'), accountant: tx('Accountant'), sales: tx('Sales'), operations: tx('Operations'),
}
const roleLabel = (r: string) => (roleLabels[r] ? t(roleLabels[r]) : r)

// --- Finance -------------------------------------------------------------------------------------------------

const paymentStatuses = [
  ['captured', tx('Captured')], ['authorized', tx('Authorised')], ['pending', tx('Pending')], ['failed', tx('Failed')],
  ['refunded', tx('Refunded')], ['partially_refunded', tx('Partly refunded')], ['cancelled', tx('Cancelled')],
].map(([value, label]) => ({ value, label }))

const methodLabels: Record<string, string> = { CARD: tx('Card'), MOBILE_MONEY: tx('Mobile money'), BANK_TRANSFER: tx('Bank transfer'), CASH: tx('Cash'), WALLET: tx('Wallet'), OTHER: tx('Other') }
export const methodLabel = (m: string) => (methodLabels[m] ? t(methodLabels[m]) : m)

export function paymentColumns(withBusiness: boolean): Column<Payment>[] {
  const cols: Column<Payment>[] = [
    { id: 'payment', header: t('Payment'), primary: true, label: t('Payment'), sort: (p) => p.createdAt, cell: (p) => <><strong>{formatMinor(p.amountMinor, p.currency)}</strong><small>{shortId(p.id)} · {t('Order')} {shortId(p.orderId)}</small></> },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (p) => p.status, cell: (p) => <StatusBadge status={p.status} /> },
    { id: 'method', header: t('Method'), label: t('Method'), sort: (p) => p.method, cell: (p) => `${methodLabel(p.method)}${p.provider ? ` · ${p.provider}` : ''}` },
    { id: 'refunded', header: t('Refunded'), label: t('Refunded'), align: 'end', sort: (p) => p.refundedMinor, cell: (p) => (p.refundedMinor ? formatMinor(p.refundedMinor, p.currency) : '—') },
    { id: 'created', header: t('Date'), label: t('Date'), sort: (p) => p.createdAt, cell: (p) => isoDateTime(p.createdAt) },
  ]
  if (withBusiness) cols.splice(1, 0, { id: 'business', header: t('Business'), label: t('Business'), sort: (p) => p.organizationName, cell: (p) => p.organizationName })
  return cols
}

export function PaymentFacts({ p }: { p: Payment }) {
  return (
    <dl className="tz-facts">
      <Fact label={t('Status')}><StatusBadge status={p.status} /></Fact>
      <Fact label={t('Amount')}>{formatMinor(p.amountMinor, p.currency)}{p.refundedMinor ? ` · ${t('refunded {amount}', { amount: formatMinor(p.refundedMinor, p.currency) })}` : ''}</Fact>
      <Fact label={t('Method')}>{methodLabel(p.method)}{p.provider ? ` · ${p.provider}` : ''}</Fact>
      {p.organizationName && <Fact label={t('Business')}>{p.organizationName}</Fact>}
      <Fact label={t('Order')}><code>{p.orderId}</code></Fact>
      <Fact label={t('Created')}>{isoDateTime(p.createdAt)}</Fact>
      {p.capturedAt && <Fact label={t('Captured')}>{isoDateTime(p.capturedAt)}</Fact>}
      {p.failureReason && <Fact label={t('Why it failed')}>{p.failureReason}</Fact>}
      <Fact label={t('Payment ID')}><code>{p.id}</code></Fact>
    </dl>
  )
}

export function SellerFinancePage() {
  const { org, canManage } = useSeller()
  const [days, setDays] = useState(30)
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<Payment | null>(null)
  const data = useAsync(() => consoleApi.finance(org.organization_id, { days, status, page }), [org.organization_id, days, status, page], canManage)
  const totals = data.data?.totals ?? []
  return (
    <ModulePage eyebrow={t('Money')} title={t('Finance')} copy={t('What you sold and what was paid online, per currency, with every payment.')} allowed={canManage}>
      <div className="tz-toolbar">
        <p className="tz-asof">{t('Over the last {n} days. Currencies are never added together.', { n: days })}</p>
        <PeriodTabs value={days} onChange={(d) => { setDays(d); setPage(1) }} options={[7, 30, 90, 365]} />
      </div>
      {data.loading && !data.data && <Loading label={t('Loading your payments…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (totals.length === 0
        ? <p className="tzx-note">{t('Nothing sold or paid in this period.')}</p>
        : totals.map((m) => (
          <StatTiles key={m.currency} label={t('Totals in {currency}', { currency: m.currency })} items={[
            { label: t('Sold ({currency})', { currency: m.currency }), value: formatMinor(m.earnedMinor, m.currency), note: tn(m.orders, '{n} order', '{n} orders') },
            { label: t('Paid online'), value: formatMinor(m.capturedMinor, m.currency), note: tn(m.payments, '{n} payment', '{n} payments') },
            { label: t('Refunded'), value: formatMinor(m.refundedMinor, m.currency) },
            { label: t('Net received online'), value: formatMinor(m.netMinor, m.currency), tone: 'good' },
            { label: t('Failed payments'), value: count(m.failed), tone: m.failed ? 'attention' : undefined },
          ]} />
        )))}
      <p className="tzx-note">{t('“Sold” counts your paid to completed orders whatever the way they were paid; “paid online” counts card payments taken by Trezutil. Cash and mobile money settled directly with you appear only in “sold”. Payouts are not handled by Trezutil yet.')}</p>
      <FilterBar label={t('Filter payments')} active={!!status} onReset={() => { setStatus(''); setPage(1) }}>
        <select aria-label={t('Status')} value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">{t('Every status')}</option>
          {paymentStatuses.map((s) => <option key={s.value} value={s.value}>{t(s.label)}</option>)}
        </select>
      </FilterBar>
      <DataTable caption={t('Payments')} columns={paymentColumns(false)} rows={data.data?.payments.items ?? []} rowKey={(p) => p.id} onOpen={setOpen}
        loading={data.loading && !data.data} empty={status ? t('No payment with this status.') : t('No payment has been made online to this business yet.')}
        serverPaging={{ page, pageSize: PAGE_SIZE, total: data.data?.payments.total ?? 0, onPage: setPage }} />
      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open ? formatMinor(open.amountMinor, open.currency) : ''} subtitle={open ? isoDateTime(open.createdAt) : undefined}>
        {open && <PaymentFacts p={open} />}
      </DetailDrawer>
    </ModulePage>
  )
}

// --- Customers -----------------------------------------------------------------------------------------------

export function SellerCustomersPage() {
  const { org, canManage } = useSeller()
  const [text, setText] = useState('')
  const q = useDebounced(text)
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<Customer | null>(null)
  useEffect(() => setPage(1), [q])
  const data = useAsync(() => consoleApi.customers(org.organization_id, { q, page }), [org.organization_id, q, page], canManage)
  const columns: Column<Customer>[] = [
    { id: 'name', header: t('Customer'), primary: true, label: t('Customer'), sort: (c) => c.displayName, cell: (c) => <><strong>{c.displayName || t('Customer')}</strong><small>{shortId(c.customerId)}</small></> },
    { id: 'orders', header: t('Orders'), label: t('Orders'), align: 'end', sort: (c) => c.orders, cell: (c) => c.orders },
    { id: 'bookings', header: t('Bookings'), label: t('Bookings'), align: 'end', sort: (c) => c.reservations, cell: (c) => c.reservations },
    { id: 'spent', header: t('Spent'), label: t('Spent'), align: 'end', cell: (c) => perCurrency(c.spent, formatMinor) },
    { id: 'last', header: t('Last activity'), label: t('Last activity'), sort: (c) => c.lastAt, cell: (c) => isoDate(c.lastAt) },
  ]
  return (
    <ModulePage eyebrow={t('Grow')} title={t('Customers')} copy={t('The people who bought or booked from you, and when. Contact details stay with each order.')} allowed={canManage}>
      <FilterBar label={t('Search customers')} active={!!text} onReset={() => setText('')} search={{ value: text, onChange: setText, label: t('Search customers'), placeholder: t('Name') }} />
      <DataTable caption={t('Customers')} columns={columns} rows={data.data?.items ?? []} rowKey={(c) => c.customerId} onOpen={setOpen}
        loading={data.loading && !data.data} error={data.error ? <ErrorState error={data.error} onRetry={data.reload} /> : undefined}
        empty={q ? t('Nobody matches this name.') : t('Customers appear here after their first order or booking.')}
        serverPaging={{ page, pageSize: PAGE_SIZE, total: data.data?.total ?? 0, onPage: setPage }} />
      <p className="tzx-note">{t('A customer’s phone number and address are shared with you in the order or booking that needs them, not in this list.')}</p>
      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open?.displayName || t('Customer')}>
        {open && (
          <dl className="tz-facts">
            <Fact label={t('Orders')}>{open.orders}</Fact>
            <Fact label={t('Bookings')}>{open.reservations}</Fact>
            <Fact label={t('Spent')}>{perCurrency(open.spent, formatMinor)}</Fact>
            <Fact label={t('First activity')}>{isoDateTime(open.firstAt)}</Fact>
            <Fact label={t('Last activity')}>{isoDateTime(open.lastAt)}</Fact>
            <Fact label={t('Customer ID')}><code>{open.customerId}</code></Fact>
          </dl>
        )}
      </DetailDrawer>
    </ModulePage>
  )
}

// --- Fulfilment ----------------------------------------------------------------------------------------------

const fulfilmentTypes: Record<string, string> = {
  DELIVERY: tx('Delivery'), PICKUP: tx('Pickup'), APPOINTMENT: tx('Appointment'), DIGITAL: tx('Digital'), EVENT_ENTRY: tx('Event entry'),
  TRAVEL_TICKET: tx('Travel ticket'), SERVICE_COMPLETION: tx('Service'), PROPERTY_VISIT: tx('Property visit'), NONE: tx('No hand-over'),
}
const stepLabels: Record<string, string> = { ACCEPTED: tx('Accept'), IN_PROGRESS: tx('Start'), READY: tx('Mark ready'), COMPLETED: tx('Mark handed over') }
const fulfilmentFilters = [['open', tx('Open')], ['PENDING', tx('Pending')], ['IN_PROGRESS', tx('In progress')], ['READY', tx('Ready')], ['COMPLETED', tx('Completed')], ['CANCELLED', tx('Cancelled')], ['FAILED', tx('Failed')]]

export function SellerFulfilmentPage() {
  const { org, canManage } = useSeller()
  const notify = useToast()
  const [status, setStatus] = useState('open')
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<Fulfilment | null>(null)
  const [stopping, setStopping] = useState<Fulfilment | null>(null)
  const [tracking, setTracking] = useState('')
  const [busy, setBusy] = useState('')
  const data = useAsync(() => consoleApi.fulfilments(org.organization_id, { status, page }), [org.organization_id, status, page], canManage)
  const act = async (f: Fulfilment, run: () => Promise<unknown>, done: string) => {
    setBusy(f.id)
    try {
      await run()
      notify(done, 'success')
      setOpen(null)
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setBusy('')
      data.reload()
    }
  }
  const columns: Column<Fulfilment>[] = [
    { id: 'what', header: t('What'), primary: true, label: t('What'), sort: (f) => f.title, cell: (f) => <><strong>{f.title || t('Order')}</strong><small>{f.reference} · {t(fulfilmentTypes[f.type] ?? f.type)}</small></> },
    { id: 'customer', header: t('Customer'), label: t('Customer'), sort: (f) => f.customerName, cell: (f) => f.customerName || '—' },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (f) => f.status, cell: (f) => <StatusBadge status={f.status.toLowerCase()} /> },
    { id: 'who', header: t('Handled by'), label: t('Handled by'), cell: (f) => f.assignedName || '—' },
    {
      id: 'next', header: t('Next step'), label: t('Next step'), cell: (f) => FULFILMENT_NEXT[f.status]
        ? <Button className="tz-small-btn" busy={busy === f.id} onClick={() => void act(f, () => consoleApi.fulfilmentStep(org.organization_id, f.id, FULFILMENT_NEXT[f.status]), t('{ref} updated.', { ref: f.reference }))}>{t(stepLabels[FULFILMENT_NEXT[f.status]])}</Button>
        : '—',
    },
  ]
  return (
    <ModulePage eyebrow={t('Operate')} title={t('Fulfilment')} copy={t('Every delivery, pickup and appointment to carry out, from acceptance to hand-over.')} allowed={canManage}>
      <FilterBar label={t('Filter fulfilments')} active={status !== 'open'} onReset={() => { setStatus('open'); setPage(1) }}>
        <select aria-label={t('Status')} value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">{t('Every status')}</option>
          {fulfilmentFilters.map(([v, l]) => <option key={v} value={v}>{t(l)}</option>)}
        </select>
      </FilterBar>
      <DataTable caption={t('Fulfilments')} columns={columns} rows={data.data?.items ?? []} rowKey={(f) => f.id} onOpen={(f) => { setOpen(f); setTracking(f.trackingRef) }}
        loading={data.loading && !data.data} error={data.error ? <ErrorState error={data.error} onRetry={data.reload} /> : undefined}
        empty={status === 'open' ? t('Nothing to hand over. A fulfilment appears when you mark an order ready.') : t('No fulfilment with this status.')}
        serverPaging={{ page, pageSize: PAGE_SIZE, total: data.data?.total ?? 0, onPage: setPage }} />
      <p className="tzx-note">{t('A fulfilment is created when an order is marked ready on the Orders page. Couriers outside your team cannot be assigned yet: “Take it” assigns it to you.')}</p>
      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open?.title || t('Fulfilment')} subtitle={open?.reference}>
        {open && (
          <>
            <dl className="tz-facts">
              <Fact label={t('Status')}><StatusBadge status={open.status.toLowerCase()} /></Fact>
              <Fact label={t('Kind')}>{t(fulfilmentTypes[open.type] ?? open.type)}</Fact>
              <Fact label={t('Customer')}>{open.customerName || '—'}</Fact>
              {open.orderId && <Fact label={t('Order')}><Link to="/seller/orders">{shortId(open.orderId)}</Link> · <StatusBadge status={open.orderStatus} /></Fact>}
              <Fact label={t('Handled by')}>{open.assignedName || t('Nobody yet')}</Fact>
              <Fact label={t('Created')}>{isoDateTime(open.createdAt)}</Fact>
              <Fact label={t('Last change')}>{isoDateTime(open.updatedAt)}</Fact>
            </dl>
            {FULFILMENT_NEXT[open.status] && (
              <div className="tzx-actions">
                {!open.assignedTo && <Button className="tz-small-btn" busy={busy === open.id} onClick={() => void act(open, () => consoleApi.takeFulfilment(org.organization_id, open.id), t('You are handling {ref}.', { ref: open.reference }))}>{t('Take it')}</Button>}
                {FULFILMENT_STOP[open.status] && <Button className="tz-small-btn danger" onClick={() => setStopping(open)}>{FULFILMENT_STOP[open.status] === 'FAILED' ? t('Mark failed') : t('Cancel')}</Button>}
                <Button className="primary-button" busy={busy === open.id} onClick={() => void act(open, () => consoleApi.fulfilmentStep(org.organization_id, open.id, FULFILMENT_NEXT[open.status]), t('{ref} updated.', { ref: open.reference }))}>{t(stepLabels[FULFILMENT_NEXT[open.status]])}</Button>
              </div>
            )}
            <form className="tzx-actions" onSubmit={(e) => { e.preventDefault(); void act(open, () => consoleApi.trackFulfilment(org.organization_id, open.id, tracking.trim()), t('Tracking saved.')) }}>
              <TextField label={t('Tracking reference')} value={tracking} onChange={setTracking} maxLength={200} hint={t('A courier or parcel number the customer can quote.')} />
              <Button type="submit" className="tz-small-btn" disabled={!tracking.trim() || tracking.trim() === open.trackingRef}>{t('Save tracking')}</Button>
            </form>
          </>
        )}
      </DetailDrawer>
      <ConfirmationDialog open={!!stopping} tone="danger"
        title={stopping && FULFILMENT_STOP[stopping.status] === 'FAILED' ? t('Mark {ref} as failed?', { ref: stopping.reference }) : t('Cancel {ref}?', { ref: stopping?.reference ?? '' })}
        confirmLabel={stopping && FULFILMENT_STOP[stopping.status] === 'FAILED' ? t('Mark failed') : t('Cancel fulfilment')}
        onConfirm={async () => { if (stopping) { await consoleApi.fulfilmentStep(org.organization_id, stopping.id, FULFILMENT_STOP[stopping.status]); notify(t('{ref} updated.', { ref: stopping.reference }), 'success'); setOpen(null); data.reload() } }}
        onClose={() => setStopping(null)}>
        <p>{t('This cannot be undone. The order itself is not cancelled or refunded; do that from the order if needed.')}</p>
      </ConfirmationDialog>
    </ModulePage>
  )
}

// --- Availability --------------------------------------------------------------------------------------------

const dayKey = (d: Date) => d.toISOString().slice(0, 10)

export function SellerAvailabilityPage() {
  const { org, canManage } = useSeller()
  const [days, setDays] = useState(14)
  const data = useAsync(() => consoleApi.capacity(org.organization_id, undefined, days), [org.organization_id, days], canManage)
  const columns = useMemo(() => {
    if (!data.data) return []
    const start = new Date(`${data.data.from}T00:00:00Z`)
    return Array.from({ length: data.data.days }, (_, i) => new Date(start.getTime() + i * 86_400_000))
  }, [data.data])
  return (
    <ModulePage eyebrow={t('Operate')} title={t('Availability')} copy={t('What can still be booked, day by day, for every room, table, slot or item you track.')} allowed={canManage}>
      <div className="tz-toolbar">
        <p className="tz-asof">{t('Free places per day (booked / capacity). Days are Kinshasa days.')}</p>
        <PeriodTabs value={days} onChange={setDays} options={[7, 14, 31]} />
      </div>
      {data.loading && !data.data && <Loading label={t('Loading availability…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && data.data.items.length === 0 && <EmptyState icon="calendar" title={t('Nothing tracked yet')} copy={t('Availability is tracked for listings created with a quantity or a capacity, such as hotel rooms.')} />}
      {data.data && data.data.items.length > 0 && (
        <Panel title={t('Calendar')} copy={t('Stock without dates is shown as one figure: available now.')} actions={<Link className="tz-small-btn" to="/seller/inventory">{t('Adjust stock')}</Link>}>
          <div className="tzx-cal-wrap" tabIndex={0}>
            <table className="tzx-cal">
              <caption className="sr-only">{t('Availability per day')}</caption>
              <thead><tr><th scope="col">{t('Listing')}</th><th scope="col">{t('Now')}</th>{columns.map((d) => <th key={dayKey(d)} scope="col">{d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', timeZone: 'UTC' })}</th>)}</tr></thead>
              <tbody>
                {data.data.items.map((r) => {
                  const available = Math.max(0, r.total - r.reserved - r.consumed)
                  return (
                    <tr key={r.inventoryId}>
                      <th scope="row" title={r.listingTitle}>{r.listingTitle || shortId(r.listingId)}</th>
                      <td className={available === 0 ? 'full' : 'free'}>{available}</td>
                      {columns.map((d) => {
                        const k = dayKey(d)
                        const w = r.windows.filter((x) => x.startAt.slice(0, 10) <= k && x.endAt.slice(0, 10) > k)
                        if (w.length === 0) return <td key={k} className="none">·</td>
                        const cap = w.reduce((s, x) => s + x.capacity, 0)
                        const res = w.reduce((s, x) => s + x.reserved, 0)
                        return <td key={k} className={res >= cap ? 'full' : 'free'} title={t('{reserved} booked of {capacity}', { reserved: res, capacity: cap })}>{cap - res}</td>
                      })}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
      <p className="tzx-note">{t('Hotel nightly capacity is set on the Hotel rooms page. Opening hours and slot calendars for restaurants and services are not available yet.')}</p>
    </ModulePage>
  )
}

// --- Usage and credits ---------------------------------------------------------------------------------------

export function SellerUsagePage() {
  const { org, canManage } = useSeller()
  const data = useAsync(() => consoleApi.usage(org.organization_id), [org.organization_id], canManage)
  const u = data.data
  const current = u?.accounts[0]
  return (
    <ModulePage eyebrow={t('Money')} title={t('Usage and credits')} copy={t('Your plan, your credit balance and what used it.')} allowed={canManage}>
      {data.loading && !u && <Loading label={t('Loading your usage…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {u && (
        <>
          <StatTiles label={t('Credits')} items={[
            { label: t('Plan'), value: u.subscription ? u.subscription.plan.name : t('No plan'), note: u.subscription ? t('Status: {status}', { status: u.subscription.status }) : t('Free use, nothing is metered') },
            { label: t('Balance'), value: current ? count(current.balance) : '—', note: current ? t('Period {period}', { period: current.periodId }) : t('No credit account yet'), tone: current && current.balance <= 0 ? 'attention' : undefined },
            { label: t('Used this period'), value: current ? count(current.consumed) : '0' },
            { label: t('Granted this period'), value: current ? count(current.granted) : '0' },
          ]} />
          {u.policies.filter((p) => p.active).length === 0 && <p className="tzx-note">{t('No action is metered on the platform today: using the assistant and the console costs no credit.')}</p>}
          <div className="tzx-grid2">
            <Panel title={t('Recent usage')}>
              {u.ledger.length === 0 ? <p className="tzx-note">{t('Nothing has used credits yet.')}</p> : (
                <ul className="tzx-list">
                  {u.ledger.map((e) => <li key={e.id}><span><strong>{e.actionType || e.kind}</strong><small>{isoDateTime(e.createdAt)}</small></span><span>{e.amount > 0 && e.kind !== 'consume' ? '+' : ''}{count(e.amount)} · {t('balance {n}', { n: count(e.balanceAfter) })}</span></li>)}
                </ul>
              )}
            </Panel>
            <Panel title={t('What costs credits')}>
              {u.policies.length === 0 ? <p className="tzx-note">{t('No pricing rule is set.')}</p> : (
                <ul className="tzx-list">{u.policies.map((p) => <li key={`${p.actionType}|${p.channel}|${p.actorType}`}><span><strong>{p.actionType}</strong><small>{p.channel} · {p.actorType}</small></span><span>{p.active ? tn(p.cost, '{n} credit', '{n} credits') : t('Off')}</span></li>)}</ul>
              )}
            </Panel>
          </div>
          <p className="tzx-note">{t('Plans and credit packs are changed with the Trezutil team during this phase.')}</p>
        </>
      )}
    </ModulePage>
  )
}

// --- Integrations --------------------------------------------------------------------------------------------

export function SellerIntegrationsPage() {
  const { org, canManage } = useSeller()
  const data = useAsync(() => consoleApi.integrations(org.organization_id), [org.organization_id], canManage)
  return (
    <ModulePage eyebrow={t('Business')} title={t('Integrations')} copy={t('The payment and messaging connections of your business and how they behave. Keys are never shown.')} allowed={canManage}>
      {data.loading && !data.data && <Loading label={t('Loading your connections…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <div className="tzx-grid2">
          <Panel title={t('Payment providers')} copy={t('Calls over the last 7 days.')}>
            {data.data.providers.length === 0 ? <p className="tzx-note">{t('No payment provider is connected: customers pay you directly (cash or mobile money). Ask the Trezutil team to connect card payments.')}</p> : (
              <ul className="tzx-list">
                {data.data.providers.map((p) => (
                  <li key={p.id}>
                    <span><strong>{p.name}</strong><small>{p.capabilities.join(', ').toLowerCase()} · {tn(p.attempts, '{n} call', '{n} calls')}{p.failures ? ` · ${tn(p.failures, '{n} failure', '{n} failures')}` : ''}{p.lastError ? ` · ${p.lastError}` : ''}</small></span>
                    <span><StatusBadge status={p.status.toLowerCase()} /> <StatusBadge status={p.healthStatus.toLowerCase()} /></span>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
          <Panel title={t('Messaging channels')}>
            {data.data.channels.length === 0 ? <p className="tzx-note">{t('No WhatsApp number is linked to this business. Customers reach you through the Trezutil assistant.')}</p> : (
              <ul className="tzx-list">{data.data.channels.map((c) => <li key={c.id}><span><strong>{c.channel === 'whatsapp' ? 'WhatsApp' : c.channel}</strong><small>{c.externalId}</small></span><StatusBadge status={c.status} /></li>)}</ul>
            )}
          </Panel>
        </div>
      )}
      <p className="tzx-note">{t('Connections are set up with the Trezutil team, who hold the keys; this page shows their state.')}</p>
    </ModulePage>
  )
}

// --- Team ----------------------------------------------------------------------------------------------------

export function SellerTeamPage() {
  const { org } = useSeller()
  const notify = useToast()
  const data = useAsync(() => consoleApi.team(org.organization_id), [org.organization_id])
  const [inviting, setInviting] = useState(false)
  const [phone, setPhone] = useState('')
  const [role, setRole] = useState('staff')
  const [changing, setChanging] = useState<{ m: TeamMember; role: string } | null>(null)
  const [removing, setRemoving] = useState<TeamMember | null>(null)
  const [revoking, setRevoking] = useState<{ id: string; phone: string } | null>(null)
  const team = data.data
  const owner = team?.yourRole === 'owner'
  const offered = INVITABLE_ROLES.filter((r) => owner || r !== 'admin')
  const manageable = (m: TeamMember) => !!team?.canManage && !m.you && m.role !== 'owner' && (owner || m.role !== 'admin')
  return (
    <ModulePage eyebrow={t('Business')} title={t('Team')} copy={t('Who works in your business, and with which role.')}>
      {data.loading && !team && <Loading label={t('Loading your team…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {team && (
        <>
          <Panel title={t('Members')} copy={tn(team.members.length, '{n} person', '{n} people')}
            actions={team.canManage ? <Button className="tz-small-btn" onClick={() => { setPhone(''); setRole('staff'); setInviting(true) }}>{t('Invite someone')}</Button> : undefined}>
            <ul className="tzx-list">
              {team.members.map((m) => (
                <li key={m.userId}>
                  <span><strong>{m.displayName || t('Member')}{m.you ? t(' (you)') : ''}</strong><small>{m.phone} · {t('since {date}', { date: isoDate(m.joinedAt) })}</small></span>
                  <span className="tzx-panel-actions">
                    {manageable(m) ? (
                      <>
                        <select className="tzx-select" aria-label={t('Role of {name}', { name: m.displayName })} value={m.role} onChange={(e) => setChanging({ m, role: e.target.value })}>
                          {[...new Set([m.role, ...offered])].map((r) => <option key={r} value={r}>{roleLabel(r)}</option>)}
                        </select>
                        <Button className="tz-small-btn danger" onClick={() => setRemoving(m)}>{t('Remove')}<span className="sr-only"> {m.displayName}</span></Button>
                      </>
                    ) : <b className="status-badge">{roleLabel(m.role)}</b>}
                  </span>
                </li>
              ))}
            </ul>
          </Panel>
          {team.canManage && (
            <Panel title={t('Pending invitations')} copy={t('An invitation becomes a membership when that number next signs in with a code. It expires after 14 days.')}>
              {team.invitations.length === 0 ? <p className="tzx-note">{t('No invitation is waiting.')}</p> : (
                <ul className="tzx-list">
                  {team.invitations.map((i) => (
                    <li key={i.id}>
                      <span><strong>{i.phone}</strong><small>{roleLabel(i.role)} · {t('invited by {name}', { name: i.invitedBy || '—' })} · {t('expires {date}', { date: isoDate(i.expiresAt) })}</small></span>
                      <Button className="tz-small-btn" onClick={() => setRevoking({ id: i.id, phone: i.phone })}>{t('Withdraw')}</Button>
                    </li>
                  ))}
                </ul>
              )}
            </Panel>
          )}
          {!team.canManage && <p className="tzx-note">{t('Only the owner and admins of this business can invite or change members.')}</p>}
          <p className="tzx-note">{t('Today every member below manager can view the workspace but not change it; the accountant, sales and operations roles do not yet open finance, customers or operations on their own.')}</p>
        </>
      )}
      <ConfirmationDialog open={inviting} title={t('Invite someone')} confirmLabel={t('Send invitation')}
        onConfirm={async () => {
          if (phone.replace(/\D/g, '').length < 8) throw new Error(t('Enter a phone number with its country code.'))
          await consoleApi.invite(org.organization_id, phone.trim(), role)
          notify(t('Invitation recorded. It applies when {phone} next signs in.', { phone: phone.trim() }), 'success')
          data.reload()
        }}
        onClose={() => setInviting(false)}>
        <p>{t('The person joins your business the next time they sign in to Trezutil with this number. We don’t tell you whether the number already has an account.')}</p>
        <TextField label={t('Phone number')} value={phone} onChange={setPhone} type="tel" inputMode="tel" autoComplete="off" hint={t('With the country code: +243 81 234 5678')} required />
        <label className="tz-dialog-field tzx-field"><span>{t('Role')}</span>
          <select value={role} onChange={(e) => setRole(e.target.value)}>{offered.map((r) => <option key={r} value={r}>{roleLabel(r)}</option>)}</select>
        </label>
      </ConfirmationDialog>
      <ConfirmationDialog open={!!changing} title={t('Make {name} {role}?', { name: changing?.m.displayName ?? '', role: changing ? roleLabel(changing.role) : '' })} confirmLabel={t('Change role')}
        onConfirm={async () => { if (changing) { await consoleApi.changeRole(org.organization_id, changing.m.userId, changing.role); notify(t('Role changed.'), 'success'); data.reload() } }}
        onClose={() => setChanging(null)}>
        <p>{t('Owners, admins and managers can change listings, orders and bookings; other roles can only view.')}</p>
      </ConfirmationDialog>
      <ConfirmationDialog open={!!removing} tone="danger" title={t('Remove {name} from the team?', { name: removing?.displayName ?? '' })} confirmLabel={t('Remove')}
        onConfirm={async () => { if (removing) { await consoleApi.removeMember(org.organization_id, removing.userId); notify(t('{name} no longer has access.', { name: removing.displayName }), 'success'); data.reload() } }}
        onClose={() => setRemoving(null)}>
        <p>{t('They lose access to this business at once. Their account and what they did stay.')}</p>
      </ConfirmationDialog>
      <ConfirmationDialog open={!!revoking} title={t('Withdraw the invitation for {phone}?', { phone: revoking?.phone ?? '' })} confirmLabel={t('Withdraw')}
        onConfirm={async () => { if (revoking) { await consoleApi.revokeInvitation(org.organization_id, revoking.id); notify(t('Invitation withdrawn.'), 'success'); data.reload() } }}
        onClose={() => setRevoking(null)}>
        <p>{t('The number will not join your business when it signs in.')}</p>
      </ConfirmationDialog>
    </ModulePage>
  )
}

// --- Settings (the signed-in person) -------------------------------------------------------------------------

export function SellerSettingsPage() {
  const { user, refreshUser } = useSession()
  const notify = useToast()
  const [name, setName] = useState(user?.display_name ?? '')
  const [email, setEmail] = useState(user?.email ?? '')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  useEffect(() => { setName(user?.display_name ?? ''); setEmail(user?.email ?? '') }, [user])
  const changed = name.trim() !== (user?.display_name ?? '') || email.trim() !== (user?.email ?? '')
  const save = async () => {
    setSaving(true)
    setError('')
    try {
      await consoleApi.updateMe({ display_name: name.trim(), email: email.trim() })
      await refreshUser()
      notify(t('Your details are saved.'), 'success')
    } catch (err) {
      setError(toApiError(err).message)
    } finally {
      setSaving(false)
    }
  }
  return (
    <ModulePage eyebrow={t('Business')} title={t('Settings')} copy={t('Your name and email on Trezutil, and how you sign in.')}>
      <Panel title={t('Your details')} copy={t('Your name is what your team and customers see.')}>
        <form className="tzx-form-grid" onSubmit={(e) => { e.preventDefault(); void save() }}>
          <TextField label={t('Name')} value={name} onChange={setName} maxLength={80} required autoComplete="name" />
          <TextField label={t('Email address')} value={email} onChange={setEmail} type="email" inputMode="email" maxLength={254} autoComplete="email" hint={t('Optional. Leave empty to remove it.')} />
          {error && <p className="tzx-wide inline-notice error" role="alert">{error}</p>}
          <div className="tzx-actions tzx-wide"><Button type="submit" className="primary-button" busy={saving} disabled={!changed || name.trim().length < 2}>{t('Save')}</Button></div>
        </form>
      </Panel>
      <Panel title={t('Sign-in')}>
        <dl className="tz-facts">
          <Fact label={t('Phone number')}>{user?.phone || '—'}</Fact>
          <Fact label={t('How you sign in')}>{t('A one-time code sent to this number on WhatsApp.')}</Fact>
        </dl>
        <p className="tzx-note">{t('Changing your phone number needs a new verification and is done with the Trezutil team. Notification preferences and the list of open sessions are not available yet.')}</p>
      </Panel>
      <p className="tzx-note">{t('Your business details are edited on the Business profile page.')} <Link to="/seller/business">{t('Open business profile')}</Link></p>
    </ModulePage>
  )
}

// --- Verticals -----------------------------------------------------------------------------------------------

export function SellerVerticalsPage() {
  const { org } = useSeller()
  const data = useAsync(async () => {
    const [verticals, listings] = await Promise.all([searchApi.verticals(), sellerApi.listings(org.organization_id)])
    const used: Record<string, { live: number; all: number }> = {}
    for (const l of listings) {
      const v = l.vertical || 'OTHER'
      used[v] = used[v] ?? { live: 0, all: 0 }
      used[v].all++
      if (l.status === 'published') used[v].live++
    }
    return { verticals, used }
  }, [org.organization_id])
  return (
    <ModulePage eyebrow={t('Business')} title={t('Verticals')} copy={t('The kinds of business you run on Trezutil and what each one lets customers do.')}>
      {data.loading && !data.data && <Loading label={t('Loading verticals…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <>
          <Panel title={t('Yours')} copy={t('A vertical is active for you as soon as one of your listings uses it.')}>
            {Object.keys(data.data.used).length === 0 ? <p className="tzx-note">{t('You have no listing yet.')}</p> : (
              <ul className="tzx-list">
                {Object.entries(data.data.used).map(([code, n]) => (
                  <li key={code}><span><strong>{verticalLabel(code)}</strong><small>{tn(n.all, '{n} listing', '{n} listings')}</small></span><span>{tn(n.live, '{n} live', '{n} live')}</span></li>
                ))}
              </ul>
            )}
          </Panel>
          <Panel title={t('Every vertical')}>
            <ul className="tzx-list">
              {data.data.verticals.map((v) => (
                <li key={v.code}>
                  <span><strong>{v.name || verticalLabel(v.code)}</strong><small>{v.description}</small></span>
                  <ul className="tzx-chips">{v.flows.map((f) => <li key={f}>{f === 'BOOK' ? t('Booking') : t('Purchase')}</li>)}{data.data!.used[v.code] ? <li>{t('In use')}</li> : null}</ul>
                </li>
              ))}
            </ul>
          </Panel>
        </>
      )}
    </ModulePage>
  )
}
