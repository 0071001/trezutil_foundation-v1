import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { toApiError } from '@/api/client'
import { reservationsApi, type ReservationSummary } from '@/api/commerce'
import { listingsApi } from '@/api/listings'
import { ListingImage } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { EmptyState, ErrorState, Loading, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Heading, Icon, PageIntro, StatusBadge, Tabs } from '@/components/ui'
import { formatDate, formatDateTime, formatMinor, sentence } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { plural, t, tx } from '@/lib/i18n'

const noReservations: Record<'Upcoming' | 'Pending' | 'Past' | 'Cancelled', string> = { Upcoming: tx('No upcoming reservations'), Pending: tx('No pending reservations'), Past: tx('No past reservations'), Cancelled: tx('No cancelled reservations') }
type ResTab = 'Upcoming' | 'Pending' | 'Past' | 'Cancelled'

function bucket(r: ReservationSummary): ResTab {
  if (r.status === 'cancelled') return 'Cancelled'
  if (r.status === 'completed' || r.end_at < Date.now() / 1000) return 'Past'
  if (r.status === 'pending') return 'Pending'
  return 'Upcoming'
}

function ReservationRow({ r }: { r: ReservationSummary }) {
  const listing = useAsync(() => listingsApi.get(r.listing_id), [r.listing_id], !!r.listing_id)
  return (
    <article className="reservation-row">
      <div className="operation-thumb"><ListingImage src={listing.data?.media[0]?.url} alt="" type={listing.data?.listing_type ?? 'real_estate'} /></div>
      <div>
        <span>{listing.data ? sentence(listing.data.listing_type) : t('Reservation')} · #{r.id.slice(0, 8)}</span>
        <Heading level={3}>{listing.data?.title ?? t('Reservation')}</Heading>
        <p><Icon name="calendar" size={15} />{formatDate(r.start_at)} → {formatDate(r.end_at)}{r.party_size ? ` · ${plural(r.party_size, 'guest')}` : ''}</p>
      </div>
      <StatusBadge status={r.status} />
      <ActionLink className="outline-button" href={`/reservations/${encodeURIComponent(r.id)}`}>{t('View booking')}</ActionLink>
    </article>
  )
}

export function ReservationsPage() {
  const list = useAsync(() => reservationsApi.list(), [])
  const [tab, setTab] = useState<ResTab>('Upcoming')
  useEffect(() => { document.title = t('Reservations — Trezutil') }, [])
  const items = (list.data ?? []).filter((r) => bucket(r) === tab).sort((a, b) => (tab === 'Past' ? b.start_at - a.start_at : a.start_at - b.start_at))
  return (
    <AppShell>
      <section className="page-section">
        <PageIntro eyebrow={t('Plans in one place')} title={t('Reservations')} copy={t('Stays, tables and services — clear and connected.')} actions={<ActionLink className="primary-button" href="/search?type=property">{t('Book something')} <Icon name="arrow" /></ActionLink>} />
        <Tabs label={t('Reservation status')} items={(['Upcoming', 'Pending', 'Past', 'Cancelled'] as ResTab[]).map((t) => ({ value: t, label: t, count: list.data ? list.data.filter((r) => bucket(r) === t).length : undefined }))} value={tab} onChange={setTab} />
        {list.loading && <SkeletonGrid count={3} layout="list" />}
        {list.error && <ErrorState error={list.error} onRetry={list.reload} />}
        {list.data && items.length === 0 && <EmptyState icon="calendar" title={t(noReservations[tab])} copy={t('Stays, tables and services you book appear here.')} action={t('Find something to book')} href="/discover" />}
        {items.length > 0 && <div className="reservation-list">{items.map((r) => <ReservationRow key={r.id} r={r} />)}</div>}
      </section>
    </AppShell>
  )
}

export function ReservationDetailPage() {
  const { id = '' } = useParams()
  const notify = useToast()
  const res = useAsync(() => reservationsApi.get(id), [id])
  const r = res.data
  const listing = useAsync(() => listingsApi.get(r!.listing_id), [r?.listing_id], !!r?.listing_id)
  const [cancelling, setCancelling] = useState(false)
  const [confirmCancel, setConfirmCancel] = useState(false)
  useEffect(() => { document.title = t('Reservation — Trezutil') }, [])

  if (res.loading) return <AppShell><Loading label={t('Loading reservation…')} /></AppShell>
  if (res.error || !r) return <AppShell><section className="page-section narrow-page"><ErrorState error={res.error ?? t('Reservation not found')} onRetry={res.reload} /></section></AppShell>

  const l = listing.data
  const cancellable = (r.status === 'pending' || r.status === 'confirmed') && r.end_at > Date.now() / 1000
  const cancel = async () => {
    setCancelling(true)
    try {
      await reservationsApi.cancel(r.id)
      notify(t('Reservation cancelled.'), 'success')
      setConfirmCancel(false)
      res.reload()
    } catch (err) {
      notify(toApiError(err).message, 'error')
    } finally {
      setCancelling(false)
    }
  }

  return (
    <AppShell>
      <section className="page-section narrow-page">
        <div className="back-link"><ActionLink href="/reservations">{t('← Back to reservations')}</ActionLink></div>
        <div className="reservation-detail-hero">
          <ListingImage src={l?.media[0]?.url} alt={l?.title ?? t('Reservation')} type={l?.listing_type ?? 'real_estate'} eager />
          <div><StatusBadge status={r.status} /><Heading level={1}>{l?.title ?? t('Your reservation')}</Heading>{l && <p><Icon name="calendar" />{sentence(l.listing_type)}</p>}</div>
        </div>
        <div className="reservation-summary">
          <div><span>{t('From')}</span><strong>{formatDate(r.start_at)}</strong><small>{formatDateTime(r.start_at)}</small></div>
          <div><span>{t('To')}</span><strong>{formatDate(r.end_at)}</strong><small>{formatDateTime(r.end_at)}</small></div>
          <div><span>{t('Guests')}</span><strong>{r.party_size || '—'}</strong><small>{r.quantity > 1 ? plural(r.quantity, 'unit') : ''}</small></div>
          <div><span>{t('Total')}</span><strong>{r.total_minor ? formatMinor(r.total_minor, r.currency) : '—'}</strong><small>{r.total_minor ? t('Agreed price') : t('Arranged with the seller')}</small></div>
        </div>
        <div className="reservation-detail-grid">
          <div>
            <Heading level={2}>{t('Booking details')}</Heading>
            <div className="info-rows">
              <div><span>{t('Confirmation')}</span><strong>#{r.id.slice(0, 8)}</strong></div>
              <div><span>{t('Status')}</span><strong>{sentence(r.status)}</strong></div>
              <div><span>{t('Booked')}</span><strong>{formatDateTime(r.created_at)}</strong></div>
              {l && <div><span>{t('Listing')}</span><strong><ActionLink href={`/products/${encodeURIComponent(l.id)}`}>{l.title}</ActionLink></strong></div>}
            </div>
          </div>
          <aside className="summary-card">
            <Heading level={3}>{t('Need to make a change?')}</Heading>
            <p>{t('Ask Trezutil to help with your booking, or cancel it here.')}</p>
            <ActionLink className="primary-button full-button" href={`/conversation?q=${encodeURIComponent(`Show my reservation ${r.id}`)}`}><Icon name="spark" />{t('Ask about this booking')}</ActionLink>
            {cancellable && !confirmCancel && <Button className="text-danger" onClick={() => setConfirmCancel(true)}>{t('Cancel reservation')}</Button>}
            {confirmCancel && (
              <div className="inline-confirm" role="alertdialog" aria-label={t('Cancel this reservation?')}>
                <p>{t('Cancel this reservation? This can’t be undone.')}</p>
                <div><Button className="text-danger" busy={cancelling} onClick={() => void cancel()}>{t('Yes, cancel')}</Button><Button className="outline-button" onClick={() => setConfirmCancel(false)}>{t('Keep it')}</Button></div>
              </div>
            )}
          </aside>
        </div>
      </section>
    </AppShell>
  )
}
