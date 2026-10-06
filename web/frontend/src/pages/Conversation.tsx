import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router'
import { AssistantTurn, MessageText, Typing, UserTurn, clock } from '@/components/chat'
import { AppShell, initials } from '@/components/shell'
import { ActionLink, Button, Heading, Icon } from '@/components/ui'
import { thinkingLabel, useChat } from '@/lib/chat'
import { useSession } from '@/lib/session'
import { useVoiceInput } from '@/lib/voice'
import { t, tx } from '@/lib/i18n'
import { useToast } from '@/lib/toast'

/*
 * Conversational commerce (Figma ConversationPage). The assistant only
 * interprets: every action runs through the backend runtime, which checks
 * identity and asks for confirmation before anything is ordered or booked.
 * The thread is shared with the floating chat (see lib/chat.ts).
 */

const topics: { icon: 'shop' | 'building' | 'food' | 'service' | 'home'; label: string; prompts: string[] }[] = [
  { icon: 'shop', label: tx('Shop'), prompts: [tx('A laptop for work under $600'), tx('A smartphone under $300')] },
  { icon: 'building', label: tx('Stay'), prompts: [tx('A hotel in Gombe this weekend'), tx('A room for 2 nights in Ngaliema')] },
  { icon: 'food', label: tx('Eat'), prompts: [tx('A good restaurant in Gombe'), tx('Grilled chicken near Limete')] },
  { icon: 'service', label: tx('Get things done'), prompts: [tx('An electrician available today'), tx('Someone to clean my apartment')] },
  { icon: 'home', label: tx('Find a home'), prompts: [tx('A two-bedroom apartment to rent'), tx('Land for sale in Mont-Ngafula')] },
]

function greeting(name: string) {
  const hour = new Date().getHours()
  const part = hour < 12 ? t('Good morning') : hour < 18 ? t('Good afternoon') : t('Good evening')
  return name ? `${part}, ${name}` : part
}

export default function ConversationPage() {
  const { user, signedIn, signOut } = useSession()
  const navigate = useNavigate()
  const location = useLocation()
  const [params, setParams] = useSearchParams()
  const { thread, pending, confirming, send, confirm, reset: resetThread } = useChat({
    signedIn,
    onNeedLogin: () => navigate(`/login?returnTo=${encodeURIComponent(location.pathname)}`),
  })
  const [draft, setDraft] = useState('')
  const [sidebar, setSidebar] = useState(false)
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const voice = useVoiceInput(setDraft)
  const notify = useToast()

  useEffect(() => { document.title = t('Ask Trezutil') }, [])
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' }) }, [thread.turns.length, pending])

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

  const reset = () => {
    resetThread()
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
    <AppShell compact>
      <div className="conversation-app">
        <aside className={`chat-sidebar ${sidebar ? 'open' : ''}`} aria-label={t('Conversation')}>
          <Button className="new-chat" onClick={reset}><Icon name="spark" />{t('New conversation')}</Button>
          <div className="sidebar-label">{t('In this conversation')}</div>
          {myRequests.length === 0 && <p className="sidebar-empty">{t('Your requests will appear here.')}</p>}
          {myRequests.map((t, index) => (
            <div className={`chat-history ${index === 0 ? 'active' : ''}`} key={t.id}><span><Icon name="search" size={16} /></span><div><strong>{t.text}</strong><small>{clock(t.at)}</small></div></div>
          ))}
          <div className="sidebar-account">
            {user
              ? <><span>{initials(user.display_name || user.phone.slice(-2))}</span><div><strong>{user.display_name || user.phone}</strong><small>{t('Signed in')}</small></div><Button className="icon-button" ariaLabel={t('Sign out')} onClick={() => { signOut(); reset() }}><Icon name="chevron" size={16} /></Button></>
              : <><span><Icon name="user" size={16} /></span><div><strong>{t('Browsing as a guest')}</strong><small><ActionLink href={`/login?returnTo=${encodeURIComponent('/conversation')}`}>{t('Sign in')}</ActionLink> {t('to order or book')}</small></div></>}
          </div>
        </aside>
        <section className="chat-main" aria-label={t('Chat with Trezutil')}>
          <div className="chat-header">
            <div><span className="chat-logo" aria-hidden="true"><Icon name="spark" /></span><div><strong>{t('Trezutil')}</strong><small><span className="status-dot" />{pending ? t('Working on it…') : t('Ready to help')}</small></div></div>
            <Button className="icon-button" ariaLabel={sidebar ? t('Hide conversation panel') : t('Show conversation panel')} onClick={() => setSidebar(!sidebar)}><Icon name="menu" /></Button>
          </div>
          <div className="chat-messages" aria-live="polite" aria-relevant="additions">
            {thread.turns.length === 0 && !pending && (
              <div className="empty-chat">
                <span className="empty-mark"><Icon name="spark" size={28} /></span>
                <Heading level={1}>{greeting(user?.display_name?.split(' ')[0] ?? '')} {t('— what do you need today?')}</Heading>
                <p>{t('Tell me in your own words. I can find products, services, places to eat and stay, and homes around you — then book or order them with you, step by step.')}</p>
                <div className="topic-grid">
                  {topics.map((topic) => (
                    <section key={topic.label} className="topic-card" aria-label={t(topic.label)}>
                      <h2><Icon name={topic.icon} size={16} />{t(topic.label)}</h2>
                      {topic.prompts.map((item) => <Button key={item} onClick={() => void send(t(item))}>{t(item)}<Icon name="arrow" size={14} /></Button>)}
                    </section>
                  ))}
                </div>
                <small className="empty-hint">{t('You can type, or tap the microphone to speak. I always ask before I order or book anything.')}</small>
              </div>
            )}
            {thread.turns.map((turn) => turn.role === 'user' ? (
              <UserTurn key={turn.id} turn={turn} onRetry={(text, key, id) => void send(text, key, id)} />
            ) : turn.reply ? (
              <AssistantTurn key={turn.id} turn={turn} latest={turn === lastAssistant} onSend={(text) => void send(text)} onConfirm={(tok, ok) => void confirm(tok, ok)} confirming={confirming} />
            ) : (
              <div key={turn.id} className="trez-response"><span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span><div className="message-content"><MessageText text={turn.text} /></div></div>
            ))}
            {pending && <Typing label={thinkingLabel([...thread.turns].reverse().find((x) => x.role === 'user')?.text ?? '')} />}
            <div ref={endRef} />
          </div>
          <form className="chat-composer" onSubmit={submit}>
            <textarea ref={inputRef} aria-label={t('Message Trezutil')} value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={onKey} placeholder={t('What are you looking for?')} maxLength={4000} rows={2} />
            <div>
              <span>
                <ActionLink className="tool-button" href="/search" ariaLabel={t('Search with filters instead')}><Icon name="filter" /></ActionLink>
                <Button className={`tool-button ${voice.listening ? 'listening' : ''}`} ariaLabel={voice.listening ? t('Stop listening') : t('Speak your message')} pressed={voice.listening} onClick={voice.supported ? voice.toggle : () => notify(t('Voice input is not available in this browser.'), 'info')}><Icon name="mic" /></Button>
                <Button className="tool-button" ariaLabel={t('Attach a file')} onClick={() => notify(t('Attachments are not supported yet.'), 'info')}><Icon name="attach" /></Button>
              </span>
              <Button className="send-button" type="submit" ariaLabel={t('Send message')} disabled={pending}><Icon name="arrow" /></Button>
            </div>
          </form>
        </section>
      </div>
    </AppShell>
  )
}
