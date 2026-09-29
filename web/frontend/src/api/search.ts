import { http, query, type RequestOptions } from '@/api/client'
import { arr, num, obj, read, str, strOrNull } from '@/api/json'
import type { Category, SearchRequest, SearchResponse, SearchResult, Suggestion } from '@/types'

export function mapSearchResult(value: unknown): SearchResult {
  const item = obj(value)
  const location = read(item, 'location', 'Location')
  const seller = read(item, 'seller', 'Seller')
  const store = read(item, 'store', 'Store')
  const distance = read(item, 'distance_meters', 'DistanceMeters')
  return {
    listing_id: str(read(item, 'listing_id', 'ListingID')),
    title: str(read(item, 'title', 'Title')),
    price_amount: num(read(item, 'price_amount', 'PriceAmount')),
    price_currency: str(read(item, 'price_currency', 'PriceCurrency')),
    availability_status: str(read(item, 'availability_status', 'AvailabilityStatus')),
    distance_meters: typeof distance === 'number' ? distance : null,
    location: location ? {
      city: strOrNull(read(obj(location), 'city', 'City')),
      neighborhood: strOrNull(read(obj(location), 'neighborhood', 'Neighborhood')),
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

  async categories(): Promise<Category[]> {
    const data = obj(await http.get<unknown>('/categories'))
    return arr(data.items).map((c) => {
      const raw = obj(c)
      return { id: str(raw.id), name: str(raw.name), slug: str(raw.slug) }
    }).filter((c) => c.id)
  },
}
