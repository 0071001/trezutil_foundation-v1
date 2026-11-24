import { type CSSProperties, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { useSession } from '@/lib/session'
import { conversationStore, useConversations, type TurnAttachment } from '@/conversation/store'
import { Composer, type ComposerHandle } from '@/conversation/Composer'
import { AssistantMessage, StreamingAnswer, UserMessage } from '@/conversation/Message'
import { openOperationFor } from '@/operations/operations'
import { OfferPanel } from '@/commerce/OfferPanel'
import type { Offer } from '@/commerce/offers'
import { Icon, type IconName } from '@/ui/Icon'
import { Empty, ErrorNote, Loading } from '@/ui/kit'
import { getLang, t, type Lang } from '@/lib/i18n'

/*
 * The conversation: where every operation starts. "/" asks the open question; "/c/:id" is a thread with the composer
 * docked at the bottom. A suggestion prepares the request in the composer, it never sends on its own.
 */

const PROMPTS: [IconName, string, string, string][] = [
  ['search', t('Trouver un fournisseur'), t('Identifier des partenaires fiables'), t('Je cherche un fournisseur fiable pour ')],
  ['orders', t('Acheter un équipement'), t('Comparer prix et conditions'), t('Je souhaite acheter ')],
  ['layers', t('Comparer des offres'), t('Décider avec les bons critères'), t('Compare pour moi les offres de ')],
  ['calendar', t('Organiser un événement'), t('Coordonner prestataires et budget'), t('J’organise un événement : ')],
]

/** The logo's three dots, appearing one after the other (the green one last, then breathing once). */
function BrandDots() {
  return <span className="brand-dots" aria-hidden="true"><i /><i /><i /></span>
}

/** Words of the welcome title, each rising in turn; the three verbs get the logo's green underline. */
type TitleWord = { w: string; verb?: boolean; br?: boolean; tail?: string }
const TITLES: Record<Lang, TitleWord[]> = {
  fr: [
    { w: 'Qu’est-ce' }, { w: 'que' }, { w: 'tu' }, { w: 'souhaites', br: true },
    { w: 'acheter', verb: true, tail: ',' }, { w: 'vendre', verb: true }, { w: 'ou' }, { w: 'faire réaliser', verb: true }, { w: '?' },
  ],
  en: [
    { w: 'What' }, { w: 'do' }, { w: 'you' }, { w: 'want' }, { w: 'to', br: true },
    { w: 'buy', verb: true, tail: ',' }, { w: 'sell', verb: true }, { w: 'or' }, { w: 'get done', verb: true, tail: '?' },
  ],
}

const TITLE = () => TITLES[getLang()]

export function Welcome() {
  const navigate = useNavigate()
  const [draft, setDraft] = useState('')
  const composer = useRef<ComposerHandle>(null)
  useEffect(() => { document.title = 'Trezutil' }, [])
  const start = (text: string, attachments: TurnAttachment[]) => {
    const id = conversationStore.send(undefined, text, attachments)
    if (id) navigate(`/c/${id}`)
  }
  return (
    <div className="welcome welcome-anim">
      <div className="welcome-mark"><BrandDots /></div>
      <h1 aria-label={t('Qu’est-ce que tu souhaites acheter, vendre ou faire réaliser ?')}>
        {TITLE().map((word, i) => (
          <span key={word.w} aria-hidden="true">
            <span className={`welcome-word${word.verb ? ' verb' : ''}`} style={{ '--i': i } as CSSProperties}>{word.w}</span>
            {word.tail && <span className="welcome-word" style={{ '--i': i } as CSSProperties}>{word.tail}</span>}
            {word.br ? <br /> : i < TITLE().length - 1 ? (TITLE()[i + 1]!.w === '?' ? ' ' : ' ') : null}
          </span>
        ))}
      </h1>
      <p>{t('Trezutil t’aide à trouver les bons fournisseurs, comparer les offres, négocier et concrétiser tes opérations commerciales.')}</p>
      <div className="suggestions" role="group" aria-label={t('Pour commencer')}>
        {PROMPTS.map(([icon, title, desc, start]) => (
          <button type="button" key={title} onClick={() => { setDraft(start); requestAnimationFrame(() => composer.current?.focus()) }}>
            <Icon name={icon} /><span><b>{title}</b><small>{desc}</small></span><Icon name="arrow" size={15} />
          </button>
        ))}
      </div>
      <div className="welcome-composer">
        <Composer ref={composer} value={draft} onChange={setDraft} onSubmit={start} onVoice={() => navigate('/voice')} autoFocus />
      </div>
    </div>
  )
}

export function Thread() {
  const { id = '' } = useParams()
  const { signedIn } = useSession()
  const location = useLocation()
  const navigate = useNavigate()
  const { conversations, pending, confirming, streaming } = useConversations()
  const conv = conversations.find((c) => c.id === id)
  const [draft, setDraft] = useState(() => (location.state as { draft?: string } | null)?.draft ?? '')
  const [opening, setOpening] = useState(false)
  const [opError, setOpError] = useState<ApiError | null>(null)
  const [offerOpen, setOfferOpen] = useState<{ turnId: string; offer: Offer; latest: boolean } | null>(null)
  const scroller = useRef<HTMLDivElement>(null)
  const stick = useRef(true)
  const [below, setBelow] = useState(false)
  const composer = useRef<ComposerHandle>(null)
  const busy = !!pending[id]
  const turns = conv?.turns ?? []
  const lastAssistant = [...turns].reverse().find((x) => x.role === 'assistant')
  const lastIsUser = turns[turns.length - 1]?.role === 'user'

  useEffect(() => { if (conv) document.title = `${conv.title} — Trezutil` }, [conv?.title]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const prefill = (location.state as { draft?: string } | null)?.draft
    setDraft(prefill ?? '')
    stick.current = true
    requestAnimationFrame(() => composer.current?.focus())
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  // Follows the thread while the person is at the bottom (or has just written); never pulls them away from what they read.
  useLayoutEffect(() => {
    const el = scroller.current
    if (!el) return
    if (stick.current || lastIsUser) { el.scrollTop = el.scrollHeight; setBelow(false) } else setBelow(true)
  }, [turns.length, busy, lastIsUser])

  // A thread known only from the server: its transcript is read now.
  const [transcript, setTranscript] = useState<'idle' | 'loading' | 'failed'>('idle')
  const remote = !!conv?.remote
  useEffect(() => {
    if (!remote) return
    let alive = true
    setTranscript('loading')
    void conversationStore.loadTranscript(id).then((r) => { if (alive) setTranscript(r === 'failed' ? 'failed' : 'idle') })
    return () => { alive = false }
  }, [id, remote])

  // A progressive answer grows the thread: keep following it.
  useEffect(() => {
    const el = scroller.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const inner = el.firstElementChild
    if (!inner) return
    const ro = new ResizeObserver(() => { if (stick.current) el.scrollTop = el.scrollHeight })
    ro.observe(inner)
    return () => ro.disconnect()
  }, [conv?.id])

  if (!conv) {
    return (
      <div className="chat-layout">
        <Empty icon="message" title={t('Cette conversation n’est pas sur cet appareil')} action={<Link className="primary-btn" to="/">{t('Nouvelle conversation')}</Link>}>
          {t('Les conversations sont conservées sur l’appareil où elles ont eu lieu, pour le compte qui les a tenues.')}
        </Empty>
      </div>
    )
  }

  const onScroll = () => {
    const el = scroller.current
    if (!el) return
    const near = el.scrollHeight - el.scrollTop - el.clientHeight < 120
    stick.current = near
    if (near) setBelow(false)
  }
  const send = (text: string, attachments: TurnAttachment[] = []) => {
    stick.current = true
    conversationStore.send(id, text, attachments)
    setDraft('')
  }
  const openOperation = async () => {
    setOpening(true)
    setOpError(null)
    try { await openOperationFor(conv) } catch (err) { setOpError(toApiError(err)) } finally { setOpening(false) }
  }
  const signInHref = `/login?returnTo=${encodeURIComponent(location.pathname)}`

  return (
    <div className="chat-layout">
      <div className="messages" ref={scroller} onScroll={onScroll}>
        <div className="messages-inner" aria-live="polite" aria-relevant="additions">
          {transcript === 'loading' && conv.remote && <Loading label={t('Chargement de la conversation…')} />}
          {transcript === 'failed' && (
            <div className="state-error" role="alert"><span>{t('La conversation n’a pas pu être lue sur le serveur.')}</span>
              <button type="button" className="secondary-btn small-btn" onClick={() => { setTranscript('loading'); void conversationStore.loadTranscript(id).then((r) => setTranscript(r === 'failed' ? 'failed' : 'idle')) }}>{t('Réessayer')}</button>
            </div>
          )}
          {conv.turns.length > 0 && conv.turns.every((x) => !x.reply) && conv.turns.some((x) => x.role === 'assistant') && conv.serverId && (
            <p className="restored-note">{t('Conversation reprise depuis le serveur : les réponses enregistrées avant la conservation des offres n’ont que leur texte.')}</p>
          )}
          {turns.map((turn) => turn.role === 'user'
            ? <UserMessage key={turn.id} turn={turn} conversationId={id} busy={busy} />
            : (
              <AssistantMessage
                key={turn.id} turn={turn} conversationId={id} latest={turn === lastAssistant && !busy} busy={busy}
                confirming={confirming[id] === turn.id} signedIn={signedIn} signInHref={signInHref}
                operationId={conv.operationId} openingOperation={opening} onOpenOperation={() => void openOperation()}
                onSend={(text) => send(text)}
                onCompare={(turnId, offer) => navigate(`/c/${id}/compare/${turnId}${offer ? `?offer=${offer.n}` : ''}`)}
                onOpenOffer={(turnId, offer, latest) => setOfferOpen({ turnId, offer, latest })}
              />
            ))}
          {busy && <StreamingAnswer text={streaming[id]?.text ?? ''} label={pending[id]!} />}
          {opError && <ErrorNote error={opError} onRetry={() => void openOperation()} />}
        </div>
      </div>
      {below && (
        <button type="button" className="jump-latest" onClick={() => { const el = scroller.current; if (el) { el.scrollTop = el.scrollHeight; stick.current = true; setBelow(false) } }}>
          <Icon name="arrowDown" size={14} />{t('Derniers messages')}
        </button>
      )}
      {offerOpen && (
        <OfferPanel offer={offerOpen.offer} onClose={() => setOfferOpen(null)}
          onCompare={() => navigate(`/c/${id}/compare/${offerOpen.turnId}?offer=${offerOpen.offer.n}`)}
          onAsk={offerOpen.latest && !busy ? () => { send(t('Parle-moi de l’offre {n} : {title}', { n: offerOpen.offer.n, title: offerOpen.offer.title })); setOfferOpen(null) } : undefined} />
      )}
      <Composer ref={composer} value={draft} onChange={setDraft} onSubmit={send} busy={busy} onStop={() => conversationStore.stop(id)} onVoice={() => navigate(`/voice?c=${id}`)} placeholder={t('Réponds, précise un détail ou demande une comparaison…')} />
    </div>
  )
}
