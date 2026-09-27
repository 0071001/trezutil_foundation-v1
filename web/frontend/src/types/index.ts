/**
 * Trezutil frontend type definitions.
 * Mirrors the OpenAPI contract in api/openapi/openapi.yaml and the
 * domain SearchResult / Listing types from internal/domain.
 */

export type ListingType =
  | "product"
  | "property"
  | "vehicle"
  | "food"
  | "service"
  | "accommodation"
  | "other"

export type ListingStatus =
  | "draft"
  | "review"
  | "published"
  | "paused"
  | "sold"
  | "rented"
  | "completed"
  | "archived"

export interface Money {
  amount_minor: number
  currency: string
}

export interface Media {
  id: string
  listing_id: string
  url: string
  alt?: string
  type: string
  position: number
}

export interface Listing {
  id: string
  seller_id: string
  type: ListingType
  title: string
  description: string
  status: ListingStatus
  price?: Money
  attributes: Record<string, unknown>
  media?: Media[]
}

export interface ListingPage {
  items: Listing[]
  next_page_token?: string
}

/** A single search result projection — what the search service returns. */
export interface SearchResult {
  listing_id: string
  title: string
  price_amount: number
  price_currency: string
  availability_status: string
  distance_meters?: number | null
  score: number
  location?: {
    city?: string | null
    neighborhood?: string | null
  } | null
  seller?: {
    seller_id: string
    display_name: string
    trust_level: number
  } | null
  store?: {
    store_id: string
    name: string
  } | null
  primary_media_url?: string | null
  published_at: number
}

export interface SearchResponse {
  items: SearchResult[]
  total: number
  has_next: boolean
  next_cursor?: string | null
}

export interface SearchRequest {
  text: string
  category_id?: string
  page_size?: number
  cursor?: string
  sort_field?: string
  sort_direction?: string
}

export interface Suggestion {
  text: string
  category_id?: string | null
  type: string
}

export interface SuggestionResponse {
  suggestions: Suggestion[]
}

export type VerticalPackStatus = "ACTIVE" | "INACTIVE" | "DEPRECATED"

export interface VerticalPack {
  id: string
  code: string
  name: string
  description: string
  status: VerticalPackStatus
  capabilities: string[]
  supported_actions: string[]
  ui_metadata?: {
    route_prefix?: string
    icon?: string
    color?: string
    order?: number
  }
  ai_profile?: string
  created_at: number
  updated_at: number
}

export interface UserStatus {
  status: "active" | "suspended" | "pending_verification"
}

export interface User {
  id: string
  phone: string
  email?: string | null
  display_name: string
  status: string
  created_at: string
  updated_at: string
  verified_at?: string | null
}

export interface UserRolesResponse {
  roles: string[]
}

export interface AuthRequestOTP {
  phone: string
}

export interface AuthVerifyOTP {
  request_id: string
  code: string
}

export interface AuthVerifyResponse {
  user: User
  tokens: {
    access_token: string
    refresh_token: string
    expires_at: string
  }
}

export interface Reservation {
  id: string
  customer_id: string
  listing_id: string
  status: "pending" | "confirmed" | "completed" | "cancelled"
  start_at: number
  end_at: number
  quantity: number
  party_size: number
  pricing_snapshot?: {
    subtotal?: Money
    tax?: Money
    total?: Money
  }
  created_at: number
  updated_at: number
}

export interface OrderStatus {
  status: string
}

export interface Order {
  id: string
  buyer_id: string
  status: string
  total?: Money
}

export type Category = {
  label: string
  icon: IconName
  className: string
  image?: string
}

export type IconName =
  | "arrow"
  | "bell"
  | "briefcase"
  | "building"
  | "calendar"
  | "car"
  | "chevron"
  | "filter"
  | "food"
  | "heart"
  | "home"
  | "location"
  | "menu"
  | "mic"
  | "moon"
  | "orders"
  | "search"
  | "send"
  | "service"
  | "shop"
  | "spark"
  | "sun"
  | "user"
  | "whatsapp"

export type Product = {
  name: string
  price: string
  seller: string
  distance: string
  image: string
}

export type ServiceItem = {
  title: string
  note: string
  image: string
}

export type Property = {
  title: string
  type: string
  location: string
  price: string
  image: string
}

export type ErrorResponse = {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
    request_id?: string
  }
}
