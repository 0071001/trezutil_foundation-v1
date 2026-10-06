import { useEffect, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { adminApi, canSeeOverview, type AdminOrder, type AdminOrganization, type AdminPage, type AdminReservation } from '@/api/admin'
import { DataTable, type Column } from '@/components/portal/DataTable'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { FilterBar } from '@/components/portal/FilterBar'
import { ModuleHead } from '@/components/portal/ModuleState'
import { EmptyState, ErrorState } from '@/components/states'
import { StatusBadge } from '@/components/ui'
import { formatMinor } from '@/lib/format'
import { locale, t, tn, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'

/*
 * Orders, bookings and businesses across the whole platform: the lists an operator needs to find one and to see
 * where it stands. They are read-only (nothing here changes an order or a business) and show names and states,
 * never a phone number or an email address. Every list is paged by the server and searchable.
 */

const PAGE_SIZE = 25

const date = (iso: string) => (iso ? new Date(iso).toLocaleDateString(locale(), { dateStyle: 'medium' }) : '—')
const dateTime = (iso: string) => (iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) : '—')
const short = (id: string) => `#${id.slice(0, 8)}`

interface ListProps<T> {
  title: string
  copy: string
  /** What the search box accepts: "Order number, business or customer". */
  searchLabel: string
  searchHint: string
  statuses: { value: string; label: string }[]
  load: (params: { q: string; status: string; page: number }) => Promise<AdminPage<T>>
  columns: Column<T>[]
  rowKey: (row: T) => string
  /** The record beside the list, with what the columns leave out. */
  detail: (row: T) => { title: string; subtitle?: string; body: ReactNode }
  emptyAll: string
}

function CommerceList<T>({ title, copy, searchLabel, searchHint, statuses, load, columns, rowKey, detail, emptyAll }: ListProps<T>) {
  const { user } = useSession()
  const allowed = canSeeOverview(user)
  const [text, setText] = useState('')
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<T | null>(null)

  // search as the person types, without a request per keystroke
  useEffect(() => {
    const timer = setTimeout(() => { setQ(text.trim()); setPage(1) }, 300)
    return () => clearTimeout(timer)
  }, [text])
  useEffect(() => { document.title = `${t(title)} — Trezutil` }, [title])

  const rows = useAsync(() => load({ q, status, page }), [q, status, page], allowed)
  const active = !!(text || status)
  const opened = open ? detail(open) : null

  return (
    <div className="tz-module">
      <ModuleHead eyebrow={t('Administration')} title={t(title)} copy={t(copy)} />
      {!allowed ? (
        <EmptyState icon="user" title={t('You don’t have access')} copy={t('This list is open to administrators and operators.')} />
      ) : (
        <>
          <FilterBar label={t(searchLabel)} active={active} onReset={() => { setText(''); setQ(''); setStatus(''); setPage(1) }}
            search={{ value: text, onChange: setText, label: t(searchLabel), placeholder: t(searchHint) }}>
            <select aria-label={t('Status')} value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
              <option value="">{t('Every status')}</option>
              {statuses.map((s) => <option key={s.value} value={s.value}>{t(s.label)}</option>)}
            </select>
          </FilterBar>
          <DataTable
            caption={t(title)}
            columns={columns}
            rows={rows.data?.items ?? []}
            rowKey={rowKey}
            onOpen={setOpen}
            loading={rows.loading && !rows.data}
            error={rows.error ? <ErrorState error={rows.error} onRetry={rows.reload} /> : undefined}
            empty={active ? t('Nothing matches. Try another word or status.') : t(emptyAll)}
            serverPaging={{ page, pageSize: PAGE_SIZE, total: rows.data?.total ?? 0, onPage: setPage }}
          />
        </>
      )}
      <DetailDrawer open={!!opened} onClose={() => setOpen(null)} title={opened?.title ?? ''} subtitle={opened?.subtitle}>
        {opened?.body}
      </DetailDrawer>
    </div>
  )
}

/** One fact in a drawer. */
const Fact = ({ label, children }: { label: string; children: ReactNode }) => <div><dt>{label}</dt><dd>{children}</dd></div>
/** The same record's history in the audit trail. */
const History = ({ id }: { id: string }) => <Fact label={t('History')}><Link to={`/admin/audit?target_id=${encodeURIComponent(id)}`}>{t('See what was done to it')}</Link></Fact>

const orderStatuses = [
  ['pending_payment', tx('Awaiting payment')], ['paid', tx('Paid')], ['confirmed', tx('Confirmed')], ['processing', tx('Processing')],
  ['ready_for_fulfillment', tx('Ready')], ['fulfilled', tx('Delivered')], ['completed', tx('Completed')],
  ['cancelled', tx('Cancelled')], ['refunded', tx('Refunded')], ['disputed', tx('Disputed')], ['expired', tx('Expired')],
].map(([value, label]) => ({ value, label }))

export function AdminOrdersPage() {
  const columns: Column<AdminOrder>[] = [
    { id: 'order', header: t('Order'), primary: true, label: t('Order'), sort: (o) => o.title || o.id, cell: (o) => <><strong>{o.title || t('Order')}</strong><small>{short(o.id)}{o.itemCount > 1 ? ` · ${t('{n} items', { n: o.itemCount })}` : ''}</small></> },
    { id: 'business', header: t('Business'), label: t('Business'), sort: (o) => o.organizationName, cell: (o) => o.organizationName },
    { id: 'customer', header: t('Customer'), label: t('Customer'), sort: (o) => o.customerName, cell: (o) => o.customerName || '—' },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (o) => o.status, cell: (o) => <StatusBadge status={o.status} /> },
    { id: 'total', header: t('Total'), label: t('Total'), align: 'end', sort: (o) => o.totalMinor, cell: (o) => formatMinor(o.totalMinor, o.currency) },
    { id: 'created', header: t('Placed'), label: t('Placed'), sort: (o) => o.createdAt, cell: (o) => dateTime(o.createdAt) },
  ]
  return (
    <CommerceList<AdminOrder>
      title={tx('Orders')} copy={tx('Every order on the platform, by business and customer. Read-only.')}
      searchLabel={tx('Search orders')} searchHint={tx('Order number, business or customer')}
      statuses={orderStatuses} columns={columns} rowKey={(o) => o.id}
      emptyAll={tx('No order has been placed yet.')}
      load={({ q, status, page }) => adminApi.orders({ q, status, page, pageSize: PAGE_SIZE })}
      detail={(o) => ({
        title: o.title || t('Order'), subtitle: `${short(o.id)} · ${dateTime(o.createdAt)}`,
        body: (
          <dl className="tz-facts">
            <Fact label={t('Status')}><StatusBadge status={o.status} /></Fact>
            <Fact label={t('Total')}>{formatMinor(o.totalMinor, o.currency)}{o.itemCount > 1 ? ` · ${tn(o.itemCount, '{n} item', '{n} items')}` : ''}</Fact>
            <Fact label={t('Business')}>{o.organizationName} <small>{o.organizationId}</small></Fact>
            <Fact label={t('Customer')}>{o.customerName || '—'} <small>{o.customerId}</small></Fact>
            <Fact label={t('Last change')}>{dateTime(o.updatedAt)}</Fact>
            <Fact label={t('Order ID')}><code>{o.id}</code></Fact>
            <History id={o.id} />
          </dl>
        ),
      })}
    />
  )
}

const bookingStatuses = [['pending', tx('Pending')], ['confirmed', tx('Confirmed')], ['completed', tx('Completed')], ['cancelled', tx('Cancelled')]].map(([value, label]) => ({ value, label }))

export function AdminReservationsPage() {
  const columns: Column<AdminReservation>[] = [
    { id: 'booking', header: t('Booking'), primary: true, label: t('Booking'), sort: (r) => r.title, cell: (r) => <><strong>{r.title || t('Reservation')}</strong><small>{short(r.id)}</small></> },
    { id: 'business', header: t('Business'), label: t('Business'), sort: (r) => r.organizationName, cell: (r) => r.organizationName },
    { id: 'customer', header: t('Customer'), label: t('Customer'), sort: (r) => r.customerName, cell: (r) => r.customerName || '—' },
    { id: 'dates', header: t('Dates'), label: t('Dates'), sort: (r) => r.startAt, cell: (r) => `${date(r.startAt)} → ${date(r.endAt)}` },
    { id: 'guests', header: t('Guests'), label: t('Guests'), align: 'end', sort: (r) => r.partySize, cell: (r) => r.partySize },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (r) => r.status, cell: (r) => <StatusBadge status={r.status} /> },
  ]
  return (
    <CommerceList<AdminReservation>
      title={tx('Reservations')} copy={tx('Every booking on the platform, by business and customer. Read-only.')}
      searchLabel={tx('Search bookings')} searchHint={tx('Booking number, listing, business or customer')}
      statuses={bookingStatuses} columns={columns} rowKey={(r) => r.id}
      emptyAll={tx('No booking has been made yet.')}
      load={({ q, status, page }) => adminApi.reservations({ q, status, page, pageSize: PAGE_SIZE })}
      detail={(r) => ({
        title: r.title || t('Reservation'), subtitle: `${short(r.id)} · ${dateTime(r.createdAt)}`,
        body: (
          <dl className="tz-facts">
            <Fact label={t('Status')}><StatusBadge status={r.status} /></Fact>
            <Fact label={t('Dates')}>{dateTime(r.startAt)} → {dateTime(r.endAt)}</Fact>
            <Fact label={t('Guests')}>{r.partySize} · {t('Quantity')} {r.quantity}</Fact>
            <Fact label={t('Business')}>{r.organizationName} <small>{r.organizationId}</small></Fact>
            <Fact label={t('Customer')}>{r.customerName || '—'} <small>{r.customerId}</small></Fact>
            <Fact label={t('Booking ID')}><code>{r.id}</code></Fact>
            <History id={r.id} />
          </dl>
        ),
      })}
    />
  )
}

const orgStatuses = [['active', tx('Active')], ['suspended', tx('Suspended')], ['closed', tx('Closed')]].map(([value, label]) => ({ value, label }))

export function AdminOrganizationsPage() {
  const place = (o: AdminOrganization) => [o.city, o.country].filter(Boolean).join(', ')
  const columns: Column<AdminOrganization>[] = [
    { id: 'business', header: t('Business'), primary: true, label: t('Business'), sort: (o) => o.displayName, cell: (o) => <><strong>{o.displayName}</strong><small>{place(o) || short(o.id)}</small></> },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (o) => o.status, cell: (o) => <StatusBadge status={o.status} /> },
    { id: 'verification', header: t('Verification'), label: t('Verification'), sort: (o) => o.verificationStatus, cell: (o) => <StatusBadge status={o.verificationStatus} /> },
    { id: 'listings', header: t('Live listings'), label: t('Live listings'), align: 'end', sort: (o) => o.publishedListings, cell: (o) => o.publishedListings },
    { id: 'orders', header: t('Orders'), label: t('Orders'), align: 'end', sort: (o) => o.orders, cell: (o) => o.orders },
    { id: 'team', header: t('Team'), label: t('Team'), align: 'end', sort: (o) => o.members, cell: (o) => o.members },
    { id: 'created', header: t('Joined'), label: t('Joined'), sort: (o) => o.createdAt, cell: (o) => date(o.createdAt) },
  ]
  return (
    <CommerceList<AdminOrganization>
      title={tx('Organizations and sellers')} copy={tx('Every business on the platform: status, verification, team and activity. Read-only; businesses are onboarded with the team.')}
      searchLabel={tx('Search businesses')} searchHint={tx('Name or city')}
      statuses={orgStatuses} columns={columns} rowKey={(o) => o.id}
      emptyAll={tx('No business has joined yet.')}
      load={({ q, status, page }) => adminApi.organizations({ q, status, page, pageSize: PAGE_SIZE })}
      detail={(o) => ({
        title: o.displayName, subtitle: place(o) || undefined,
        body: (
          <dl className="tz-facts">
            <Fact label={t('Status')}><StatusBadge status={o.status} /></Fact>
            <Fact label={t('Verification')}><StatusBadge status={o.verificationStatus} /></Fact>
            <Fact label={t('Type')}>{o.type}</Fact>
            <Fact label={t('Activity')}>{tn(o.publishedListings, '{n} live listing', '{n} live listings')} · {tn(o.orders, '{n} order', '{n} orders')}</Fact>
            <Fact label={t('Team')}>{tn(o.members, '{n} member', '{n} members')} · {tn(o.stores, '{n} store', '{n} stores')}</Fact>
            <Fact label={t('Joined')}>{date(o.createdAt)}</Fact>
            <Fact label={t('Business ID')}><code>{o.id}</code></Fact>
            <History id={o.id} />
          </dl>
        ),
      })}
    />
  )
}
