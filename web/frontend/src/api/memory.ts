import { http } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/* /memory: the signed-in person's commercial memory, kept on the server and read by the assistant as hints. */

export interface ServerMemoryItem { id: string; kind: string; text: string; createdAt: number; updatedAt: number }

function mapItem(value: unknown): ServerMemoryItem {
  const m = obj(value)
  return { id: str(m.id), kind: str(m.kind), text: str(m.text), createdAt: num(m.created_at) * 1000, updatedAt: num(m.updated_at) * 1000 }
}

export const memoryApi = {
  async get(): Promise<{ enabled: boolean; items: ServerMemoryItem[] }> {
    const raw = obj(await http.get<unknown>('/memory'))
    return { enabled: raw.enabled !== false, items: arr(raw.items).map(mapItem).filter((m) => m.id) }
  },
  async add(kind: string, text: string): Promise<ServerMemoryItem> {
    return mapItem(await http.post<unknown>('/memory/items', { kind, text }))
  },
  async update(id: string, text: string): Promise<ServerMemoryItem> {
    return mapItem(await http.patch<unknown>(`/memory/items/${encodeURIComponent(id)}`, { text }))
  },
  async remove(id: string): Promise<void> {
    await http.del<unknown>(`/memory/items/${encodeURIComponent(id)}`)
  },
  async clear(): Promise<void> {
    await http.del<unknown>('/memory/items')
  },
  async setEnabled(enabled: boolean): Promise<void> {
    await http.put<unknown>('/memory/settings', { enabled })
  },
}
