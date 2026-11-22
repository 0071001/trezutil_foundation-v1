import { useEffect, useLayoutEffect, useRef, useState } from 'react'
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
import { Empty, ErrorNote, Loading, Logo } from '@/ui/kit'

/*
 * The conversation: where every operation starts. "/" asks the open question; "/c/:id" is a thread with the composer
 * docked at the bottom. A suggestion prepares the request in the composer, it never sends on its own.
 */

const PROMPTS: [IconName, string, string, string][] = [
  ['search', 'Trouver un fournisseur', 'Identifier des partenaires fiables', 'Je cherche un fournisseur fiable pour '],
  ['orders', 'Acheter un équipement', 'Comparer prix et conditions', 'Je souhaite acheter '],
  ['layers', 'Comparer des offres', 'Décider avec les bons critères', 'Compare pour moi les offres de '],
  ['calendar', 'Organiser un événement', 'Coordonner prestataires et budget', 'J’organise un événement : '],
]

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
    <div className="welcome">
      <div className="welcome-mark"><Logo compact /></div>
      <h1>Qu’est-ce que tu souhaites<br />acheter, vendre ou faire réaliser&nbsp;?</h1>
      <p>Trezutil t’aide à trouver les bons fournisseurs, comparer les offres, négocier et concrétiser tes opérations commerciales.</p>
      <div className="suggestions" role="group" aria-label="Pour commencer">
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
        <Empty icon="message" title="Cette conversation n’est pas sur cet appareil" action={<Link className="primary-btn" to="/">Nouvelle conversation</Link>}>
          Les conversations sont conservées sur l’appareil où elles ont eu lieu, pour le compte qui les a tenues.
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
          {transcript === 'loading' && conv.remote && <Loading label="Chargement de la conversation…" />}
          {transcript === 'failed' && (
            <div className="state-error" role="alert"><span>La conversation n’a pas pu être lue sur le serveur.</span>
              <button type="button" className="secondary-btn small-btn" onClick={() => { setTranscript('loading'); void conversationStore.loadTranscript(id).then((r) => setTranscript(r === 'failed' ? 'failed' : 'idle')) }}>Réessayer</button>
            </div>
          )}
          {conv.turns.length > 0 && conv.turns.every((t) => !t.reply) && conv.turns.some((t) => t.role === 'assistant') && conv.serverId && (
            <p className="restored-note">Conversation reprise depuis le serveur : les réponses enregistrées avant la conservation des offres n’ont que leur texte.</p>
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
          <Icon name="arrowDown" size={14} />Derniers messages
        </button>
      )}
      {offerOpen && (
        <OfferPanel offer={offerOpen.offer} onClose={() => setOfferOpen(null)}
          onCompare={() => navigate(`/c/${id}/compare/${offerOpen.turnId}?offer=${offerOpen.offer.n}`)}
          onAsk={offerOpen.latest && !busy ? () => { send(`Parle-moi de l’offre ${offerOpen.offer.n} : ${offerOpen.offer.title}`); setOfferOpen(null) } : undefined} />
      )}
      <Composer ref={composer} value={draft} onChange={setDraft} onSubmit={send} busy={busy} onStop={() => conversationStore.stop(id)} onVoice={() => navigate(`/voice?c=${id}`)} placeholder="Réponds, précise un détail ou demande une comparaison…" />
    </div>
  )
}
