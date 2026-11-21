import { useMemo } from 'react'
import type { OrderSummary } from '@/api/commerce'
import type { Mission } from '@/api/missions'
import { createLocalStore } from '@/lib/localStore'
import { openConfirmation, type Conversation } from '@/conversation/store'

/*
 * The notification centre shows what really waits for the person, read from their data: a confirmation the runtime
 * asked for, an operation on hold or waiting to start, an order on its way. Nothing is invented and nothing is pushed:
 * the backend has no notification or price-watch service yet (services.ts → notifications). The preferences chosen
 * here decide what this centre shows; they will drive the notification service when it exists.
 */

export type NoticeKind = 'decision' | 'operation' | 'order'

export interface Notice { id: string; kind: NoticeKind; title: string; text: string; at: number; to: string; tone: 'green' | 'amber' | 'neutral' }

export interface NoticePrefs {
  enabled: boolean
  kinds: Record<NoticeKind, boolean>
  /** The price watch and opportunity categories the person wants, for when the service exists. */
  categories: string[]
  frequency: 'realtime' | 'daily' | 'weekly'
  read: Record<string, true>
}

export const NOTICE_KINDS: Record<NoticeKind, string> = {
  decision: 'Décisions à prendre',
  operation: 'Suivi des opérations',
  order: 'Commandes et livraisons',
}

export const OPPORTUNITY_CATEGORIES = ['Informatique', 'Mobilier', 'Événementiel', 'Transport', 'Hébergement', 'Construction', 'Alimentation']

export const noticePrefs = createLocalStore<NoticePrefs>('notices', {
  enabled: true, kinds: { decision: true, operation: true, order: true }, categories: [], frequency: 'daily', read: {},
})

const DONE_ORDER = new Set(['completed', 'delivered', 'cancelled', 'refunded'])

export function useNotices(conversations: Conversation[], missions: Mission[], orders: OrderSummary[]) {
  const prefs = noticePrefs.use()
  return useMemo(() => {
    const all: Notice[] = []
    for (const c of conversations) {
      const waiting = openConfirmation(c)
      if (waiting) all.push({ id: `d-${waiting.id}`, kind: 'decision', title: 'Décision requise', text: waiting.reply?.confirmation?.summary || c.title, at: waiting.at, to: `/c/${c.id}`, tone: 'amber' })
    }
    for (const m of missions) {
      const at = Date.parse(m.updatedAt) || Date.now()
      if (m.status === 'paused') all.push({ id: `o-${m.id}-${m.updatedAt}`, kind: 'operation', title: 'Opération en pause', text: m.title || m.objective, at, to: `/operations/${m.id}`, tone: 'neutral' })
      else if (m.status === 'draft') all.push({ id: `o-${m.id}-${m.updatedAt}`, kind: 'operation', title: 'Opération à lancer', text: m.title || m.objective, at, to: `/operations/${m.id}`, tone: 'neutral' })
      else if (m.status === 'completed') all.push({ id: `o-${m.id}-${m.updatedAt}`, kind: 'operation', title: 'Opération terminée', text: m.title || m.objective, at, to: `/operations/${m.id}`, tone: 'green' })
    }
    for (const o of orders) {
      if (DONE_ORDER.has(o.status)) continue
      all.push({ id: `c-${o.id}-${o.status}-${o.delivery_status}`, kind: 'order', title: o.delivery_status ? 'Livraison en cours' : 'Commande en cours', text: o.title || 'Commande', at: (o.updated_at || o.created_at) * 1000, to: `/purchases/${o.id}`, tone: 'neutral' })
    }
    const shown = prefs.enabled ? all.filter((n) => prefs.kinds[n.kind]).sort((a, b) => b.at - a.at) : []
    const unread = shown.filter((n) => !prefs.read[n.id])
    return { notices: shown, unread: unread.length, prefs }
  }, [conversations, missions, orders, prefs])
}

export function markRead(ids: string[]) {
  noticePrefs.set((p) => ({ ...p, read: { ...p.read, ...Object.fromEntries(ids.map((id) => [id, true as const])) } }))
}
