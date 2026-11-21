import { useSyncExternalStore } from 'react'
import { toApiError } from '@/api/client'
import { conversationsApi, type ConversationReply } from '@/api/conversations'
import { newId } from '@/ui/kit'

/*
 * The conversations. The gateway has no endpoint to list or read back a conversation (see services.ts → history), so
 * the threads are kept on this device, per account. Each thread has its own channel identity (`sender`): the server
 * keeps one conversation, with its context, per sender, so an older thread continues on the server where it stopped.
 *
 * The assistant only interprets. Every action runs in the backend's Commerce Action Runtime, which checks identity,
 * charges credits and asks for an explicit confirmation before anything is ordered, booked or paid. This store never
 * decides an action: it sends the person's words and the person's answer to a confirmation, nothing else.
 */

/** A file chosen in the composer. The gateway does not accept files yet: only its description is kept. */
export interface TurnAttachment { name: string; size: number; type: string; sent: false }

export interface Turn {
  id: string
  role: 'user' | 'assistant'
  text: string
  at: number
  reply?: ConversationReply
  attachments?: TurnAttachment[]
  /** A user turn that could not be delivered: sent again with the same key, so nothing runs twice. */
  failed?: { message: string; network: boolean; key: string }
  /** On an assistant turn that asked for a confirmation: what the person answered. */
  resolution?: 'confirmed' | 'rejected' | 'expired'
  /** A user turn written again: the earlier version, for the record. */
  editedFrom?: string
}

export interface Conversation {
  id: string
  sender: string
  serverId: string
  title: string
  turns: Turn[]
  createdAt: number
  updatedAt: number
  archived?: boolean
  /** Known from the server's list only: its transcript is read when the thread is opened. */
  remote?: boolean
  /** The operation (mission) opened from this conversation. */
  operationId?: string
}

interface State {
  owner: string
  conversations: Conversation[]
  /** Conversation id → what the runtime is doing, while a message is out. */
  pending: Record<string, string>
  /** Conversation id → the turn whose confirmation is being sent. */
  confirming: Record<string, string>
  /** Answers received during this visit (shown progressively once); never stored. */
  fresh: Record<string, true>
}

const MAX_CONVERSATIONS = 80
const MAX_TURNS = 150
const storageKey = (owner: string) => `trezutil_conversations:${owner || 'guest'}`

function load(owner: string): Conversation[] {
  try {
    const raw = localStorage.getItem(storageKey(owner))
    const list = raw ? (JSON.parse(raw) as Conversation[]) : []
    return Array.isArray(list) ? list.filter((c) => c && typeof c.id === 'string' && Array.isArray(c.turns)) : []
  } catch {
    return []
  }
}

function persist(s: State) {
  try {
    const trimmed = s.conversations.slice(0, MAX_CONVERSATIONS).map((c) => ({ ...c, turns: c.turns.slice(-MAX_TURNS) }))
    localStorage.setItem(storageKey(s.owner), JSON.stringify(trimmed))
  } catch { /* storage full or unavailable: the thread still works for this visit */ }
}

let state: State = { owner: '', conversations: load(''), pending: {}, confirming: {}, fresh: {} }
const listeners = new Set<() => void>()

function set(next: Partial<State>, save = true) {
  state = { ...state, ...next }
  if (save) persist(state)
  listeners.forEach((l) => l())
}

function update(id: string, change: (c: Conversation) => Conversation) {
  set({ conversations: state.conversations.map((c) => (c.id === id ? change(c) : c)) })
}

function without<T>(record: Record<string, T>, id: string): Record<string, T> {
  const next = { ...record }
  delete next[id]
  return next
}

/** What the runtime is doing, phrased from what was asked (shown while the answer is on its way). */
export function activityLabel(text: string): string {
  const low = text.toLowerCase()
  if (/compar/.test(low)) return 'Comparaison des offres…'
  if (/(réserv|reserv|dispo|nuit|book)/.test(low)) return 'Vérification des disponibilités…'
  if (/(command|achète|acheter|payer|order)/.test(low)) return 'Préparation de l’opération…'
  if (/^(oui|non|yes|no|ok|d’accord|d'accord)\b/.test(low.trim())) return 'Un instant…'
  return 'Recherche des bons fournisseurs…'
}

/** A message that only answers a confirmation must never be sent again by a "regenerate". */
const confirmationWords = /^(oui|non|yes|no|ok|confirme|confirmer|valide|valider|annule|annuler|d’accord|d'accord)\b/i

function titleFrom(text: string): string {
  const one = text.replace(/\s+/g, ' ').trim()
  return one.length > 60 ? `${one.slice(0, 57).trimEnd()}…` : one
}

async function deliver(id: string, turnId: string, text: string, key: string) {
  const conv = state.conversations.find((c) => c.id === id)
  if (!conv) return
  set({ pending: { ...state.pending, [id]: activityLabel(text) } }, false)
  try {
    const reply = await conversationsApi.send(text, key, conv.sender)
    const answer: Turn = { id: newId(), role: 'assistant', text: reply.text, at: Date.now(), reply }
    state = { ...state, fresh: { ...state.fresh, [answer.id]: true } }
    update(id, (c) => ({ ...c, serverId: reply.conversation_id || c.serverId, updatedAt: Date.now(), turns: [...c.turns, answer] }))
  } catch (err) {
    const e = toApiError(err)
    const message = e.status === 403 ? 'Cette conversation appartient à un autre compte. Commence-en une nouvelle.' : e.isNetwork ? 'Trezutil est injoignable.' : e.message || 'Le message n’a pas pu être traité.'
    update(id, (c) => ({ ...c, turns: c.turns.map((x) => (x.id === turnId ? { ...x, failed: { message, network: e.isNetwork, key } } : x)) }))
  } finally {
    set({ pending: without(state.pending, id) }, false)
  }
}

function moveToTop(id: string) {
  const moved = state.conversations.find((c) => c.id === id)
  if (moved) set({ conversations: [moved, ...state.conversations.filter((c) => c.id !== id)] })
}

export const conversationStore = {
  subscribe(listener: () => void) {
    listeners.add(listener)
    return () => { listeners.delete(listener) }
  },
  get: () => state,

  /** Threads belong to an account: signing in or out shows that account's threads only. */
  setOwner(owner: string) {
    if (owner === state.owner) return
    state = { owner, conversations: load(owner), pending: {}, confirming: {}, fresh: {} }
    listeners.forEach((l) => l())
  },

  /** Reads the threads again (another tab changed them), keeping what is in flight here. */
  reload() {
    state = { ...state, conversations: load(state.owner) }
    listeners.forEach((l) => l())
  },

  find: (id: string | undefined) => (id ? state.conversations.find((c) => c.id === id) : undefined),
  byOperation: (operationId: string) => state.conversations.find((c) => c.operationId === operationId),

  /** Sends a message; without a conversation id a new thread starts. Returns the thread id. */
  send(conversationId: string | undefined, text: string, attachments: TurnAttachment[] = []): string | null {
    const message = text.trim().slice(0, 4000)
    if (!message) return null
    let id = conversationId && state.conversations.some((c) => c.id === conversationId) ? conversationId : ''
    if (id && state.pending[id]) return id
    const now = Date.now()
    const turn: Turn = { id: newId(), role: 'user', text: message, at: now, ...(attachments.length ? { attachments } : {}) }
    if (!id) {
      id = newId()
      const conv: Conversation = { id, sender: `web-${newId()}`, serverId: '', title: titleFrom(message), turns: [turn], createdAt: now, updatedAt: now }
      set({ conversations: [conv, ...state.conversations] })
    } else {
      update(id, (c) => ({ ...c, updatedAt: now, archived: false, turns: [...c.turns, turn] }))
      moveToTop(id)
    }
    void deliver(id, turn.id, message, newId())
    return id
  },

  /** Sends a failed message again with the key of the first attempt: the server runs it at most once. */
  retry(conversationId: string, turnId: string) {
    const turn = state.conversations.find((c) => c.id === conversationId)?.turns.find((x) => x.id === turnId)
    if (!turn?.failed || state.pending[conversationId]) return
    const { key } = turn.failed
    update(conversationId, (c) => ({ ...c, turns: c.turns.map((x) => (x.id === turnId ? { ...x, failed: undefined } : x)) }))
    void deliver(conversationId, turnId, turn.text, key)
  },

  /**
   * Writes a message again. The turns after it leave the thread. The server keeps one linear context per thread and
   * cannot forget an exchange, so editing the first message restarts the thread on a fresh server conversation; a
   * later edit continues the same one (the server still remembers what was said before).
   */
  edit(conversationId: string, turnId: string, text: string) {
    const conv = state.conversations.find((c) => c.id === conversationId)
    const index = conv?.turns.findIndex((x) => x.id === turnId) ?? -1
    const message = text.trim().slice(0, 4000)
    if (!conv || index < 0 || !message || state.pending[conversationId]) return
    const original = conv.turns[index]!
    if (original.role !== 'user' || message === original.text) return
    const first = conv.turns.findIndex((x) => x.role === 'user') === index
    const turn: Turn = { id: newId(), role: 'user', text: message, at: Date.now(), editedFrom: original.text }
    update(conversationId, (c) => ({
      ...c,
      ...(first ? { sender: `web-${newId()}`, serverId: '', title: titleFrom(message) } : {}),
      updatedAt: Date.now(),
      turns: [...c.turns.slice(0, index), turn],
    }))
    void deliver(conversationId, turn.id, message, newId())
  },

  /** Whether an answer can be asked again: never a confirmation, never an answer to "yes" or "no". */
  canRegenerate(conversationId: string, assistantTurnId: string): boolean {
    const conv = state.conversations.find((c) => c.id === conversationId)
    if (!conv) return false
    const index = conv.turns.findIndex((x) => x.id === assistantTurnId)
    const turn = conv.turns[index]
    const asked = [...conv.turns.slice(0, index)].reverse().find((x) => x.role === 'user')
    if (!turn || !asked || turn.reply?.is_confirmation || turn.reply?.refs.order_id || turn.reply?.refs.reservation_id) return false
    return !confirmationWords.test(asked.text.trim()) && index === conv.turns.length - 1
  },

  /** Asks the last question again (a new request: a search runs again). */
  regenerate(conversationId: string, assistantTurnId: string) {
    if (!conversationStore.canRegenerate(conversationId, assistantTurnId) || state.pending[conversationId]) return
    const conv = state.conversations.find((c) => c.id === conversationId)!
    const index = conv.turns.findIndex((x) => x.id === assistantTurnId)
    const asked = [...conv.turns.slice(0, index)].reverse().find((x) => x.role === 'user')!
    update(conversationId, (c) => ({ ...c, turns: c.turns.filter((x) => x.id !== assistantTurnId) }))
    void deliver(conversationId, asked.id, asked.text, newId())
  },

  /**
   * The person's answer to a confirmation the runtime asked for. Only this explicit answer confirms an order, a
   * booking or a payment; the server checks the token and the account.
   */
  async answer(conversationId: string, turnId: string, accept: boolean) {
    const conv = state.conversations.find((c) => c.id === conversationId)
    const turn = conv?.turns.find((x) => x.id === turnId)
    const token = turn?.reply?.confirmation?.token
    if (!conv || !turn || state.confirming[conversationId]) return
    if (!token || !conv.serverId) {
      update(conversationId, (c) => ({ ...c, turns: c.turns.map((x) => (x.id === turnId ? { ...x, resolution: accept ? 'confirmed' : 'rejected' } : x)) }))
      conversationStore.send(conversationId, accept ? 'Oui' : 'Non')
      return
    }
    set({ confirming: { ...state.confirming, [conversationId]: turnId } }, false)
    try {
      const reply = accept ? await conversationsApi.confirm(conv.serverId, token) : await conversationsApi.reject(conv.serverId, token)
      const answer: Turn = { id: newId(), role: 'assistant', text: reply.text || (accept ? 'C’est fait.' : 'Annulé : rien n’a été modifié.'), at: Date.now(), reply }
      state = { ...state, fresh: { ...state.fresh, [answer.id]: true } }
      update(conversationId, (c) => ({
        ...c,
        updatedAt: Date.now(),
        turns: [...c.turns.map((x) => (x.id === turnId ? { ...x, resolution: accept ? 'confirmed' as const : 'rejected' as const } : x)), answer],
      }))
    } catch (err) {
      const e = toApiError(err)
      const expired = e.status === 400
      update(conversationId, (c) => ({
        ...c,
        turns: [
          ...c.turns.map((x) => (x.id === turnId && expired ? { ...x, resolution: 'expired' as const } : x)),
          { id: newId(), role: 'assistant', text: expired ? 'Cette confirmation a expiré. Redemande-moi et je la prépare à nouveau.' : e.message || 'La confirmation n’a pas pu être envoyée.', at: Date.now() },
        ],
      }))
    } finally {
      set({ confirming: without(state.confirming, conversationId) }, false)
    }
  },

  /** An answer has been shown progressively once: it is shown whole from now on. */
  seen(turnId: string) {
    if (state.fresh[turnId]) set({ fresh: without(state.fresh, turnId) }, false)
  },

  linkOperation(conversationId: string, operationId: string) {
    update(conversationId, (c) => ({ ...c, operationId }))
  },

  rename(conversationId: string, title: string) {
    const clean = titleFrom(title)
    if (clean) update(conversationId, (c) => ({ ...c, title: clean }))
  },

  archive(conversationId: string, archived = true) {
    update(conversationId, (c) => ({ ...c, archived }))
  },

  /**
   * Removes a thread from this device. The server keeps its conversations (there is no delete endpoint): a thread it
   * knows is remembered as hidden, so the next synchronisation does not bring it back.
   */
  remove(conversationId: string) {
    const conv = state.conversations.find((c) => c.id === conversationId)
    if (conv?.serverId) hide([conv.serverId])
    set({ conversations: state.conversations.filter((c) => c.id !== conversationId) })
  },

  removeAll() {
    hide(state.conversations.map((c) => c.serverId).filter(Boolean))
    set({ conversations: [] })
  },

  /**
   * Brings in the account's conversations the server knows (GET /conversations): a thread already on this device is
   * matched by its channel identity; one held elsewhere (another device, the earlier site) is listed, and its
   * transcript is read when it is opened. Local state (titles, archive, operation links) is never overwritten.
   */
  async syncFromServer(): Promise<'synced' | 'failed'> {
    const owner = state.owner
    if (!owner) return 'failed'
    try {
      const items = await conversationsApi.list(80)
      if (state.owner !== owner) return 'failed'
      const hidden = loadHidden(owner)
      const bySender = new Map(state.conversations.map((c) => [c.sender, c]))
      const added: Conversation[] = []
      let changed = false
      const conversations = state.conversations.map((c) => {
        const server = items.find((s) => s.externalId === c.sender)
        if (server && c.serverId !== server.id) { changed = true; return { ...c, serverId: server.id } }
        return c
      })
      for (const s of items) {
        if (bySender.has(s.externalId) || hidden.has(s.id) || state.conversations.some((c) => c.serverId === s.id)) continue
        added.push({
          id: s.id, sender: s.externalId, serverId: s.id, title: titleFrom(s.title) || 'Conversation', turns: [],
          createdAt: s.createdAt * 1000, updatedAt: (s.lastActivityAt || s.createdAt) * 1000, remote: true,
        })
      }
      if (changed || added.length) {
        const all = [...conversations, ...added].sort((a, b) => b.updatedAt - a.updatedAt)
        set({ conversations: all })
      }
      return 'synced'
    } catch {
      return 'failed'
    }
  },

  /** Reads the transcript of a thread known only from the server (text only: the server keeps no offers or cards). */
  async loadTranscript(conversationId: string): Promise<'loaded' | 'failed'> {
    const conv = state.conversations.find((c) => c.id === conversationId)
    if (!conv?.remote || !conv.serverId) return 'loaded'
    try {
      const messages = await conversationsApi.messages(conv.serverId)
      const turns: Turn[] = messages.map((m) => ({ id: m.id, role: m.direction === 'inbound' ? 'user' as const : 'assistant' as const, text: m.text, at: m.createdAt * 1000 }))
      update(conversationId, (c) => ({ ...c, remote: false, turns: turns.length ? turns : c.turns }))
      return 'loaded'
    } catch {
      return 'failed'
    }
  },
}

const hiddenKey = (owner: string) => `trezutil_conversations_hidden:${owner || 'guest'}`

function loadHidden(owner: string): Set<string> {
  try {
    const raw = localStorage.getItem(hiddenKey(owner))
    const list = raw ? (JSON.parse(raw) as string[]) : []
    return new Set(Array.isArray(list) ? list : [])
  } catch {
    return new Set()
  }
}

function hide(serverIds: string[]) {
  if (!serverIds.length) return
  const hidden = loadHidden(state.owner)
  serverIds.forEach((id) => hidden.add(id))
  try { localStorage.setItem(hiddenKey(state.owner), JSON.stringify([...hidden].slice(-500))) } catch { /* this visit only */ }
}

export function useConversations() {
  return useSyncExternalStore(conversationStore.subscribe, conversationStore.get, conversationStore.get)
}

/** The confirmation a thread is still waiting for (the latest unanswered one), if any. */
export function openConfirmation(conv: Conversation | undefined): Turn | undefined {
  if (!conv) return undefined
  const last = [...conv.turns].reverse().find((x) => x.role === 'assistant')
  return last?.reply?.is_confirmation && last.reply.confirmation && !last.resolution ? last : undefined
}
