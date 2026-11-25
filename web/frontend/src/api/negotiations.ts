import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/*
 * The negotiation engine (conversation-service, docs/architecture/NEGOTIATION_ENGINE.md). The buyer sets a mandate
 * (target, ceiling, rounds, deadline); suppliers answer in their portal; Trezutil counters within the mandate; only the
 * buyer accepts, which creates an order at the agreed price. A supplier's view never carries the buyer's prices.
 */

export type NegotiationStatus = 'soliciting' | 'negotiating' | 'awaiting_decision' | 'accepted' | 'rejected' | 'expired' | 'cancelled'
export type ParticipantStatus = 'invited' | 'responded' | 'countered' | 'declined' | 'accepted' | 'rejected' | 'expired'

export interface NegotiationOffer {
  id: string
  participant_id: string
  round: number
  author: 'supplier' | 'buyer'
  source: 'portal' | 'buyer' | 'strategy'
  unit_minor: number
  quantity: number
  delivery_days: number | null
  valid_until: number | null
  note: string
  status: 'open' | 'superseded' | 'accepted' | 'declined'
  created_at: number
}

export interface NegotiationParticipant {
  id: string
  organization_id: string
  supplier: string
  listing_id: string
  listing_title: string
  list_unit_minor: number
  status: ParticipantStatus
  round: number
  reminded: boolean
}

export interface NegotiationEvent { kind: string; actor: string; participant_id: string; data: Record<string, unknown>; created_at: number }
export interface NegotiationMessage { participant_id: string; channel: 'portal' | 'whatsapp'; body: string; state: string; last_error: string; created_at: number }

export interface Negotiation {
  id: string
  title: string
  quantity: number
  currency: string
  status: NegotiationStatus
  deadline: number
  order_id: string
  conversation_id: string
  accepted_offer_id: string
  created_at: number
  updated_at: number
  /** The buyer sees target and ceiling; a supplier sees only the number of rounds. */
  mandate: { target_unit_minor: number; max_unit_minor: number; max_rounds: number; auto_counter: boolean }
  participants: NegotiationParticipant[]
  offers: NegotiationOffer[]
  events: NegotiationEvent[]
  messages: NegotiationMessage[]
}

const nullableNum = (v: unknown): number | null => (typeof v === 'number' && Number.isFinite(v) ? v : null)

export function mapNegotiation(value: unknown): Negotiation {
  const raw = obj(value)
  const m = obj(raw.mandate)
  return {
    id: str(raw.id), title: str(raw.title), quantity: num(raw.quantity), currency: str(raw.currency),
    status: str(raw.status) as NegotiationStatus, deadline: num(raw.deadline), order_id: str(raw.order_id),
    conversation_id: str(raw.conversation_id), accepted_offer_id: str(raw.accepted_offer_id),
    created_at: num(raw.created_at), updated_at: num(raw.updated_at),
    mandate: { target_unit_minor: num(m.target_unit_minor), max_unit_minor: num(m.max_unit_minor), max_rounds: num(m.max_rounds), auto_counter: m.auto_counter === true },
    participants: arr(raw.participants).map((p) => {
      const x = obj(p)
      return {
        id: str(x.id), organization_id: str(x.organization_id), supplier: str(x.supplier), listing_id: str(x.listing_id),
        listing_title: str(x.listing_title), list_unit_minor: num(x.list_unit_minor), status: str(x.status) as ParticipantStatus,
        round: num(x.round), reminded: x.reminded === true,
      }
    }),
    offers: arr(raw.offers).map((o) => {
      const x = obj(o)
      return {
        id: str(x.id), participant_id: str(x.participant_id), round: num(x.round), author: str(x.author) as 'supplier' | 'buyer',
        source: str(x.source) as NegotiationOffer['source'], unit_minor: num(x.unit_minor), quantity: num(x.quantity),
        delivery_days: nullableNum(x.delivery_days), valid_until: nullableNum(x.valid_until), note: str(x.note),
        status: str(x.status) as NegotiationOffer['status'], created_at: num(x.created_at),
      }
    }),
    events: arr(raw.events).map((e) => {
      const x = obj(e)
      return { kind: str(x.kind), actor: str(x.actor), participant_id: str(x.participant_id), data: obj(x.data), created_at: num(x.created_at) }
    }),
    messages: arr(raw.messages).map((e) => {
      const x = obj(e)
      return { participant_id: str(x.participant_id), channel: str(x.channel) as 'portal' | 'whatsapp', body: str(x.body), state: str(x.state), last_error: str(x.last_error), created_at: num(x.created_at) }
    }),
  }
}

/** The open offer of an author for one supplier, if any. */
export function openOffer(n: Negotiation, participantId: string, author: 'supplier' | 'buyer'): NegotiationOffer | undefined {
  return [...n.offers].reverse().find((o) => o.participant_id === participantId && o.author === author && o.status === 'open')
}

export const isOpen = (s: NegotiationStatus) => s === 'soliciting' || s === 'negotiating' || s === 'awaiting_decision'

export interface StartNegotiation {
  listing_ids: string[]
  quantity: number
  target_price: number
  max_price: number
  deadline_hours: number
  max_rounds: number
  auto_counter: boolean
  conversation_id?: string
}

const list = (data: unknown) => arr(obj(data).items).map(mapNegotiation)

export const negotiationsApi = {
  async start(req: StartNegotiation): Promise<Negotiation> { return mapNegotiation(await http.post<unknown>('/negotiations', req)) },
  async list(): Promise<Negotiation[]> { return list(await http.get<unknown>('/negotiations')) },
  async get(id: string): Promise<Negotiation> { return mapNegotiation(await http.get<unknown>(`/negotiations/${encodeURIComponent(id)}`)) },
  async counter(id: string, participantId: string, unitPrice: number, note = ''): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/negotiations/${encodeURIComponent(id)}/counter`, { participant_id: participantId, unit_price: unitPrice, note }))
  },
  async accept(id: string, offerId: string): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/negotiations/${encodeURIComponent(id)}/accept`, { offer_id: offerId }))
  },
  async reject(id: string, participantId: string): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/negotiations/${encodeURIComponent(id)}/participants/${encodeURIComponent(participantId)}/reject`))
  },
  async cancel(id: string): Promise<Negotiation> { return mapNegotiation(await http.post<unknown>(`/negotiations/${encodeURIComponent(id)}/cancel`)) },
}

export interface SupplierAnswer { unit_price: number; delivery_days: number | null; valid_days: number; note: string }

export const supplierRequestsApi = {
  async list(): Promise<Negotiation[]> { return list(await http.get<unknown>('/supplier-requests')) },
  async get(participantId: string): Promise<Negotiation> { return mapNegotiation(await http.get<unknown>(`/supplier-requests/${encodeURIComponent(participantId)}`)) },
  async offer(participantId: string, answer: SupplierAnswer): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/supplier-requests/${encodeURIComponent(participantId)}/offer`, answer))
  },
  async acceptCounter(participantId: string): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/supplier-requests/${encodeURIComponent(participantId)}/accept-counter`))
  },
  async decline(participantId: string, reason: string): Promise<Negotiation> {
    return mapNegotiation(await http.post<unknown>(`/supplier-requests/${encodeURIComponent(participantId)}/decline`, { reason }))
  },
}
