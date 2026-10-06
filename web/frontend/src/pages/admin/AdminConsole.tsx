import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { canSeeOverview, isAdmin } from '@/api/admin'
import { analyticsApi } from '@/api/analytics'
import { adminConsoleApi, type AdminListing, type Payment, type Provider } from '@/api/console'
import { searchApi } from '@/api/search'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import { count, Fact, isoDate, isoDateTime, ModulePage, Panel, ReasonField, shortId, StatTiles, TextField } from '@/components/portal/ConsoleBits'
import { DataTable, type Column } from '@/components/portal/DataTable'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { FilterBar } from '@/components/portal/FilterBar'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { Button, StatusBadge } from '@/components/ui'
import { verticalLabel } from '@/components/dashboard/shared'
import { formatMinor, formatMoney } from '@/lib/format'
import { t, tn, tx } from '@/lib/i18n'
import { useSession } from '@/lib/session'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { PaymentFacts, paymentColumns } from '@/pages/seller/SellerConsole'

/*
 * The administration modules beyond orders, bookings, businesses, accounts and audit: the listing queue and its
 * moderation, payments, operations, providers, channels, security, the assistant, monetization, health, search,
 * verticals and categories. Every list and figure is the server's; every change goes through a confirmation and
 * the audit trail.
 */

const PAGE_SIZE = 25
const EYEBROW = tx('Administration')

function useStaff() {
  const { user } = useSession()
  return { operator: canSeeOverview(user), admin: isAdmin(user) }
}

function useDebounced(value: string, ms = 300): string {
  const [v, setV] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setV(value.trim()), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return v
}

const staffOnly = tx('This page is open to administrators and operators.')

// --- Listings ------------------------------------------------------------------------------------------------

const listingStatuses = [['published', tx('Live')], ['paused', tx('Paused')], ['draft', tx('Draft')], ['pending_review', tx('In review')], ['suspended', tx('Suspended')], ['closed', tx('Closed')], ['sold', tx('Sold')], ['expired', tx('Expired')]]
const flagLabels: Record<string, string> = { no_image: tx('No image'), no_price: tx('No price') }

export function AdminListingsPage() {
  const { operator } = useStaff()
  const notify = useToast()
  const [text, setText] = useState('')
  const q = useDebounced(text)
  const [status, setStatus] = useState('')
  const [flag, setFlag] = useState('')
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<AdminListing | null>(null)
  const [acting, setActing] = useState<{ l: AdminListing; suspend: boolean } | null>(null)
  const [reason, setReason] = useState('')
  useEffect(() => setPage(1), [q])
  const data = useAsync(() => adminConsoleApi.listings({ q, status, flag, page }), [q, status, flag, page], operator)
  const columns: Column<AdminListing>[] = [
    { id: 'listing', header: t('Listing'), primary: true, label: t('Listing'), sort: (l) => l.title, cell: (l) => <><strong>{l.title}</strong><small>{shortId(l.id)}{l.vertical ? ` · ${verticalLabel(l.vertical)}` : ''}</small></> },
    { id: 'business', header: t('Business'), label: t('Business'), sort: (l) => l.organizationName, cell: (l) => l.organizationName },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (l) => l.status, cell: (l) => <StatusBadge status={l.status} /> },
    { id: 'price', header: t('Price'), label: t('Price'), align: 'end', sort: (l) => l.priceAmount, cell: (l) => (l.priceAmount > 0 ? formatMoney(l.priceAmount, l.currency) : '—') },
    { id: 'flags', header: t('Checks'), label: t('Checks'), cell: (l) => (l.flags.length ? <ul className="tzx-chips">{l.flags.map((f) => <li key={f} className="attention">{t(flagLabels[f] ?? f)}</li>)}</ul> : '—') },
    { id: 'updated', header: t('Last change'), label: t('Last change'), sort: (l) => l.updatedAt, cell: (l) => isoDate(l.updatedAt) },
  ]
  const canSuspend = (l: AdminListing) => ['published', 'paused', 'pending_review'].includes(l.status)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Listings')} copy={t('Every listing across businesses, with what needs a look. Suspend one that breaks the rules; the seller cannot lift it.')} allowed={operator} deniedCopy={t(staffOnly)}>
      <FilterBar label={t('Search listings')} active={!!(text || status || flag)} onReset={() => { setText(''); setStatus(''); setFlag(''); setPage(1) }}
        search={{ value: text, onChange: setText, label: t('Search listings'), placeholder: t('Title, business or listing number') }}>
        <select aria-label={t('Status')} value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">{t('Every status')}</option>
          {listingStatuses.map(([v, l]) => <option key={v} value={v}>{t(l)}</option>)}
        </select>
        <select aria-label={t('Checks')} value={flag} onChange={(e) => { setFlag(e.target.value); setPage(1) }}>
          <option value="">{t('Every listing')}</option>
          <option value="no_image">{t('No image')}</option>
          <option value="no_price">{t('No price')}</option>
        </select>
      </FilterBar>
      <DataTable caption={t('Listings')} columns={columns} rows={data.data?.items ?? []} rowKey={(l) => l.id} onOpen={setOpen}
        loading={data.loading && !data.data} error={data.error ? <ErrorState error={data.error} onRetry={data.reload} /> : undefined}
        empty={text || status || flag ? t('Nothing matches. Try another word or filter.') : t('No listing has been created yet.')}
        serverPaging={{ page, pageSize: PAGE_SIZE, total: data.data?.total ?? 0, onPage: setPage }} />
      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open?.title ?? ''} subtitle={open?.organizationName}>
        {open && (
          <>
            <dl className="tz-facts">
              <Fact label={t('Status')}><StatusBadge status={open.status} /></Fact>
              <Fact label={t('Vertical')}>{open.vertical ? verticalLabel(open.vertical) : '—'} · {open.listingType.replace(/_/g, ' ')}</Fact>
              <Fact label={t('Category')}>{open.category || '—'}</Fact>
              <Fact label={t('Price')}>{open.priceAmount > 0 ? formatMoney(open.priceAmount, open.currency) : t('No price')}</Fact>
              <Fact label={t('Images')}>{open.images}</Fact>
              <Fact label={t('Created')}>{isoDateTime(open.createdAt)}</Fact>
              <Fact label={t('Last change')}>{isoDateTime(open.updatedAt)}</Fact>
              <Fact label={t('Listing ID')}><code>{open.id}</code></Fact>
              <Fact label={t('History')}><Link to={`/admin/audit?target_id=${encodeURIComponent(open.id)}`}>{t('See what was done to it')}</Link></Fact>
            </dl>
            <div className="tzx-actions">
              {open.status === 'published' && <Link className="tz-small-btn" to={`/listings/${encodeURIComponent(open.id)}`}>{t('View as a customer')}</Link>}
              {canSuspend(open) && <Button className="tz-small-btn danger" onClick={() => { setReason(''); setActing({ l: open, suspend: true }) }}>{t('Suspend')}</Button>}
              {open.status === 'suspended' && <Button className="tz-small-btn" onClick={() => { setReason(''); setActing({ l: open, suspend: false }) }}>{t('Reinstate')}</Button>}
            </div>
          </>
        )}
      </DetailDrawer>
      <ConfirmationDialog open={!!acting} tone={acting?.suspend ? 'danger' : 'default'}
        title={acting?.suspend ? t('Suspend “{title}”?', { title: acting.l.title }) : t('Reinstate “{title}”?', { title: acting?.l.title ?? '' })}
        confirmLabel={acting?.suspend ? t('Suspend listing') : t('Reinstate listing')}
        onConfirm={async () => {
          if (!acting) return
          if (acting.suspend) {
            if (reason.trim().length < 3) throw new Error(t('Give a reason of at least 3 characters.'))
            await adminConsoleApi.suspendListing(acting.l.id, reason.trim())
            notify(t('“{title}” is suspended and leaves search within a minute.', { title: acting.l.title }), 'success')
          } else {
            await adminConsoleApi.reinstateListing(acting.l.id, reason.trim())
            notify(t('“{title}” is back, paused: its seller decides when it goes live.', { title: acting.l.title }), 'success')
          }
          setOpen(null)
          data.reload()
        }}
        onClose={() => setActing(null)}>
        <p>{acting?.suspend
          ? t('It leaves search and cannot be bought or booked. Its seller sees it as suspended and cannot publish it again. Orders already placed are not changed.')
          : t('It comes back paused; its seller can then publish it again.')}</p>
        <ReasonField value={reason} onChange={setReason} optional={!acting?.suspend} />
      </ConfirmationDialog>
    </ModulePage>
  )
}

// --- Payments ------------------------------------------------------------------------------------------------

const paymentStatuses = [['captured', tx('Captured')], ['authorized', tx('Authorised')], ['pending', tx('Pending')], ['failed', tx('Failed')], ['refunded', tx('Refunded')], ['partially_refunded', tx('Partly refunded')], ['cancelled', tx('Cancelled')]]

export function AdminPaymentsPage() {
  const { operator } = useStaff()
  const [text, setText] = useState('')
  const q = useDebounced(text)
  const [status, setStatus] = useState('')
  const [days, setDays] = useState(30)
  const [page, setPage] = useState(1)
  const [open, setOpen] = useState<Payment | null>(null)
  useEffect(() => setPage(1), [q])
  const data = useAsync(() => adminConsoleApi.payments({ q, status, days, page }), [q, status, days, page], operator)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Payments and finance')} copy={t('Every payment taken online, by business, and the totals per currency. Read-only: refunds are made from the payment provider.')} allowed={operator} deniedCopy={t(staffOnly)}>
      <div className="tz-toolbar">
        <p className="tz-asof">{t('Totals over the last {n} days. Currencies are never added together.', { n: days })}</p>
        <PeriodTabs value={days} onChange={(d) => { setDays(d); setPage(1) }} options={[7, 30, 90, 365]} />
      </div>
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (data.data.totals.length === 0
        ? <p className="tzx-note">{t('Nothing sold or paid in this period.')}</p>
        : data.data.totals.map((m) => (
          <StatTiles key={m.currency} label={t('Totals in {currency}', { currency: m.currency })} items={[
            { label: t('Orders that count as sales ({currency})', { currency: m.currency }), value: formatMinor(m.earnedMinor, m.currency), note: tn(m.orders, '{n} order', '{n} orders') },
            { label: t('Paid online'), value: formatMinor(m.capturedMinor, m.currency), note: tn(m.payments, '{n} payment', '{n} payments') },
            { label: t('Refunded'), value: formatMinor(m.refundedMinor, m.currency) },
            { label: t('Failed payments'), value: count(m.failed), tone: m.failed ? 'attention' : undefined },
          ]} />
        )))}
      <FilterBar label={t('Search payments')} active={!!(text || status)} onReset={() => { setText(''); setStatus(''); setPage(1) }}
        search={{ value: text, onChange: setText, label: t('Search payments'), placeholder: t('Payment or order number, business') }}>
        <select aria-label={t('Status')} value={status} onChange={(e) => { setStatus(e.target.value); setPage(1) }}>
          <option value="">{t('Every status')}</option>
          {paymentStatuses.map(([v, l]) => <option key={v} value={v}>{t(l)}</option>)}
        </select>
      </FilterBar>
      <DataTable caption={t('Payments')} columns={paymentColumns(true)} rows={data.data?.payments.items ?? []} rowKey={(p) => p.id} onOpen={setOpen}
        loading={data.loading && !data.data} empty={text || status ? t('Nothing matches. Try another word or status.') : t('No payment has been made online yet.')}
        serverPaging={{ page, pageSize: PAGE_SIZE, total: data.data?.payments.total ?? 0, onPage: setPage }} />
      <DetailDrawer open={!!open} onClose={() => setOpen(null)} title={open ? formatMinor(open.amountMinor, open.currency) : ''} subtitle={open?.organizationName}>
        {open && <PaymentFacts p={open} />}
      </DetailDrawer>
    </ModulePage>
  )
}

// --- Operations ----------------------------------------------------------------------------------------------

const exceptionKinds: Record<string, string> = {
  failed_payment: tx('Failed payments'), failed_webhook: tx('Messages not processed'), failed_payment_webhook: tx('Payment notifications not processed'), unconfirmed_order: tx('Paid orders not confirmed'),
}

export function AdminOperationsPage() {
  const { operator } = useStaff()
  const [days, setDays] = useState(7)
  const data = useAsync(() => adminConsoleApi.operations(days), [days], operator)
  const d = data.data
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Incidents and support')} copy={t('What went wrong and may need somebody: failed payments, messages and notifications not processed, paid orders nobody confirmed.')} allowed={operator} deniedCopy={t(staffOnly)}>
      <div className="tz-toolbar">
        <p className="tz-asof">{d ? t('Over the last {n} days; an order counts as unconfirmed after {h} hours.', { n: d.days, h: d.stuckAfterHours }) : ''}</p>
        <PeriodTabs value={days} onChange={setDays} options={[1, 7, 30]} />
      </div>
      {data.loading && !d && <Loading label={t('Loading…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {d && (
        <>
          <StatTiles label={t('Exceptions')} items={Object.keys(exceptionKinds).map((k) => ({ label: t(exceptionKinds[k]), value: count(d.counts[k] ?? 0), tone: (d.counts[k] ?? 0) > 0 ? 'attention' as const : 'good' as const }))} />
          <Panel title={t('Most recent')}>
            {d.recent.length === 0 ? <EmptyState icon="check" title={t('Nothing needs attention')} copy={t('No failure in this period.')} /> : (
              <ul className="tzx-list">
                {d.recent.map((e) => (
                  <li key={`${e.kind}-${e.id}`}>
                    <span><strong>{t(exceptionKinds[e.kind] ?? e.kind)} · {e.reference || shortId(e.id)}</strong><small>{[e.business, e.detail, isoDateTime(e.at)].filter(Boolean).join(' · ')}</small></span>
                    {e.kind === 'unconfirmed_order' || e.kind === 'failed_payment'
                      ? <Link className="tz-small-btn" to={`/admin/audit?target_id=${encodeURIComponent(e.kind === 'failed_payment' ? e.reference : e.id)}`}>{t('History')}</Link>
                      : <StatusBadge status={e.status} />}
                  </li>
                ))}
              </ul>
            )}
          </Panel>
          <p className="tzx-note">{t('Support cases are not tracked in Trezutil yet: follow them in your support tool and quote the request id from the audit trail.')}</p>
        </>
      )}
    </ModulePage>
  )
}

// --- Providers -----------------------------------------------------------------------------------------------

export function AdminProvidersPage() {
  const { operator, admin } = useStaff()
  const notify = useToast()
  const [days, setDays] = useState(7)
  const [acting, setActing] = useState<{ p: Provider; enable: boolean } | null>(null)
  const [reason, setReason] = useState('')
  const data = useAsync(() => adminConsoleApi.providers(days), [days], operator)
  const columns: Column<Provider>[] = [
    { id: 'provider', header: t('Provider'), primary: true, label: t('Provider'), sort: (p) => p.name, cell: (p) => <><strong>{p.name}</strong><small>{p.code} · {p.capabilities.join(', ').toLowerCase()}</small></> },
    { id: 'business', header: t('Business'), label: t('Business'), sort: (p) => p.organizationName, cell: (p) => p.organizationName },
    { id: 'status', header: t('Status'), label: t('Status'), sort: (p) => p.status, cell: (p) => <><StatusBadge status={p.status.toLowerCase()} /> <StatusBadge status={p.healthStatus.toLowerCase()} /></> },
    { id: 'calls', header: t('Calls'), label: t('Calls'), align: 'end', sort: (p) => p.attempts, cell: (p) => `${count(p.attempts)}${p.failures ? ` · ${tn(p.failures, '{n} failure', '{n} failures')}` : ''}` },
    { id: 'latency', header: t('Latency'), label: t('Latency'), align: 'end', sort: (p) => p.p95Ms, cell: (p) => (p.attempts ? t('{p50} ms typical · {p95} ms slow', { p50: p.p50Ms, p95: p.p95Ms }) : '—') },
    { id: 'error', header: t('Last error'), label: t('Last error'), cell: (p) => (p.lastError ? `${p.lastError} · ${isoDate(p.lastFailureAt)}` : '—') },
    {
      id: 'act', header: t('Actions'), label: t('Actions'), cell: (p) => !admin ? '—'
        : p.status === 'ACTIVE' ? <Button className="tz-small-btn danger" onClick={() => { setReason(''); setActing({ p, enable: false }) }}>{t('Turn off')}</Button>
          : p.status === 'INACTIVE' ? <Button className="tz-small-btn" onClick={() => { setReason(''); setActing({ p, enable: true }) }}>{t('Turn on')}</Button> : '—',
    },
  ]
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Providers')} copy={t('Every provider connection, its health and how its calls went. Configuration and keys are never shown.')} allowed={operator} deniedCopy={t(staffOnly)}>
      <div className="tz-toolbar">
        <p className="tz-asof">{t('Calls over the last {n} days.', { n: days })}</p>
        <PeriodTabs value={days} onChange={setDays} options={[1, 7, 30]} />
      </div>
      <DataTable caption={t('Providers')} columns={columns} rows={data.data ?? []} rowKey={(p) => p.id}
        loading={data.loading && !data.data} error={data.error ? <ErrorState error={data.error} onRetry={data.reload} /> : undefined}
        empty={t('No provider is configured. Native sellers do not need one: customers pay them directly.')} />
      {!admin && <p className="tzx-note">{t('Only administrators can turn a provider on or off.')}</p>}
      <p className="tzx-note">{t('A change applies to payments within a minute, when the payment service reloads its providers.')}</p>
      <ConfirmationDialog open={!!acting} tone={acting?.enable ? 'default' : 'danger'}
        title={acting?.enable ? t('Turn on {name} for {business}?', { name: acting.p.name, business: acting.p.organizationName }) : t('Turn off {name} for {business}?', { name: acting?.p.name ?? '', business: acting?.p.organizationName ?? '' })}
        confirmLabel={acting?.enable ? t('Turn on') : t('Turn off')}
        onConfirm={async () => {
          if (!acting) return
          if (acting.enable) await adminConsoleApi.enableProvider(acting.p.id)
          else {
            if (reason.trim().length < 3) throw new Error(t('Give a reason of at least 3 characters.'))
            await adminConsoleApi.disableProvider(acting.p.id, reason.trim())
          }
          notify(t('Saved. It applies within a minute.'), 'success')
          data.reload()
        }}
        onClose={() => setActing(null)}>
        <p>{acting?.enable ? t('Customers of this business can pay through it again.') : t('Customers of this business can no longer pay through it until it is turned on again.')}</p>
        {!acting?.enable && <ReasonField value={reason} onChange={setReason} />}
      </ConfirmationDialog>
    </ModulePage>
  )
}

// --- Channels ------------------------------------------------------------------------------------------------

const configuredLabels: Record<string, string> = {
  whatsapp_sender: tx('WhatsApp sending'), whatsapp_otp_template: tx('WhatsApp sign-in code template'), stripe: tx('Card payments (Stripe)'), ai_assistant: tx('AI assistant'), metrics_token: tx('Metrics access'),
}

function Configured({ flags }: { flags: Record<string, boolean> }) {
  const keys = Object.keys(configuredLabels).filter((k) => k in flags)
  if (keys.length === 0) return null
  return (
    <ul className="tzx-list">
      {keys.map((k) => <li key={k}><span><strong>{t(configuredLabels[k])}</strong></span><StatusBadge status={flags[k] ? 'active' : 'inactive'} label={flags[k] ? t('Configured') : t('Not configured')} /></li>)}
    </ul>
  )
}

const channelName = (c: string) => (c === 'whatsapp' ? 'WhatsApp' : c.startsWith('payments:') ? t('Payment notifications · {provider}', { provider: c.slice(9) }) : c)

export function AdminChannelsPage() {
  const { operator } = useStaff()
  const data = useAsync(() => adminConsoleApi.channels(), [], operator)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Channels')} copy={t('How messages and notifications reach the platform, and whether each sender is configured.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !data.data && <Loading label={t('Loading…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <>
          <Panel title={t('Inbound, last 24 hours')}>
            {data.data.items.length === 0 ? <p className="tzx-note">{t('Nothing has arrived on any channel yet.')}</p> : (
              <ul className="tzx-list">
                {data.data.items.map((c) => (
                  <li key={c.channel}>
                    <span>
                      <strong>{channelName(c.channel)}</strong>
                      <small>{tn(c.events24h, '{n} event', '{n} events')} · {tn(c.failed24h, '{n} failed', '{n} failed')} · {tn(c.pending, '{n} waiting', '{n} waiting')}{c.channel.startsWith('payments:') ? '' : ` · ${tn(c.conversations7d, '{n} conversation this week', '{n} conversations this week')}`}</small>
                      {c.lastError && <small>{t('Last error: {error}', { error: c.lastError })} · {isoDateTime(c.lastFailureAt)}</small>}
                    </span>
                    <span><StatusBadge status={c.failed24h > 0 ? 'degraded' : c.events24h > 0 ? 'healthy' : 'unknown'} /><small>{c.lastEventAt ? t('last {date}', { date: isoDateTime(c.lastEventAt) }) : t('never')}</small></span>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
          <Panel title={t('Senders')} copy={t('Whether the settings are present; their values are never read here.')}><Configured flags={data.data.configured} /></Panel>
        </>
      )}
    </ModulePage>
  )
}

// --- Security ------------------------------------------------------------------------------------------------

export function AdminSecurityPage() {
  const { admin } = useStaff()
  const data = useAsync(() => adminConsoleApi.security(), [], admin)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Security')} copy={t('Staff accounts and their sign-ins, and the sensitive changes of the last 30 days.')} allowed={admin} deniedCopy={t('This page shows staff email addresses and is reserved for administrators.')}>
      {data.loading && !data.data && <Loading label={t('Loading…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <>
          <StatTiles label={t('Staff accounts')} items={[
            { label: t('Staff accounts'), value: count(data.data.staff.length) },
            { label: t('Locked now'), value: count(data.data.staff.filter((s) => s.locked).length), tone: data.data.staff.some((s) => s.locked) ? 'attention' : 'good' },
            { label: t('With failed attempts'), value: count(data.data.staff.filter((s) => s.failedAttempts > 0).length) },
            { label: t('Never signed in'), value: count(data.data.staff.filter((s) => !s.lastLoginAt).length) },
          ]} />
          <Panel title={t('Staff accounts')}>
            <ul className="tzx-list">
              {data.data.staff.map((s) => (
                <li key={s.userId}>
                  <span><strong>{s.displayName || s.email}</strong><small>{s.email} · {s.roles.join(', ')} · {s.lastLoginAt ? t('last sign-in {date}', { date: isoDateTime(s.lastLoginAt) }) : t('never signed in')}{s.failedAttempts ? ` · ${tn(s.failedAttempts, '{n} failed attempt', '{n} failed attempts')}` : ''}</small></span>
                  {s.locked ? <StatusBadge status="suspended" label={t('Locked until {date}', { date: isoDateTime(s.lockedUntil) })} /> : <StatusBadge status={s.status} />}
                </li>
              ))}
            </ul>
          </Panel>
          <Panel title={t('Sensitive changes')} actions={<Link className="tz-small-btn" to="/admin/audit">{t('Open the audit trail')}</Link>}>
            {data.data.events.length === 0 ? <p className="tzx-note">{t('Nothing in the last 30 days.')}</p> : (
              <ul className="tzx-list">
                {data.data.events.map((e) => <li key={e.id}><span><strong>{e.action}</strong><small>{e.actorName || t('System')} · {e.targetType} {shortId(e.targetId)}</small></span><small>{isoDateTime(e.at)}</small></li>)}
              </ul>
            )}
          </Panel>
          <p className="tzx-note">{t('Sign-in sessions are not stored yet, so they cannot be listed or ended from here; suspending an account stops it within 15 minutes. Request limits apply to sign-in codes and staff passwords only.')}</p>
        </>
      )}
    </ModulePage>
  )
}

// --- AI centre -----------------------------------------------------------------------------------------------

export function AdminAIPage() {
  const { operator } = useStaff()
  const [days, setDays] = useState(30)
  const data = useAsync(() => adminConsoleApi.ai(days), [days], operator)
  const d = data.data
  const max = d ? Math.max(1, ...d.perDay.map((x) => x.count)) : 1
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('AI control centre')} copy={t('What the assistant did: conversations, messages, results shown and actions confirmed.')} allowed={operator} deniedCopy={t(staffOnly)}>
      <div className="tz-toolbar"><p className="tz-asof">{t('Over the last {n} days.', { n: days })}</p><PeriodTabs value={days} onChange={setDays} /></div>
      {data.loading && !d && <Loading label={t('Loading…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {d && (
        <>
          <StatTiles label={t('Assistant activity')} items={[
            { label: t('Conversations'), value: count(d.conversations), note: tn(d.activeCustomers, '{n} person', '{n} people') },
            { label: t('Messages received'), value: count(d.messagesIn) },
            { label: t('Answers sent'), value: count(d.messagesOut) },
            { label: t('Actions confirmed'), value: count(d.confirmations.confirmed ?? 0), note: t('{n} asked', { n: count(Object.values(d.confirmations).reduce((a, b) => a + b, 0)) }) },
          ]} />
          <div className="tzx-grid2">
            <Panel title={t('Conversations per day')}>
              {d.perDay.length === 0 ? <p className="tzx-note">{t('No conversation in this period.')}</p> : (
                <ul className="tzx-list">{d.perDay.slice(-14).map((x) => <li key={x.date}><small>{x.date}</small><span style={{ flex: 1, marginLeft: '0.6rem' }}><span style={{ background: 'var(--tz-accent)', borderRadius: 4, display: 'block', height: 8, width: `${(x.count / max) * 100}%` }} /></span><strong>{x.count}</strong></li>)}</ul>
              )}
            </Panel>
            <Panel title={t('Where and what')}>
              <dl className="tz-facts">
                <Fact label={t('By channel')}>{Object.entries(d.byChannel).map(([k, v]) => `${channelName(k)} ${v}`).join(' · ') || '—'}</Fact>
                <Fact label={t('Results shown')}>{Object.entries(d.resultsByType).map(([k, v]) => `${k} ${v}`).join(' · ') || '—'}</Fact>
                <Fact label={t('Confirmations')}>{Object.entries(d.confirmations).map(([k, v]) => `${k} ${v}`).join(' · ') || '—'}</Fact>
              </dl>
            </Panel>
          </div>
          <Panel title={t('Model and cost')}><Configured flags={d.configured} />
            <p className="tzx-note">{d.aiExecutionRecords === 0
              ? t('The model, latency and cost of each AI call are not recorded yet: they are in the service metrics and logs only.')
              : t('{n} AI calls recorded.', { n: count(d.aiExecutionRecords) })}</p>
          </Panel>
        </>
      )}
    </ModulePage>
  )
}

// --- Monetization --------------------------------------------------------------------------------------------

export function AdminMonetizationPage() {
  const { operator } = useStaff()
  const data = useAsync(() => adminConsoleApi.monetization(30), [], operator)
  const m = data.data
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Monetization')} copy={t('Plans, subscriptions, credits and what the usage ledger recorded.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !m && <Loading label={t('Loading…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {m && (
        <>
          <StatTiles label={t('Credits')} items={[
            { label: t('Plans'), value: count(m.plans.length) },
            { label: t('Active subscriptions'), value: count(m.subscriptionsByStatus.active ?? 0) },
            { label: t('Credit accounts'), value: count(m.credits.accounts) },
            { label: t('Credits used'), value: count(m.credits.consumed), note: t('of {n} granted', { n: count(m.credits.granted) }) },
          ]} />
          {m.policies.filter((p) => p.active).length === 0 && <p className="tzx-note">{t('No action is metered today: usage pricing rules are not set, so nobody is charged credits.')}</p>}
          <div className="tzx-grid2">
            <Panel title={t('Plans')}>
              {m.plans.length === 0 ? <p className="tzx-note">{t('No plan is defined.')}</p> : (
                <ul className="tzx-list">{m.plans.map((p) => <li key={p.id}><span><strong>{p.name}</strong><small>{p.priceMinor ? formatMinor(p.priceMinor, p.currency) : t('Free')} · {p.billingCycle} · {tn(p.quota, '{n} credit', '{n} credits')}</small></span><span><StatusBadge status={p.status} /> {tn(p.subscribers, '{n} subscriber', '{n} subscribers')}</span></li>)}</ul>
              )}
            </Panel>
            <Panel title={t('Subscriptions')}>
              {m.subscriptions.length === 0 ? <p className="tzx-note">{t('No business has a subscription.')}</p> : (
                <ul className="tzx-list">{m.subscriptions.map((s) => <li key={s.id}><span><strong>{s.organizationName}</strong><small>{s.plan.name} · {t('since {date}', { date: isoDate(s.startedAt) })}{s.cancelAtPeriodEnd ? ` · ${t('ends with the period')}` : ''}</small></span><StatusBadge status={s.status} /></li>)}</ul>
              )}
            </Panel>
          </div>
          <Panel title={t('Usage ledger, last 30 days')}>
            {Object.keys(m.ledgerByType).length === 0 ? <p className="tzx-note">{t('Nothing recorded.')}</p> : <dl className="tz-facts">{Object.entries(m.ledgerByType).map(([k, v]) => <Fact key={k} label={k}>{count(v)}</Fact>)}</dl>}
          </Panel>
          <p className="tzx-note">{t('Plans and prices are changed by the platform team; this page reads them.')}</p>
        </>
      )}
    </ModulePage>
  )
}

// --- Health and search ---------------------------------------------------------------------------------------

const componentLabels: Record<string, string> = {
  gateway: tx('Gateway'), search: tx('Search (with OpenSearch)'), conversation: tx('Assistant and WhatsApp'), commerce: tx('Orders and bookings'), payment: tx('Payments'), postgresql: tx('Database'), identity: tx('Accounts'),
}

export function AdminHealthPage() {
  const { operator } = useStaff()
  const data = useAsync(() => adminConsoleApi.health(), [], operator)
  const h = data.data
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('System health')} copy={t('Each service checked now, the database, and the providers’ recorded health.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !h && <Loading label={t('Checking…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {h && (
        <>
          <div className="tz-toolbar">
            <p className="tz-asof">{t('Checked {date}.', { date: isoDateTime(h.checkedAt) })} <StatusBadge status={h.status} /></p>
            <Button className="tz-small-btn" onClick={data.reload}>{t('Check again')}</Button>
          </div>
          <Panel title={t('Services')}>
            <ul className="tzx-health">
              {h.components.map((c) => (
                <li key={c.name}>
                  <strong>{componentLabels[c.name] ? t(componentLabels[c.name]) : c.name}</strong>
                  <span><StatusBadge status={c.status} /></span>
                  <small>{t('{ms} ms', { ms: c.latencyMs })}{c.detail ? ` · ${c.detail}` : ''}</small>
                </li>
              ))}
            </ul>
          </Panel>
          <div className="tzx-grid2">
            <Panel title={t('Active providers by health')}>
              {Object.keys(h.providersByHealth).length === 0 ? <p className="tzx-note">{t('No active provider.')}</p> : <dl className="tz-facts">{Object.entries(h.providersByHealth).map(([k, v]) => <Fact key={k} label={k}>{v}</Fact>)}</dl>}
            </Panel>
            <Panel title={t('Integrations')}><Configured flags={h.configured} /></Panel>
          </div>
          <p className="tzx-note">{t('Error rates and latency over time are in the monitoring dashboards (Prometheus and Grafana), not here.')}</p>
        </>
      )}
    </ModulePage>
  )
}

export function AdminSearchPage() {
  const { operator } = useStaff()
  const data = useAsync(async () => {
    const [health, overview] = await Promise.all([adminConsoleApi.health(), analyticsApi.platform(7)])
    return { health, overview }
  }, [], operator)
  const search = data.data?.health.components.find((c) => c.name === 'search')
  const byStatus = data.data?.overview.listingsByStatus ?? {}
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Search and indexing')} copy={t('Whether search answers, and what it should hold.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !data.data && <Loading label={t('Checking…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <>
          <StatTiles label={t('Search')} items={[
            { label: t('Search service'), value: search ? (search.status === 'ok' ? t('Working') : search.status === 'down' ? t('Down') : t('Degraded')) : t('Not checked'), tone: search?.status === 'ok' ? 'good' : 'attention', note: search ? t('{ms} ms', { ms: search.latencyMs }) : undefined },
            { label: t('Live listings to index'), value: count(byStatus.published ?? 0) },
            { label: t('Hidden from search'), value: count(Object.entries(byStatus).filter(([k]) => k !== 'published').reduce((a, [, v]) => a + v, 0)), note: t('paused, drafts, closed, suspended') },
          ]} />
          <p className="tzx-note">{t('Search follows the database: a listing published, paused, closed or suspended appears or leaves within seconds, and a full re-sync runs every hour. The index size and lag are not reported yet.')}</p>
        </>
      )}
    </ModulePage>
  )
}

// --- Verticals and categories --------------------------------------------------------------------------------

export function AdminVerticalsPage() {
  const { operator } = useStaff()
  const data = useAsync(async () => {
    const [verticals, analytics] = await Promise.all([searchApi.verticals(), analyticsApi.platform(30)])
    return { verticals, live: analytics.publishedByVertical }
  }, [], operator)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Verticals')} copy={t('The commerce verticals, what customers can do in each and how many live listings each holds.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !data.data && <Loading label={t('Loading verticals…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <Panel title={t('Catalogue')} actions={<Link className="tz-small-btn" to="/admin/listings">{t('Open listings')}</Link>}>
          <ul className="tzx-list">
            {data.data.verticals.map((v) => (
              <li key={v.code}>
                <span><strong>{v.name || verticalLabel(v.code)}</strong><small>{v.description}</small><small>{v.capabilities.join(', ').toLowerCase()}</small></span>
                <span><ul className="tzx-chips">{v.flows.map((f) => <li key={f}>{f === 'BOOK' ? t('Booking') : t('Purchase')}</li>)}</ul><small>{tn(data.data!.live[v.code] ?? 0, '{n} live listing', '{n} live listings')}</small></span>
              </li>
            ))}
          </ul>
        </Panel>
      )}
      <p className="tzx-note">{t('Verticals are defined in the platform code; turning one on or off per business is not available yet.')}</p>
    </ModulePage>
  )
}

const slugOf = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')

export function AdminCategoriesPage() {
  const { operator } = useStaff()
  const notify = useToast()
  const data = useAsync(() => adminConsoleApi.categories(), [], operator)
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [parent, setParent] = useState('')
  const all = data.data ?? []
  const roots = all.filter((c) => !c.parentId)
  const children = (id: string) => all.filter((c) => c.parentId === id)
  return (
    <ModulePage eyebrow={t(EYEBROW)} title={t('Categories')} copy={t('The category tree listings are published under.')} allowed={operator} deniedCopy={t(staffOnly)}>
      {data.loading && !data.data && <Loading label={t('Loading categories…')} />}
      {data.error && <ErrorState error={data.error} onRetry={data.reload} />}
      {data.data && (
        <Panel title={tn(all.length, '{n} category', '{n} categories')} actions={<Button className="tz-small-btn" onClick={() => { setName(''); setSlug(''); setParent(''); setAdding(true) }}>{t('Add a category')}</Button>}>
          {all.length === 0 ? <p className="tzx-note">{t('No category yet.')}</p> : (
            <ul className="tzx-list">
              {roots.map((c) => (
                <li key={c.id}>
                  <span><strong>{c.name}</strong><small>{c.slug}</small></span>
                  {children(c.id).length > 0 && <ul className="tzx-chips">{children(c.id).map((k) => <li key={k.id}>{k.name}</li>)}</ul>}
                </li>
              ))}
            </ul>
          )}
        </Panel>
      )}
      <p className="tzx-note">{t('A category cannot be renamed or removed from the console yet, and attributes per category are not editable here.')}</p>
      <ConfirmationDialog open={adding} title={t('Add a category')} confirmLabel={t('Add')}
        onConfirm={async () => {
          const s = slug.trim() || slugOf(name)
          if (name.trim().length < 2 || !s) throw new Error(t('Give the category a name.'))
          await adminConsoleApi.createCategory(name.trim(), s, parent || undefined)
          notify(t('Category added.'), 'success')
          data.reload()
        }}
        onClose={() => setAdding(false)}>
        <TextField label={t('Name')} value={name} onChange={(v) => { setName(v); setSlug(slugOf(v)) }} maxLength={80} required />
        <TextField label={t('Address in links')} value={slug} onChange={setSlug} maxLength={80} hint={t('Lower case letters, digits and dashes.')} />
        <label className="tz-dialog-field tzx-field"><span>{t('Inside')}</span>
          <select value={parent} onChange={(e) => setParent(e.target.value)}>
            <option value="">{t('Top level')}</option>
            {roots.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        </label>
      </ConfirmationDialog>
    </ModulePage>
  )
}
