import { createContext, Fragment, useContext, useSyncExternalStore, type ReactNode } from 'react'
import { fr } from '@/i18n/fr'

/*
 * Interface language. French is the primary language; English is the source
 * text, so every message is written once in English and looked up by that
 * exact text: t('Sign in') -> "Se connecter". An unknown key shows the English
 * text (never a blank), and a test guarantees every key has a French entry.
 *
 * `t` is a plain function (no hook) so helpers outside components can use it.
 * Switching language re-renders the whole tree (see I18nProvider).
 */

export type Lang = 'fr' | 'en'
export const LANGUAGES: { code: Lang; label: string; short: string }[] = [
  { code: 'fr', label: 'Français', short: 'FR' },
  { code: 'en', label: 'English', short: 'EN' },
]

const STORAGE_KEY = 'trezutil_lang'
export const DEFAULT_LANG: Lang = 'fr'

function stored(): Lang | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'fr' || v === 'en' ? v : null
  } catch {
    return null
  }
}

let current: Lang = stored() ?? DEFAULT_LANG
const listeners = new Set<() => void>()

function apply(lang: Lang) {
  if (typeof document !== 'undefined') document.documentElement.lang = lang
}
apply(current)

export function getLang(): Lang {
  return current
}

export function setLang(lang: Lang, persist = true) {
  if (lang === current) return
  current = lang
  apply(lang)
  if (persist) { try { localStorage.setItem(STORAGE_KEY, lang) } catch { /* storage unavailable */ } }
  listeners.forEach((l) => l())
}

/** The BCP 47 tag used by Intl for dates, numbers and currencies. */
export function locale(): string {
  return current === 'fr' ? 'fr-FR' : 'en-US'
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

function fill(template: string, vars?: Record<string, string | number>): string {
  return vars ? template.replace(/\{(\w+)\}/g, (_, k: string) => (k in vars ? String(vars[k]) : `{${k}}`)) : template
}

/** Translate an English message; `{name}` placeholders are filled from vars. */
export function t(key: string, vars?: Record<string, string | number>): string {
  const template = current === 'fr' ? (fr[key] ?? key) : key
  return fill(template, vars)
}

/** Pick the singular or plural message for n (French treats 0 and 1 as singular). */
export function tn(n: number, one: string, other: string, vars?: Record<string, string | number>): string {
  const singular = current === 'fr' ? n === 0 || n === 1 : n === 1
  return t(singular ? one : other, { n, ...vars })
}

/** Re-renders on a language change; returns the current language. */
export function useLang(): Lang {
  return useSyncExternalStore(subscribe, getLang, getLang)
}

const LangContext = createContext<Lang>(current)
export const useCurrentLang = () => useContext(LangContext)

/** Remounts the tree on a language change so every translated string refreshes. */
export function I18nProvider({ children }: { children: ReactNode }) {
  const lang = useLang()
  return <LangContext.Provider value={lang}><Fragment key={lang}>{children}</Fragment></LangContext.Provider>
}

/**
 * Marks an English message in a module-level constant (nav items, option lists)
 * so it is picked up as a translation key; the component translates it where it
 * renders it with t(). Returns the text unchanged.
 */
export const tx = (key: string): string => key

/** "3 nights" / "1 night": the count with its noun, singular or plural by language. */
export function plural(n: number, noun: string): string {
  return tn(n, `{n} ${noun}`, `{n} ${noun}s`)
}
