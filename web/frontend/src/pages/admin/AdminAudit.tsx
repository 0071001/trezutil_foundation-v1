import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { adminApi, canSeeOverview, type AuditEvent, type AuditFilters } from '@/api/admin'
import { downloadText, toCsv } from '@/components/dashboard/csv'
import { DataTable, type Column } from '@/components/portal/DataTable'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { FilterBar } from '@/components/portal/FilterBar'
import { ModuleHead } from '@/components/portal/ModuleState'
import { EmptyState, ErrorState } from '@/components/states'
import { Icon, statusLabel } from '@/components/ui'
import { locale, t, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'

/*
 * The audit trail: who did what, to which resource, and when. It reads the durable table the services write to;
 * the console can filter and export it and can never change an entry (the database refuses it too). A
 * state-changing action in the console or the seller workspace appears here within a moment of being taken.
 */

const PAGE_SIZE = 50

/** What the "What" filter offers: the start of an action name. */
const kinds: { value: string; label: string }[] = [
  { value: '', label: tx('All activity') },
  { value: 'order.', label: tx('Orders') },
  { value: 'reservation.', label: tx('Reservations') },
  { value: 'listing.', label: tx('Listings') },
  { value: 'inventory.', label: tx('Inventory') },
  { value: 'fulfillment.', label: tx('Fulfilment') },
  { value: 'user.', label: tx('Accounts') },
  { value: 'organization.', label: tx('Organizations') },
]

const actionLabels: Record<string, string> = {
  'order.created': tx('Order placed'), 'order.confirmed': tx('Order confirmed'), 'order.processing': tx('Order being prepared'),
  'order.ready_for_fulfillment': tx('Order ready'), 'order.fulfilled': tx('Order delivered'), 'order.completed': tx('Order completed'),
  'order.cancelled': tx('Order cancelled'),
  'reservation.created': tx('Booking made'), 'reservation.cancelled': tx('Booking cancelled'),
  'listing.created': tx('Listing created'), 'listing.updated': tx('Listing edited'), 'listing.published': tx('Listing published'),
  'listing.paused': tx('Listing paused'), 'listing.resumed': tx('Listing resumed'),
  'listing.promotion_started': tx('Sale started'), 'listing.promotion_ended': tx('Sale ended'),
  'inventory.adjusted': tx('Stock adjusted'), 'capacity.reserved': tx('Capacity reserved'), 'capacity.released': tx('Capacity released'),
  'fulfillment.assigned': tx('Delivery assigned'), 'fulfillment.transitioned': tx('Delivery step'), 'fulfillment.tracking_updated': tx('Tracking updated'),
  'user.suspended': tx('Account suspended'), 'user.reactivated': tx('Account reactivated'),
  'organization.created': tx('Business created'), 'organization.updated': tx('Business edited'),
  'organization.member_added': tx('Team member added'), 'organization.member_removed': tx('Team member removed'),
  'seller.created': tx('Seller profile created'), 'seller.updated': tx('Seller profile edited'),
  'store.created': tx('Store created'), 'store.updated': tx('Store edited'),
  'role.assigned': tx('Role granted'), 'role.revoked': tx('Role removed'),
}

/** What kind of record an entry is about, in words. */
const targetLabels: Record<string, string> = {
  order: tx('Order'), reservation: tx('Reservation'), listing: tx('Listing'), user: tx('Account'), organization: tx('Business'),
  store: tx('Store'), seller: tx('Seller profile'), payment: tx('Payment'), fulfillment: tx('Delivery'), inventory: tx('Stock'),
}
const targetLabel = (type: string): string => (targetLabels[type] ? t(targetLabels[type]) : type)

const actionLabel = (action: string): string => (actionLabels[action] ? t(actionLabels[action]) : action)
const when = (iso: string, seconds = false): string => (iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: seconds ? 'medium' : 'short' }) : '—')
const shortId = (id: string): string => (id.length > 12 ? `${id.slice(0, 8)}…` : id)
const who = (e: AuditEvent): string => e.actorName || (e.actorId ? shortId(e.actorId) : t('The platform'))

/** One line of what changed: "Confirmed → Ready", a reason, or the first few facts. */
function summary(e: AuditEvent): string {
  const m = e.metadata
  const parts: string[] = []
  if (typeof m.previous_status === 'string' && typeof m.new_status === 'string') parts.push(`${statusLabel(m.previous_status)} → ${statusLabel(m.new_status)}`)
  if (typeof m.reason === 'string' && m.reason) parts.push(`“${m.reason}”`)
  for (const [k, v] of Object.entries(m)) {
    if (parts.length >= 3) break
    if (['previous_status', 'new_status', 'reason'].includes(k)) continue
    if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') parts.push(`${k.replace(/_/g, ' ')}: ${String(v)}`)
  }
  return parts.join(' · ')
}

const csv = (events: AuditEvent[]): string => toCsv(
  [t('When'), t('Who'), t('Actor ID'), t('Action'), t('Target type'), t('Target ID'), t('Request ID'), t('Details')],
  events.map((e) => [e.at, e.actorName, e.actorId, e.action, e.targetType, e.targetId, e.requestId, JSON.stringify(e.metadata)]),
)

export default function AdminAuditPage() {
  const { user } = useSession()
  const allowed = canSeeOverview(user)
  const [kind, setKind] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  // a record's page links here with ?target_id=: its history, filtered
  const [params] = useSearchParams()
  const initialTarget = (params.get('target_id') ?? '').slice(0, 64)
  const [target, setTarget] = useState(initialTarget)
  const [targetId, setTargetId] = useState(initialTarget)
  const [actor, setActor] = useState<{ id: string; name: string } | null>(null)
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<AuditEvent | null>(null)

  // search by target id as the person types, without a request per keystroke
  useEffect(() => {
    const timer = setTimeout(() => { setTargetId(target.trim()); setPage(1) }, 300)
    return () => clearTimeout(timer)
  }, [target])
  useEffect(() => { document.title = `${t('Audit')} — Trezutil` }, [])

  const filters: AuditFilters = { action: kind, actorId: actor?.id, targetId, from, to }
  const trail = useAsync(() => adminApi.audit(filters, page, PAGE_SIZE), [kind, actor?.id, targetId, from, to, page], allowed)
  const active = !!(kind || from || to || target || actor)
  const reset = () => { setKind(''); setFrom(''); setTo(''); setTarget(''); setTargetId(''); setActor(null); setPage(1) }
  const narrow = <T,>(set: (v: T) => void) => (v: T) => { set(v); setPage(1) }

  const columns: Column<AuditEvent>[] = [
    { id: 'when', header: t('When'), cell: (e) => when(e.at), sort: (e) => e.at, label: t('When') },
    {
      id: 'who', header: t('Who'), label: t('Who'),
      cell: (e) => e.actorId
        ? <button type="button" className="tz-inline-link" title={t('Show only this person’s activity')} onClick={(ev) => { ev.stopPropagation(); setActor({ id: e.actorId, name: who(e) }); setPage(1) }}>{who(e)}</button>
        : t('The platform'),
    },
    { id: 'what', header: t('What'), cell: (e) => actionLabel(e.action), primary: true, label: t('What'), sort: (e) => actionLabel(e.action) },
    { id: 'target', header: t('Target'), cell: (e) => (e.targetType ? `${targetLabel(e.targetType)} ${shortId(e.targetId)}` : '—'), label: t('Target') },
    { id: 'details', header: t('Details'), cell: (e) => summary(e) || '—', label: t('Details') },
  ]

  return (
    <div className="tz-module">
      <ModuleHead eyebrow={t('Operations')} title={t('Audit')} copy={t('Who did what, to which resource, and when. Entries cannot be edited or deleted.')} />
      {!allowed ? (
        <EmptyState icon="user" title={t('You don’t have access')} copy={t('The audit trail is open to administrators and operators.')} />
      ) : (
        <>
          <FilterBar label={t('Filter the audit trail')} active={active} onReset={reset}
            search={{ value: target, onChange: setTarget, label: t('Target ID'), placeholder: t('Order, listing or account ID') }}>
            <select aria-label={t('What')} value={kind} onChange={(e) => narrow(setKind)(e.target.value)}>
              {kinds.map((k) => <option key={k.value} value={k.value}>{t(k.label)}</option>)}
            </select>
            <label className="tz-date"><span>{t('From')}</span><input type="date" value={from} max={to || undefined} onChange={(e) => narrow(setFrom)(e.target.value)} /></label>
            <label className="tz-date"><span>{t('To')}</span><input type="date" value={to} min={from || undefined} onChange={(e) => narrow(setTo)(e.target.value)} /></label>
            {actor && <button type="button" className="tz-chip" onClick={() => { setActor(null); setPage(1) }}>{t('Only {name}', { name: actor.name })} <span aria-hidden="true">×</span><span className="sr-only">{t('Remove this filter')}</span></button>}
            <button type="button" className="tz-small-btn" disabled={!trail.data || trail.data.items.length === 0}
              onClick={() => trail.data && downloadText(`trezutil-audit-${new Date().toISOString().slice(0, 10)}-page-${page}.csv`, csv(trail.data.items))}>
              <Icon name="download" size={14} /> {t('Export this page')}
            </button>
          </FilterBar>
          <DataTable
            caption={t('Audit trail')}
            columns={columns}
            rows={trail.data?.items ?? []}
            rowKey={(e) => String(e.id)}
            onOpen={setOpen}
            loading={trail.loading && !trail.data}
            error={trail.error ? <ErrorState error={trail.error} onRetry={trail.reload} /> : undefined}
            empty={active ? t('No entry matches these filters.') : t('Nothing has been recorded yet.')}
            serverPaging={{ page, pageSize: PAGE_SIZE, total: trail.data?.total ?? 0, onPage: setPage }}
          />
          <p className="field-hint">{t('Entries are written when something is done on the platform. Reading the trail is itself not recorded here.')}</p>
        </>
      )}

      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open ? actionLabel(open.action) : ''} subtitle={open ? when(open.at, true) : undefined}>
        {open && (
          <dl className="tz-facts">
            <div><dt>{t('Who')}</dt><dd>{open.actorName || t('The platform')}{open.actorId && <small> {open.actorId}</small>}</dd></div>
            <div><dt>{t('Action')}</dt><dd><code>{open.action}</code></dd></div>
            <div><dt>{t('Target')}</dt><dd>{open.targetType ? <>{targetLabel(open.targetType)} <small>{open.targetId}</small></> : '—'}</dd></div>
            {open.requestId && <div><dt>{t('Request ID')}</dt><dd><code>{open.requestId}</code></dd></div>}
            <div><dt>{t('Details')}</dt><dd>{Object.keys(open.metadata).length === 0 ? '—' : <pre>{JSON.stringify(open.metadata, null, 2)}</pre>}</dd></div>
          </dl>
        )}
      </DetailDrawer>
    </div>
  )
}
