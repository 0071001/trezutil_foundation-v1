import { afterEach, describe, expect, it } from 'vitest'
import { addMemory, memoryStore, removeMemory, setMemoryEnabled, setMemoryOwner, syncMemory } from '@/memory/memory'
import { mockApi, signIn } from '@/test/utils'

const item = (id: string, kind: string, text: string) => ({ id, kind, text, created_at: 1, updated_at: 1 })

afterEach(() => { setMemoryOwner('') })

describe('commercial memory on the server', () => {
  it('sends a new item, keeps the server id, and drops what the server refuses', async () => {
    signIn('tok')
    const server: ReturnType<typeof item>[] = []
    const { calls } = mockApi({
      'GET /memory': () => ({ body: { enabled: true, items: server } }),
      'POST /memory/items': (c) => {
        const body = c.body as { kind: string; text: string }
        if (body.text.includes('refusé')) return { status: 422, body: { error: { code: 'SENSITIVE_DATA', message: 'no' } } }
        const saved = item(`srv-${server.length + 1}`, body.kind, body.text)
        server.unshift(saved)
        return { status: 201, body: saved }
      },
      'DELETE /memory/items/srv-1': () => { server.splice(0, server.length); return { body: {} } },
      'PUT /memory/settings': () => ({ body: { enabled: false } }),
    })
    setMemoryOwner('user-sync-1')
    await syncMemory()

    expect(addMemory('brand', 'Dell')).toBeNull()
    await syncMemory()
    expect(memoryStore.get().items.map((m) => [m.id, m.text, m.pending])).toEqual([['srv-1', 'Dell', undefined]])

    expect(addMemory('preference', 'texte refusé par le serveur')).toBeNull()
    await syncMemory()
    expect(memoryStore.get().items.map((m) => m.text)).toEqual(['Dell'])

    removeMemory('srv-1')
    await syncMemory()
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/memory/items/srv-1')).toBe(true)
    expect(memoryStore.get().items).toEqual([])

    setMemoryEnabled(false)
    await syncMemory()
    expect(calls.some((c) => c.method === 'PUT' && JSON.stringify(c.body) === '{"enabled":false}')).toBe(true)
  })

  it('keeps an item added offline and sends it later', async () => {
    signIn('tok')
    mockApi({}) // every call fails: unmocked routes answer 599
    setMemoryOwner('user-sync-2')
    await syncMemory()
    expect(addMemory('budget', 'Moins de 800 USD')).toBeNull()
    await syncMemory()
    expect(memoryStore.get().items[0]).toMatchObject({ text: 'Moins de 800 USD', pending: 'add' })

    const { calls } = mockApi({
      'POST /memory/items': () => ({ status: 201, body: item('srv-9', 'budget', 'Moins de 800 USD') }),
      'GET /memory': () => ({ body: { enabled: true, items: [item('srv-9', 'budget', 'Moins de 800 USD')] } }),
    })
    await syncMemory()
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(1)
    expect(memoryStore.get().items.map((m) => [m.id, m.pending])).toEqual([['srv-9', undefined]])
  })

  it('keeps a guest memory on this device only', async () => {
    const { calls } = mockApi({})
    setMemoryOwner('')
    expect(addMemory('brand', 'HP')).toBeNull()
    await syncMemory()
    expect(calls).toHaveLength(0)
    expect(memoryStore.get().items[0]).toMatchObject({ text: 'HP' })
    expect(memoryStore.get().items[0].pending).toBeUndefined()
  })
})
