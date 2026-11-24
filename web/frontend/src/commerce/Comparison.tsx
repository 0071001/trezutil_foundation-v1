import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { conversationStore, useConversations } from '@/conversation/store'
import { offersOf, trustLabel, type Offer } from '@/commerce/offers'
import { rememberSupplier } from '@/memory/memory'
import { Icon } from '@/ui/Icon'
import { Badge, Empty, NotConnected } from '@/ui/kit'
import { t, tn } from '@/lib/i18n'

/*
 * The comparison of the offers one answer returned, opened from the thread without losing it. Every cell is a value
 * the backend sent; what it does not send (delivery, warranty, payment terms) is marked as not provided, so the person
 * knows what to ask. Choosing an offer commits nothing: it goes back to the conversation, where the runtime asks for
 * an explicit confirmation before any order.
 */

const NOT_GIVEN = t('Non communiqué')

type Row = [label: string, value: (o: Offer) => string, strong?: boolean]

function rows(offers: Offer[]): Row[] {
  const all: Row[] = [
    [t('Produit ou service'), (o) => o.title],
    [t('Prix affiché'), (o) => o.price, true],
    [t('Prix avant promotion'), (o) => (o.wasPrice ? `${o.wasPrice} (−${o.discount} %)` : '—')],
    [t('Disponibilité'), (o) => o.availability || NOT_GIVEN],
    [t('Localisation'), (o) => o.place || NOT_GIVEN],
    [t('Niveau de confiance'), (o) => trustLabel(o.trustLevel)],
    [t('Livraison'), () => NOT_GIVEN],
    [t('Garantie'), () => NOT_GIVEN],
  ]
  return all.filter(([label]) => label !== 'Prix avant promotion' || offers.some((o) => o.wasPrice))
}

function csv(offers: Offer[]): string {
  const table = rows(offers)
  const esc = (v: string) => `"${v.replace(/"/g, '""')}"`
  const lines = [[t('Critère'), ...offers.map((o) => o.supplier)], ...table.map(([label, get]) => [label, ...offers.map(get)])]
  return lines.map((l) => l.map(esc).join(';')).join('\n')
}

export function Comparison() {
  const { id = '', turnId = '' } = useParams()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { conversations } = useConversations()
  const conv = conversations.find((c) => c.id === id)
  const turn = conv?.turns.find((x) => x.id === turnId)
  const offers = turn?.reply ? offersOf(turn.reply.result_items) : []
  const [selected, setSelected] = useState(() => Math.max(0, Number(params.get('offer') ?? 1) - 1))
  const [negotiating, setNegotiating] = useState(false)
  const [saved, setSaved] = useState<Record<string, string>>({})
  useEffect(() => { document.title = t('Comparer les propositions — Trezutil') }, [])

  const back = () => navigate(`/c/${id}`)
  if (!conv || !offers.length) {
    return (
      <div className="workspace">
        <div className="workspace-head"><button type="button" className="back-btn" onClick={() => (conv ? back() : navigate('/'))}><span>‹</span> {t('Retour')}</button></div>
        <Empty icon="layers" title={t('Aucune offre à comparer')}>{t('Cette comparaison n’existe plus sur cet appareil.')}</Empty>
      </div>
    )
  }
  const pick = offers[Math.min(selected, offers.length - 1)]!
  const table = rows(offers)
  const choose = () => {
    conversationStore.send(id, t('Je retiens l’offre {n} : {title} ({supplier}). Quelles sont les prochaines étapes ?', { n: pick.n, title: pick.title, supplier: pick.supplier }))
    back()
  }
  const exportCsv = () => {
    const blob = new Blob([`﻿${csv(offers)}`], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'comparaison-trezutil.csv'
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="workspace">
      <div className="workspace-head">
        <button type="button" className="back-btn" onClick={back}><span>‹</span> {t('Retour à la conversation')}</button>
        <div className="title-row">
          <div>
            <p className="eyebrow">{conv.operationId ? t('OPÉRATION · ') : ''}{t('COMPARAISON')}</p>
            <h1>{t('Comparer les propositions')}</h1>
            <p>{tn(offers.length, '{n} offre pour « {title} »', '{n} offres pour « {title} »', { title: conv.title })}</p>
          </div>
          <button type="button" className="secondary-btn" onClick={exportCsv}><Icon name="download" size={16} /> {t('Exporter')}</button>
        </div>
      </div>
      <div className="compare-table-wrap">
        <table className="compare-table" style={{ minWidth: Math.max(560, 170 + offers.length * 220) }}>
          <thead>
            <tr>
              <th scope="col">{t('Critère')}</th>
              {offers.map((o, i) => (
                <th scope="col" key={o.id} className={selected === i ? 'selected' : ''}>
                  <div className="table-supplier">
                    <span className="mini-brand" style={{ background: o.color }}>{o.initials}</span>
                    <div>{o.supplier}{o.verified ? <small><Icon name="shield" size={12} /> {trustLabel(o.trustLevel)}</small> : <small className="muted">{t('Inscrit')}</small>}</div>
                    <button type="button" className="icon-btn star" aria-label={t('Ajouter {supplier} aux fournisseurs favoris', { supplier: o.supplier })} title={t('Ajouter aux favoris')}
                      onClick={() => setSaved((s) => ({ ...s, [o.id]: rememberSupplier(o.supplier) === 'added' ? t('Ajouté à ta mémoire') : t('Déjà en mémoire') }))}>
                      <Icon name={saved[o.id] ? 'check' : 'star'} size={15} />
                    </button>
                  </div>
                  {saved[o.id] && <small className="saved-note">{saved[o.id]}</small>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {table.map(([label, get, strong]) => (
              <tr key={label}>
                <td>{label}</td>
                {offers.map((o, i) => { const v = get(o); return <td key={o.id} className={`${selected === i ? 'selected' : ''} ${strong ? 'strong' : ''} ${v === NOT_GIVEN ? 'not-given' : ''}`}>{v}</td> })}
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td>{t('Choisir une offre')}</td>
              {offers.map((o, i) => (
                <td key={o.id} className={selected === i ? 'selected' : ''}>
                  <button type="button" className={selected === i ? 'select-offer active' : 'select-offer'} aria-pressed={selected === i} onClick={() => setSelected(i)}>
                    {selected === i ? <><Icon name="check" size={15} /> {t('Sélectionnée')}</> : t('Sélectionner')}
                  </button>
                </td>
              ))}
            </tr>
          </tfoot>
        </table>
      </div>
      <p className="compare-hint">{t('« {notGiven} » : le fournisseur ne l’a pas indiqué. Demande-le dans la conversation avant de décider.', { notGiven: NOT_GIVEN })}</p>
      <div className="compare-footer">
        <div><span><Icon name="shield" /></span><p><b>{t('Décision sous ton contrôle')}</b><small>{t('Aucun engagement n’est pris sans ton approbation explicite.')}</small></p></div>
        <div className="compare-actions">
          <button type="button" className="secondary-btn" onClick={() => setNegotiating(true)}>{t('Négocier')}</button>
          <button type="button" className="primary-btn" onClick={choose}>{t('Continuer avec l’offre')} {pick.n} <Icon name="arrow" size={16} /></button>
        </div>
      </div>
      {negotiating && <NegotiationPanel offer={pick} conversationId={id} onClose={() => setNegotiating(false)} />}
    </div>
  )
}

export function NegotiationPanel({ offer, conversationId, onClose }: { offer: Offer; conversationId: string; onClose: () => void }) {
  const navigate = useNavigate()
  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [onClose])
  const draft = t('Je souhaite négocier l’offre {n} de {supplier} ({title}, {price}). Mon objectif : ', { n: offer.n, supplier: offer.supplier, title: offer.title, price: offer.price })
  return (
    <div className="modal-layer" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div className="negotiation-panel" role="dialog" aria-modal="true" aria-labelledby="nego-title">
        <div className="modal-head">
          <div><p className="eyebrow">{t('NÉGOCIATION')}</p><h2 id="nego-title">{offer.supplier}</h2></div>
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t('Fermer')}><Icon name="x" /></button>
        </div>
        <div className="negotiation-summary">
          <div><small>{t('Offre actuelle')}</small><strong>{offer.price}</strong></div>
          <div><small>{t('Produit')}</small><b>{offer.title}</b></div>
          {offer.verified ? <Badge tone="green">{trustLabel(offer.trustLevel)}</Badge> : <Badge>{t('Inscrit')}</Badge>}
        </div>
        <NotConnected title={t('La négociation automatique n’est pas encore disponible')}>
          {t('Trezutil ne peut pas encore solliciter le fournisseur, recevoir ses propositions ni lui envoyer une contre-proposition. Aucune négociation n’est simulée ici.')}
        </NotConnected>
        <p className="panel-copy">{t('Tu peux préparer ta demande dans la conversation : objectif de prix, quantité, délai, garantie. Trezutil te répondra avec ce qu’il sait faire aujourd’hui.')}</p>
        <div className="modal-actions">
          <button type="button" className="secondary-btn" onClick={onClose}>{t('Fermer')}</button>
          <button type="button" className="primary-btn" onClick={() => navigate(`/c/${conversationId}`, { state: { draft } })}>{t('Préparer ma demande')} <Icon name="arrow" size={15} /></button>
        </div>
        <p className="panel-foot"><Link to="/settings/services">{t('Voir ce qui est branché')}</Link></p>
      </div>
    </div>
  )
}
