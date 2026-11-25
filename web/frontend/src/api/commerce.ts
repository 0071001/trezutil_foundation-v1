import { http, query } from '@/api/client'
import { arr, num, obj, read, str } from '@/api/json'
import type { ClientAction } from '@/api/stripe'

export interface OrderItemSummary {
  listing_id: string
  title: string
  quantity: number
  unit_minor: number
}

export interface OrderSummary {
  id: string
  organization_id: string
  status: string
  total_minor: number
  currency: string
  items: OrderItemSummary[]
  /** First item title (orders are single-listing today). */
  title: string
  listing_id: string
  quantity: number
  delivery_status: string
  tracking_no: string
  payment_status: string
  created_at: number
  updated_at: number
}

export interface ReservationSummary {
  id: string
  organization_id: string
  listing_id: string
  status: string
  start_at: number
  end_at: number
  quantity: number
  party_size: number
  total_minor: number
  currency: string
  created_at: number
  updated_at: number
}

export function mapOrder(value: unknown): OrderSummary {
  const raw = obj(value)
  const total = obj(read(raw, 'total_amount', 'TotalAmount') ?? raw.total)
  const items = arr(read(raw, 'items', 'Items')).map((v) => {
    const item = obj(v)
    return {
      listing_id: str(read(item, 'listing_id', 'ListingID')),
      title: str(read(item, 'title', 'Title')),
      quantity: num(read(item, 'quantity', 'Quantity')),
      unit_minor: num(read(obj(read(item, 'unit_price', 'UnitPrice')), 'amount_minor', 'AmountMinor')),
    }
  })
  const delivery = obj(read(raw, 'delivery_reference', 'DeliveryReference'))
  const payment = obj(read(raw, 'payment_reference', 'PaymentReference'))
  return {
    id: str(read(raw, 'id', 'ID')) || str(raw.order_id),
    organization_id: str(read(raw, 'organization_id', 'OrganizationID')),
    status: str(read(raw, 'status', 'Status')),
    total_minor: num(read(total, 'amount_minor', 'AmountMinor')),
    currency: str(read(raw, 'currency', 'Currency')) || str(read(total, 'currency', 'Currency')),
    items,
    title: items[0]?.title ?? '',
    listing_id: items[0]?.listing_id ?? '',
    quantity: items[0]?.quantity ?? 0,
    delivery_status: str(read(delivery, 'status', 'Status')),
    tracking_no: str(read(delivery, 'tracking_no', 'TrackingNo')),
    payment_status: str(read(payment, 'status', 'Status')),
    created_at: num(read(raw, 'created_at', 'CreatedAt')),
    updated_at: num(read(raw, 'updated_at', 'UpdatedAt')),
  }
}

export function mapReservation(value: unknown): ReservationSummary {
  const raw = obj(value)
  const pricing = obj(read(raw, 'pricing_snapshot', 'PricingSnapshot'))
  const total = obj(read(pricing, 'total_amount', 'TotalAmount'))
  return {
    id: str(read(raw, 'id', 'ID')) || str(raw.reservation_id),
    organization_id: str(read(raw, 'organization_id', 'OrganizationID')),
    listing_id: str(read(raw, 'listing_id', 'ListingID')),
    status: str(read(raw, 'status', 'Status')),
    start_at: num(read(raw, 'start_at', 'StartAt')),
    end_at: num(read(raw, 'end_at', 'EndAt')),
    quantity: num(read(raw, 'quantity', 'Quantity')),
    party_size: num(read(raw, 'party_size', 'PartySize')),
    total_minor: num(read(total, 'amount_minor', 'AmountMinor')) || num(read(pricing, 'amount_minor', 'AmountMinor')),
    currency: str(read(total, 'currency', 'Currency')) || str(read(pricing, 'currency', 'Currency')),
    created_at: num(read(raw, 'created_at', 'CreatedAt')),
    updated_at: num(read(raw, 'updated_at', 'UpdatedAt')),
  }
}

export const ordersApi = {
  /** The idempotency key makes a retried checkout return the same order. */
  async create(listingId: string, quantity: number, idempotencyKey: string): Promise<OrderSummary> {
    // The create response is a short DTO (order_id, status, total); the
    // canonical order (organization, items) is read back for payment.
    const created = mapOrder(await http.post<unknown>('/orders', { listing_id: listingId, quantity }, { idempotencyKey }))
    return ordersApi.get(created.id)
  },
  /** One order for several items of one store (a bag). The key makes a retry return the same order. */
  async createCart(lines: { listing_id: string; quantity: number }[], idempotencyKey: string): Promise<OrderSummary> {
    const created = mapOrder(await http.post<unknown>('/orders', { items: lines }, { idempotencyKey }))
    return ordersApi.get(created.id)
  },
  async get(orderId: string): Promise<OrderSummary> {
    return mapOrder(await http.get<unknown>(`/orders/${encodeURIComponent(orderId)}`))
  },
  async list(): Promise<OrderSummary[]> {
    const data = obj(await http.get<unknown>('/orders'))
    return arr(data.items).map(mapOrder)
  },
  async cancel(orderId: string): Promise<void> {
    await http.post(`/orders/${encodeURIComponent(orderId)}/cancel`)
  },
}

export const reservationsApi = {
  async create(listingId: string, startAt: number, endAt: number, partySize: number, idempotencyKey: string): Promise<ReservationSummary> {
    const created = mapReservation(await http.post<unknown>('/reservations',
      { listing_id: listingId, start_at: startAt, end_at: endAt, quantity: 1, party_size: partySize },
      { idempotencyKey }))
    return reservationsApi.get(created.id)
  },
  async get(id: string): Promise<ReservationSummary> {
    return mapReservation(await http.get<unknown>(`/reservations/${encodeURIComponent(id)}`))
  },
  async list(): Promise<ReservationSummary[]> {
    const data = obj(await http.get<unknown>('/reservations'))
    return arr(data.items).map(mapReservation)
  },
  async cancel(id: string): Promise<void> {
    await http.post(`/reservations/${encodeURIComponent(id)}/cancel`)
  },
}

export interface PaymentStart {
  payment_id: string
  status: string
  client_action?: ClientAction
}

/** How the payer pays: a DR Congo mobile money network (SasPay pushes the request to the phone) or a card. */
export type PayNetwork = 'vodacom_cd' | 'airtel_cd' | 'orange_cd' | 'card'

/** Who pays: SasPay asks for the payer's name, email and (mobile money) phone. */
export interface Payer { phone: string; email: string; first_name: string; last_name: string }

export const paymentsApi = {
  /**
   * Starts the payment of an order. Amount and currency are the order total (server-validated, never converted). The
   * idempotency key is the attempt's: a retried request returns the same payment instead of charging twice, and a new
   * attempt (another network, or after a failure) gets a new key.
   */
  payOrder(order: OrderSummary, attempt: string, network: PayNetwork = 'card', payer?: Payer): Promise<PaymentStart> {
    const metadata: Record<string, string> = payer ? { ...payer, reference: order.id, description: `Trezutil ${order.id.slice(0, 8)}` } : {}
    if (network === 'card') metadata.return_url = `${window.location.origin}/purchases/${order.id}?payment=return`
    return http.post<PaymentStart>(`/payments${query({ organization_id: order.organization_id })}`, {
      order_id: order.id,
      amount_minor: order.total_minor,
      currency: order.currency,
      payment_method_type: network === 'card' ? 'CARD' : 'MOBILE_MONEY',
      payment_method_id: network,
      idempotency_key: `pay-${order.id}-${attempt}`,
      metadata,
    })
  },
  /** Reads the payment back; the server asks the provider, so an approval on the phone shows even without a webhook. */
  status(paymentId: string, organizationId: string): Promise<PaymentStart> {
    return http.get<PaymentStart>(`/payments/${encodeURIComponent(paymentId)}/status${query({ organization_id: organizationId })}`)
  },
}
