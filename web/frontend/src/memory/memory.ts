import { toApiError } from '@/api/client'
import { memoryApi, type ServerMemoryItem } from '@/api/memory'
import { createLocalStore } from '@/lib/localStore'
import { newId } from '@/ui/kit'

/*
 * Commercial memory: what the person chose to let Trezutil remember (buying preferences, brands, usual budgets,
 * constraints, favourite suppliers, recurring projects). It is not the conversation history.
 *
 * For a signed-in account it is kept on the server (/memory): every device sees it, and the assistant reads it as
 * hints when it interprets a request. This device keeps a copy, so the memory shows at once and a change made offline
 * is sent at the next synchronisation. A guest's memory stays on this device. An operation opened from a conversation
 * also receives it as its preferences (POST /missions → preferences). Nothing is remembered without the person adding
 * it, and payment data, codes and secrets are refused (here and again by the server).
 */

export type MemoryKind = 'preference' | 'brand' | 'budget' | 'constraint' | 'supplier' | 'project'

export const MEMORY_KINDS: Record<MemoryKind, string> = {
  preference: 'Préférences d’achat',
  brand: 'Marques préférées',
  budget: 'Budgets habituels',
  constraint: 'Contraintes commerciales',
  supplier: 'Fournisseurs favoris',
  project: 'Projets récurrents',
}

export interface MemoryItem {
  id: string
  kind: MemoryKind
  text: string
  createdAt: number
  updatedAt: number
  /** Not on the server yet: 'add' (new) or 'update' (changed text); sent at the next synchronisation. */
  pending?: 'add' | 'update'
}

interface MemoryState {
  enabled: boolean
  items: MemoryItem[]
  /** Server items deleted here while the server could not be reached. */
  pendingDeletes?: string[]
  /** The on/off switch changed here and is not on the server yet. */
  enabledPending?: boolean
}

export const memoryStore = createLocalStore<MemoryState>('memory', { enabled: true, items: [] })

let account = ''

/** Payment data, codes and secrets are never remembered. */
const SENSITIVE: [RegExp, string][] = [
  [/\b(?:\d[ -]?){13,19}\b/, 'un numéro de carte'],
  [/\b(cvv|cvc|code pin|\bpin\b|mot de passe|password|otp|code de vérification)\b/i, 'un code ou un mot de passe'],
  [/\b(iban|swift|bic)\b/i, 'des coordonnées bancaires'],
]

export function sensitiveReason(text: string): string | null {
  for (const [pattern, what] of SENSITIVE) if (pattern.test(text)) return what
  return null
}

function clean(text: string): string { return text.replace(/\s+/g, ' ').trim().slice(0, 400) }

function check(text: string): string | null {
  if (!text) return 'Écris ce qu’il faut retenir.'
  const reason = sensitiveReason(text)
  return reason ? `Trezutil ne mémorise pas ${reason}.` : null
}

const isKind = (k: string): k is MemoryKind => k in MEMORY_KINDS
const fromServer = (m: ServerMemoryItem): MemoryItem | null => (isKind(m.kind) ? { id: m.id, kind: m.kind, text: m.text, createdAt: m.createdAt, updatedAt: m.updatedAt } : null)

/** Memory belongs to the account (or to the guest). For an account, it is read from and sent to the server. */
export function setMemoryOwner(owner: string) {
  memoryStore.setOwner(owner)
  account = owner
  if (owner) void syncMemory()
}

let syncing: Promise<void> | null = null

/**
 * Sends what changed here (new and edited items, deletions, the switch) and reads the server's memory back. A change
 * the server refuses (a secret it recognises) is dropped; one it could not receive (offline) waits for the next sync.
 */
export function syncMemory(): Promise<void> {
  if (!account) return Promise.resolve()
  if (syncing) return syncing
  const owner = account
  syncing = (async () => {
    try {
      const state = memoryStore.get()
      const keep = new Map<string, MemoryItem>()
      // Signed out or another account meanwhile: stop, so nothing of one account is sent or written for another.
      const stale = () => account !== owner
      for (const item of state.items.filter((m) => m.pending)) {
        if (stale()) return
        try {
          const saved = item.pending === 'add' ? await memoryApi.add(item.kind, item.text) : await memoryApi.update(item.id, item.text)
          const mapped = fromServer(saved)
          if (mapped) keep.set(item.id, mapped)
        } catch (err) {
          const e = toApiError(err)
          if (e.isNetwork || e.status >= 500) keep.set(item.id, item) // try again later
          // refused (422, 400) or gone (404): dropped
        }
      }
      const deletesLeft: string[] = []
      for (const id of state.pendingDeletes ?? []) {
        if (stale()) return
        try { await memoryApi.remove(id) } catch (err) {
          const e = toApiError(err)
          if (e.isNetwork || e.status >= 500) deletesLeft.push(id)
        }
      }
      let enabledPending = state.enabledPending
      if (enabledPending && !stale()) {
        try { await memoryApi.setEnabled(state.enabled); enabledPending = false } catch { /* next time */ }
      }
      if (stale()) return
      const server = await memoryApi.get()
      if (stale()) return
      const serverItems = server.items.map(fromServer).filter((m): m is MemoryItem => !!m)
      const known = new Set(serverItems.map((m) => m.id))
      const stillPending = [...keep.values()].filter((m) => m.pending && !known.has(m.id))
      let again = false
      memoryStore.set((s) => {
        // Changes made here while this synchronisation ran are kept, and sent by the next one.
        const seen = new Set(state.items.map((m) => m.id))
        const newer = s.items.filter((m) => m.pending && !seen.has(m.id))
        const newerDeletes = (s.pendingDeletes ?? []).filter((id) => !(state.pendingDeletes ?? []).includes(id))
        const deletes = [...deletesLeft, ...newerDeletes]
        again = newer.length > 0 || newerDeletes.length > 0 || (s.enabledPending === true && s.enabled !== state.enabled)
        return {
          ...s,
          enabled: enabledPending || again ? s.enabled : server.enabled,
          enabledPending: enabledPending || s.enabled !== state.enabled,
          pendingDeletes: deletes,
          items: [...newer, ...stillPending, ...serverItems.filter((m) => !deletes.includes(m.id))],
        }
      })
      if (again) queueMicrotask(() => { void syncMemory() })
    } catch {
      /* the server cannot be reached: this device's copy stays as it is */
    } finally {
      syncing = null
    }
  })()
  return syncing
}

export function addMemory(kind: MemoryKind, text: string): string | null {
  const value = clean(text)
  const problem = check(value)
  if (problem) return problem
  const now = Date.now()
  memoryStore.set((s) => ({ ...s, items: [{ id: newId(), kind, text: value, createdAt: now, updatedAt: now, ...(account ? { pending: 'add' as const } : {}) }, ...s.items] }))
  void syncMemory()
  return null
}

export function updateMemory(id: string, text: string): string | null {
  const value = clean(text)
  const problem = check(value)
  if (problem) return problem
  memoryStore.set((s) => ({
    ...s,
    items: s.items.map((m) => (m.id === id ? { ...m, text: value, updatedAt: Date.now(), ...(account ? { pending: m.pending === 'add' ? 'add' as const : 'update' as const } : {}) } : m)),
  }))
  void syncMemory()
  return null
}

export function removeMemory(id: string) {
  const item = memoryStore.get().items.find((m) => m.id === id)
  memoryStore.set((s) => ({
    ...s,
    items: s.items.filter((m) => m.id !== id),
    // An item the server knows is deleted there too; one never sent only leaves this device.
    pendingDeletes: account && item && item.pending !== 'add' ? [...(s.pendingDeletes ?? []), id] : s.pendingDeletes,
  }))
  void syncMemory()
}

/** Forgets everything, here and on the server. */
export function clearMemory() {
  const ids = memoryStore.get().items.filter((m) => m.pending !== 'add').map((m) => m.id)
  memoryStore.set((s) => ({ ...s, items: [], pendingDeletes: account ? [...(s.pendingDeletes ?? []), ...ids] : [] }))
  if (account) void memoryApi.clear().then(() => memoryStore.set((s) => ({ ...s, pendingDeletes: [] }))).catch(() => { void syncMemory() })
}

/** Switches the memory on or off. Off, it is kept but neither the assistant nor new operations use it. */
export function setMemoryEnabled(enabled: boolean) {
  memoryStore.set((s) => ({ ...s, enabled, ...(account ? { enabledPending: true } : {}) }))
  void syncMemory()
}

export function rememberSupplier(name: string): 'added' | 'known' {
  const text = name.trim()
  const known = memoryStore.get().items.some((m) => m.kind === 'supplier' && m.text.toLowerCase() === text.toLowerCase())
  if (known) return 'known'
  addMemory('supplier', text)
  return 'added'
}

/** What an operation receives as its preferences, when memory is on (string → string, as the mission API expects). */
export function memoryAsPreferences(): Record<string, string> {
  const { enabled, items } = memoryStore.get()
  if (!enabled || !items.length) return {}
  const out: Record<string, string> = {}
  for (const kind of Object.keys(MEMORY_KINDS) as MemoryKind[]) {
    const texts = items.filter((m) => m.kind === kind).map((m) => m.text)
    if (texts.length) out[`memory_${kind}`] = texts.join(' · ').slice(0, 500)
  }
  return out
}
