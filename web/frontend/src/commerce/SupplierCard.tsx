import type { Offer } from '@/commerce/offers'
import { trustLabel } from '@/commerce/offers'
import { Icon } from '@/ui/Icon'
import { Badge } from '@/ui/kit'

/* A small commercial card in the thread: picture, supplier and trust, product, availability, place, price. */

export function SupplierCard({ offer, onOpen, onAsk }: { offer: Offer; onOpen: () => void; onAsk?: () => void }) {
  return (
    <article className="supplier-card" aria-label={`Offre ${offer.n} : ${offer.title}`}>
      <div className="product-visual" style={{ background: offer.image ? '#eef1ef' : offer.color }}>
        {offer.image ? <img src={offer.image} alt="" loading="lazy" /> : <><span>{offer.initials}</span><small>N°{offer.n}</small></>}
      </div>
      <div className="supplier-info">
        <div className="supplier-line">
          <span>{offer.supplier}</span>
          {offer.verified ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(offer.trustLevel)}</Badge> : <Badge>Inscrit</Badge>}
        </div>
        <h4>{offer.title}</h4>
        <div className="meta-row">
          {offer.availability && <span><Icon name="clock" size={14} /> {offer.availability}</span>}
          {offer.place && <span><Icon name="target" size={14} /> {offer.place}</span>}
          {offer.discount > 0 && <span>Promotion −{offer.discount} %</span>}
        </div>
      </div>
      <div className="price-block">
        <strong>{offer.price}</strong>
        <small>{offer.wasPrice ? <s>{offer.wasPrice}</s> : 'prix affiché'}</small>
        <span className="price-actions">
          {onAsk && <button type="button" onClick={onAsk}>Demander</button>}
          <button type="button" onClick={onOpen}>Voir l’offre <Icon name="arrow" size={14} /></button>
        </span>
      </div>
    </article>
  )
}
