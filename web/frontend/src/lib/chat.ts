import { useCallback, useEffect, useState } from 'react'
import { toApiError, type ApiError } from '@/api/client'
import { conversationsApi, resetConversationChannel, type ConversationReply } from '@/api/conversations'
import { t } from '@/lib/i18n'

/*
 * One conversation, two places to see it: the full page and the floating chat
 * on every other page. Both read and write the same per-tab thread, so a
 * question started in the floating chat continues on the full page and back.
 * The assistant only interprets: every action runs through the backend runtime,
 * which checks identity and asks for confirmation before anything is ordered
 * or booked.
 */

export interface Turn {
  id: string
  role: 'user' | 'assistant'
  text: string
  at: number
  reply?: ConversationReply
  /** Set on a failed user turn so it can be retried with the same key. */
  failed?: { error: ApiError; key: string }
}

export interface Thread {
  conversationId: string
  turns: Turn[]
}

const THREAD_KEY = 'trezutil_thread'

export function loadThread(): Thread {
  try {
    const raw = sessionStorage.getItem(THREAD_KEY)
    if (raw) return JSON.parse(raw) as Thread
  } catch { /* ignore */ }
  return { conversationId: '', turns: [] }
}

export function saveThread(thread: Thread) {
  try { sessionStorage.setItem(THREAD_KEY, JSON.stringify({ ...thread, turns: thread.turns.slice(-60) })) } catch { /* ignore */ }
}

/** What the assistant is doing, phrased from what was just asked. */
export function thinkingLabel(text: string) {
  const low = text.toLowerCase()
  if (/\b(book|reserve|night|nights|available|availability)\b/.test(low)) return t('Checking availability…')
  if (/\b(buy|order|pay)\b/.test(low)) return t('Getting that ready…')
  if (/\b(yes|confirm|no|cancel)\b/.test(low)) return t('One moment…')
  return t('Looking for the best matches…')
}

interface ChatOptions {
  signedIn: boolean
  /** Confirming an order or booking needs an account: send the person to sign in. */
  onNeedLogin: () => void
}

export function useChat({ signedIn, onNeedLogin }: ChatOptions) {
  const [thread, setThread] = useState<Thread>(loadThread)
  const [pending, setPending] = useState(false)
  const [confirming, setConfirming] = useState(false)

  useEffect(() => { saveThread(thread) }, [thread])

  const send = useCallback(async (text: string, retryKey?: string, retryId?: string) => {
    const message = text.trim()
    if (!message || pending) return
    const key = retryKey ?? crypto.randomUUID()
    const turnId = retryId ?? crypto.randomUUID()
    setThread((prev) => ({
      ...prev,
      turns: retryId
        ? prev.turns.map((x) => (x.id === retryId ? { ...x, failed: undefined } : x))
        : [...prev.turns, { id: turnId, role: 'user', text: message, at: Date.now() }],
    }))
    setPending(true)
    try {
      const reply = await conversationsApi.send(message, key)
      setThread((prev) => ({
        conversationId: reply.conversation_id || prev.conversationId,
        turns: [...prev.turns, { id: crypto.randomUUID(), role: 'assistant', text: reply.text, at: Date.now(), reply }],
      }))
    } catch (err) {
      const error = toApiError(err)
      setThread((prev) => ({ ...prev, turns: prev.turns.map((x) => (x.id === turnId ? { ...x, failed: { error, key } } : x)) }))
    } finally {
      setPending(false)
    }
  }, [pending])

  const confirm = useCallback(async (token: string, accept: boolean) => {
    if (!signedIn) { onNeedLogin(); return }
    if (!token || !thread.conversationId) { void send(accept ? 'yes' : 'no'); return }
    setConfirming(true)
    try {
      const reply = accept ? await conversationsApi.confirm(thread.conversationId, token) : await conversationsApi.reject(thread.conversationId, token)
      setThread((prev) => ({ ...prev, turns: [...prev.turns, { id: crypto.randomUUID(), role: 'assistant', text: reply.text || (accept ? t('Done.') : t('Cancelled — nothing was changed.')), at: Date.now(), reply }] }))
    } catch (err) {
      const error = toApiError(err)
      setThread((prev) => ({ ...prev, turns: [...prev.turns, { id: crypto.randomUUID(), role: 'assistant', text: error.status === 400 ? t('That confirmation has expired. Ask me again and I’ll prepare it once more.') : error.message, at: Date.now() }] }))
    } finally {
      setConfirming(false)
    }
  }, [signedIn, onNeedLogin, thread.conversationId, send])

  const reset = useCallback(() => {
    resetConversationChannel()
    setThread({ conversationId: '', turns: [] })
  }, [])

  return { thread, pending, confirming, send, confirm, reset }
}
