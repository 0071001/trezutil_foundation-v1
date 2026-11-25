import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { conversationStore } from '@/conversation/store'
import { OfferPanel } from '@/commerce/OfferPanel'
import type { Offer } from '@/commerce/offers'
import { KINDS, useKindCounts, useOffers } from '@/mobile/catalog'
import { Eyebrow, HeadTools, OfferTile, TileSkeleton } from '@/mobile/parts'
import { Icon } from '@/ui/Icon'
import { t, tn } from '@/lib/i18n'

/*
 * Discover (Design Trezutil Mobile App → Découvrir): a search that starts a conversation, the kinds of offers as
 * filters, the busiest kinds, the published offers of the chosen kind, and collections that ask Trezutil.
 * Every count is the real number of published offers.
 */

export function Discover() {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const kind = KINDS.some((k) => k.vertical === params.get('kind')) ? params.get('kind')! : ''
  const [text, setText] = useState('')
  const [focused, setFocused] = useState(false)
  const [open, setOpen] = useState<Offer | null>(null)
  const counts = useKindCounts()
  const offers = useOffers('', kind, 10)
  useEffect(() => { document.title = `${t('Découvrir')} — Trezutil` }, [])

  const ask = (q: string) => {
    const id = conversationStore.send(undefined, q.trim())
    if (id) navigate(`/c/${id}`)
  }
  const trending = [...(counts ?? [])].sort((a, b) => b.total - a.total).slice(0, 2)

  return (
    <div className="m-screen m-discover">
      <header className="m-page-head">
        <div className="m-head-row"><Eyebrow>{t('Découvrir')}</Eyebrow><HeadTools /></div>
        <div className={`m-search${focused ? ' focused' : ''}`}>
          <Icon name="search" size={17} />
          <label className="sr-only" htmlFor="m-discover-field">{t('Cherche ou décris ce que tu veux')}</label>
          <input id="m-discover-field" type="search" enterKeyHint="search" value={text} placeholder={t('Cherche ou décris ce que tu veux…')}
            onChange={(e) => setText(e.target.value)} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
            onKeyDown={(e) => { if (e.key === 'Enter' && text.trim()) ask(text) }} />
        </div>
        {focused && <p className="m-hint">{t('Appuie sur Entrée pour lancer une conversation')}</p>}
        <div className="m-hscroll m-pills" role="group" aria-label={t('Catégories')}>
          <button type="button" className={kind === '' ? 'active' : ''} aria-pressed={kind === ''} onClick={() => setParams({}, { replace: true })}>{t('Tout')}</button>
          {(counts ?? []).map(({ kind: k }) => (
            <button type="button" key={k.vertical} className={kind === k.vertical ? 'active' : ''} aria-pressed={kind === k.vertical} onClick={() => setParams({ kind: k.vertical }, { replace: true })}>{t(k.label)}</button>
          ))}
        </div>
      </header>

      <div className="m-scroll">
        {kind === '' && trending.length > 0 && (
          <section className="m-section">
            <Eyebrow icon="trending">{t('Tendances à Kinshasa')}</Eyebrow>
            <div className="m-stack">
              {trending.map(({ kind: k, total, image }, i) => (
                <button type="button" key={k.vertical} className="m-feature" onClick={() => setParams({ kind: k.vertical }, { replace: true })}>
                  {image ? <img src={image} alt="" loading="lazy" /> : <span className="m-feature-fill" />}
                  <span className="m-feature-text"><strong>{t(k.label)}</strong><small>{tn(total, k.unit[0], k.unit[1])}</small></span>
                  <em>{i === 0 ? t('Le plus d’offres') : t('Populaire')}</em>
                </button>
              ))}
            </div>
          </section>
        )}

        <section className="m-section">
          <div className="m-section-head">
            <Eyebrow icon="pin">{kind ? t(KINDS.find((k) => k.vertical === kind)!.label) : t('À Kinshasa')}</Eyebrow>
          </div>
          <div className="m-hscroll">
            {offers.offers === null ? <TileSkeleton /> : offers.offers.map((o) => <OfferTile key={o.id} offer={o} tag="kind" onOpen={() => setOpen(o)} />)}
          </div>
          {offers.offers?.length === 0 && !offers.failed && <p className="m-note">{t('Aucune offre publiée dans cette catégorie pour l’instant.')}</p>}
          {offers.failed && <p className="m-note">{t('Les offres n’ont pas pu être chargées. Vérifie ta connexion.')}</p>}
        </section>

        {counts && counts.length > 0 && (
          <section className="m-section">
            <Eyebrow>{t('Collections')}</Eyebrow>
            <div className="m-collections">
              {counts.map(({ kind: k, total }) => (
                <button type="button" key={k.vertical} className={`m-collection ${k.tone}`} onClick={() => ask(t(k.ask))}>
                  <strong>{t(k.label)}</strong>
                  <small>{tn(total, k.unit[0], k.unit[1])}</small>
                  <Icon name="chevron" size={14} />
                </button>
              ))}
            </div>
          </section>
        )}
      </div>

      {open && (
        <OfferPanel offer={open} onClose={() => setOpen(null)}
          onAsk={() => { const o = open; setOpen(null); ask(t('Je cherche « {title} »', { title: o.title })) }} />
      )}
    </div>
  )
}
