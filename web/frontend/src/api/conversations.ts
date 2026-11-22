import { ApiError, http } from '@/api/client'
import { getLang } from '@/lib/i18n'
import { arr, num, obj, read, str } from '@/api/json'

export interface ConversationCard {
  id: string
  type: string
  title: string
  description: string
  listing_id: string
  position: number
}

export interface ConversationReply {
  conversation_id: string
  text: string
  cards: ConversationCard[]
  action_type: string
  source: string
  requires_user_input: boolean
  is_confirmation: boolean
  is_clarification: boolean
  confirmation?: { summary: string; token: string; action_type: string }
  missing_params: { name: string; prompt: string }[]
  /** Search result rows (with images) when the reply is a search. */
  result_items: Record<string, unknown>[]
  /** Short follow-ups the person is likely to want next (tappable quick replies). */
  suggestions: string[]
  /** Entities created or opened by the action (links in the thread). */
  refs: { order_id: string; reservation_id: string; listing_id: string }
  /** A message sent again whose first delivery is still being answered: ask again in a moment. */
  in_progress: boolean
}

/** What a streamed answer reports while it is produced (POST /conversations/message/stream). */
export interface StreamHandlers {
  /** A real step of the work: interpreting, acting (detail: the action type) or writing. */
  onStage?: (stage: string, detail: string) => void
  /** A piece of the answer's text, as the model writes it. */
  onDelta?: (text: string) => void
  /** The text received so far is withdrawn: the final answer replaces it. */
  onReset?: () => void
}

export function mapReply(value: unknown): ConversationReply {
  const raw = obj(value)
  const response = obj(read(raw, 'response', 'Response'))
  const metadata = obj(read(response, 'metadata', 'Metadata'))
  const confirmation = obj(read(response, 'confirmation', 'Confirmation'))
  const details = obj(read(confirmation, 'details', 'Details'))
  const resultData = obj(metadata.result_data)
  return {
    conversation_id: str(read(raw, 'conversation_id', 'ConversationID')),
    text: str(read(response, 'text', 'Text')),
    cards: arr(read(response, 'cards', 'Cards')).map((c) => {
      const card = obj(c)
      const data = obj(read(card, 'data', 'Data'))
      return {
        id: str(read(card, 'id', 'ID')),
        type: str(read(card, 'type', 'Type')),
        title: str(read(card, 'title', 'Title')),
        description: str(read(card, 'description', 'Description')),
        listing_id: str(data.listing_id) || str(read(card, 'id', 'ID')),
        position: num(data.position),
      }
    }).filter((c) => c.id),
    action_type: str(metadata.action_type),
    source: str(metadata.source),
    requires_user_input: read(raw, 'requires_user_input', 'RequiresUserInput') === true,
    is_confirmation: read(raw, 'is_confirmation', 'IsConfirmation') === true,
    is_clarification: read(raw, 'is_clarification', 'IsClarification') === true,
    confirmation: Object.keys(confirmation).length ? {
      summary: str(read(confirmation, 'summary', 'Summary')),
      token: str(details.confirmation_token),
      action_type: str(details.action_type),
    } : undefined,
    missing_params: arr(read(raw, 'missing_params', 'MissingParams')).map((p) => {
      const param = obj(p)
      return { name: str(read(param, 'name', 'Name')), prompt: str(read(param, 'description', 'Description')) }
    }),
    result_items: arr(read(resultData, 'items', 'Items')).map(obj),
    suggestions: arr(metadata.suggestions).map(String).map((x) => x.trim()).filter(Boolean).slice(0, 4),
    in_progress: metadata.in_progress === true,
    refs: {
      order_id: str(resultData.order_id) || (str(metadata.action_type) === 'CREATE_ORDER' ? str(read(resultData, 'id', 'ID')) : ''),
      reservation_id: str(resultData.reservation_id) || (str(metadata.action_type) === 'CREATE_RESERVATION' ? str(read(resultData, 'id', 'ID')) : ''),
      listing_id: str(metadata.action_type) === 'GET_LISTING' ? str(read(resultData, 'id', 'ID')) : '',
    },
  }
}

const SENDER_KEY = 'trezutil_web_sender'

/** A per-tab channel identity; the signed-in user is taken from the token. */
function senderExternalID(): string {
  try {
    let id = sessionStorage.getItem(SENDER_KEY)
    if (!id) {
      id = crypto.randomUUID()
      sessionStorage.setItem(SENDER_KEY, id)
    }
    return id
  } catch {
    return 'web-ephemeral'
  }
}

export function resetConversationChannel() {
  try { sessionStorage.removeItem(SENDER_KEY) } catch { /* ignore */ }
}

export const conversationsApi = {
  /**
   * The idempotency key makes a retried send safe (no duplicate side effects). `sender` is the channel identity of
   * the thread: the server keeps one conversation per sender, so a caller holding several threads (the conversational
   * workspace in src/redesign) passes each thread's own; without it the per-tab identity is used, as before.
   */
  async send(text: string, idempotencyKey: string, sender?: string): Promise<ConversationReply> {
    return mapReply(await http.post<unknown>('/conversations/message', {
      message_text: text,
      channel: 'web',
      sender_external_id: sender || senderExternalID(),
      message_idempotency_key: idempotencyKey,
      locale: getLang(),
    }))
  },

  /**
   * The same message as send(), answered as a stream: the real steps, then the text as it is written, then the whole
   * answer (the "done" event, identical to send()'s). Rejects like send(); a broken connection rejects with status 0,
   * and sending again with the same idempotency key returns the stored answer without running anything twice.
   */
  async sendStream(text: string, idempotencyKey: string, sender: string | undefined, handlers: StreamHandlers, signal?: AbortSignal): Promise<ConversationReply> {
    let done: ConversationReply | null = null
    let failure: { code: string; message: string } | null = null
    await http.stream('/conversations/message/stream', {
      message_text: text,
      channel: 'web',
      sender_external_id: sender || senderExternalID(),
      message_idempotency_key: idempotencyKey,
      locale: getLang(),
    }, (name, data) => {
      const d = obj(data)
      if (name === 'stage') handlers.onStage?.(str(d.stage), str(d.detail))
      else if (name === 'delta') handlers.onDelta?.(str(d.text))
      else if (name === 'reset') handlers.onReset?.()
      else if (name === 'done') done = mapReply(data)
      else if (name === 'error') failure = { code: str(d.code), message: str(d.message) }
    }, signal)
    if (done) return done
    const f = failure as { code: string; message: string } | null
    if (f) throw new ApiError(f.message || 'Le message n’a pas pu être traité.', f.code === 'FORBIDDEN' ? 403 : 500, f.code)
    throw new ApiError('La réponse a été interrompue.', 0, 'STREAM_INTERRUPTED')
  },

  async confirm(conversationId: string, token: string): Promise<ConversationReply> {
    return mapReply(await http.post<unknown>(`/conversations/${encodeURIComponent(conversationId)}/confirm`, { token }))
  },

  async reject(conversationId: string, token: string): Promise<ConversationReply> {
    return mapReply(await http.post<unknown>(`/conversations/${encodeURIComponent(conversationId)}/reject`, { token }))
  },

  /** GET /conversations: the signed-in user's own web conversations, the most recently active first. */
  async list(limit = 50): Promise<ServerConversation[]> {
    const raw = obj(await http.get<unknown>(`/conversations?channel=web&limit=${limit}`))
    return arr(raw.items).map((v) => {
      const c = obj(v)
      return {
        id: str(c.id), externalId: str(c.external_id), title: str(c.title), messageCount: num(c.message_count),
        createdAt: num(c.created_at), lastActivityAt: num(c.last_activity_at), renamed: c.renamed === true, archived: c.archived === true,
      }
    }).filter((c) => c.id && c.externalId)
  },

  /**
   * GET /conversations/{id}/messages: the transcript of one of the user's conversations, oldest first. An answer
   * comes back with the cards and results it showed (never a confirmation: it has expired by then).
   */
  async messages(conversationId: string): Promise<ServerMessage[]> {
    const raw = obj(await http.get<unknown>(`/conversations/${encodeURIComponent(conversationId)}/messages`))
    return arr(raw.items).map((v) => {
      const m = obj(v)
      const response = obj(m.response)
      const outbound = str(m.direction) !== 'inbound'
      return {
        id: str(m.id), direction: outbound ? 'outbound' as const : 'inbound' as const, text: str(m.text), createdAt: num(m.created_at),
        reply: outbound && Object.keys(response).length ? mapReply({ conversation_id: conversationId, response: { ...response, text: str(m.text) } }) : undefined,
      }
    }).filter((m) => m.id && m.text)
  },

  /** PATCH /conversations/{id}: renames (an empty title gives back the first message as name) and/or archives. */
  async update(conversationId: string, change: { title?: string; archived?: boolean }): Promise<void> {
    await http.patch<unknown>(`/conversations/${encodeURIComponent(conversationId)}`, change)
  },

  /** DELETE /conversations/{id}: deletes the conversation and its messages on the server (orders are kept). */
  async remove(conversationId: string): Promise<void> {
    await http.del<unknown>(`/conversations/${encodeURIComponent(conversationId)}`)
  },
}

export interface ServerConversation { id: string; externalId: string; title: string; messageCount: number; createdAt: number; lastActivityAt: number; renamed: boolean; archived: boolean }
export interface ServerMessage { id: string; direction: 'inbound' | 'outbound'; text: string; createdAt: number; reply?: ConversationReply }
