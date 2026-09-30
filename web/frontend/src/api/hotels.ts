import { http, query } from '@/api/client'
import { ordersApi, reservationsApi, type OrderSummary, type ReservationSummary, mapReservation } from '@/api/commerce'
import { arr, num, obj, read, str } from '@/api/json'

/*
 * Hotel Commerce Pack API. A room type is a HOTEL listing; stays are core
 * reservations; a stay is paid through a core order (POST /payments).
 * Nights are UTC calendar dates: a stay from 2026-10-10 to 2026-10-12 covers
 * the nights of the 10th and 11th.
 */

export interface RoomType {
  listing_id: string
  organization_id: string
  store_id: string
  title: string
  description: string
  status: string
  nightly_price: number
  currency: string
  max_occupancy: number
  bed_configuration: string
  amenities: string[]
  units: number
  images: string[]
}

export interface HotelProperty {
  store_id: string
  name: string
  city: string
  address: string
  room_types: RoomType[]
}

export interface RoomOffer extends RoomType {
  rooms: number
  available_rooms: number
  available: boolean
  fits_guests: boolean
  nights: number
  total_price: number
  reason: string
}

export interface StayQuote {
  check_in: string
  check_out: string
  nights: number
  guests: number
  offers: RoomOffer[]
}

export interface NightCapacity {
  date: string
  capacity: number
  reserved: number
  available: number
}

export interface NewRoomType {
  store_id: string
  seller_id: string
  category_id: string
  title: string
  description: string
  price_amount: number
  price_currency: string
  max_occupancy: number
  bed_configuration: string
  amenities: string[]
  units: number
  image_urls: string[]
}

/** "2026-10-10" → the UTC midnight starting that night (seconds). */
export function nightStart(date: string): number {
  const [y, m, d] = date.split('-').map(Number)
  return Date.UTC(y, (m || 1) - 1, d || 1) / 1000
}

/** A date input value `days` after today (local calendar). */
export function dateInput(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() + days)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function nightsBetween(checkIn: string, checkOut: string): number {
  return Math.round((nightStart(checkOut) - nightStart(checkIn)) / 86400)
}

export function mapRoomType(value: unknown): RoomType {
  const raw = obj(value)
  return {
    listing_id: str(raw.listing_id), organization_id: str(raw.organization_id), store_id: str(raw.store_id),
    title: str(raw.title), description: str(raw.description), status: str(raw.status),
    nightly_price: num(raw.nightly_price), currency: str(raw.currency) || 'USD', max_occupancy: num(raw.max_occupancy),
    bed_configuration: str(raw.bed_configuration), amenities: arr(raw.amenities).map(String), units: num(raw.units),
    images: arr(raw.images).map(String).filter(Boolean),
  }
}

function mapNights(value: unknown): NightCapacity[] {
  return arr(obj(value).nights).map((n) => {
    const raw = obj(n)
    return { date: str(raw.date), capacity: num(raw.capacity), reserved: num(raw.reserved), available: num(raw.available) }
  })
}

export const hotelsApi = {
  /** Public: a property and its published room types. */
  async property(storeId: string): Promise<HotelProperty> {
    const raw = obj(await http.get<unknown>(`/hotels/property${query({ store_id: storeId })}`))
    return { store_id: str(raw.store_id), name: str(raw.name), city: str(raw.city), address: str(raw.address), room_types: arr(raw.room_types).map(mapRoomType) }
  },

  /** Public: every room type's answer for a stay (the tightest night decides). */
  async availability(storeId: string, checkIn: string, checkOut: string, guests: number, rooms?: number): Promise<StayQuote> {
    const raw = obj(await http.get<unknown>(`/hotels/availability${query({ store_id: storeId, check_in: checkIn, check_out: checkOut, guests, rooms })}`))
    return {
      check_in: str(raw.check_in), check_out: str(raw.check_out), nights: num(raw.nights), guests: num(raw.guests),
      offers: arr(raw.offers).map((o) => {
        const r = obj(o)
        return { ...mapRoomType(o), rooms: num(r.rooms), available_rooms: num(r.available_rooms), available: r.available === true,
          fits_guests: r.fits_guests === true, nights: num(r.nights), total_price: num(r.total_price), reason: str(r.reason) }
      }),
    }
  },

  /** Books a stay: a core reservation of `rooms` rooms for the nights of the stay. */
  async bookStay(roomTypeId: string, checkIn: string, checkOut: string, rooms: number, guests: number, idempotencyKey: string): Promise<ReservationSummary> {
    const created = mapReservation(await http.post<unknown>('/reservations',
      { listing_id: roomTypeId, start_at: nightStart(checkIn), end_at: nightStart(checkOut), quantity: rooms, party_size: guests },
      { idempotencyKey }))
    return reservationsApi.get(created.id)
  },

  /** The order that pays a stay (created once per reservation), read back for payment. */
  async checkout(reservationId: string): Promise<OrderSummary> {
    const raw = obj(await http.post<unknown>(`/reservations/${encodeURIComponent(reservationId)}/checkout`))
    return ordersApi.get(str(read(raw, 'order_id', 'OrderID')))
  },

  // Seller (organization owner/admin/manager; staff read only — enforced server-side)
  async roomTypes(organizationId: string): Promise<RoomType[]> {
    return arr(obj(await http.get<unknown>(`/hotels/room-types${query({ organization_id: organizationId })}`)).items).map(mapRoomType)
  },
  async createRoomType(input: NewRoomType): Promise<RoomType> {
    return mapRoomType(await http.post<unknown>('/hotels/room-types', input))
  },
  async updateRoomType(id: string, patch: { max_occupancy?: number; bed_configuration?: string; amenities?: string[] }): Promise<RoomType> {
    return mapRoomType(await http.post<unknown>(`/hotels/room-types/${encodeURIComponent(id)}`, patch))
  },
  async setUnits(id: string, units: number): Promise<RoomType> {
    return mapRoomType(await http.post<unknown>(`/hotels/room-types/${encodeURIComponent(id)}/units`, { units }))
  },
  async setCapacity(id: string, from: string, to: string, rooms: number): Promise<NightCapacity[]> {
    return mapNights(await http.post<unknown>(`/hotels/room-types/${encodeURIComponent(id)}/capacity`, { from, to, rooms }))
  },
  async capacity(id: string, from: string, to: string): Promise<NightCapacity[]> {
    return mapNights(await http.get<unknown>(`/hotels/room-types/${encodeURIComponent(id)}/capacity${query({ from, to })}`))
  },
}
