import { useSyncExternalStore } from 'react'

/*
 * A tiny persisted store for per-account preferences kept on this device (memory, notification preferences, profile).
 * Every read and write survives a storage that is full, blocked or missing: the value then lives for the visit only.
 */

export function createLocalStore<T>(name: string, initial: T) {
  let owner = ''
  const keyOf = (o: string) => `trezutil_${name}:${o || 'guest'}`
  const read = (o: string): T => {
    try {
      const raw = localStorage.getItem(keyOf(o))
      return raw ? { ...initial, ...(JSON.parse(raw) as T) } : initial
    } catch {
      return initial
    }
  }
  let value: T = read('')
  const listeners = new Set<() => void>()
  const emit = () => listeners.forEach((l) => l())

  return {
    subscribe(l: () => void) { listeners.add(l); return () => { listeners.delete(l) } },
    get: () => value,
    setOwner(next: string) {
      if (next === owner) return
      owner = next
      value = read(owner)
      emit()
    },
    set(change: Partial<T> | ((v: T) => T)) {
      value = typeof change === 'function' ? change(value) : { ...value, ...change }
      try { localStorage.setItem(keyOf(owner), JSON.stringify(value)) } catch { /* this visit only */ }
      emit()
    },
    use(): T {
      // eslint-disable-next-line react-hooks/rules-of-hooks
      return useSyncExternalStore(this.subscribe, this.get, this.get)
    },
  }
}
