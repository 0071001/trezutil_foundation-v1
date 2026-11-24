import { useSyncExternalStore } from 'react'
import { toApiError, type ApiError } from '@/api/client'
import { conversationsApi, type ConversationReply } from '@/api/conversations'
import { newId } from '@/ui/kit'
import { t } from '@/lib/i18n'

/*
 * The conversations. They are kept on this device, per account, and for a signed-in account also on the server
 * (GET /conversations, the transcript with its cards, rename, archive, delete), so they follow the person on every
 * device. Each thread has its own channel identity (`sender`): the server
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
  /**
   * On an assistant turn the person stopped while it was being written: the text shown is what had arrived. The
   * server still stored the whole answer; asking again with the same key reads it back (nothing runs twice).
   */
  interrupted?: { key: string; question: string }
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
  /** Conversation id → the answer being streamed: the text received so far and the current step. Never stored. */
  streaming: Record<string, { text: string; stage: string }>
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

let state: State = { owner: '', conversations: load(''), pending: {}, confirming: {}, streaming: {} }
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
  if (/compar/.test(low)) return t('Comparaison des offres…')
  if (/(réserv|reserv|dispo|nuit|book)/.test(low)) return t('Vérification des disponibilités…')
  if (/(command|achète|acheter|payer|order)/.test(low)) return t('Préparation de l’opération…')
  if (/^(oui|non|yes|no|ok|d’accord|d'accord)\b/.test(low.trim())) return t('Un instant…')
  return t('Recherche des bons fournisseurs…')
}

/** A message that only answers a confirmation must never be sent again by a "regenerate". */
const confirmationWords = /^(oui|non|yes|no|ok|confirme|confirmer|valide|valider|annule|annuler|d’accord|d'accord)\b/i

function titleFrom(text: string): string {
  const one = text.replace(/\s+/g, ' ').trim()
  return one.length > 60 ? `${one.slice(0, 57).trimEnd()}…` : one
}

/** The step the server reports, in words (shown while the answer is on its way). */
function stageLabel(stage: string, detail: string, asked: string): string {
  if (stage === 'interpreting') return t('Compréhension de ta demande…')
  if (stage === 'acting') {
    if (detail === 'SEARCH_LISTINGS') return t('Recherche des fournisseurs…')
    if (detail === 'GET_LISTING') return t('Lecture de l’offre…')
    return t('Préparation de l’opération…')
  }
  if (stage === 'writing') return t('Rédaction de la réponse…')
  return activityLabel(asked)
}

/** Conversation id → the request in flight, so the person can stop it. */
const inFlight = new Map<string, AbortController>()

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

/**
 * Asks again with the same idempotency key after the answer could not be received: the server answers from what it
 * stored, or says the first delivery is still being answered (then this waits and asks again). Nothing runs twice.
 */
async function readBack(text: string, key: string, sender: string): Promise<ConversationReply> {
  for (let attempt = 0; ; attempt++) {
    const reply = await conversationsApi.send(text, key, sender)
    if (!reply.in_progress || attempt >= 12) return reply
    await wait(2500)
  }
}

function setStreaming(id: string, change: (s: { text: string; stage: string }) => { text: string; stage: string }) {
  const current = state.streaming[id] ?? { text: '', stage: '' }
  set({ streaming: { ...state.streaming, [id]: change(current) } }, false)
}

async function deliver(id: string, turnId: string, text: string, key: string) {
  const conv = state.conversations.find((c) => c.id === id)
  if (!conv) return
  const controller = new AbortController()
  inFlight.set(id, controller)
  set({ pending: { ...state.pending, [id]: activityLabel(text) }, streaming: { ...state.streaming, [id]: { text: '', stage: '' } } }, false)
  const answered = (reply: ConversationReply) => {
    const answer: Turn = { id: newId(), role: 'assistant', text: reply.text, at: Date.now(), reply }
    update(id, (c) => ({ ...c, serverId: reply.conversation_id || c.serverId, updatedAt: Date.now(), turns: [...c.turns, answer] }))
  }
  const failed = (e: ApiError) => {
    const message = e.status === 403 ? t('Cette conversation appartient à un autre compte. Commence-en une nouvelle.') : e.isNetwork ? t('Trezutil est injoignable.') : e.message || t('Le message n’a pas pu être traité.')
    update(id, (c) => ({ ...c, turns: c.turns.map((x) => (x.id === turnId ? { ...x, failed: { message, network: e.isNetwork, key } } : x)) }))
  }
  try {
    let reply = await conversationsApi.sendStream(text, key, conv.sender, {
      onStage: (stage, detail) => {
        set({ pending: { ...state.pending, [id]: stageLabel(stage, detail, text) } }, false)
        setStreaming(id, (s) => ({ ...s, stage }))
      },
      onDelta: (piece) => setStreaming(id, (s) => ({ ...s, text: s.text + piece })),
      onReset: () => setStreaming(id, (s) => ({ ...s, text: '' })),
    }, controller.signal)
    if (reply.in_progress) reply = await readBack(text, key, conv.sender)
    answered(reply)
  } catch (err) {
    const e = toApiError(err)
    if (e.code === 'ABORTED') {
      // Stopped by the person: what had arrived stays, marked; the whole answer can be read back later.
      const partial = state.streaming[id]?.text.trim() ?? ''
      const turn: Turn = { id: newId(), role: 'assistant', text: partial || t('Réponse interrompue.'), at: Date.now(), interrupted: { key, question: text } }
      update(id, (c) => ({ ...c, updatedAt: Date.now(), turns: [...c.turns, turn] }))
    } else if (e.isNetwork || e.status === 404 || e.status === 405) {
      // The connection broke (or a server without the streamed route): ask again with the same key, which returns
      // the stored answer and never runs the action a second time.
      try { answered(await readBack(text, key, conv.sender)) } catch (again) { failed(toApiError(again)) }
    } else {
      failed(e)
    }
  } finally {
    inFlight.delete(id)
    set({ pending: without(state.pending, id), streaming: without(state.streaming, id) }, false)
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
    state = { owner, conversations: load(owner), pending: {}, confirming: {}, streaming: {} }
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
      conversationStore.send(conversationId, accept ? t('Oui') : t('Non'))
      return
    }
    set({ confirming: { ...state.confirming, [conversationId]: turnId } }, false)
    try {
      const reply = accept ? await conversationsApi.confirm(conv.serverId, token) : await conversationsApi.reject(conv.serverId, token)
      const answer: Turn = { id: newId(), role: 'assistant', text: reply.text || (accept ? t('C’est fait.') : t('Annulé : rien n’a été modifié.')), at: Date.now(), reply }
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
          { id: newId(), role: 'assistant', text: expired ? t('Cette confirmation a expiré. Redemande-moi et je la prépare à nouveau.') : e.message || t('La confirmation n’a pas pu être envoyée.'), at: Date.now() },
        ],
      }))
    } finally {
      set({ confirming: without(state.confirming, conversationId) }, false)
    }
  },

  /** Stops the answer being written in a conversation: what arrived stays, marked as interrupted. */
  stop(conversationId: string) {
    inFlight.get(conversationId)?.abort()
  },

  /** Reads back the whole answer of an interrupted turn (same key: the stored answer, nothing runs again). */
  async completeInterrupted(conversationId: string, turnId: string): Promise<'done' | 'failed'> {
    const conv = state.conversations.find((c) => c.id === conversationId)
    const turn = conv?.turns.find((x) => x.id === turnId)
    if (!conv || !turn?.interrupted) return 'failed'
    try {
      const reply = await readBack(turn.interrupted.question, turn.interrupted.key, conv.sender)
      update(conversationId, (c) => ({ ...c, turns: c.turns.map((x) => (x.id === turnId ? { ...x, text: reply.text, reply, interrupted: undefined } : x)) }))
      return 'done'
    } catch {
      return 'failed'
    }
  },

  linkOperation(conversationId: string, operationId: string) {
    update(conversationId, (c) => ({ ...c, operationId }))
  },

  /** Renames a thread here and, for a signed-in account, on the server (PATCH /conversations/{id}): every device sees it. */
  rename(conversationId: string, title: string) {
    const clean = titleFrom(title)
    if (!clean) return
    const conv = state.conversations.find((c) => c.id === conversationId)
    update(conversationId, (c) => ({ ...c, title: clean }))
    if (state.owner && conv?.serverId) void conversationsApi.update(conv.serverId, { title: clean }).catch(() => { /* kept on this device */ })
  },

  /** Archives (or restores) a thread here and on the server. */
  archive(conversationId: string, archived = true) {
    const conv = state.conversations.find((c) => c.id === conversationId)
    update(conversationId, (c) => ({ ...c, archived }))
    if (state.owner && conv?.serverId) void conversationsApi.update(conv.serverId, { archived }).catch(() => { /* kept on this device */ })
  },

  /**
   * Deletes a thread: on the server for a signed-in account (DELETE /conversations/{id}: its messages go with it;
   * orders and bookings it created stay), and from this device. If the server cannot be reached, the thread is
   * remembered as hidden so the next synchronisation does not bring it back.
   */
  remove(conversationId: string) {
    const conv = state.conversations.find((c) => c.id === conversationId)
    set({ conversations: state.conversations.filter((c) => c.id !== conversationId) })
    if (conv?.serverId) deleteOnServer([conv.serverId])
  },

  removeAll() {
    const ids = state.conversations.map((c) => c.serverId).filter(Boolean)
    set({ conversations: [] })
    deleteOnServer(ids)
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
      // The server holds the name and the archive of a conversation it knows: another device may have changed them.
      const conversations = state.conversations.map((c) => {
        const server = items.find((s) => s.externalId === c.sender || (c.serverId && s.id === c.serverId))
        if (!server) return c
        const title = server.renamed ? titleFrom(server.title) : c.title
        if (c.serverId === server.id && c.title === title && !!c.archived === server.archived) return c
        changed = true
        return { ...c, serverId: server.id, title, archived: server.archived }
      })
      for (const s of items) {
        if (bySender.has(s.externalId) || hidden.has(s.id) || state.conversations.some((c) => c.serverId === s.id)) continue
        added.push({
          id: s.id, sender: s.externalId, serverId: s.id, title: titleFrom(s.title) || t('Conversation'), turns: [],
          createdAt: s.createdAt * 1000, updatedAt: (s.lastActivityAt || s.createdAt) * 1000, remote: true, archived: s.archived,
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

  /** Reads the transcript of a thread known only from the server, with the offers and cards its answers showed. */
  async loadTranscript(conversationId: string): Promise<'loaded' | 'failed'> {
    const conv = state.conversations.find((c) => c.id === conversationId)
    if (!conv?.remote || !conv.serverId) return 'loaded'
    try {
      const messages = await conversationsApi.messages(conv.serverId)
      const turns: Turn[] = messages.map((m) => ({
        id: m.id, role: m.direction === 'inbound' ? 'user' as const : 'assistant' as const, text: m.text, at: m.createdAt * 1000,
        ...(m.reply ? { reply: m.reply } : {}),
      }))
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

/** Deletes conversations on the server (signed-in account); one that could not be deleted is hidden here instead. */
function deleteOnServer(serverIds: string[]) {
  if (!serverIds.length) return
  if (!state.owner) { hide(serverIds); return }
  for (const id of serverIds) {
    void conversationsApi.remove(id).catch((err) => {
      if (toApiError(err).status !== 404) hide([id]) // 404: already gone
    })
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
