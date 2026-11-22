import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'
import { setLang } from '@/lib/i18n'
import { setMemoryOwner } from '@/memory/memory'

// Several journeys chain many awaited steps; under a loaded machine the 1 s default made two of them flaky.
configure({ asyncUtilTimeout: 3000 })

// Node 26 defines its own (file-backed, here unusable) localStorage global,
// which shadows the DOM environment's. Tests get an in-memory Storage.
class MemoryStorage implements Storage {
  private data = new Map<string, string>()
  get length() { return this.data.size }
  clear() { this.data.clear() }
  getItem(key: string) { return this.data.has(key) ? this.data.get(key)! : null }
  key(index: number) { return [...this.data.keys()][index] ?? null }
  removeItem(key: string) { this.data.delete(key) }
  setItem(key: string, value: string) { this.data.set(key, String(value)) }
}
for (const name of ['localStorage', 'sessionStorage'] as const) {
  const store = new MemoryStorage()
  Object.defineProperty(globalThis, name, { value: store, configurable: true, writable: true })
  if (typeof window !== 'undefined') Object.defineProperty(window, name, { value: store, configurable: true, writable: true })
}

beforeEach(() => {
  localStorage.clear()
  sessionStorage.clear()
  // French is the product default; the suites assert the English source text.
  setLang('en', false)
  window.scrollTo = vi.fn() as unknown as typeof window.scrollTo
  Element.prototype.scrollIntoView = vi.fn()
})

afterEach(() => {
  cleanup()
  // A memory synchronisation still running stops before the mocked fetch is taken away.
  setMemoryOwner('')
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
