import { useEffect } from 'react'
import { listingsApi } from '@/api/listings'
import { formatMoney, safeImage } from '@/lib/format'
import { useAsync } from '@/lib/useAsync'
import { trustLabel, type Offer } from '@/commerce/offers'
import { Icon } from '@/ui/Icon'
import { Badge, ErrorNote, Loading } from '@/ui/kit'

/*
 * The detail of one offer, read from the catalogue (GET /listings?listing_id=) beside the conversation: pictures,
 * description, characteristics. The search row is shown at once; the listing completes it when it arrives.
 */

const HIDDEN_KEYS = /^(internal|_|seo|search|embedding|tenant|organization|seller|store)/i

function facts(metadata: Record<string, unknown>): [string, string][] {
  return Object.entries(metadata)
    .filter(([k, v]) => !HIDDEN_KEYS.test(k) && (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') && String(v).trim())
    .slice(0, 12)
    .map(([k, v]) => [k.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase()), typeof v === 'boolean' ? (v ? 'Oui' : 'Non') : String(v)])
}

export function OfferPanel({ offer, onClose, onAsk, onCompare }: { offer: Offer; onClose: () => void; onAsk?: () => void; onCompare: () => void }) {
  const listing = useAsync(() => listingsApi.get(offer.id), [offer.id])
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
      <aside className="notification-drawer offer-panel" role="dialog" aria-modal="true" aria-label={`Offre : ${offer.title}`}>
        <div className="drawer-head">
          <div><p className="eyebrow">OFFRE {offer.n}</p><h2>{offer.title}</h2></div>
          <button type="button" className="icon-btn" onClick={onClose} aria-label="Fermer"><Icon name="x" /></button>
        </div>
        <div className="offer-body">
          {pictures.length > 0 && <div className="offer-pictures">{pictures.slice(0, 4).map((src) => <img key={src} src={src} alt="" loading="lazy" />)}</div>}
          <div className="offer-line">
            <span>{offer.supplier}</span>
            {offer.verified ? <Badge tone="green"><Icon name="shield" size={12} /> {trustLabel(offer.trustLevel)}</Badge> : <Badge>Inscrit</Badge>}
          </div>
          <div className="offer-price">
            <strong>{l && l.price_amount > 0 ? formatMoney(l.price_amount, l.price_currency) : offer.price}</strong>
            {offer.wasPrice && <s>{offer.wasPrice}</s>}
          </div>
          <dl>
            {offer.availability && <div><dt>Disponibilité</dt><dd>{offer.availability}</dd></div>}
            {offer.place && <div><dt>Localisation</dt><dd>{offer.place}</dd></div>}
            {l?.pricing_model && <div><dt>Tarification</dt><dd>{l.pricing_model}</dd></div>}
            {l?.published_at ? <div><dt>Publiée le</dt><dd>{new Date(l.published_at * 1000).toLocaleDateString('fr-FR')}</dd></div> : null}
            {details.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}
          </dl>
          {listing.loading && <Loading label="Lecture de l’offre dans le catalogue…" />}
          {listing.error && <ErrorNote error={listing.error} onRetry={listing.reload} />}
          {l?.description && <><h3 className="memory-heading">Description</h3><p className="offer-description">{l.description}</p></>}
          <p className="muted-line">Livraison, garantie et conditions de paiement ne sont pas indiquées dans le catalogue : demande-les avant de décider.</p>
        </div>
        <div className="drawer-foot offer-actions">
          <button type="button" className="secondary-btn" onClick={onCompare}><Icon name="layers" size={15} />Comparer</button>
          {onAsk && <button type="button" className="primary-btn" onClick={onAsk}>Demander à Trezutil <Icon name="arrow" size={15} /></button>}
        </div>
      </aside>
    </>
  )
}
