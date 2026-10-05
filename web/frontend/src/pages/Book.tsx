import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { reservationsApi, type ReservationSummary } from '@/api/commerce'
import { listingsApi, type Availability } from '@/api/listings'
import { ListingImage } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Brand, Button, Field, Heading, Icon } from '@/components/ui'
import { formatDateTime, formatMoney } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { HotelBooking } from '@/pages/HotelBooking'
import { t, plural } from '@/lib/i18n'

/* Booking flow in the Figma checkout visual language:
   dates → live availability → reserve → confirmation. */

function toLocalInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export default function BookPage() {
  const { id = '' } = useParams()
  const listing = useAsync(() => listingsApi.get(id), [id])
  const tomorrow = useMemo(() => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(14, 0, 0, 0); return d }, [])
  const [start, setStart] = useState(toLocalInput(tomorrow))
  const [end, setEnd] = useState(toLocalInput(new Date(tomorrow.getTime() + 2 * 86400000)))
  const [guests, setGuests] = useState(2)
  const [availability, setAvailability] = useState<Availability | null>(null)
  const [checking, setChecking] = useState(false)
  const [booking, setBooking] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const [booked, setBooked] = useState<ReservationSummary | null>(null)
  const idempotencyKey = useMemo(() => crypto.randomUUID(), [])

  useEffect(() => { document.title = t('Reserve — Trezutil') }, [])
  useEffect(() => { setAvailability(null) }, [start, end])

  const range = () => ({ startAt: Math.floor(new Date(start).getTime() / 1000), endAt: Math.floor(new Date(end).getTime() / 1000) })
  const validRange = () => {
    const { startAt, endAt } = range()
    if (!(endAt > startAt)) return t('The end must be after the start.')
    if (startAt < Date.now() / 1000 - 60) return t('Choose a start in the future.')
    return null
  }

  const check = async () => {
    const invalid = validRange()
    if (invalid) { setError(null); setAvailability({ available: false, remaining: 0, reason: invalid }); return }
    setChecking(true); setError(null)
    try {
      const { startAt, endAt } = range()
      setAvailability(await listingsApi.availability(id, startAt, endAt))
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setChecking(false)
    }
  }

  const book = async (event: FormEvent) => {
    event.preventDefault()
    const invalid = validRange()
    if (invalid) { setAvailability({ available: false, remaining: 0, reason: invalid }); return }
    setBooking(true); setError(null)
    try {
      const { startAt, endAt } = range()
      setBooked(await reservationsApi.create(id, startAt, endAt, guests, idempotencyKey))
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBooking(false)
    }
  }

  if (listing.loading) return <AppShell hideDock><Loading label={t('Loading…')} /></AppShell>
  if (listing.error || !listing.data) return <AppShell hideDock><section className="page-section"><ErrorState error={listing.error ?? t('Listing not found')} onRetry={listing.reload} /></section></AppShell>
  const l = listing.data
  // Hotel stays book whole nights and several rooms; payment follows the stay.
  if (l.vertical === 'HOTEL') return <HotelBooking listing={l} />
  const step = booked ? 3 : availability?.available ? 2 : 1

  return (
    <AppShell hideDock>
      <section className="page-section checkout-page">
        <div className="checkout-head"><Brand /><span><Icon name="calendar" size={15} />{t('Reservation')}</span></div>
        <div role="list" className="checkout-progress" aria-label={t('Booking progress')}>
          {[t('Dates'), t('Reserve'), t('Confirmation')].map((item, i) => <div role="listitem" className={step >= i + 1 ? 'active' : ''} key={item} aria-current={step === i + 1 ? 'step' : undefined}><span>{step > i + 1 ? '✓' : i + 1}</span><strong>{item}</strong></div>)}
        </div>
        <div className="checkout-layout">
          <div className="checkout-form">
            {booked ? (
              <div className="checkout-success"><span className="empty-mark"><Icon name="check" size={26} /></span>
                <Heading level={2}>{booked.status === 'confirmed' ? t('You’re booked!') : t('Request sent')}</Heading>
                <p>{booked.status === 'confirmed' ? t('{title} is reserved for you.', { title: l.title }) : t('The seller will confirm your reservation shortly.')}</p>
                <div className="info-rows"><div><span>{t('From')}</span><strong>{formatDateTime(booked.start_at)}</strong></div><div><span>{t('To')}</span><strong>{formatDateTime(booked.end_at)}</strong></div><div><span>{t('Guests')}</span><strong>{booked.party_size}</strong></div></div>
                <div className="checkout-buttons"><ActionLink className="primary-button" href={`/reservations/${encodeURIComponent(booked.id)}`}>{t('View reservation')} <Icon name="arrow" /></ActionLink><ActionLink className="outline-button" href="/reservations">{t('All reservations')}</ActionLink></div>
              </div>
            ) : (
              <form onSubmit={book}>
                <Heading level={2}>{t('Choose your dates')}</Heading>
                <p>{t('We check live availability before anything is reserved.')}</p>
                <div className="form-grid">
                  <Field label={t('From')}><input type="datetime-local" value={start} min={toLocalInput(new Date())} onChange={(e) => setStart(e.target.value)} required /></Field>
                  <Field label={t('To')}><input type="datetime-local" value={end} min={start} onChange={(e) => setEnd(e.target.value)} required /></Field>
                  <Field label={t('Guests')}><input type="number" min={1} max={50} value={guests} onChange={(e) => setGuests(Math.max(1, Math.min(50, Number(e.target.value) || 1)))} /></Field>
                </div>
                {availability && (
                  <div className={`inline-notice ${availability.available ? 'success' : 'error'}`} role="status">
                    <Icon name={availability.available ? 'check' : 'bell'} size={16} />
                    <span>{availability.available ? t('Available for these dates.') : (availability.reason ? t('Not available — {reason}. Try other dates.', { reason: availability.reason }) : t('Not available. Try other dates.'))}</span>
                  </div>
                )}
                {error && (error.status === 409 || error.code === 'UNAVAILABLE' || error.code === 'CONFLICT'
                  ? <div className="inline-notice error" role="alert"><Icon name="bell" size={16} /><span>{t('Those dates were just taken. Please choose other dates.')}</span></div>
                  : <ErrorState error={error} compact />)}
                <div className="checkout-buttons">
                  <Button className="outline-button" onClick={() => void check()} busy={checking}>{checking ? t('Checking…') : t('Check availability')}</Button>
                  <Button className="primary-button" type="submit" busy={booking} disabled={!availability?.available || l.status !== 'published'}>{booking ? t('Reserving…') : t('Reserve')}<Icon name="calendar" /></Button>
                </div>
              </form>
            )}
          </div>
          <aside className="checkout-summary" aria-label={t('Reservation summary')}>
            <Heading level={3}>{t('Your reservation')}</Heading>
            <div className="mini-order"><ListingImage src={l.media[0]?.url} alt="" type={l.listing_type} /><div><strong>{l.title}</strong><span>{plural(guests, 'guest')}</span></div><b>{formatMoney(l.price_amount, l.price_currency)}</b></div>
            <p className="summary-note">{t('Payment, if any, is arranged with the seller. You can cancel from your reservations.')}</p>
          </aside>
        </div>
      </section>
    </AppShell>
  )
}
