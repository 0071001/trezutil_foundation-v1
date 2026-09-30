import { useCallback, useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router'
import { toApiError, type ApiError } from '@/api/client'
import { conversationsApi, resetConversationChannel, type ConversationReply } from '@/api/conversations'
import { mapSearchResult } from '@/api/search'
import { ResultCard } from '@/components/cards'
import { AppShell, initials } from '@/components/shell'
import { ActionLink, Button, Heading, Icon } from '@/components/ui'
import { detailPath } from '@/lib/format'
import { useSession } from '@/lib/session'
import { useVoiceInput } from '@/lib/voice'

/*
 * Conversational commerce (Figma ConversationPage). The assistant only
 * interprets: every action runs through the backend runtime, which checks
 * identity and asks for confirmation before anything is ordered or booked.
 */

interface Turn {
  id: string
  role: 'user' | 'assistant'
  text: string
  at: number
  reply?: ConversationReply
  /** Set on a failed user turn so it can be retried with the same key. */
  failed?: { error: ApiError; key: string }
}

interface Thread {
  conversationId: string
  turns: Turn[]
}

const THREAD_KEY = 'trezutil_thread'
const starters = ['A laptop for work under $600', 'A hotel in Gombe this weekend', 'An electrician available today', 'A two-bedroom apartment nearby']

function loadThread(): Thread {
  try {
    const raw = sessionStorage.getItem(THREAD_KEY)
    if (raw) return JSON.parse(raw) as Thread
  } catch { /* ignore */ }
  return { conversationId: '', turns: [] }
}

function saveThread(thread: Thread) {
  try { sessionStorage.setItem(THREAD_KEY, JSON.stringify({ ...thread, turns: thread.turns.slice(-60) })) } catch { /* ignore */ }
}

function clock(ts: number) {
  return new Date(ts).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}

/** Plain-text message body; newlines kept, nothing rendered as HTML. */
function MessageText({ text }: { text: string }) {
  return <>{text.split('\n').map((line, i) => <p key={i}>{line}</p>)}</>
}

function AssistantTurn({ turn, onSend, onConfirm, confirming, latest }: { turn: Turn; onSend: (text: string) => void; onConfirm: (token: string, accept: boolean) => void; confirming: boolean; latest: boolean }) {
  const reply = turn.reply!
  const results = reply.result_items.map(mapSearchResult).filter((r) => r.listing_id)
  const listingId = reply.refs.listing_id
  return (
    <div className="trez-response">
      <span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span>
      <div className="message-content">
        <MessageText text={reply.text} />
        {results.length > 0 && (
          <div className="chat-results">
            {results.slice(0, 6).map((item, i) => (
              <div className="chat-result" key={item.listing_id}>
                <span className="chat-result-index" aria-hidden="true">{i + 1}</span>
                <ResultCard item={item} layout="list" askHref={`/conversation?q=${encodeURIComponent(`Tell me about number ${i + 1}`)}`} />
              </div>
            ))}
          </div>
        )}
        {results.length === 0 && reply.cards.length > 0 && (
          <div className="chat-card-list">
            {reply.cards.map((card) => (
              <Button key={card.id} className="chat-card" onClick={() => onSend(String(card.position || card.title))}>
                <strong>{card.position ? `${card.position}. ` : ''}{card.title}</strong>{card.description && <small>{card.description}</small>}
              </Button>
            ))}
          </div>
        )}
        {results.length > 1 && latest && (
          <div className="chat-followup"><strong>Want me to narrow these down?</strong><div>
            <Button className="filter-pill" onClick={() => onSend('Show me the second one')}>Open the second one</Button>
            <Button className="filter-pill" onClick={() => onSend('Only the cheapest ones')}>Lowest price</Button>
            <Button className="filter-pill" onClick={() => onSend('Which are available today?')}>Available today</Button>
          </div></div>
        )}
        {listingId && latest && (
          <div className="chat-followup"><strong>What next?</strong><div>
            <Button className="filter-pill" onClick={() => onSend('Book it')}>Book it</Button>
            <Button className="filter-pill" onClick={() => onSend('Buy it')}>Buy it</Button>
            <ActionLink className="filter-pill" href={detailPath(listingId)}>View listing</ActionLink>
          </div></div>
        )}
        {reply.is_clarification && reply.missing_params.length > 0 && (
          <div className="message-notice question"><span><Icon name="spark" /></span><div><strong>One quick question</strong>
            {reply.missing_params.map((p) => <p key={p.name}>{p.prompt || `Please tell me the ${p.name.replace(/_/g, ' ')}.`}</p>)}
          </div></div>
        )}
        {reply.is_confirmation && reply.confirmation && latest && (
          <div className="message-notice confirm" role="group" aria-label="Confirmation required">
            <span><Icon name="orders" /></span>
            <div>
              <strong>Please confirm</strong>
              <p>{reply.confirmation.summary || 'Confirm to continue. Nothing happens until you do.'}</p>
              <div className="notice-actions">
                <Button className="primary-button" busy={confirming} onClick={() => onConfirm(reply.confirmation!.token, true)}>Confirm <Icon name="check" size={15} /></Button>
                <Button className="outline-button" disabled={confirming} onClick={() => onConfirm(reply.confirmation!.token, false)}>Cancel</Button>
              </div>
            </div>
          </div>
        )}
        {reply.refs.order_id && <div className="message-notice success"><span><Icon name="orders" /></span><div><strong>Order created</strong><p>Review and pay for it whenever you’re ready.</p><ActionLink className="button" href={`/orders/${encodeURIComponent(reply.refs.order_id)}`}>View order <Icon name="arrow" size={15} /></ActionLink></div></div>}
        {reply.refs.reservation_id && reply.action_type === 'CREATE_RESERVATION' && <div className="message-notice success"><span><Icon name="calendar" /></span><div><strong>Reservation confirmed</strong><p>Your booking is saved to your reservations.</p><ActionLink className="button" href={`/reservations/${encodeURIComponent(reply.refs.reservation_id)}`}>View booking <Icon name="arrow" size={15} /></ActionLink></div></div>}
      </div>
    </div>
  )
}

export default function ConversationPage() {
  const { user, signedIn, signOut } = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const [params, setParams] = useSearchParams()
  const [thread, setThread] = useState<Thread>(loadThread)
  const [draft, setDraft] = useState('')
  const [pending, setPending] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [sidebar, setSidebar] = useState(false)
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const voice = useVoiceInput(setDraft)

  useEffect(() => { document.title = 'Ask Trezutil' }, [])
  useEffect(() => { saveThread(thread) }, [thread])
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }) }, [thread.turns.length, pending])

  const send = useCallback(async (text: string, retryKey?: string, retryId?: string) => {
    const message = text.trim()
    if (!message || pending) return
    const key = retryKey ?? crypto.randomUUID()
    const turnId = retryId ?? crypto.randomUUID()
    setThread((t) => ({
      ...t,
      turns: retryId
        ? t.turns.map((x) => (x.id === retryId ? { ...x, failed: undefined } : x))
        : [...t.turns, { id: turnId, role: 'user', text: message, at: Date.now() }],
    }))
    setPending(true)
    try {
      const reply = await conversationsApi.send(message, key)
      setThread((t) => ({
        conversationId: reply.conversation_id || t.conversationId,
        turns: [...t.turns, { id: crypto.randomUUID(), role: 'assistant', text: reply.text, at: Date.now(), reply }],
      }))
    } catch (err) {
      const error = toApiError(err)
      setThread((t) => ({ ...t, turns: t.turns.map((x) => (x.id === turnId ? { ...x, failed: { error, key } } : x)) }))
    } finally {
      setPending(false)
    }
  }, [pending])

  // A request typed elsewhere (home composer, "Ask" buttons) arrives as ?q=.
  const handled = useRef(false)
  useEffect(() => {
    const q = params.get('q')
    if (q && !handled.current) {
      handled.current = true
      setParams({}, { replace: true })
      void send(q)
    }
  }, [params, setParams, send])

  const confirm = async (token: string, accept: boolean) => {
    if (!signedIn) {
      navigate(`/login?returnTo=${encodeURIComponent(location.pathname)}`)
      return
    }
    if (!token || !thread.conversationId) { void send(accept ? 'yes' : 'no'); return }
    setConfirming(true)
    try {
      const reply = accept ? await conversationsApi.confirm(thread.conversationId, token) : await conversationsApi.reject(thread.conversationId, token)
      setThread((t) => ({ ...t, turns: [...t.turns, { id: crypto.randomUUID(), role: 'assistant', text: reply.text || (accept ? 'Done.' : 'Cancelled — nothing was changed.'), at: Date.now(), reply }] }))
    } catch (err) {
      const error = toApiError(err)
      setThread((t) => ({ ...t, turns: [...t.turns, { id: crypto.randomUUID(), role: 'assistant', text: error.status === 400 ? 'That confirmation has expired. Ask me again and I’ll prepare it once more.' : error.message, at: Date.now() }] }))
    } finally {
      setConfirming(false)
    }
  }

  const reset = () => {
    resetConversationChannel()
    setThread({ conversationId: '', turns: [] })
    setSidebar(false)
    inputRef.current?.focus()
  }

  const submit = (e: FormEvent) => { e.preventDefault(); const text = draft; setDraft(''); void send(text) }
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); const text = draft; setDraft(''); void send(text) }
  }

  const lastAssistant = [...thread.turns].reverse().find((t) => t.role === 'assistant')
  const myRequests = thread.turns.filter((t) => t.role === 'user').slice(-6).reverse()

  return (
    <AppShell compact hideDock>
      <div className="conversation-app">
        <aside className={`chat-sidebar ${sidebar ? 'open' : ''}`} aria-label="Conversation">
          <Button className="new-chat" onClick={reset}><Icon name="spark" />New conversation</Button>
          <div className="sidebar-label">In this conversation</div>
          {myRequests.length === 0 && <p className="sidebar-empty">Your requests will appear here.</p>}
          {myRequests.map((t, index) => (
            <div className={`chat-history ${index === 0 ? 'active' : ''}`} key={t.id}><span><Icon name="search" size={16} /></span><div><strong>{t.text}</strong><small>{clock(t.at)}</small></div></div>
          ))}
          <div className="sidebar-account">
            {user
              ? <><span>{initials(user.display_name || user.phone.slice(-2))}</span><div><strong>{user.display_name || user.phone}</strong><small>Signed in</small></div><Button className="icon-button" ariaLabel="Sign out" onClick={() => { signOut(); reset() }}><Icon name="chevron" size={16} /></Button></>
              : <><span><Icon name="user" size={16} /></span><div><strong>Browsing as a guest</strong><small><ActionLink href={`/login?returnTo=${encodeURIComponent('/conversation')}`}>Sign in</ActionLink> to order or book</small></div></>}
          </div>
        </aside>
        <section className="chat-main" aria-label="Chat with Trezutil">
          <div className="chat-header">
            <div><span className="chat-logo" aria-hidden="true"><Icon name="spark" /></span><div><strong>Trezutil</strong><small><span className="status-dot" />{pending ? 'Working on it…' : 'Ready to help'}</small></div></div>
            <Button className="icon-button" ariaLabel={sidebar ? 'Hide conversation panel' : 'Show conversation panel'} onClick={() => setSidebar(!sidebar)}><Icon name="menu" /></Button>
          </div>
          <div className="chat-messages" aria-live="polite" aria-relevant="additions">
            {thread.turns.length === 0 && !pending && (
              <div className="empty-chat">
                <span className="empty-mark"><Icon name="spark" size={28} /></span>
                <Heading level={1}>What do you need today?</Heading>
                <p>Tell me naturally. I can find products, services, places and bookings around you — then order or book them with you.</p>
                <div>{starters.map((item) => <Button key={item} onClick={() => void send(item)}><Icon name="arrow" size={15} />{item}</Button>)}</div>
              </div>
            )}
            {thread.turns.map((turn) => turn.role === 'user' ? (
              <div key={turn.id} className="user-turn">
                <div className="user-bubble">{turn.text}<span>{clock(turn.at)}</span></div>
                {turn.failed && (
                  <div className="message-notice error" role="alert"><span><Icon name="bell" /></span><div><strong>{turn.failed.error.isNetwork ? 'I couldn’t reach Trezutil.' : 'I couldn’t process that.'}</strong><p>{turn.failed.error.message}</p><Button onClick={() => void send(turn.text, turn.failed!.key, turn.id)}>Try again <Icon name="arrow" size={15} /></Button></div></div>
                )}
              </div>
            ) : turn.reply ? (
              <AssistantTurn key={turn.id} turn={turn} latest={turn === lastAssistant} onSend={(t) => void send(t)} onConfirm={(tok, ok) => void confirm(tok, ok)} confirming={confirming} />
            ) : (
              <div key={turn.id} className="trez-response"><span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span><div className="message-content"><MessageText text={turn.text} /></div></div>
            ))}
            {pending && <div className="trez-response"><span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span><div className="message-content"><div className="thinking" role="status"><i /><i /><i /><span>Looking into it…</span></div></div></div>}
            <div ref={endRef} />
          </div>
          <form className="chat-composer" onSubmit={submit}>
            <textarea ref={inputRef} aria-label="Message Trezutil" value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={onKey} placeholder="What are you looking for?" maxLength={4000} rows={2} />
            <div>
              <span>
                <ActionLink className="tool-button" href="/search" ariaLabel="Search with filters instead"><Icon name="filter" /></ActionLink>
                {voice.supported && <Button className={`tool-button ${voice.listening ? 'listening' : ''}`} ariaLabel={voice.listening ? 'Stop listening' : 'Speak your message'} onClick={voice.toggle}><Icon name="mic" /></Button>}
              </span>
              <Button className="send-button" type="submit" ariaLabel="Send message" disabled={pending}><Icon name="arrow" /></Button>
            </div>
          </form>
        </section>
      </div>
    </AppShell>
  )
}
