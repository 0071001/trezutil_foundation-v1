import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { AssistantTurn, MessageText, Typing, UserTurn } from '@/components/chat'
import { ActionLink, Button, Icon } from '@/components/ui'
import { thinkingLabel, useChat } from '@/lib/chat'
import { whatsappHref } from '@/lib/config'
import { t, tx } from '@/lib/i18n'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { useVoiceInput } from '@/lib/voice'

/*
 * The floating assistant and the WhatsApp button, shown on every page except
 * the full conversation and the sign-in pages.
 *
 *  - The chat panel talks to the same assistant as /conversation and shares its
 *    thread, so a question can move between the panel and the full page.
 *  - The WhatsApp button opens a real chat with the Trezutil WhatsApp Business
 *    number (VITE_WHATSAPP_NUMBER). With no number configured it is not shown:
 *    there is no placeholder link.
 */

interface PromptRequest { prompt: string; nonce: number }

interface WidgetContext {
  /** True when a provider is mounted (pages rendered on their own fall back to links). */
  available: boolean
  open: boolean
  show: (prompt?: string) => void
  hide: () => void
  /** Returns a prompt raised by a page once, then ''. */
  takePrompt: () => string
}

const unavailable: WidgetContext = { available: false, open: false, show: () => {}, hide: () => {}, takePrompt: () => '' }
const WidgetCtx = createContext<WidgetContext>(unavailable)

export const useChatWidget = () => useContext(WidgetCtx)

export function ChatWidgetProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [request, setRequest] = useState<PromptRequest>({ prompt: '', nonce: 0 })
  const taken = useRef(0)
  const show = useCallback((prompt = '') => {
    setOpen(true)
    if (prompt) setRequest((r) => ({ prompt, nonce: r.nonce + 1 }))
  }, [])
  const hide = useCallback(() => setOpen(false), [])
  const takePrompt = useCallback(() => {
    if (!request.prompt || request.nonce === taken.current) return ''
    taken.current = request.nonce
    return request.prompt
  }, [request])
  const value = useMemo(() => ({ available: true, open, show, hide, takePrompt }), [open, show, hide, takePrompt])
  return <WidgetCtx.Provider value={value}>{children}</WidgetCtx.Provider>
}

const HIDDEN_ON = ['/conversation', '/login', '/register', '/admin', '/seller']
const STARTERS = [tx('A two-bedroom apartment to rent'), tx('A hotel in Gombe this weekend'), tx('A good restaurant in Gombe'), tx('An electrician available today')]

function ChatPanel({ hasWhatsApp, onClose }: { hasWhatsApp: boolean; onClose: () => void }) {
  const { signedIn } = useSession()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { takePrompt } = useChatWidget()
  const { thread, pending, confirming, send, confirm, reset } = useChat({
    signedIn,
    onNeedLogin: () => { onClose(); navigate(`/login?returnTo=${encodeURIComponent(pathname)}`) },
  })
  const [draft, setDraft] = useState('')
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const voice = useVoiceInput(setDraft)

  useEffect(() => { inputRef.current?.focus() }, [])
  useEffect(() => { endRef.current?.scrollIntoView?.({ behavior: 'smooth', block: 'end' }) }, [thread.turns.length, pending])
  // A question raised by a page ("Ask Trezutil about this listing") is sent once.
  useEffect(() => {
    const prompt = takePrompt()
    if (prompt) void send(prompt)
  }, [takePrompt, send])

  const submit = (e: FormEvent) => { e.preventDefault(); const text = draft; setDraft(''); void send(text) }
  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); const text = draft; setDraft(''); void send(text) }
  }
  // On a phone the panel covers the page: following a link must reveal it.
  const onBodyClick = (e: MouseEvent<HTMLDivElement>) => {
    const link = (e.target as HTMLElement).closest('a')
    if (link && window.matchMedia?.('(max-width: 760px)').matches) onClose()
  }

  const lastAssistant = [...thread.turns].reverse().find((x) => x.role === 'assistant')
  const wa = whatsappHref(t('Hello Trezutil! I need some help.'))

  return (
    <section className="chat-widget" role="dialog" aria-label={t('Chat with Trezutil')} onKeyDown={(e) => { if (e.key === 'Escape') onClose() }}>
      <header className="chat-widget-head">
        <span className="chat-logo" aria-hidden="true"><Icon name="spark" /></span>
        <div><strong>{t('Trezutil')}</strong><small><span className="status-dot" />{pending ? t('Working on it…') : t('Ready to help')}</small></div>
        <div className="chat-widget-tools">
          <ActionLink className="icon-button" href="/conversation" ariaLabel={t('Open the full conversation')}><Icon name="menu" size={18} /></ActionLink>
          <Button className="icon-button" ariaLabel={t('Close the chat')} onClick={onClose}><Icon name="close" size={18} /></Button>
        </div>
      </header>
      <div className="chat-widget-body" aria-live="polite" aria-relevant="additions" onClick={onBodyClick}>
        {thread.turns.length === 0 && !pending && (
          <div className="chat-widget-welcome">
            <span className="empty-mark"><Icon name="spark" size={24} /></span>
            <strong>{t('Hello! Tell me what you need.')}</strong>
            <p>{t('I’ll find it and take you through it, step by step.')}</p>
            <div className="chat-suggestions" role="group" aria-label={t('Try one of these')}>
              {STARTERS.map((s) => <Button key={s} className="suggestion-pill" onClick={() => void send(t(s))}>{t(s)}</Button>)}
            </div>
          </div>
        )}
        {thread.turns.map((turn) => turn.role === 'user' ? (
          <UserTurn key={turn.id} turn={turn} onRetry={(text, key, id) => void send(text, key, id)} />
        ) : turn.reply ? (
          <AssistantTurn key={turn.id} turn={turn} compact latest={turn === lastAssistant} onSend={(text) => void send(text)} onConfirm={(tok, ok) => void confirm(tok, ok)} confirming={confirming} />
        ) : (
          <div key={turn.id} className="trez-response"><span className="chat-logo" aria-hidden="true"><Icon name="spark" size={18} /></span><div className="message-content"><MessageText text={turn.text} /></div></div>
        ))}
        {pending && <Typing label={thinkingLabel([...thread.turns].reverse().find((x) => x.role === 'user')?.text ?? '')} />}
        <div ref={endRef} />
      </div>
      <form className="chat-widget-form" onSubmit={submit}>
        <textarea ref={inputRef} aria-label={t('Message Trezutil')} value={draft} onChange={(e) => setDraft(e.target.value)} onKeyDown={onKey} placeholder={t('What are you looking for?')} maxLength={4000} rows={2} />
        <div>
          <span>
            {voice.supported && <Button className={`tool-button ${voice.listening ? 'listening' : ''}`} ariaLabel={voice.listening ? t('Stop listening') : t('Speak your message')} pressed={voice.listening} onClick={voice.toggle}><Icon name="mic" /></Button>}
            {thread.turns.length > 0 && <Button className="chat-widget-new" onClick={() => { reset(); inputRef.current?.focus() }}>{t('New conversation')}</Button>}
          </span>
          <Button className="send-button" type="submit" ariaLabel={t('Send message')} disabled={pending}><Icon name="arrow" /></Button>
        </div>
      </form>
      {hasWhatsApp && wa && (
        <a className="chat-widget-wa" href={wa} target="_blank" rel="noopener noreferrer"><Icon name="whatsapp" size={16} />{t('Prefer WhatsApp? Continue the conversation there.')}</a>
      )}
    </section>
  )
}

export function ChatWidget() {
  const { pathname } = useLocation()
  const { open, show, hide } = useChatWidget()
  const { theme } = usePrefs()
  const launcher = useRef<HTMLButtonElement>(null)
  const wasOpen = useRef(false)
  // The home page is itself the place to say what you need: no second chat on top of it.
  const hidden = pathname === '/' || HIDDEN_ON.some((p) => pathname === p || pathname.startsWith(`${p}/`))
  const wa = whatsappHref(t('Hello Trezutil! I need some help.'))
  const visible = open && !hidden

  // Page styles hide the "Ask Trezutil" bar while the panel is open (and on phones, where the button replaces it).
  useEffect(() => {
    if (hidden) return
    document.body.dataset.chatWidget = 'on'
    return () => { delete document.body.dataset.chatWidget }
  }, [hidden])
  useEffect(() => {
    document.body.dataset.chat = visible ? 'open' : 'closed'
    return () => { delete document.body.dataset.chat }
  }, [visible])
  // Closing the panel returns focus to the button that opened it.
  useEffect(() => {
    if (wasOpen.current && !visible) launcher.current?.focus()
    wasOpen.current = visible
  }, [visible])

  if (hidden) return null
  return (
    <div className={`chat-root ${theme === 'dark' ? 'dark' : ''} ${wa ? 'has-wa' : ''}`}>
      {visible && <ChatPanel hasWhatsApp={Boolean(wa)} onClose={hide} />}
      <div className={`float-stack ${visible ? 'panel-open' : ''}`}>
        {!visible && (
          <button ref={launcher} type="button" className="fab fab-ai" aria-label={t('Open the chat with Trezutil')} aria-haspopup="dialog" aria-expanded={false} onClick={() => show()}>
            <Icon name="chat" size={22} /><span className="fab-label">{t('Ask Trezutil')}</span>
          </button>
        )}
        {wa && (
          <a className="fab fab-wa" href={wa} target="_blank" rel="noopener noreferrer" aria-label={t('Chat with Trezutil on WhatsApp')} title={t('Chat with Trezutil on WhatsApp')}>
            <Icon name="whatsapp" size={24} />
          </a>
        )}
      </div>
    </div>
  )
}
