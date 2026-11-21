import { http } from '@/api/client'
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
        createdAt: num(c.created_at), lastActivityAt: num(c.last_activity_at),
      }
    }).filter((c) => c.id && c.externalId)
  },

  /** GET /conversations/{id}/messages: the transcript (text only) of one of the user's conversations, oldest first. */
  async messages(conversationId: string): Promise<ServerMessage[]> {
    const raw = obj(await http.get<unknown>(`/conversations/${encodeURIComponent(conversationId)}/messages`))
    return arr(raw.items).map((v) => {
      const m = obj(v)
      return { id: str(m.id), direction: str(m.direction) === 'inbound' ? 'inbound' as const : 'outbound' as const, text: str(m.text), createdAt: num(m.created_at) }
    }).filter((m) => m.id && m.text)
  },
}

export interface ServerConversation { id: string; externalId: string; title: string; messageCount: number; createdAt: number; lastActivityAt: number }
export interface ServerMessage { id: string; direction: 'inbound' | 'outbound'; text: string; createdAt: number }
