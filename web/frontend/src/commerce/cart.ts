import { useSyncExternalStore } from 'react'
import type { Offer } from '@/commerce/offers'

/*
 * The cart: offers a person intends to buy, kept on this device until they order. Nothing is reserved or charged by
 * being in the cart; the price is read again by the server when the order is placed. An order is for one shop in one
 * currency (the server refuses a mix), so the cart is grouped that way and each group becomes its own order.
 *
 * Only products can be bought this way (vertical RETAIL with a price): a service, a table, a room or a home is asked
 * for, booked or visited, not put in a cart.
 */

export interface CartItem {
  listingId: string
  title: string
  supplier: string
  storeId: string
  image: string | null
  unitPrice: number
  currency: string
  quantity: number
}

export interface CartGroup {
  key: string
  supplier: string
  currency: string
  items: CartItem[]
  total: number
}

const KEY = 'trezutil_cart'
export const MAX_QUANTITY = 99
let items: CartItem[] = read()
const listeners = new Set<() => void>()

function read(): CartItem[] {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) ?? '[]') as unknown
    return Array.isArray(raw) ? (raw as CartItem[]).filter((i) => i && typeof i.listingId === 'string' && i.quantity > 0) : []
  } catch {
    return []
  }
}

function save(next: CartItem[]) {
  items = next
  try { localStorage.setItem(KEY, JSON.stringify(next)) } catch { /* this visit only */ }
  listeners.forEach((l) => l())
}

/** Whether an offer can be bought straight away (a product with a price), rather than asked for or booked. */
export function canBuy(offer: Pick<Offer, 'vertical' | 'priceValue'>): boolean {
  return (offer.vertical || 'RETAIL').toUpperCase() === 'RETAIL' && offer.priceValue > 0
}

export const cart = {
  items: () => items,
  count: () => items.reduce((n, i) => n + i.quantity, 0),
  add(offer: Offer, quantity = 1) {
    const q = Math.max(1, Math.min(MAX_QUANTITY, Math.round(quantity)))
    const existing = items.find((i) => i.listingId === offer.id)
    if (existing) {
      save(items.map((i) => (i.listingId === offer.id ? { ...i, quantity: Math.min(MAX_QUANTITY, i.quantity + q) } : i)))
      return
    }
    save([...items, {
      listingId: offer.id, title: offer.title, supplier: offer.supplier, storeId: offer.storeId, image: offer.image,
      unitPrice: offer.priceValue, currency: offer.currency, quantity: q,
    }])
  },
  setQuantity(listingId: string, quantity: number) {
    if (quantity <= 0) { cart.remove(listingId); return }
    save(items.map((i) => (i.listingId === listingId ? { ...i, quantity: Math.min(MAX_QUANTITY, Math.round(quantity)) } : i)))
  },
  remove(listingId: string) { save(items.filter((i) => i.listingId !== listingId)) },
  /** After a group was ordered. */
  removeMany(listingIds: string[]) { save(items.filter((i) => !listingIds.includes(i.listingId))) },
  clear() { save([]) },
}

/** One group per shop and currency: each becomes one order. */
export function groupCart(list: CartItem[]): CartGroup[] {
  const groups = new Map<string, CartGroup>()
  for (const i of list) {
    const key = `${i.storeId || i.supplier}|${i.currency}`
    const g = groups.get(key) ?? { key, supplier: i.supplier, currency: i.currency, items: [], total: 0 }
    g.items.push(i)
    g.total += i.unitPrice * i.quantity
    groups.set(key, g)
  }
  return [...groups.values()]
}

export function useCart(): CartItem[] {
  return useSyncExternalStore((l) => { listeners.add(l); return () => { listeners.delete(l) } }, () => items, () => items)
}

// Another tab changed the cart.
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => { if (e.key === KEY) { items = read(); listeners.forEach((l) => l()) } })
}
