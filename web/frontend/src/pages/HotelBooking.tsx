import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import type { OrderSummary, ReservationSummary } from '@/api/commerce'
import { dateInput, hotelsApi, nightsBetween, type RoomOffer } from '@/api/hotels'
import { ListingImage } from '@/components/cards'
import { PaymentPanel } from '@/components/PaymentPanel'
import { AppShell } from '@/components/shell'
import { ErrorState } from '@/components/states'
import { ActionLink, Brand, Button, Field, Heading, Icon } from '@/components/ui'
import { formatMoney } from '@/lib/format'
import { validateStay } from '@/pages/HotelDetail'
import type { ListingDetail } from '@/types'
import { t, plural } from '@/lib/i18n'

/*
 * Hotel stay booking in the Figma checkout language:
 * stay → live availability → reserve (core reservation) → pay (core order +
 * Payment Core) or pay at the property.
 */


export function HotelBooking({ listing }: { listing: ListingDetail }) {
  const [params] = useSearchParams()
  const [checkIn, setCheckIn] = useState(params.get('check_in') || dateInput(1))
  const [checkOut, setCheckOut] = useState(params.get('check_out') || dateInput(3))
  const [guests, setGuests] = useState(Math.max(1, Number(params.get('guests')) || 2))
  const [rooms, setRooms] = useState(Math.max(1, Number(params.get('rooms')) || 1))
  const [offer, setOffer] = useState<RoomOffer | null>(null)
  const [checking, setChecking] = useState(false)
  const [booking, setBooking] = useState(false)
  const [paying, setPaying] = useState(false)
  const [problem, setProblem] = useState<string | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const [booked, setBooked] = useState<ReservationSummary | null>(null)
  const [order, setOrder] = useState<OrderSummary | null>(null)
  const idempotencyKey = useMemo(() => crypto.randomUUID(), [])

  useEffect(() => { document.title = t('Book {title} — Trezutil', { title: listing.title }) }, [listing.title])

  const check = async () => {
    const invalid = validateStay(checkIn, checkOut, guests)
    setProblem(invalid); setError(null); setOffer(null)
    if (invalid) return
    setChecking(true)
    try {
      const quote = await hotelsApi.availability(listing.store_id, checkIn, checkOut, guests, rooms)
      setOffer(quote.offers.find((o) => o.listing_id === listing.id) ?? null)
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setChecking(false)
    }
  }
  // Check once on arrival with the stay chosen on the hotel page, and again when it changes.
  useEffect(() => { void check() }, [checkIn, checkOut, guests, rooms]) // eslint-disable-line react-hooks/exhaustive-deps

  const reserve = async (event: FormEvent) => {
    event.preventDefault()
    const invalid = validateStay(checkIn, checkOut, guests)
    if (invalid) { setProblem(invalid); return }
    setBooking(true); setError(null)
    try {
      setBooked(await hotelsApi.bookStay(listing.id, checkIn, checkOut, rooms, guests, idempotencyKey))
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setBooking(false)
    }
  }

  const payNow = async () => {
    if (!booked) return
    setPaying(true); setError(null)
    try {
      setOrder(await hotelsApi.checkout(booked.id))
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setPaying(false)
    }
  }

  const nights = Math.max(0, nightsBetween(checkIn, checkOut))
  const total = listing.price_amount * nights * rooms
  const step = order ? 3 : booked ? 3 : offer?.available ? 2 : 1

  return (
    <AppShell hideDock>
      <section className="page-section checkout-page">
        <div className="checkout-head"><Brand /><span><Icon name="calendar" size={15} />{t('Hotel stay')}</span></div>
        <div role="list" className="checkout-progress" aria-label={t('Booking progress')}>
          {[t('Stay'), t('Reserve'), t('Confirmation')].map((item, i) => <div role="listitem" className={step >= i + 1 ? 'active' : ''} key={item} aria-current={step === i + 1 ? 'step' : undefined}><span>{step > i + 1 ? '✓' : i + 1}</span><strong>{item}</strong></div>)}
        </div>
        <div className="checkout-layout">
          <div className="checkout-form">
            {booked ? (
              <div className="checkout-success"><span className="empty-mark"><Icon name="check" size={26} /></span>
                <Heading level={2}>{booked.status === 'confirmed' ? t('You’re booked!') : t('Request sent')}</Heading>
                <p>{listing.title} · {plural(nights, 'night')} · {plural(booked.party_size, 'guest')}</p>
                <div className="info-rows">
                  <div><span>{t('Check-in')}</span><strong>{checkIn}</strong></div>
                  <div><span>{t('Check-out')}</span><strong>{checkOut}</strong></div>
                  <div><span>{t('Rooms')}</span><strong>{booked.quantity}</strong></div>
                  <div><span>{t('Total')}</span><strong>{formatMoney(total, listing.price_currency)}</strong></div>
                </div>
                {order ? <PaymentPanel order={order} onUpdated={setOrder} /> : (
                  <div className="checkout-buttons">
                    <Button className="primary-button" onClick={() => void payNow()} busy={paying}>{paying ? t('Preparing payment…') : t('Pay now')}<Icon name="arrow" /></Button>
                    <ActionLink className="outline-button" href={`/reservations/${encodeURIComponent(booked.id)}`}>{t('Pay at the property')}</ActionLink>
                  </div>
                )}
                {error && <ErrorState error={error} compact />}
                <div className="checkout-buttons"><ActionLink className="text-action" href={`/reservations/${encodeURIComponent(booked.id)}`}>{t('View reservation')}</ActionLink><ActionLink className="text-action" href="/reservations">{t('All reservations')}</ActionLink></div>
              </div>
            ) : (
              <form onSubmit={reserve} noValidate>
                <Heading level={2}>{t('Your stay')}</Heading>
                <p>{t('Rooms are held only when you reserve; we check every night of your stay first.')}</p>
                <div className="form-grid">
                  <Field label={t('Check-in')}><input type="date" value={checkIn} min={dateInput(0)} onChange={(e) => setCheckIn(e.target.value)} /></Field>
                  <Field label={t('Check-out')}><input type="date" value={checkOut} min={checkIn} onChange={(e) => setCheckOut(e.target.value)} /></Field>
                  <Field label={t('Guests')}><input type="number" min={1} max={100} value={guests} onChange={(e) => setGuests(Math.max(1, Math.min(100, Number(e.target.value) || 1)))} /></Field>
                  <Field label={t('Rooms')}><input type="number" min={1} max={20} value={rooms} onChange={(e) => setRooms(Math.max(1, Math.min(20, Number(e.target.value) || 1)))} /></Field>
                </div>
                {problem && <p className="auth-error" role="alert">{problem}</p>}
                {checking && <p className="field-hint" role="status">{t('Checking availability…')}</p>}
                {offer && !checking && (
                  <div className={`inline-notice ${offer.available ? 'success' : 'error'}`} role="status">
                    <Icon name={offer.available ? 'check' : 'bell'} size={16} />
                    <span>{offer.available ? t('{plural} left for these nights.', { plural: plural(offer.available_rooms, 'room') }) : t('{v}. Try other dates or more rooms.', { v: offer.reason || t('Not available') })}</span>
                  </div>
                )}
                {error && (error.status === 409 || error.code === 'UNAVAILABLE'
                  ? <div className="inline-notice error" role="alert"><Icon name="bell" size={16} /><span>{t('Those rooms were just taken. Please choose other dates.')}</span></div>
                  : <ErrorState error={error} compact />)}
                <div className="checkout-buttons">
                  <ActionLink className="outline-button" href={`/hotels/${encodeURIComponent(listing.id)}`}>{t('Other rooms')}</ActionLink>
                  <Button className="primary-button" type="submit" busy={booking} disabled={!offer?.available || listing.status !== 'published'}>{booking ? t('Reserving…') : t('Reserve')}<Icon name="calendar" /></Button>
                </div>
              </form>
            )}
          </div>
          <aside className="checkout-summary" aria-label={t('Stay summary')}>
            <Heading level={3}>{t('Your stay')}</Heading>
            <div className="mini-order"><ListingImage src={listing.media[0]?.url} alt="" type="real_estate" /><div><strong>{listing.title}</strong><span>{plural(nights, 'night')} × {plural(rooms, 'room')} · {plural(guests, 'guest')}</span></div><b>{formatMoney(total, listing.price_currency)}</b></div>
            <p className="summary-note">{formatMoney(listing.price_amount, listing.price_currency)} {t('per room per night. Pay online after reserving, or at the property. You can cancel from your reservations.')}</p>
          </aside>
        </div>
      </section>
    </AppShell>
  )
}
