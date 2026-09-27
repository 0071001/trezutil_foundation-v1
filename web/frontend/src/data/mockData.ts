/**
 * TEMPORARY MOCK DATA
 *
 * These datasets are placeholder content extracted from the Figma Make
 * visual reference. They exist ONLY so the homepage renders a complete
 * visual experience without a running backend.
 *
 * Each dataset is isolated here so it can be replaced with real API
 * responses as soon as the corresponding endpoint is wired up:
 *
 *   - products  → GET /api/v1/listings  (or POST /api/v1/search)
 *   - services  → GET /api/v1/listings?type=service
 *   - properties → GET /api/v1/listings?type=property
 *
 * TODO: Replace all mock data with real API responses from the Trezutil
 *       backend once the frontend is authenticated. The conversation
 *       composer already attempts a real API call and falls back to
 *       this mock data on failure.
 */
import type { Product, ServiceItem, Property, Category } from "@/types"

/** Unsplash photo URLs used by the Figma Make reference design. */
export const img = {
  phone: "https://images.unsplash.com/photo-1565366196840-0be49f137ae7?auto=format&fit=crop&w=900&q=85",
  sneaker: "https://images.unsplash.com/photo-1560769629-975ec94e6a86?auto=format&fit=crop&w=900&q=85",
  laptop: "https://images.unsplash.com/photo-1627691673558-cf76f304f273?auto=format&fit=crop&w=900&q=85",
  headphones: "https://images.unsplash.com/photo-1621476737725-9530d664a0f4?auto=format&fit=crop&w=900&q=85",
  service: "https://images.unsplash.com/photo-1621905251189-08b45d6a269e?auto=format&fit=crop&w=1200&q=85",
  repair: "https://images.unsplash.com/photo-1646640381839-02748ae8ddf0?auto=format&fit=crop&w=900&q=85",
  tools: "https://images.unsplash.com/photo-1562259929-b4e1fd3aef09?auto=format&fit=crop&w=900&q=85",
  apartment: "https://images.unsplash.com/photo-1649769425782-8cdb757da2b4?auto=format&fit=crop&w=1200&q=85",
  apartment2: "https://images.unsplash.com/photo-1592383809697-4986ac3151c6?auto=format&fit=crop&w=900&q=85",
  patio: "https://images.unsplash.com/photo-1738873441645-ebef693d3012?auto=format&fit=crop&w=900&q=85",
  hotel: "https://images.unsplash.com/photo-1776763018970-9fdf66bd4666?auto=format&fit=crop&w=1200&q=85",
  restaurant: "https://images.unsplash.com/photo-1603122747359-ec6e6bd89c7c?auto=format&fit=crop&w=900&q=85",
  merchant: "https://images.unsplash.com/photo-1687422808384-c896d0efd4ab?auto=format&fit=crop&w=1200&q=85",
}

/** Mock product suggestions shown under the conversation composer. */
export const suggestions = [
  "A phone under $300",
  "A plumber today",
  "Hotel for Saturday",
  "Sneakers near me",
]

/** TEMPORARY: Replace with real product listings from GET /api/v1/listings. */
export const products: Product[] = [
  { name: "Samsung Galaxy A55", price: "$349", seller: "Tech Store", distance: "2.1 km", image: img.phone },
  { name: "Air Max Pulse", price: "$118", seller: "Motion Kicks", distance: "1.4 km", image: img.sneaker },
  { name: "MacBook Air M2", price: "$899", seller: "Digital House", distance: "3.2 km", image: img.laptop },
  { name: "Studio Headphones", price: "$94", seller: "Sound Lab", distance: "800 m", image: img.headphones },
]

/** TEMPORARY: Replace with service listings from GET /api/v1/listings?type=service. */
export const services: ServiceItem[] = [
  { title: "Electrician", note: "12 available today", image: img.service },
  { title: "Home repair", note: "From $18 · 1.8 km", image: img.repair },
  { title: "Plumbing", note: "Fast response", image: img.tools },
]

/** TEMPORARY: Replace with property listings from GET /api/v1/listings?type=property. */
export const properties: Property[] = [
  { title: "Riverside apartment", type: "Apartment · 2 beds", location: "Gombe, Kinshasa", price: "$850 / month", image: img.apartment },
  { title: "Garden residence", type: "House · 3 beds", location: "Ngaliema, Kinshasa", price: "$1,200 / month", image: img.apartment2 },
  { title: "Courtyard studio", type: "Studio · Furnished", location: "Lingwala, Kinshasa", price: "$540 / month", image: img.patio },
]

/**
 * Commerce category tiles for the "Explore Trezutil" grid.
 * These are structural navigation buckets; the icon/label mapping mirrors
 * the vertical pack codes (ECOMMERCE, SERVICES, etc.).
 */
export const categories: Category[] = [
  { label: "Products", icon: "shop", className: "category-large", image: img.sneaker },
  { label: "Services", icon: "service", className: "category-navy" },
  { label: "Food", icon: "food", className: "category-mint" },
  { label: "Real Estate", icon: "building", className: "category-wide", image: img.apartment },
  { label: "Hotels", icon: "calendar", className: "category-photo", image: img.hotel },
  { label: "Transport", icon: "car", className: "category-blue" },
  { label: "Local Shops", icon: "shop", className: "category-soft" },
  { label: "Reservations", icon: "calendar", className: "category-line" },
]

/** Sample avatar initials used in the hero proof bar. */
export const avatarStack = ["JM", "AN", "PK"]
