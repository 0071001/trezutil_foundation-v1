import { useSyncExternalStore } from 'react'

/*
 * Light / dark theme. "system" follows the device (prefers-color-scheme); "light" and "dark" force a theme through
 * html[data-theme], which the stylesheet's tokens read. The choice is kept on this device and applied before the
 * first render (see main.tsx), so the page does not flash the other theme.
 */

export type Theme = 'system' | 'light' | 'dark'
const KEY = 'trezutil_theme'
const listeners = new Set<() => void>()

function read(): Theme {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

let current: Theme = typeof window === 'undefined' ? 'system' : read()

/** Whether the page is dark right now (the system's choice when the theme is "system"). */
export function isDark(theme: Theme = current): boolean {
  if (theme !== 'system') return theme === 'dark'
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches
}

/** Puts the theme on <html> and tells the browser chrome (mobile address bar) its colour. */
export function applyTheme(theme: Theme = current) {
  const root = document.documentElement
  if (theme === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', theme)
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', isDark(theme) ? '#212121' : '#ffffff')
}

export function setTheme(theme: Theme) {
  current = theme
  try {
    if (theme === 'system') localStorage.removeItem(KEY)
    else localStorage.setItem(KEY, theme)
  } catch { /* this visit only */ }
  applyTheme(theme)
  listeners.forEach((l) => l())
}

export function getTheme(): Theme { return current }

export function useTheme(): Theme {
  return useSyncExternalStore((l) => { listeners.add(l); return () => { listeners.delete(l) } }, getTheme, getTheme)
}

// A system theme change (dark mode at night) updates the browser chrome colour.
if (typeof window !== 'undefined') {
  window.matchMedia?.('(prefers-color-scheme: dark)').addEventListener?.('change', () => { if (current === 'system') applyTheme('system') })
}
