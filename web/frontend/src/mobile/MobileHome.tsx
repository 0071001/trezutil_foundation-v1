import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { conversationStore, useConversations, type Conversation } from '@/conversation/store'
import { OfferPanel } from '@/commerce/OfferPanel'
import type { Offer } from '@/commerce/offers'
import { useSession } from '@/lib/session'
import { timeAgo } from '@/lib/format'
import { useKindCounts, useOffers } from '@/mobile/catalog'
import { Eyebrow, HeadTools, OfferRow, OfferTile, SectionHead, TileSkeleton } from '@/mobile/parts'
import { Icon } from '@/ui/Icon'
import { initials } from '@/ui/kit'
import { t, tn, tx } from '@/lib/i18n'

/*
 * The mobile home (Design Trezutil Mobile App → Accueil): the greeting, the composer with its rotating examples, the
 * kinds of offers, published offers, offers matching the last request, and the conversation to pick up again.
 */

const EXAMPLES = [
  tx('Trouve un appartement à Gombe'),
  tx('Je cherche un collier en or'),
  tx('Réserve une table pour ce soir'),
  tx('Trouve un plombier disponible aujourd’hui'),
  tx('Hôtels disponibles ce weekend'),
]

export function latestConversation(list: Conversation[]): Conversation | undefined {
  return list.filter((c) => !c.archived && c.turns.length > 0).sort((a, b) => b.updatedAt - a.updatedAt)[0]
}

/** The last request written in a conversation, and how many offers its last answer listed. */
export function lastRequest(conv: Conversation | undefined): { text: string; offers: number } {
  if (!conv) return { text: '', offers: 0 }
  const text = [...conv.turns].reverse().find((x) => x.role === 'user')?.text ?? ''
  const answer = [...conv.turns].reverse().find((x) => x.role === 'assistant' && x.reply?.result_items.length)
  return { text, offers: answer?.reply?.result_items.length ?? 0 }
}

function useRotating(count: number, every = 3800): [number, boolean] {
  const [i, setI] = useState(0)
  const [shown, setShown] = useState(true)
  useEffect(() => {
    if (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    let swap = 0
    const id = window.setInterval(() => {
      setShown(false)
      swap = window.setTimeout(() => { setI((x) => (x + 1) % count); setShown(true) }, 280)
    }, every)
    return () => { window.clearInterval(id); window.clearTimeout(swap) }
  }, [count, every])
  return [i, shown]
}

export function MobileHome() {
  const { user } = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const { conversations } = useConversations()
  const [query, setQuery] = useState('')
  const field = useRef<HTMLTextAreaElement>(null)
  const [example, shown] = useRotating(EXAMPLES.length)
  const [open, setOpen] = useState<Offer | null>(null)
  const kinds = useKindCounts()
  const near = useOffers('', '', 8)
  const latest = latestConversation(conversations)
  const last = lastRequest(latest)
  const forYou = useOffers(last.text, '', 2, !!last.text)
  const name = user?.display_name || ''
  const first = name.split(/\s+/)[0] ?? ''

  useEffect(() => { document.title = 'Trezutil' }, [])
  // "Ask" in the tab bar with no conversation yet lands here, ready to write.
  useEffect(() => { if ((location.state as { focus?: boolean } | null)?.focus) field.current?.focus() }, [location.state])

  const submit = (text = query) => {
    const id = conversationStore.send(undefined, text.trim())
    if (id) { setQuery(''); navigate(`/c/${id}`) }
  }

  return (
    <div className="m-screen m-home">
      <header className="m-head">
        <span className="m-wordmark">Trezutil</span>
        <div className="m-head-actions">
          <span className="m-chip"><Icon name="pin" size={11} />Kinshasa</span>
          <HeadTools />
          {user
            ? <Link className="m-avatar" to="/profile" aria-label={t('Mon profil')}>{initials(name)}</Link>
            : user === null ? <Link className="m-chip strong" to="/login?returnTo=%2F">{t('Se connecter')}</Link> : null}
        </div>
      </header>

      <div className="m-scroll">
        <div className="m-greeting">
          <p>{first ? t('Bonjour, {name}', { name: first }) : t('Bonjour')}</p>
          <h1>{t('Que cherches-tu')}<br />{t('aujourd’hui ?')}</h1>
        </div>

        <div className="m-composer">
          <div className="m-composer-field">
            <label className="sr-only" htmlFor="m-home-field">{t('Ton besoin')}</label>
            <textarea id="m-home-field" ref={field} rows={1} value={query} maxLength={4000} onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) { e.preventDefault(); submit() } }} />
            {!query && <span className={`m-placeholder${shown ? '' : ' out'}`} aria-hidden="true">{t(EXAMPLES[example]!)}</span>}
          </div>
          <div className="m-composer-bar">
            <button type="button" className="m-pill" onClick={() => navigate('/voice')}><Icon name="mic" size={13} />{t('Parler')}</button>
            <div>
              {query.length > 0 && <small>{t('{n} car.', { n: query.length })}</small>}
              <button type="button" className={`m-send${query.trim() ? ' on' : ''}`} onClick={() => submit()} disabled={!query.trim()} aria-label={t('Envoyer')}><Icon name="send" size={14} /></button>
            </div>
          </div>
        </div>

        {kinds && kinds.length > 0 && (
          <section className="m-section">
            <Eyebrow>{t('Explorer par catégorie')}</Eyebrow>
            <div className="m-hscroll">
              {kinds.map(({ kind }) => (
                <button type="button" key={kind.vertical} className={`m-cat ${kind.tone}`} onClick={() => navigate(`/discover?kind=${kind.vertical}`)}>{t(kind.label)}</button>
              ))}
            </div>
          </section>
        )}

        <section className="m-section">
          <SectionHead title={t('À Kinshasa')} action={<Link className="m-link" to="/discover">{t('Tout voir')}</Link>} />
          <div className="m-hscroll">
            {near.offers === null ? <TileSkeleton /> : near.offers.map((o) => <OfferTile key={o.id} offer={o} onOpen={() => setOpen(o)} />)}
          </div>
          {near.failed && <p className="m-note">{t('Les offres n’ont pas pu être chargées. Vérifie ta connexion.')}</p>}
        </section>

        {last.text && forYou.offers && forYou.offers.length > 0 && (
          <section className="m-section">
            <SectionHead title={t('Pour toi')} action={latest ? <Link className="m-link" to={`/c/${latest.id}`}>{t('Voir plus')}</Link> : undefined} />
            <div className="m-list">
              {forYou.offers.map((o) => <OfferRow key={o.id} offer={o} context={t('Car tu as cherché « {q} »', { q: last.text.length > 40 ? `${last.text.slice(0, 40)}…` : last.text })} onOpen={() => setOpen(o)} />)}
            </div>
          </section>
        )}

        {latest && (
          <Link className="m-continue" to={`/c/${latest.id}`}>
            <b>{t('Continuer la recherche')}</b>
            <span>{latest.title}{last.offers > 0 ? ` · ${tn(last.offers, '{n} résultat', '{n} résultats')}` : ''}</span>
            <small>{timeAgo(latest.updatedAt / 1000)} · {t('Conversation en cours')}</small>
          </Link>
        )}
      </div>

      {open && (
        <OfferPanel offer={open} onClose={() => setOpen(null)}
          onAsk={() => { const o = open; setOpen(null); submit(t('Je cherche « {title} »', { title: o.title })) }} />
      )}
    </div>
  )
}
