import { http, query, type RequestOptions } from '@/api/client'
import { arr, num, obj, read, str, strOrNull } from '@/api/json'
import { discountPercent } from '@/lib/promo'
import type { Category, CustomerFlow, ListingType, SearchRequest, SearchResponse, SearchResult, Suggestion, VerticalInfo } from '@/types'

/** A map coordinate, or null when the listing has none. */
function coord(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

/** A sale is shown only when it is a real saving, whatever the server sent. */
function sale(item: Record<string, unknown>): Pick<SearchResult, 'compare_at_amount' | 'discount_percent'> {
  const compareAt = read(item, 'compare_at_amount', 'CompareAtAmount')
  const price = num(read(item, 'price_amount', 'PriceAmount'))
  const percent = typeof compareAt === 'number' ? discountPercent(compareAt, price) : 0
  return percent > 0 ? { compare_at_amount: compareAt as number, discount_percent: percent } : { compare_at_amount: null, discount_percent: 0 }
}

export function mapSearchResult(value: unknown): SearchResult {
  const item = obj(value)
  const location = read(item, 'location', 'Location')
  const seller = read(item, 'seller', 'Seller')
  const store = read(item, 'store', 'Store')
  const distance = read(item, 'distance_meters', 'DistanceMeters')
  return {
    listing_id: str(read(item, 'listing_id', 'ListingID')),
    title: str(read(item, 'title', 'Title')),
    vertical: str(read(item, 'vertical', 'Vertical')),
    price_amount: num(read(item, 'price_amount', 'PriceAmount')),
    price_currency: str(read(item, 'price_currency', 'PriceCurrency')),
    availability_status: str(read(item, 'availability_status', 'AvailabilityStatus')),
    distance_meters: typeof distance === 'number' ? distance : null,
    location: location ? {
      city: strOrNull(read(obj(location), 'city', 'City')),
      neighborhood: strOrNull(read(obj(location), 'neighborhood', 'Neighborhood')),
      latitude: coord(read(obj(location), 'latitude', 'Latitude')),
      longitude: coord(read(obj(location), 'longitude', 'Longitude')),
    } : null,
    seller: seller ? {
      seller_id: str(read(obj(seller), 'seller_id', 'SellerID')),
      display_name: str(read(obj(seller), 'display_name', 'DisplayName')),
      trust_level: num(read(obj(seller), 'trust_level', 'TrustLevel')),
    } : null,
    store: store ? {
      store_id: str(read(obj(store), 'store_id', 'StoreID')),
      name: str(read(obj(store), 'name', 'Name')),
    } : null,
    primary_media_url: strOrNull(read(item, 'primary_media_url', 'PrimaryMediaURL')),
    ...sale(item),
    published_at: num(read(item, 'published_at', 'PublishedAt')),
  }
}

export function mapSearchResponse(value: unknown): SearchResponse {
  const raw = obj(value)
  return {
    items: arr(read(raw, 'items', 'Items')).map(mapSearchResult).filter((i) => i.listing_id),
    total: num(read(raw, 'total', 'Total')),
    has_next: read(raw, 'has_next', 'HasNext') === true,
    next_cursor: strOrNull(read(raw, 'next_cursor', 'NextCursor')),
  }
}

export const searchApi = {
  /** Public: guests can search. Text search, category filter, sort and cursor paging. */
  async search(req: SearchRequest, options?: RequestOptions): Promise<SearchResponse> {
    return mapSearchResponse(await http.post<unknown>('/search', { page_size: 12, ...req }, options))
  },

  async suggest(text: string, options?: RequestOptions): Promise<Suggestion[]> {
    const data = obj(await http.get<unknown>(`/search/suggest${query({ text, limit: 6 })}`, options))
    return arr(read(data, 'suggestions', 'Suggestions')).map((s) => {
      const raw = obj(s)
      return { text: str(read(raw, 'text', 'Text')), type: str(read(raw, 'type', 'Type')) }
    }).filter((s) => s.text)
  },

  /** The vertical catalogue: flows and capabilities per vertical (public). */
  async verticals(): Promise<VerticalInfo[]> {
    const data = obj(await http.get<unknown>('/verticals'))
    return arr(data.items).map((v) => {
      const raw = obj(v)
      return {
        code: str(raw.code), name: str(raw.name), description: str(raw.description),
        flows: arr(raw.flows).map(String).filter((f): f is CustomerFlow => f === 'BOOK' || f === 'BUY'),
        capabilities: arr(raw.capabilities).map(String),
        listing_types: arr(raw.listing_types).map(String) as ListingType[],
      }
    }).filter((v) => v.code)
  },

  async categories(): Promise<Category[]> {
    const data = obj(await http.get<unknown>('/categories'))
    return arr(data.items).map((c) => {
      const raw = obj(c)
      return { id: str(raw.id), name: str(raw.name), slug: str(raw.slug) }
    }).filter((c) => c.id)
  },
}
