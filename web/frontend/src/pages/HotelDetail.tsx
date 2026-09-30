import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { dateInput, hotelsApi, nightsBetween, type RoomOffer, type RoomType, type StayQuote } from '@/api/hotels'
import { ListingImage } from '@/components/cards'
import { AppShell } from '@/components/shell'
import { ErrorState, Loading } from '@/components/states'
import { ActionLink, Button, Field, Heading, Icon } from '@/components/ui'
import { formatMoney } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import type { ListingDetail } from '@/types'

/*
 * Hotel detail in the Figma DetailPage layout: the property, its room types,
 * dates and guests, live availability per room type (the tightest night
 * decides) and room selection. Reserving continues on /book (signed in).
 */

export function validateStay(checkIn: string, checkOut: string, guests: number): string | null {
  if (!checkIn || !checkOut) return 'Choose check-in and check-out dates.'
  const nights = nightsBetween(checkIn, checkOut)
  if (nights < 1) return 'Check-out must be after check-in.'
  if (nights > 30) return 'Stays are up to 30 nights.'
  if (checkIn < dateInput(0)) return 'Check-in can’t be in the past.'
  if (!(guests >= 1 && guests <= 100)) return 'Tell us how many guests.'
  return null
}

function plural(n: number, word: string) {
  return `${n} ${word}${n === 1 ? '' : 's'}`
}

function RoomCard({ room, offer, selected, onSelect }: { room: RoomType; offer?: RoomOffer; selected: boolean; onSelect: () => void }) {
  const status = offer
    ? offer.available
      ? `${plural(offer.available_rooms, 'room')} left · ${formatMoney(offer.total_price, offer.currency)} for ${plural(offer.nights, 'night')}${offer.rooms > 1 ? ` × ${offer.rooms} rooms` : ''}`
      : offer.reason || 'Not available for these dates'
    : 'Choose dates to see availability'
  const unavailable = !!offer && !offer.available
  return (
    <article className={`result-card result-list room-card ${selected ? 'selected' : ''}`}>
      <div className="result-image"><ListingImage src={room.images[0]} alt={room.title} type="real_estate" />
        <span className={`availability ${unavailable ? 'is-unavailable' : ''}`}><span />{offer ? (offer.available ? 'Available' : 'Unavailable') : 'Rooms'}</span>
      </div>
      <div className="result-body">
        <div className="product-meta">Up to {plural(room.max_occupancy, 'guest')}{room.bed_configuration ? ` · ${room.bed_configuration}` : ''}</div>
        <Heading level={3}>{room.title}</Heading>
        <strong className="price">{formatMoney(room.nightly_price, room.currency)}<small> / night</small></strong>
        {room.amenities.length > 0 && <div className="room-amenities">{room.amenities.slice(0, 6).map((a) => <span className="filter-pill" key={a}>{a}</span>)}</div>}
        <div className={`distance ${unavailable ? 'is-unavailable' : ''}`} role="status"><Icon name="calendar" size={15} />{status}</div>
        <div className="result-actions">
          <Button className={selected ? 'small-primary' : 'outline-button'} pressed={selected} onClick={onSelect} disabled={unavailable}>{selected ? 'Selected' : 'Select room'}</Button>
        </div>
      </div>
    </article>
  )
}

export function HotelDetail({ listing }: { listing: ListingDetail }) {
  const navigate = useNavigate()
  const property = useAsync(() => hotelsApi.property(listing.store_id), [listing.store_id])
  const [checkIn, setCheckIn] = useState(dateInput(1))
  const [checkOut, setCheckOut] = useState(dateInput(3))
  const [guests, setGuests] = useState(2)
  const [selected, setSelected] = useState(listing.id)
  const [quote, setQuote] = useState<StayQuote | null>(null)
  const [checking, setChecking] = useState(false)
  const [problem, setProblem] = useState<string | null>(null)
  const [error, setError] = useState<ApiError | null>(null)

  useEffect(() => { setQuote(null) }, [checkIn, checkOut, guests])
  useEffect(() => { if (property.data) document.title = `${property.data.name} — Trezutil` }, [property.data])

  if (property.loading) return <AppShell><Loading label="Loading hotel…" /></AppShell>
  if (property.error || !property.data) {
    return (
      <AppShell>
        <section className="page-section">
          <ErrorState error={property.error?.code === 'HOTEL_NOT_FOUND' ? 'This hotel isn’t taking online bookings yet.' : property.error ?? 'Hotel not found'} onRetry={property.reload} />
        </section>
      </AppShell>
    )
  }
  const hotel = property.data
  const rooms = hotel.room_types
  const current = rooms.find((r) => r.listing_id === selected) ?? rooms[0]
  const offerFor = (id: string) => quote?.offers.find((o) => o.listing_id === id)
  const chosenOffer = current ? offerFor(current.listing_id) : undefined
  const from = Math.min(...rooms.map((r) => r.nightly_price))
  const images = current?.images.length ? current.images : [null]

  const check = async (event?: FormEvent) => {
    event?.preventDefault()
    const invalid = validateStay(checkIn, checkOut, guests)
    setProblem(invalid); setError(null)
    if (invalid) return
    setChecking(true)
    try {
      const q = await hotelsApi.availability(hotel.store_id, checkIn, checkOut, guests)
      setQuote(q)
      const stillOk = q.offers.find((o) => o.listing_id === selected && o.available)
      if (!stillOk) {
        const first = q.offers.find((o) => o.available)
        if (first) setSelected(first.listing_id)
      }
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setChecking(false)
    }
  }

  const reserve = () => {
    if (!current || !chosenOffer?.available) return
    const params = new URLSearchParams({ check_in: checkIn, check_out: checkOut, guests: String(guests), rooms: String(chosenOffer.rooms) })
    navigate(`/book/${encodeURIComponent(current.listing_id)}?${params}`)
  }

  const where = [hotel.address, hotel.city].filter(Boolean).join(', ')
  return (
    <AppShell dockPrompt={`Tell me about ${hotel.name}`}>
      <section className="page-section detail-page hotel-page">
        <nav className="breadcrumbs" aria-label="Breadcrumb"><ActionLink href="/">Home</ActionLink><Icon name="chevron" size={13} /><ActionLink href="/search?vertical=HOTEL">Hotels</ActionLink><Icon name="chevron" size={13} /><span aria-current="page">{hotel.name}</span></nav>
        <div className="detail-grid">
          <div className="gallery"><div className="gallery-main"><ListingImage src={images[0]} alt={current?.title ?? hotel.name} type="real_estate" eager /></div></div>
          <div className="detail-summary">
            <div className="detail-eyebrow">Hotel · {plural(rooms.length, 'room type')}</div>
            <Heading level={1}>{hotel.name}</Heading>
            {where && <div className="detail-location"><Icon name="location" size={17} />{where}</div>}
            <strong className="detail-price">From {formatMoney(from, rooms[0]?.currency ?? 'USD')}<small> / night</small></strong>
            <form className="stay-form" onSubmit={(e) => void check(e)} noValidate>
              <div className="form-grid">
                <Field label="Check-in"><input type="date" value={checkIn} min={dateInput(0)} onChange={(e) => setCheckIn(e.target.value)} /></Field>
                <Field label="Check-out"><input type="date" value={checkOut} min={checkIn} onChange={(e) => setCheckOut(e.target.value)} /></Field>
                <Field label="Guests"><input type="number" min={1} max={100} value={guests} onChange={(e) => setGuests(Math.max(1, Math.min(100, Number(e.target.value) || 1)))} /></Field>
              </div>
              {problem && <p className="auth-error" role="alert">{problem}</p>}
              {error && <ErrorState error={error} compact />}
              <div className="detail-actions">
                <Button className="outline-button" type="submit" busy={checking}>{checking ? 'Checking…' : 'Check availability'}<Icon name="calendar" size={18} /></Button>
                <Button className="primary-button" onClick={reserve} disabled={!chosenOffer?.available}>{chosenOffer?.available ? `Reserve ${current!.title}` : 'Reserve'}<Icon name="arrow" size={18} /></Button>
              </div>
            </form>
            {quote && (
              <div className={`inline-notice ${quote.offers.some((o) => o.available) ? 'success' : 'error'}`} role="status">
                <Icon name={quote.offers.some((o) => o.available) ? 'check' : 'bell'} size={16} />
                <span>{quote.offers.some((o) => o.available)
                  ? `${plural(quote.offers.filter((o) => o.available).length, 'room type')} available for ${plural(quote.nights, 'night')}, ${plural(quote.guests, 'guest')}.`
                  : 'No rooms are free for these dates. Try other dates.'}</span>
              </div>
            )}
          </div>
        </div>
        <section className="app-rail" aria-label="Rooms">
          <div className="rail-head"><div><Heading>Choose your room</Heading><p>{quote ? `${quote.check_in} → ${quote.check_out} · ${plural(quote.guests, 'guest')}` : 'Pick your dates to see live availability.'}</p></div></div>
          <div className="room-list">{rooms.map((r) => <RoomCard key={r.listing_id} room={r} offer={offerFor(r.listing_id)} selected={current?.listing_id === r.listing_id} onSelect={() => setSelected(r.listing_id)} />)}</div>
        </section>
        {current?.description && <div className="detail-content"><div><Heading level={2}>About {current.title}</Heading><p>{current.description}</p></div></div>}
      </section>
    </AppShell>
  )
}
