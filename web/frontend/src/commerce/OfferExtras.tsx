import { useState, type FormEvent } from 'react'
import { Link, useLocation } from 'react-router'
import { useSession } from '@/lib/session'
import { toApiError, type ApiError } from '@/api/client'
import { listingsApi, type Availability } from '@/api/listings'
import { searchApi } from '@/api/search'
import { canBuy, cart, useCart } from '@/commerce/cart'
import { toOffer, type Offer } from '@/commerce/offers'
import { approx, useExchangeRate } from '@/lib/exchangeRate'
import { asDateField, fieldDate, inDays } from '@/lib/dates'
import { formatDateTime } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { Icon } from '@/ui/Icon'
import { ErrorNote } from '@/ui/kit'
import { t, tn } from '@/lib/i18n'

/*
 * What goes with one offer: whether it is free on chosen dates (for a room, a table or a service), and other offers
 * of the same category. Both are read from the server; nothing is suggested that the catalogue does not hold.
 */

const unix = (field: string) => Math.floor(new Date(field).getTime() / 1000)

export function AvailabilityCheck({ offer, onReserve }: { offer: Offer; onReserve?: (start: string, end: string) => void }) {
  const { signedIn } = useSession()
  const { pathname } = useLocation()
  const [start, setStart] = useState(() => fieldDate(inDays(1), 14))
  const [end, setEnd] = useState(() => fieldDate(inDays(2), 11))
  const [result, setResult] = useState<{ answer: Availability; start: string; end: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const min = fieldDate(new Date(), new Date().getHours())
  const valid = !!asDateField(start) && !!asDateField(end) && start >= min && end > start
  // An answer holds for the dates it was asked for only.
  const current = result && result.start === start && result.end === end ? result.answer : null
  const check = async (e: FormEvent) => {
    e.preventDefault()
    if (!signedIn || !valid || busy) return
    setBusy(true)
    setError(null)
    try { setResult({ answer: await listingsApi.availability(offer.id, unix(start), unix(end)), start, end }) } catch (err) { setError(toApiError(err)) } finally { setBusy(false) }
  }
  return (
    <section className="offer-block" aria-label={t('Disponibilité par date')}>
      <h3><Icon name="calendar" size={15} />{t('Choisis tes dates')}</h3>
      <form className="date-range" onSubmit={(e) => void check(e)} noValidate>
        <label>{t('Début')}<input type="datetime-local" value={start} min={min} onChange={(e) => { setStart(e.target.value); if (e.target.value >= end && asDateField(e.target.value)) setEnd(fieldDate(inDays(1, new Date(e.target.value)), 11)) }} /></label>
        <label>{t('Fin')}<input type="datetime-local" value={end} min={start} onChange={(e) => setEnd(e.target.value)} /></label>
        {!valid && <p className="form-error" role="alert">{t('La fin doit venir après le début, à une date à venir.')}</p>}
        {signedIn
          ? <button type="submit" className="cta" disabled={!valid || busy} aria-busy={busy}>{busy ? t('Vérification…') : t('Vérifier la disponibilité')}</button>
          : <Link className="cta" to={`/login?returnTo=${encodeURIComponent(pathname)}`}>{t('Se connecter pour vérifier la disponibilité')}</Link>}
      </form>
      {error && <ErrorNote error={error} />}
      {current && (
        <p className={`availability ${current.available ? 'yes' : 'no'}`} role="status">
          <Icon name={current.available ? 'check' : 'x'} size={15} />
          {current.available ? t('Disponible du {start} au {end}.', { start: formatDateTime(unix(start)), end: formatDateTime(unix(end)) }) : t('Pas disponible à ces dates : essaie d’autres dates.')}
        </p>
      )}
      {current?.available && onReserve && <button type="button" className="cta primary wide" onClick={() => onReserve(start.replace('T', ' '), end.replace('T', ' '))}>{t('Réserver ces dates')} <Icon name="arrow" size={15} /></button>}
    </section>
  )
}

/** Other published offers of the same category, the closest in price first. */
export function similarTo(offer: Pick<Offer, 'id' | 'priceValue' | 'currency'>, others: Offer[], limit = 4): Offer[] {
  const distance = (o: Offer) => (o.currency !== offer.currency || !(o.priceValue > 0) || !(offer.priceValue > 0) ? Number.MAX_VALUE : Math.abs(o.priceValue - offer.priceValue))
  return others.filter((o) => o.id !== offer.id).sort((a, b) => distance(a) - distance(b)).slice(0, limit)
}

export function SimilarOffers({ offer, categoryId, onOpen }: { offer: Offer; categoryId: string; onOpen: (o: Offer) => void }) {
  const rate = useExchangeRate()
  const inCart = useCart()
  const found = useAsync(() => searchApi.search({ text: '', category_id: categoryId, page_size: 12 }), [categoryId])
  // These offers are not numbered in the conversation (n = 0).
  const similar = similarTo(offer, (found.data?.items ?? []).map((r) => ({ ...toOffer(r, 0), n: 0 })))
  if (!similar.length) return null
  return (
    <section className="offer-block" aria-label={t('Offres similaires')}>
      <h3><Icon name="layers" size={15} />{t('Offres similaires')}<small>{tn(similar.length, '{n} suggestion', '{n} suggestions')}</small></h3>
      <div className="similar-rail">
        {similar.map((o) => {
          const converted = approx(o.priceValue, o.currency, rate)
          const added = inCart.some((i) => i.listingId === o.id)
          return (
            <article key={o.id} className="similar-card" aria-label={o.title}>
              <div className="similar-visual" style={{ background: o.image ? undefined : o.color }}>
                {o.image ? <img src={o.image} alt="" loading="lazy" /> : <span>{o.initials}</span>}
                {o.discount > 0 && <em>−{o.discount} %</em>}
              </div>
              <b>{o.title}</b>
              <small>{o.supplier}{o.place ? ` · ${o.place}` : ''}</small>
              <strong>{o.price}</strong>
              {converted && <small>{converted}</small>}
              <div className="similar-actions">
                <button type="button" className="cta primary" onClick={() => onOpen(o)}>{t('Voir')} <Icon name="arrow" size={13} /></button>
                {canBuy(o) && <button type="button" className="cta icon" onClick={() => cart.add(o)} aria-label={t('Ajouter {title} au panier', { title: o.title })}><Icon name={added ? 'check' : 'cart'} size={15} /></button>}
              </div>
            </article>
          )
        })}
      </div>
    </section>
  )
}
