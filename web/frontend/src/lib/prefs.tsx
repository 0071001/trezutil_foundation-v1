import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

/**
 * Per-viewer conveniences kept in localStorage: color theme, saved listings
 * (wishlist) and the bag (cart). They hold listing snapshots only — never
 * identity or payment data — and the page works without storage.
 */

export interface SavedListing {
  listing_id: string
  title: string
  price_amount: number
  price_currency: string
  image: string | null
  seller: string
  listing_type?: string
  vertical?: string
}

export interface BagItem extends SavedListing {
  quantity: number
}

function load<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

function persist(key: string, value: unknown) {
  try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* storage unavailable */ }
}

type Theme = 'light' | 'dark'

interface Prefs {
  theme: Theme
  toggleTheme: () => void
  saved: SavedListing[]
  isSaved: (id: string) => boolean
  toggleSaved: (item: SavedListing) => void
  bag: BagItem[]
  addToBag: (item: SavedListing, quantity?: number) => void
  setBagQuantity: (id: string, quantity: number) => void
  removeFromBag: (id: string) => void
}

const PrefsContext = createContext<Prefs | null>(null)

function initialTheme(): Theme {
  const stored = load<Theme | null>('trezutil_theme', null)
  if (stored === 'light' || stored === 'dark') return stored
  try { return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light' } catch { return 'light' }
}

export function PrefsProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(initialTheme)
  const [saved, setSaved] = useState<SavedListing[]>(() => load('trezutil_saved', []))
  const [bag, setBag] = useState<BagItem[]>(() => load('trezutil_bag', []))

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.documentElement.style.colorScheme = theme
  }, [theme])
  useEffect(() => persist('trezutil_saved', saved), [saved])
  useEffect(() => persist('trezutil_bag', bag), [bag])

  const toggleTheme = useCallback(() => setTheme((t) => {
    const next = t === 'dark' ? 'light' : 'dark'
    persist('trezutil_theme', next)
    return next
  }), [])

  const toggleSaved = useCallback((item: SavedListing) => setSaved((list) =>
    list.some((s) => s.listing_id === item.listing_id) ? list.filter((s) => s.listing_id !== item.listing_id) : [item, ...list].slice(0, 100)), [])

  const addToBag = useCallback((item: SavedListing, quantity = 1) => setBag((list) => {
    const existing = list.find((b) => b.listing_id === item.listing_id)
    if (existing) return list.map((b) => b.listing_id === item.listing_id ? { ...b, quantity: Math.min(99, b.quantity + quantity) } : b)
    return [...list, { ...item, quantity }]
  }), [])

  const setBagQuantity = useCallback((id: string, quantity: number) =>
    setBag((list) => list.map((b) => b.listing_id === id ? { ...b, quantity: Math.max(1, Math.min(99, quantity)) } : b)), [])

  const removeFromBag = useCallback((id: string) => setBag((list) => list.filter((b) => b.listing_id !== id)), [])

  const value = useMemo<Prefs>(() => ({
    theme, toggleTheme, saved, isSaved: (id) => saved.some((s) => s.listing_id === id), toggleSaved,
    bag, addToBag, setBagQuantity, removeFromBag,
  }), [theme, toggleTheme, saved, toggleSaved, bag, addToBag, setBagQuantity, removeFromBag])

  return <PrefsContext.Provider value={value}>{children}</PrefsContext.Provider>
}

export function usePrefs(): Prefs {
  const ctx = useContext(PrefsContext)
  if (!ctx) throw new Error('usePrefs outside PrefsProvider')
  return ctx
}
