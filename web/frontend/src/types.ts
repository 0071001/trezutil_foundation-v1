/**
 * Frontend view models. The Go domain aggregates are serialized without JSON
 * tags (PascalCase) while DTOs use snake_case; the mappers in src/api accept
 * both and produce these shapes.
 */

export type ListingType = 'physical_product' | 'real_estate' | 'vehicle' | 'food' | 'service' | 'other'

export type ListingStatus = 'draft' | 'pending_review' | 'published' | 'paused' | 'sold' | 'closed' | 'expired' | 'suspended'

export interface ListingMedia {
  url: string
  primary: boolean
}

export interface ListingDetail {
  id: string
  organization_id: string
  seller_id: string
  store_id: string
  category_id: string
  title: string
  description: string | null
  listing_type: ListingType
  /** Commerce vertical (HOTEL, RESTAURANT, RETAIL…); decides booking vs purchase. */
  vertical: string
  pricing_model: string
  price_amount: number
  price_currency: string
  status: ListingStatus
  metadata: Record<string, unknown>
  published_at: number | null
  created_at: number
  updated_at: number
  media: ListingMedia[]
}

/** A search projection row (POST /search). */
export interface SearchResult {
  listing_id: string
  title: string
  vertical: string
  price_amount: number
  price_currency: string
  availability_status: string
  distance_meters: number | null
  location: { city: string | null; neighborhood: string | null; latitude: number | null; longitude: number | null } | null
  seller: { seller_id: string; display_name: string; trust_level: number } | null
  store: { store_id: string; name: string } | null
  primary_media_url: string | null
  /** The regular price a sale replaced; null when the listing is not on promotion. */
  compare_at_amount: number | null
  /** The saving off compare_at_amount as a whole percentage (0 when not on promotion). */
  discount_percent: number
  published_at: number
}

export interface SearchResponse {
  items: SearchResult[]
  total: number
  has_next: boolean
  next_cursor: string | null
}

export type SortField = 'relevance' | 'price_asc' | 'price_desc' | 'newest' | 'availability' | 'distance' | 'discount'

export interface SearchRequest {
  text: string
  category_id?: string
  /** Vertical code; the API answers 400 for an unknown one. */
  vertical?: string
  /** Place name, e.g. "Gombe". */
  location?: string
  price_min?: number
  price_max?: number
  /** Only listings currently on sale. */
  on_promotion?: boolean
  page_size?: number
  cursor?: string
  sort_field?: SortField
  sort_direction?: 'asc' | 'desc'
}

/** A commerce vertical from GET /verticals. */
export type CustomerFlow = 'BOOK' | 'BUY'

export interface VerticalInfo {
  code: string
  name: string
  description: string
  flows: CustomerFlow[]
  capabilities: string[]
  listing_types: ListingType[]
}

export interface Suggestion {
  text: string
  type: string
}

export interface Category {
  id: string
  name: string
  slug: string
}

export interface User {
  id: string
  phone: string
  email?: string | null
  display_name: string
  status: string
  roles: string[]
  created_at: string
  verified_at?: string | null
}

export interface Session {
  user: User
  tokens: { access_token: string; refresh_token: string; expires_at: string }
}

export interface ErrorEnvelope {
  error?: { code?: string; message?: string }
}
