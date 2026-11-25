import { useEffect, useState } from 'react'
import { listingsApi } from '@/api/listings'
import { formatMoney, safeImage } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { trustLabel, type Offer } from '@/commerce/offers'
import { Icon } from '@/ui/Icon'
import { Badge, ErrorNote, Loading } from '@/ui/kit'
import { locale, t } from '@/lib/i18n'
import { approx, useExchangeRate } from '@/lib/exchangeRate'
import { canBuy, cart, MAX_QUANTITY, useCart } from '@/commerce/cart'
import { useBuy } from '@/commerce/useBuy'
import { NegotiateDialog } from '@/negotiation/NegotiateDialog'
import { AvailabilityCheck, SimilarOffers } from '@/commerce/OfferExtras'
import { isReservable } from '@/commerce/offers'

/*
 * The detail of one offer, read from the catalogue (GET /listings?listing_id=) beside the conversation: pictures,
 * description, characteristics. The search row is shown at once; the listing completes it when it arrives.
 */

const HIDDEN_KEYS = /^(internal|_|seo|search|embedding|tenant|organization|seller|store)/i

function facts(metadata: Record<string, unknown>): [string, string][] {
  return Object.entries(metadata)
    .filter(([k, v]) => !HIDDEN_KEYS.test(k) && (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') && String(v).trim())
    .slice(0, 12)
    .map(([k, v]) => [k.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase()), typeof v === 'boolean' ? (v ? t('Oui') : t('Non')) : String(v)])
}

interface PanelProps {
  offer: Offer
  onClose: () => void
  onAsk?: () => void
  onCompare?: () => void
  /** Sends a message in the conversation the offer comes from. */
  onSend?: (text: string) => void
  conversationId?: string
}

/** The offer of the conversation, or one of its similar offers opened from it. */
export function OfferPanel(props: PanelProps) {
  const [other, setOther] = useState<Offer | null>(null)
  useEffect(() => { setOther(null) }, [props.offer.id])
  const shown = other ?? props.offer
  // A similar offer has no number in the thread: nothing refers to it there.
  return <OfferDetail key={shown.id} {...props} offer={shown} onOpenOther={setOther}
    {...(other ? { onAsk: undefined, onCompare: undefined, onSend: undefined, onBack: () => setOther(null) } : {})} />
}

function OfferDetail({ offer, onClose, onAsk, onCompare, onSend, conversationId, onOpenOther, onBack }: PanelProps & { onOpenOther: (o: Offer) => void; onBack?: () => void }) {
  const rate = useExchangeRate()
  const listing = useAsync(() => listingsApi.get(offer.id), [offer.id])
  const [quantity, setQuantity] = useState(1)
  const [added, setAdded] = useState(false)
  const [negotiating, setNegotiating] = useState(false)
  const inCart = useCart().find((i) => i.listingId === offer.id)
  const { buyNow, busy, error, signedIn } = useBuy()
  const buyable = canBuy(offer)
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [onClose])
  const l = listing.data
  const pictures = (l?.media ?? []).map((m) => safeImage(m.url)).filter((u): u is string => !!u)
  if (!pictures.length && offer.image) pictures.push(offer.image)
  const details = l ? facts(l.metadata) : []
  return (
    <>
      <div className="scrim" onClick={onClose} aria-hidden="true" />
      <aside className="notification-drawer offer-panel" role="dialog" aria-modal="true" aria-label={t('Offre : {title}', { title: offer.title })}>
        <div className="drawer-head">
          {onBack && <button type="button" className="icon-btn" onClick={onBack} aria-label={t('Revenir à l’offre')}><Icon name="back" /></button>}
          <div><p className="eyebrow">{offer.n ? `${t('OFFRE')} ${offer.n}` : t('OFFRE SIMILAIRE')}</p><h2>{offer.title}</h2></div>
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t('Fermer')}><Icon name="x" /></button>
        </div>
        <div className="offer-body">
          {pictures.length > 0 && <div className="offer-pictures">{pictures.slice(0, 4).map((src) => <img key={src} src={src} alt="" loading="lazy" />)}</div>}
          <div className="offer-line">
            <span>{offer.supplier}</span>
            {offer.verified ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(offer.trustLevel)}</Badge> : <Badge>{t('Inscrit')}</Badge>}
          </div>
          <div className="offer-price">
            <strong>{l && l.price_amount > 0 ? formatMoney(l.price_amount, l.price_currency) : offer.price}</strong>
            {offer.wasPrice && <s>{offer.wasPrice}</s>}
            {approx(l && l.price_amount > 0 ? l.price_amount : offer.priceValue, l && l.price_amount > 0 ? l.price_currency : offer.currency, rate) && <small className="price-approx" title={t('Conversion indicative au taux fixé par Trezutil. Le prix et le paiement restent dans la devise du fournisseur.')}>{approx(l && l.price_amount > 0 ? l.price_amount : offer.priceValue, l && l.price_amount > 0 ? l.price_currency : offer.currency, rate)}</small>}
          </div>
          <dl>
            {offer.availability && <div><dt>{t('Disponibilité')}</dt><dd>{offer.availability}</dd></div>}
            {offer.place && <div><dt>{t('Localisation')}</dt><dd>{offer.place}</dd></div>}
            {l?.pricing_model && <div><dt>{t('Tarification')}</dt><dd>{l.pricing_model}</dd></div>}
            {l?.published_at ? <div><dt>{t('Publiée le')}</dt><dd>{new Date(l.published_at * 1000).toLocaleDateString(locale())}</dd></div> : null}
            {details.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}
          </dl>
          {listing.loading && <Loading label={t('Lecture de l’offre dans le catalogue…')} />}
          {listing.error && <ErrorNote error={listing.error} onRetry={listing.reload} />}
          {l?.description && <><h3 className="memory-heading">{t('Description')}</h3><p className="offer-description">{l.description}</p></>}
          <p className="muted-line">{t('Livraison, garantie et conditions de paiement ne sont pas indiquées dans le catalogue : demande-les avant de décider.')}</p>
          {isReservable(offer) && <AvailabilityCheck offer={offer} onReserve={onSend && offer.n ? (from, to) => onSend(t('Réserve le numéro {n} du {start} au {end}', { n: offer.n, start: from, end: to })) : undefined} />}
          <div className="offer-cta" role="group" aria-label={t('Aller plus loin')}>
            {l?.metadata?.demo === true
              ? <p className="muted-line"><Icon name="handshake" size={13} /> {t('Offre de démonstration : pas de négociation.')}</p>
              : (
                <button type="button" onClick={() => setNegotiating(true)}>
                  <span className="cta-icon"><Icon name="handshake" size={17} /></span>
                  <span><b>{t('Négocier le prix')}</b><small>{t('Trezutil demande le meilleur prix, sans révéler ton plafond.')}</small></span>
                  <Icon name="chevron" size={15} />
                </button>
              )}
            {onCompare && (
              <button type="button" onClick={onCompare}>
                <span className="cta-icon"><Icon name="layers" size={17} /></span>
                <span><b>{t('Comparer')}</b><small>{t('Avec les autres offres de cette réponse.')}</small></span>
                <Icon name="chevron" size={15} />
              </button>
            )}
            {onAsk && (
              <button type="button" onClick={onAsk}>
                <span className="cta-icon"><Icon name="message" size={17} /></span>
                <span><b>{t('Demander à Trezutil')}</b><small>{t('Livraison, garantie, paiement : pose ta question.')}</small></span>
                <Icon name="chevron" size={15} />
              </button>
            )}
          </div>
          {l?.category_id && <SimilarOffers offer={offer} categoryId={l.category_id} onOpen={onOpenOther} />}
        </div>
        {buyable && (
          <div className="offer-buy">
            <div className="qty" role="group" aria-label={t('Quantité')}>
              <button type="button" onClick={() => setQuantity((q) => Math.max(1, q - 1))} disabled={quantity <= 1} aria-label={t('Retirer un')}>−</button>
              <span aria-live="polite">{quantity}</span>
              <button type="button" onClick={() => setQuantity((q) => Math.min(MAX_QUANTITY, q + 1))} disabled={quantity >= MAX_QUANTITY} aria-label={t('Ajouter un')}>+</button>
            </div>
            <button type="button" className="secondary-btn" onClick={() => { cart.add(offer, quantity); setAdded(true) }}><Icon name="cart" size={15} />{t('Ajouter au panier')}</button>
            <button type="button" className="primary-btn" disabled={busy} aria-busy={busy} onClick={() => void buyNow(offer, quantity)}>{signedIn ? t('Acheter maintenant') : t('Se connecter pour acheter')} <Icon name="arrow" size={15} /></button>
            {(added || inCart) && <p className="muted-line" role="status"><Icon name="check" size={13} /> {t('Dans ton panier : {n}', { n: inCart?.quantity ?? quantity })}</p>}
            {error && <p className="form-error" role="alert">{error}</p>}
            <p className="muted-line"><Icon name="lock" size={13} /> {t('La commande est créée en attente de paiement : rien n’est débité avant que tu valides le paiement.')}</p>
          </div>
        )}
      </aside>
      {negotiating && <NegotiateDialog offers={[offer]} conversationId={conversationId} onClose={() => setNegotiating(false)} />}
    </>
  )
}
