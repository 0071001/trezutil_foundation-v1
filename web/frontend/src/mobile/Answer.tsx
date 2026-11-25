import { trustLabel, type Offer } from '@/commerce/offers'
import { approx, useExchangeRate } from '@/lib/exchangeRate'
import { t, tn } from '@/lib/i18n'
import { MIcon } from '@/mobile/icons'

/*
 * The pieces of an answer in the mobile design: the progress of what Trezutil really did, the featured offer and the
 * link to compare. Everything shown comes from the reply: no supplier count, score or delivery time is invented.
 */

export function ProgressList({ steps }: { steps: { label: string; state: 'done' | 'active' | 'todo' }[] }) {
  return (
    <div className="mx-progress">
      {steps.map((s) => (
        <div key={s.label} className={`mx-progress-step ${s.state}`}>
          <span>{s.state === 'done' ? <MIcon name="check" size={13} /> : s.state === 'active' ? <i /> : null}</span><p>{s.label}</p>
        </div>
      ))}
    </div>
  )
}

/** The offer with the lowest price among offers priced in one currency, or none. */
export function cheapest(offers: Offer[]): Offer | undefined {
  const priced = offers.filter((o) => o.priceValue > 0)
  if (priced.length < 2 || new Set(priced.map((o) => o.currency)).size > 1) return undefined
  return priced.reduce((a, b) => (b.priceValue < a.priceValue ? b : a))
}

export function OfferCard({ offer, best, onOpen, label }: { offer: Offer; best?: boolean; onOpen: () => void; label?: string }) {
  const rate = useExchangeRate()
  const converted = approx(offer.priceValue, offer.currency, rate)
  return (
    <article className={`mx-offer${best ? ' featured' : ''}`} aria-label={t('Offre {n} : {title}', { n: offer.n, title: offer.title })}>
      <div className="mx-offer-top">
        <span className="mx-supplier-logo">{offer.initials.slice(0, 1)}</span>
        <div>
          <strong>{offer.supplier}</strong>
          <p><MIcon name="shield" size={13} /> {offer.verified ? t('Fournisseur vérifié') : trustLabel(offer.trustLevel)}</p>
        </div>
        {best && <span className="mx-best">{t('Meilleur prix')}</span>}
      </div>
      <div className="mx-offer-body">
        <p className="mx-offer-name">{offer.title}</p>
        <div className="mx-offer-price"><strong>{offer.price}</strong>{converted && <span>{converted}</span>}</div>
        <div className="mx-offer-meta">
          {offer.availability && <span>{offer.availability}</span>}
          {offer.place && <span>{offer.place}</span>}
        </div>
      </div>
      <button type="button" onClick={onOpen}>{label ?? t('Voir la proposition')}<MIcon name="chevron" size={16} /></button>
    </article>
  )
}

export function OfferResults({ offers, onOpen, onCompare }: { offers: Offer[]; onOpen: (o: Offer) => void; onCompare?: () => void }) {
  const best = cheapest(offers)
  const featured = best ?? offers[0]!
  return (
    <>
      <OfferCard offer={featured} best={!!best} onOpen={() => onOpen(featured)} />
      {offers.length > 1 && onCompare && (
        <button type="button" className="mx-text-action" onClick={onCompare}>{tn(offers.length, 'Comparer {n} offre', 'Comparer les {n} offres')} <MIcon name="chevron" size={15} /></button>
      )}
    </>
  )
}
