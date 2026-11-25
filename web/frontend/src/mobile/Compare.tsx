import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { conversationStore } from '@/conversation/store'
import { trustLabel, type Offer } from '@/commerce/offers'
import { OfferPanel } from '@/commerce/OfferPanel'
import { NegotiateDialog } from '@/negotiation/NegotiateDialog'
import { t, tn } from '@/lib/i18n'
import { cheapest, OfferCard } from '@/mobile/Answer'
import { MIcon } from '@/mobile/icons'
import { Screen } from '@/mobile/ui'

/*
 * Comparison (design → Comparaison): the lowest-priced offer first, with its facts, then the others. The backend gives
 * no score or ranking, so none is shown: the highlighted offer is simply the cheapest. Choosing commits nothing; the
 * runtime asks for an explicit confirmation before any order.
 */

export function MobileCompare({ conversationId, title, offers, initial }: { conversationId: string; title: string; offers: Offer[]; initial?: number }) {
  const navigate = useNavigate()
  const best = cheapest(offers)
  const [chosen, setChosen] = useState(() => (initial && offers[initial - 1]) || best || offers[0]!)
  const [negotiating, setNegotiating] = useState(false)
  const [open, setOpen] = useState<Offer | null>(null)
  useEffect(() => { document.title = `${t('Comparaison')} — Trezutil` }, [])
  const others = offers.filter((o) => o.id !== chosen.id)
  const [second, ...rest] = others
  const back = () => navigate(`/c/${conversationId}`)
  const choose = () => {
    conversationStore.send(conversationId, t('Je retiens l’offre {n} : {title} ({supplier}). Quelles sont les prochaines étapes ?', { n: chosen.n, title: chosen.title, supplier: chosen.supplier }))
    back()
  }
  const isBest = best?.id === chosen.id
  return (
    <Screen title={t('Comparaison')} back={back} className="mx-compare">
      <h2>{tn(offers.length, '{n} offre', '{n} offres')}</h2>
      <p className="mx-muted">{title}</p>
      <div className="mx-score-card">
        <div className="mx-score-head">
          <span className="mx-supplier-logo">{chosen.initials.slice(0, 1)}</span>
          <div><small>{isBest ? t('MEILLEUR PRIX') : t('OFFRE CHOISIE')}</small><strong>{chosen.supplier}</strong></div>
        </div>
        <div className="mx-score-grid">
          <span><small>{t('Prix')}</small><strong>{chosen.price}</strong></span>
          <span><small>{t('Disponibilité')}</small><strong>{chosen.availability || t('Non communiqué')}</strong></span>
          <span><small>{t('Confiance')}</small><strong>{trustLabel(chosen.trustLevel)}</strong></span>
        </div>
        <div className="mx-why"><MIcon name="check" size={16} />{isBest ? t('Prix le plus bas des offres comparées') : t('Livraison et garantie : à demander au fournisseur')}</div>
        <button type="button" onClick={() => setNegotiating(true)}>{t('Négocier cette offre')}</button>
        <button type="button" className="plain" onClick={choose}>{t('Continuer avec cette offre')}</button>
      </div>
      {second && <OfferCard offer={second} label={t('Choisir cette offre')} onOpen={() => setChosen(second)} />}
      {rest.map((o) => (
        <button type="button" className="mx-compact-offer" key={o.id} onClick={() => setOpen(o)}>
          <span className="mx-supplier-logo">{o.initials.slice(0, 1)}</span>
          <div><strong>{o.supplier}</strong><p>{o.price}{o.availability ? ` · ${o.availability}` : ''}</p></div>
          <MIcon name="chevron" size={17} />
        </button>
      ))}
      {open && <OfferPanel offer={open} conversationId={conversationId} onClose={() => setOpen(null)} onCompare={() => { setChosen(open); setOpen(null) }} />}
      {negotiating && <NegotiateDialog offers={[chosen]} conversationId={conversationId} onClose={() => setNegotiating(false)} />}
    </Screen>
  )
}
