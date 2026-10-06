import { http, query } from '@/api/client'
import { arr, num, obj, read, str } from '@/api/json'
import { mapListing } from '@/api/listings'
import { mapOrder, mapReservation, type OrderSummary, type ReservationSummary } from '@/api/commerce'
import type { ListingDetail } from '@/types'

/**
 * Seller workspace API. Every call is authorized server-side against the
 * caller's organization membership and role; the organization ID sent here
 * only selects which of the caller's own organizations to show.
 */

/** The steps of an order after payment: confirm (paid), process (confirmed), ready, fulfil (delivered), complete. */
export type OrderStep = 'confirm' | 'process' | 'ready' | 'fulfil' | 'complete'

export interface SellerProfile {
  id: string
  display_name: string
  status: string
  verification_status: string
  mine: boolean
}

export interface SellerOrganization {
  organization_id: string
  display_name: string
  status: string
  verification_status: string
  role: string
  seller_profiles: SellerProfile[]
  stores: { id: string; name: string; status: string }[]
}

export interface InventoryItem {
  id: string
  listing_id: string
  total: number
  reserved: number
  consumed: number
  available: number
}

export interface NewListing {
  store_id: string
  seller_id: string
  category_id: string
  title: string
  description: string
  listing_type: string
  /** Optional: when absent the API derives the vertical from the listing type. */
  vertical?: string
  pricing_model: string
  price_amount: number
  price_currency: string
  image_urls: string[]
  availability: 'available' | 'on_request'
}

export interface ListingPatch {
  title?: string
  description?: string
  price_amount?: number
}

/** Roles allowed to manage listings, orders and bookings (mirrors the server guards). */
export const MANAGER_ROLES = new Set(['owner', 'admin', 'manager'])

function mapOrg(value: unknown): SellerOrganization {
  const raw = obj(value)
  return {
    organization_id: str(raw.organization_id),
    display_name: str(raw.display_name),
    status: str(raw.status),
    verification_status: str(raw.verification_status),
    role: str(raw.role),
    seller_profiles: arr(raw.seller_profiles).map((p) => {
      const sp = obj(p)
      return { id: str(sp.id), display_name: str(sp.display_name), status: str(sp.status), verification_status: str(sp.verification_status), mine: sp.mine === true }
    }),
    stores: arr(raw.stores).map((s) => {
      const st = obj(s)
      return { id: str(st.id), name: str(st.name), status: str(st.status) }
    }),
  }
}

export const sellerApi = {
  async context(): Promise<SellerOrganization[]> {
    const data = obj(await http.get<unknown>('/sellers/me'))
    return arr(data.organizations).map(mapOrg)
  },

  async listings(organizationId: string): Promise<ListingDetail[]> {
    const data = obj(await http.get<unknown>(`/listings/manage${query({ organization_id: organizationId, limit: 100 })}`))
    return arr(data.items).map(mapListing)
  },

  /** Creates a draft; the server assigns the ID, the organization and the status. */
  async createListing(input: NewListing, idempotencyKey: string): Promise<ListingDetail> {
    const media = input.image_urls.map((url, i) => ({ Type: 'image', URL: url, Primary: i === 0 }))
    const availability = input.availability === 'on_request'
      ? { status: 'pre_order' }
      : { status: 'in_stock' }
    return mapListing(await http.post<unknown>('/listings', {
      listing: {
        StoreID: input.store_id,
        SellerID: input.seller_id,
        CategoryID: input.category_id,
        Title: input.title,
        Description: input.description || null,
        ListingType: input.listing_type,
        ...(input.vertical ? { Vertical: input.vertical } : {}),
        PricingModel: input.pricing_model,
        PriceAmount: input.price_amount,
        PriceCurrency: input.price_currency,
      },
      media,
      availability,
    }, { idempotencyKey }))
  },

  async updateListing(id: string, patch: ListingPatch): Promise<ListingDetail> {
    return mapListing(await http.post<unknown>(`/listings/update${query({ listing_id: id })}`, patch))
  },

  /** Puts the listing on sale: it sells at salePrice and shows the regular price it replaced. Works on a live listing. */
  async setPromotion(id: string, salePrice: number): Promise<ListingDetail> {
    return mapListing(await http.post<unknown>(`/listings/promotion${query({ listing_id: id })}`, { sale_price: salePrice }))
  },

  /** Ends the sale and restores the regular price. */
  async endPromotion(id: string): Promise<ListingDetail> {
    return mapListing(await http.post<unknown>(`/listings/promotion/end${query({ listing_id: id })}`))
  },

  async setListingState(id: string, action: 'publish' | 'pause' | 'resume'): Promise<void> {
    await http.post(`/listings/${action}${query({ listing_id: id })}`)
  },

  async incomingOrders(organizationId: string): Promise<OrderSummary[]> {
    const data = obj(await http.get<unknown>(`/orders/incoming${query({ tenant_id: organizationId, limit: 100 })}`))
    return arr(data.items).map(mapOrder)
  },

  async confirmOrder(orderId: string): Promise<void> {
    await http.post(`/orders/${encodeURIComponent(orderId)}/confirm`)
  },

  /** One seller-side step. The server checks who may take it and from which status. */
  async advanceOrder(orderId: string, step: OrderStep): Promise<void> {
    await http.post(`/orders/${encodeURIComponent(orderId)}/${step}`)
  },

  async incomingReservations(organizationId: string): Promise<ReservationSummary[]> {
    const data = obj(await http.get<unknown>(`/reservations/incoming${query({ tenant_id: organizationId, limit: 100 })}`))
    return arr(data.items).map(mapReservation)
  },

  async inventory(organizationId: string): Promise<InventoryItem[]> {
    const data = obj(await http.get<unknown>(`/inventory${query({ tenant_id: organizationId, limit: 100 })}`))
    return arr(data.data).map((value) => {
      const r = obj(value)
      const total = num(read(r, 'total', 'Total'))
      const reserved = num(read(r, 'reserved', 'Reserved'))
      const consumed = num(read(r, 'consumed', 'Consumed'))
      return {
        id: str(read(r, 'id', 'ID')),
        listing_id: str(read(r, 'listing_id', 'ListingID')),
        total, reserved, consumed,
        available: Math.max(0, total - reserved - consumed),
      }
    })
  },
}
