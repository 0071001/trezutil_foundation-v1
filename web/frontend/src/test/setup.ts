import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

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
  window.scrollTo = vi.fn() as unknown as typeof window.scrollTo
  Element.prototype.scrollIntoView = vi.fn()
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
