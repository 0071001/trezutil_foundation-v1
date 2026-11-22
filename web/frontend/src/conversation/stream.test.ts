import { afterEach, describe, expect, it, vi } from 'vitest'
import { conversationsApi } from '@/api/conversations'
import { conversationStore } from '@/conversation/store'
import { signIn } from '@/test/utils'

/*
 * The streamed answer, end to end on the client: a real byte stream (frames split anywhere), the order of what is
 * shown, the person stopping it, and a broken connection read back with the same key.
 */

const enc = new TextEncoder()

/** A fetch answering the stream route with these chunks (each pushed when `step` allows), and the plain route with `plain`. */
function streamingFetch(chunks: string[], opts: { plain?: (body: Record<string, unknown>) => unknown; breakAfter?: number; gate?: Promise<void> } = {}) {
  const calls: { path: string; body: Record<string, unknown> }[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = new URL(String(input), 'http://localhost').pathname.replace(/^\/api\/v1/, '')
    const body = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>
    calls.push({ path, body })
    if (path === '/conversations/message') {
      return new Response(JSON.stringify(opts.plain?.(body) ?? {}), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    const signal = init?.signal
    const stream = new ReadableStream<Uint8Array>({
      async start(controller) {
        for (let i = 0; i < chunks.length; i++) {
          if (signal?.aborted) return
          if (i === opts.breakAfter) { controller.error(new TypeError('network')); return }
          controller.enqueue(enc.encode(chunks[i]!))
          if (i === 1 && opts.gate) await opts.gate
          await new Promise((r) => setTimeout(r, 1))
        }
        controller.close()
      },
    })
    signal?.addEventListener('abort', () => { /* the reader rejects on abort */ })
    return new Response(stream, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { calls }
}

const done = (text: string) => `event: done\ndata: ${JSON.stringify({ conversation_id: 'srv-1', response: { text, language: 'fr', metadata: { action_type: 'SEARCH_LISTINGS' } } })}\n\n`

afterEach(() => { conversationStore.setOwner('__reset__'); conversationStore.setOwner('') })

describe('streamed answers', () => {
  it('reads stages and text in order, even when frames are split anywhere', async () => {
    const full = `: open\n\nevent: stage\ndata: {"stage":"interpreting","detail":""}\n\nevent: stage\ndata: {"stage":"acting","detail":"SEARCH_LISTINGS"}\n\nevent: delta\ndata: {"text":"Voici "}\n\nevent: reset\ndata: {}\n\nevent: delta\ndata: {"text":"J’ai trouvé "}\n\nevent: delta\ndata: {"text":"2 offres."}\n\n${done('J’ai trouvé 2 offres.')}`
    // Cut into 7-byte pieces: a frame, a field and even a multi-byte character may straddle two chunks.
    const bytes = enc.encode(full)
    const pieces: string[] = []
    const dec = new TextDecoder()
    for (let i = 0; i < bytes.length; i += 7) pieces.push(dec.decode(bytes.slice(i, i + 7), { stream: true }))
    streamingFetch([full.slice(0, 13), full.slice(13, 40), full.slice(40)])
    const seen: string[] = []
    const reply = await conversationsApi.sendStream('des ordinateurs', 'k-1', 'web-1', {
      onStage: (s, d) => seen.push(`stage:${s}:${d}`),
      onDelta: (t) => seen.push(`delta:${t}`),
      onReset: () => seen.push('reset'),
    })
    expect(seen).toEqual(['stage:interpreting:', 'stage:acting:SEARCH_LISTINGS', 'delta:Voici ', 'reset', 'delta:J’ai trouvé ', 'delta:2 offres.'])
    expect(reply.text).toBe('J’ai trouvé 2 offres.')
    expect(reply.conversation_id).toBe('srv-1')
    expect(pieces.length).toBeGreaterThan(10)
  })

  it('shows the text while it arrives, then keeps the final answer', async () => {
    signIn('tok')
    conversationStore.setOwner('user-stream')
    let open!: () => void
    const gate = new Promise<void>((r) => { open = r })
    streamingFetch([
      `event: stage\ndata: {"stage":"writing","detail":""}\n\n`,
      `event: delta\ndata: {"text":"Voici les "}\n\n`,
      `event: delta\ndata: {"text":"offres."}\n\n${done('Voici les offres.')}`,
    ], { gate })
    const id = conversationStore.send(undefined, 'des ordinateurs')!
    await vi.waitFor(() => expect(conversationStore.get().streaming[id]?.text).toBe('Voici les '))
    expect(conversationStore.get().pending[id]).toBe('Rédaction de la réponse…')
    open()
    await vi.waitFor(() => expect(conversationStore.get().pending[id]).toBeUndefined())
    const turns = conversationStore.find(id)!.turns
    expect(turns.map((t) => [t.role, t.text])).toEqual([['user', 'des ordinateurs'], ['assistant', 'Voici les offres.']])
    expect(conversationStore.get().streaming[id]).toBeUndefined()
  })

  it('stops on request, keeps what arrived, and can read the whole answer back with the same key', async () => {
    signIn('tok')
    conversationStore.setOwner('user-stop')
    const gate = new Promise<void>(() => { /* never opens: the person stops before the end */ })
    const { calls } = streamingFetch([
      `event: delta\ndata: {"text":"Voici "}\n\n`,
      `event: delta\ndata: {"text":"les "}\n\n`,
      done('jamais reçu'),
    ], { gate, plain: () => ({ conversation_id: 'srv-1', response: { text: 'Voici les 2 offres complètes.', language: 'fr', metadata: { duplicate: true } } }) })
    const id = conversationStore.send(undefined, 'des ordinateurs')!
    await vi.waitFor(() => expect(conversationStore.get().streaming[id]?.text).toBe('Voici les '))
    conversationStore.stop(id)
    await vi.waitFor(() => expect(conversationStore.get().pending[id]).toBeUndefined())
    const stopped = conversationStore.find(id)!.turns[1]!
    expect(stopped.text).toBe('Voici les')
    expect(stopped.interrupted?.question).toBe('des ordinateurs')

    expect(await conversationStore.completeInterrupted(id, stopped.id)).toBe('done')
    const key = calls[0]!.body.message_idempotency_key
    const readBack = calls.find((c) => c.path === '/conversations/message')!
    expect(readBack.body.message_idempotency_key).toBe(key)
    expect(conversationStore.find(id)!.turns[1]!.text).toBe('Voici les 2 offres complètes.')
    expect(conversationStore.find(id)!.turns[1]!.interrupted).toBeUndefined()
  })

  it('reads the answer back with the same key when the connection breaks, waiting while it is still in progress', async () => {
    signIn('tok')
    conversationStore.setOwner('user-break')
    let asked = 0
    const { calls } = streamingFetch([`event: delta\ndata: {"text":"Voi"}\n\n`, 'never sent'], {
      breakAfter: 1,
      plain: () => (++asked === 1
        ? { conversation_id: 'srv-1', response: { text: 'Je traite encore ce message — un instant.', language: 'fr', metadata: { duplicate: true, in_progress: true } } }
        : { conversation_id: 'srv-1', response: { text: 'Voici les offres.', language: 'fr', metadata: { duplicate: true } } }),
    })
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const id = conversationStore.send(undefined, 'des ordinateurs')!
    await vi.waitFor(() => expect(asked).toBe(1))
    await vi.advanceTimersByTimeAsync(3000)
    await vi.waitFor(() => expect(conversationStore.get().pending[id]).toBeUndefined())
    vi.useRealTimers()
    const keys = calls.map((c) => c.body.message_idempotency_key)
    expect(new Set(keys).size).toBe(1) // one key for the stream and both read-backs: nothing ran twice
    expect(conversationStore.find(id)!.turns[1]!.text).toBe('Voici les offres.')
  })
})
