import { useEffect, useState, type FormEvent } from 'react'
import { toApiError, type ApiError } from '@/api/client'
import { dateInput, hotelsApi, type NightCapacity, type RoomType } from '@/api/hotels'
import { searchApi } from '@/api/search'
import { sellerApi } from '@/api/seller'
import { ListingImage } from '@/components/cards'
import { EmptyState, ErrorState, SkeletonGrid } from '@/components/states'
import { ActionLink, Button, Field, Heading, Icon, StatusBadge } from '@/components/ui'
import { formatMoney, safeImage } from '@/lib/format'
import { useToast } from '@/lib/toast'
import { useAsync } from '@/lib/useAsync'
import { SellerHead, ViewOnly, useSeller } from '@/pages/seller/SellerShell'
import { t, tn } from '@/lib/i18n'

/*
 * Hotel Commerce Pack — seller side, in the Figma seller workspace language.
 * Room type = a HOTEL listing; rooms = its units; nights = nightly capacity.
 * Every change is authorized server-side (owner/admin/manager); staff see
 * the same page read-only.
 */

const WINDOW = 14

function RoomTypeForm({ onCreated }: { onCreated: (rt: RoomType) => void }) {
  const { org } = useSeller()
  const notify = useToast()
  const categories = useAsync(() => searchApi.categories(), [])
  const mine = org.seller_profiles.find((p) => p.mine) ?? org.seller_profiles[0]
  const [f, setF] = useState({ title: '', description: '', category_id: '', store_id: org.stores[0]?.id ?? '', price: '', currency: 'USD', occupancy: '2', beds: '', amenities: '', units: '1', images: '', publish: true })
  const [error, setError] = useState<string | ApiError | null>(null)
  const [saving, setSaving] = useState(false)
  const set = (k: keyof typeof f, v: string | boolean) => { setF((x) => ({ ...x, [k]: v })); setError(null) }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const images = f.images.split(/\s+/).filter(Boolean)
    if (!f.title.trim()) return setError(t('Give the room type a name.'))
    if (!f.category_id) return setError(t('Choose a category.'))
    if (!(Number(f.price) > 0)) return setError(t('Enter a nightly price greater than zero.'))
    if (!(Number(f.occupancy) >= 1 && Number(f.occupancy) <= 20)) return setError(t('Occupancy is 1 to 20 guests.'))
    if (!(Number(f.units) >= 1 && Number(f.units) <= 500)) return setError(t('A room type has 1 to 500 rooms.'))
    if (images.some((u) => !/^https:\/\//i.test(u) || !safeImage(u))) return setError(t('Image links must be public https:// addresses.'))
    if (!mine) return setError(t('A seller profile is needed first.'))
    setSaving(true)
    try {
      const rt = await hotelsApi.createRoomType({
        store_id: f.store_id, seller_id: mine.id, category_id: f.category_id, title: f.title.trim(), description: f.description.trim(),
        price_amount: Number(f.price), price_currency: f.currency, max_occupancy: Number(f.occupancy), bed_configuration: f.beds.trim(),
        amenities: f.amenities.split(',').map((a) => a.trim()).filter(Boolean), units: Number(f.units), image_urls: images,
      })
      if (f.publish) await sellerApi.setListingState(rt.listing_id, 'publish')
      notify(f.publish ? t('{title} is live. Open its nights to take bookings.', { title: rt.title }) : t('{title} saved as a draft.', { title: rt.title }), 'success')
      onCreated({ ...rt, status: f.publish ? 'published' : rt.status })
    } catch (err) {
      setError(toApiError(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="create-form hotel-form" noValidate onSubmit={(e) => void submit(e)} aria-label={t('New room type')}>
      <Heading level={2}>{t('New room type')}</Heading>
      <div className="form-grid">
        <Field label={t('Room type name')}><input value={f.title} onChange={(e) => set('title', e.target.value)} maxLength={255} placeholder={t('Deluxe room')} /></Field>
        <Field label={t('Category')}>
          <select value={f.category_id} onChange={(e) => set('category_id', e.target.value)}>
            <option value="">{t('Choose a category')}</option>
            {(categories.data ?? []).map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        </Field>
        {org.stores.length > 1 && <Field label={t('Property')}><select value={f.store_id} onChange={(e) => set('store_id', e.target.value)}>{org.stores.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</select></Field>}
        <Field label={t('Nightly price')}><input type="number" inputMode="decimal" min="0.01" step="0.01" value={f.price} onChange={(e) => set('price', e.target.value)} placeholder="90" /></Field>
        <Field label={t('Currency')}><select value={f.currency} onChange={(e) => set('currency', e.target.value)}><option value="USD">{t('USD ($)')}</option><option value="CDF">{t('CDF (FC)')}</option></select></Field>
        <Field label={t('Guests per room')}><input type="number" min={1} max={20} value={f.occupancy} onChange={(e) => set('occupancy', e.target.value)} /></Field>
        <Field label={t('Number of rooms')}><input type="number" min={1} max={500} value={f.units} onChange={(e) => set('units', e.target.value)} /></Field>
        <Field label={t('Beds')}><input value={f.beds} maxLength={120} onChange={(e) => set('beds', e.target.value)} placeholder={t('1 king bed')} /></Field>
        <Field label={t('Amenities')} hint={t('Separate with commas.')}><input value={f.amenities} onChange={(e) => set('amenities', e.target.value)} placeholder={t('Wi-Fi, Air conditioning, Breakfast')} /></Field>
      </div>
      <Field label={t('Description')}><textarea value={f.description} maxLength={4000} onChange={(e) => set('description', e.target.value)} /></Field>
      <Field label={t('Image links')} hint={t('Public https:// links, one per line; the first is the cover.')}><textarea value={f.images} onChange={(e) => set('images', e.target.value)} /></Field>
      <label className="check-row"><input type="checkbox" checked={f.publish} onChange={(e) => set('publish', e.target.checked)} /><span>{t('Publish now (guests can book once nights are open)')}</span></label>
      {error && (typeof error === 'string' ? <p className="auth-error" role="alert">{error}</p> : <ErrorState error={error} compact />)}
      <div className="checkout-buttons"><Button className="primary-button" type="submit" busy={saving}>{saving ? t('Saving…') : t('Create room type')}<Icon name="arrow" /></Button></div>
    </form>
  )
}

function RoomTypePanel({ rt, canManage, onChanged }: { rt: RoomType; canManage: boolean; onChanged: (rt: RoomType) => void }) {
  const notify = useToast()
  const [from, setFrom] = useState(dateInput(0))
  const [to, setTo] = useState(dateInput(WINDOW))
  const [rooms, setRooms] = useState(String(rt.units))
  const [units, setUnits] = useState(String(rt.units))
  const [error, setError] = useState<string | ApiError | null>(null)
  const [busy, setBusy] = useState<'nights' | 'units' | null>(null)
  const nights = useAsync(() => hotelsApi.capacity(rt.listing_id, dateInput(0), dateInput(WINDOW)), [rt.listing_id])
  const [shown, setShown] = useState<NightCapacity[] | null>(null)
  useEffect(() => { if (nights.data) setShown(nights.data) }, [nights.data])

  const openNights = async (e: FormEvent) => {
    e.preventDefault()
    const n = Number(rooms)
    if (!(n >= 0 && n <= rt.units)) return setError(t('A night offers 0 to {units} rooms.', { units: rt.units }))
    if (!(to > from)) return setError(t('“To” must be after “From”.'))
    setBusy('nights'); setError(null)
    try {
      await hotelsApi.setCapacity(rt.listing_id, from, to, n)
      notify(n === 0 ? t('Nights closed.') : tn(n, '{n} room open on those nights.', '{n} rooms open on those nights.'), 'success')
      nights.reload()
    } catch (err) {
      const apiErr = toApiError(err)
      setError(apiErr.code === 'CAPACITY_CONFLICT' ? apiErr.message : apiErr)
    } finally {
      setBusy(null)
    }
  }

  const saveUnits = async () => {
    const n = Number(units)
    if (!(n >= 1 && n <= 500)) return setError(t('A room type has 1 to 500 rooms.'))
    setBusy('units'); setError(null)
    try {
      onChanged(await hotelsApi.setUnits(rt.listing_id, n))
      notify(t('Rooms updated.'), 'success')
    } catch (err) {
      const apiErr = toApiError(err)
      setError(apiErr.code === 'CAPACITY_CONFLICT' ? apiErr.message : apiErr)
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="room-panel" aria-label={t('Nights for {title}', { title: rt.title })}>
      <div className="night-grid" role="table" aria-label={t('Next {WINDOW} nights', { WINDOW })}>
        <div className="table-head" role="row"><span role="columnheader">{t('Night')}</span><span role="columnheader">{t('Open')}</span><span role="columnheader">{t('Booked')}</span><span role="columnheader">{t('Left')}</span></div>
        {nights.loading && !shown && <p className="field-hint" role="status">{t('Loading nights…')}</p>}
        {nights.error && <ErrorState error={nights.error} onRetry={nights.reload} compact />}
        {(shown ?? []).map((n) => (
          <div role="row" key={n.date} className={n.capacity === 0 ? 'closed' : n.available === 0 ? 'full' : ''}>
            <span role="cell">{n.date}</span><span role="cell">{n.capacity || t('Closed')}</span><span role="cell">{n.reserved}</span><b role="cell">{n.available}</b>
          </div>
        ))}
      </div>
      {canManage && (
        <div className="room-controls">
          <form className="form-grid" noValidate onSubmit={(e) => void openNights(e)} aria-label={t('Set nightly rooms')}>
            <Field label={t('From')}><input type="date" value={from} min={dateInput(0)} onChange={(e) => setFrom(e.target.value)} /></Field>
            <Field label={t('To (not included)')}><input type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} /></Field>
            <Field label={t('Rooms per night')} hint={t('0 closes those nights.')}><input type="number" min={0} max={rt.units} value={rooms} onChange={(e) => setRooms(e.target.value)} /></Field>
            <Button className="small-primary" type="submit" busy={busy === 'nights'}>{t('Set nights')}</Button>
          </form>
          <div className="form-grid">
            <Field label={t('Rooms of this type')}><input type="number" min={1} max={500} value={units} onChange={(e) => setUnits(e.target.value)} /></Field>
            <Button className="outline-button" onClick={() => void saveUnits()} busy={busy === 'units'}>{t('Save rooms')}</Button>
          </div>
          {error && (typeof error === 'string' ? <p className="auth-error" role="alert">{error}</p> : <ErrorState error={error} compact />)}
        </div>
      )}
    </div>
  )
}

/** Seller hotel page: room types, rooms and nightly availability. */
export function SellerHotelPage() {
  const { org, canManage } = useSeller()
  const roomTypes = useAsync(() => hotelsApi.roomTypes(org.organization_id), [org.organization_id])
  const [items, setItems] = useState<RoomType[] | null>(null)
  const [adding, setAdding] = useState(false)
  const [open, setOpen] = useState<string | null>(null)
  useEffect(() => { if (roomTypes.data) setItems(roomTypes.data) }, [roomTypes.data])
  useEffect(() => { document.title = t('Hotel rooms — Trezutil') }, [])
  const list = items ?? []
  const replace = (rt: RoomType) => setItems((xs) => (xs ?? []).map((x) => (x.listing_id === rt.listing_id ? { ...x, ...rt } : x)))

  return (
    <>
      <SellerHead eyebrow={t('Hotel')} title={t('Hotel rooms')} copy={t('Room types, how many rooms you have, and which nights guests can book.')}
        action={canManage && !adding ? <Button className="primary-button" onClick={() => setAdding(true)}>{t('Add room type')} <Icon name="arrow" /></Button> : undefined} />
      {!canManage && <ViewOnly />}
      {adding && <RoomTypeForm onCreated={(rt) => { setItems((xs) => [rt, ...(xs ?? [])]); setAdding(false); setOpen(rt.listing_id) }} />}
      {roomTypes.loading && !items && <SkeletonGrid count={2} layout="list" />}
      {roomTypes.error && <ErrorState error={roomTypes.error} onRetry={roomTypes.reload} />}
      {items && list.length === 0 && !adding && (
        <EmptyState icon="building" title={t('No room types yet')} copy={t('Add a room type (for example “Deluxe room”), say how many rooms you have, then open the nights guests can book.')} action={canManage ? t('Add room type') : undefined} onAction={canManage ? () => setAdding(true) : undefined} />
      )}
      {list.length > 0 && (
        <div className="listing-table" role="table" aria-label={t('Room types')}>
          <div className="table-head" role="row"><span role="columnheader">{t('Room type')}</span><span role="columnheader">{t('Per night')}</span><span role="columnheader">{t('Status')}</span><span role="columnheader">{t('Rooms')}</span><span role="columnheader"><span className="sr-only">{t('Actions')}</span></span></div>
          {list.map((rt) => (
            <div key={rt.listing_id}>
              <div className="listing-row" role="row">
                <div role="cell"><ListingImage src={rt.images[0]} alt="" type="real_estate" /><span><ActionLink href={`/seller/listings/${encodeURIComponent(rt.listing_id)}`}><strong>{rt.title}</strong></ActionLink><small>{t('Up to')} {rt.max_occupancy} {t('guests')}{rt.bed_configuration ? ` · ${rt.bed_configuration}` : ''}</small></span></div>
                <b role="cell">{formatMoney(rt.nightly_price, rt.currency)}</b>
                <span role="cell"><StatusBadge status={rt.status} /></span>
                <span role="cell">{rt.units}</span>
                <span role="cell"><div className="row-actions"><Button className="outline-button" pressed={open === rt.listing_id} onClick={() => setOpen(open === rt.listing_id ? null : rt.listing_id)}>{open === rt.listing_id ? t('Hide nights') : t('Nights')}</Button></div></span>
              </div>
              {open === rt.listing_id && <RoomTypePanel rt={rt} canManage={canManage} onChanged={replace} />}
            </div>
          ))}
        </div>
      )}
    </>
  )
}
