import { http, query } from '@/api/client'
import { arr, num, obj, read, str } from '@/api/json'
import type { ListingDetail, ListingMedia, ListingStatus, ListingType } from '@/types'

export function mapListing(value: unknown): ListingDetail {
  const raw = obj(value)
  const description = read(raw, 'description', 'Description')
  const publishedAt = read(raw, 'published_at', 'PublishedAt')
  const media: ListingMedia[] = arr(read(raw, 'media', 'Media'))
    .map((m) => {
      const item = obj(m)
      return { url: str(read(item, 'url', 'URL')), primary: read(item, 'primary', 'Primary') === true }
    })
    .filter((m) => m.url)
  return {
    id: str(read(raw, 'id', 'ID')),
    organization_id: str(read(raw, 'organization_id', 'OrganizationID')),
    seller_id: str(read(raw, 'seller_id', 'SellerID')),
    store_id: str(read(raw, 'store_id', 'StoreID')),
    category_id: str(read(raw, 'category_id', 'CategoryID')),
    title: str(read(raw, 'title', 'Title')),
    description: typeof description === 'string' ? description : null,
    listing_type: (str(read(raw, 'listing_type', 'ListingType')) || 'other') as ListingType,
    pricing_model: str(read(raw, 'pricing_model', 'PricingModel')),
    price_amount: num(read(raw, 'price_amount', 'PriceAmount')),
    price_currency: str(read(raw, 'price_currency', 'PriceCurrency')),
    status: str(read(raw, 'status', 'Status')) as ListingStatus,
    metadata: obj(read(raw, 'metadata', 'Metadata')),
    published_at: typeof publishedAt === 'number' ? publishedAt : null,
    created_at: num(read(raw, 'created_at', 'CreatedAt')),
    updated_at: num(read(raw, 'updated_at', 'UpdatedAt')),
    media: media.sort((a, b) => Number(b.primary) - Number(a.primary)),
  }
}

export interface Availability {
  available: boolean
  remaining: number
  reason: string
}

export const listingsApi = {
  /** Published listings are public; drafts are visible to their organization only. */
  async get(id: string): Promise<ListingDetail> {
    return mapListing(await http.get<unknown>(`/listings${query({ listing_id: id })}`))
  },

  /** Public store profile (name, city) shown as "Sold by" on a listing. */
  async store(storeId: string): Promise<{ name: string; city: string; status: string }> {
    const raw = obj(await http.get<unknown>(`/stores${query({ store_id: storeId })}`))
    return { name: str(read(raw, 'name', 'Name')), city: str(read(raw, 'city', 'City')), status: str(read(raw, 'status', 'Status')) }
  },

  async availability(listingId: string, startAt: number, endAt: number, quantity = 1): Promise<Availability> {
    const raw = obj(await http.get<unknown>(`/availability${query({ listing_id: listingId, start_at: startAt, end_at: endAt, quantity })}`))
    return {
      available: read(raw, 'available', 'Available') === true,
      remaining: num(read(raw, 'remaining', 'Remaining')),
      reason: str(read(raw, 'reason', 'Reason')),
    }
  },
}
