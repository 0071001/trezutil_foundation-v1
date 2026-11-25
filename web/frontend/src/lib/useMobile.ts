import { useSyncExternalStore } from 'react'

/*
 * Whether the page is on a phone-sized screen, where Trezutil uses its mobile layout ("Design Trezutil Mobile App"):
 * a bottom tab bar, the mobile home, Discover, Activity and Profile. The same width as the stylesheet's mobile rules.
 */

export const MOBILE_QUERY = '(max-width: 760px)'

function query(): MediaQueryList | null {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia(MOBILE_QUERY) : null
}

function subscribe(onChange: () => void): () => void {
  const q = query()
  if (!q) return () => {}
  q.addEventListener?.('change', onChange)
  return () => q.removeEventListener?.('change', onChange)
}

export function isMobile(): boolean {
  return !!query()?.matches
}

export function useMobile(): boolean {
  return useSyncExternalStore(subscribe, isMobile, () => false)
}
