import type { Offer } from '@/commerce/offers'
import { trustLabel } from '@/commerce/offers'
import { Icon } from '@/ui/Icon'
import { Badge } from '@/ui/kit'
import { t } from '@/lib/i18n'
import { approx, useExchangeRate } from '@/lib/exchangeRate'
import { canBuy, cart, useCart } from '@/commerce/cart'

/* A small commercial card in the thread: picture, supplier and trust, product, availability, place, price. */

export function SupplierCard({ offer, onOpen, onAsk }: { offer: Offer; onOpen: () => void; onAsk?: () => void }) {
  const rate = useExchangeRate()
  const inCart = useCart().find((i) => i.listingId === offer.id)
  const stop = (fn: () => void) => (e: { stopPropagation: () => void }) => { e.stopPropagation(); fn() }
  return (
    <article className="supplier-card clickable" aria-label={t('Offre {n} : {title}', { n: offer.n, title: offer.title })} onClick={onOpen}>
      <div className="product-visual" style={{ background: offer.image ? '#eef1ef' : offer.color }}>
        {offer.image ? <img src={offer.image} alt="" loading="lazy" /> : <><span>{offer.initials}</span><small>{t('N°')}{offer.n}</small></>}
        <b className="m-offer-n" aria-hidden="true">{offer.n}</b>
        {offer.availability && <em className="m-offer-tag" aria-hidden="true">{offer.availability}</em>}
      </div>
      <div className="supplier-info">
        <div className="supplier-line">
          <span>{offer.supplier}</span>
          {offer.verified ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(offer.trustLevel)}</Badge> : <Badge>{t('Inscrit')}</Badge>}
        </div>
        <h4>{offer.title}</h4>
        <div className="meta-row">
          {offer.availability && <span><Icon name="clock" size={14} /> {offer.availability}</span>}
          {offer.place && <span><Icon name="target" size={14} /> {offer.place}</span>}
          {offer.discount > 0 && <span>{t('Promotion −')}{offer.discount} %</span>}
        </div>
      </div>
      <div className="price-block">
        <strong>{offer.price}</strong>
        <small>{offer.wasPrice ? <s>{offer.wasPrice}</s> : t('prix affiché')}</small>
        {approx(offer.priceValue, offer.currency, rate) && <small className="price-approx" title={t('Conversion indicative au taux fixé par Trezutil. Le prix et le paiement restent dans la devise du fournisseur.')}>{approx(offer.priceValue, offer.currency, rate)}</small>}
        <span className="price-actions">
          {onAsk && <button type="button" onClick={stop(onAsk)}>{t('Demander')}</button>}
          {canBuy(offer) && (
            <button type="button" className="cart-add" onClick={stop(() => cart.add(offer))} aria-label={t('Ajouter {title} au panier', { title: offer.title })}>
              <Icon name={inCart ? 'check' : 'cart'} size={14} />{inCart ? t('Dans le panier ({n})', { n: inCart.quantity }) : t('Panier')}
            </button>
          )}
          <button type="button" onClick={stop(onOpen)}>{t('Voir l’offre')} <Icon name="arrow" size={14} /></button>
        </span>
      </div>
    </article>
  )
}
