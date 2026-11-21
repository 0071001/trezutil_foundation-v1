import { createLocalStore } from '@/lib/localStore'
import { newId } from '@/ui/kit'

/*
 * Commercial memory: what the person chose to let Trezutil remember (buying preferences, brands, usual budgets,
 * constraints, favourite suppliers, recurring projects). It is not the conversation history.
 *
 * Today it is kept on this device, per account, and used where the backend can take it: an operation opened from a
 * conversation receives it as its preferences (POST /missions → preferences). The assistant server does not read it
 * yet (services.ts → memory). Nothing is remembered without the person adding it, and secrets are refused.
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

export interface MemoryItem { id: string; kind: MemoryKind; text: string; createdAt: number; updatedAt: number }

interface MemoryState { enabled: boolean; items: MemoryItem[] }

export const memoryStore = createLocalStore<MemoryState>('memory', { enabled: true, items: [] })

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

export function addMemory(kind: MemoryKind, text: string): string | null {
  const clean = text.replace(/\s+/g, ' ').trim().slice(0, 400)
  if (!clean) return 'Écris ce qu’il faut retenir.'
  const reason = sensitiveReason(clean)
  if (reason) return `Trezutil ne mémorise pas ${reason}.`
  const now = Date.now()
  memoryStore.set((s) => ({ ...s, items: [{ id: newId(), kind, text: clean, createdAt: now, updatedAt: now }, ...s.items] }))
  return null
}

export function updateMemory(id: string, text: string): string | null {
  const clean = text.replace(/\s+/g, ' ').trim().slice(0, 400)
  if (!clean) return 'Écris ce qu’il faut retenir.'
  const reason = sensitiveReason(clean)
  if (reason) return `Trezutil ne mémorise pas ${reason}.`
  memoryStore.set((s) => ({ ...s, items: s.items.map((m) => (m.id === id ? { ...m, text: clean, updatedAt: Date.now() } : m)) }))
  return null
}

export function removeMemory(id: string) {
  memoryStore.set((s) => ({ ...s, items: s.items.filter((m) => m.id !== id) }))
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
