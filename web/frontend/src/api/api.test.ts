import { describe, expect, it, vi } from 'vitest'
import { http, SESSION_EXPIRED_EVENT, tokens, ApiError } from '@/api/client'
import { conversationsApi, mapReply } from '@/api/conversations'
import { creditsApi } from '@/api/credits'
import { mapOrder } from '@/api/commerce'
import { mapListing } from '@/api/listings'
import { mapSearchResponse } from '@/api/search'
import { safeImage } from '@/lib/format'
import { safeReturnTo } from '@/lib/session'
import { mockApi, searchItem, signIn } from '@/test/utils'

describe('http client', () => {
  it('sends the bearer token and never an identity in the body', async () => {
    signIn('tok')
    const { calls } = mockApi({ 'GET /orders': () => ({ body: { items: [] } }) })
    await http.get('/orders')
    expect(calls[0].headers.Authorization).toBe('Bearer tok')
  })

  it('sends sign-in calls without the stored token, which may have expired long ago', async () => {
    signIn('stale')
    const { calls } = mockApi({
      'POST /auth/staff/login': () => ({ body: { tokens: { access_token: 'fresh', refresh_token: 'r' }, user: {} } }),
      'POST /auth/request-otp': () => ({ body: {} }),
      'POST /auth/verify-otp': () => ({ body: { tokens: { access_token: 'fresh', refresh_token: 'r' }, user: {} } }),
      'GET /orders': () => ({ body: { items: [] } }),
    })
    await http.post('/auth/staff/login', { email: 'a@b.co', password: 'x' })
    await http.post('/auth/request-otp', { phone: '+1' })
    await http.post('/auth/verify-otp', { request_id: 'r', code: '1' })
    await http.get('/orders')
    expect(calls.map((c) => c.headers.Authorization)).toEqual([undefined, undefined, undefined, 'Bearer stale'])
  })

  it('refreshes once on 401 and replays the request', async () => {
    signIn('old', 'r1')
    let first = true
    const { calls } = mockApi({
      'GET /orders': (c) => {
        if (first) { first = false; return { status: 401, body: { error: { code: 'TOKEN_EXPIRED' } } } }
        expect(c.headers.Authorization).toBe('Bearer new')
        return { body: { items: [1] } }
      },
      'POST /auth/refresh': (c) => { expect(c.body).toEqual({ refresh_token: 'r1' }); return { body: { tokens: { access_token: 'new', refresh_token: 'r2' } } } },
    })
    await expect(http.get('/orders')).resolves.toEqual({ items: [1] })
    expect(calls.map((c) => c.path)).toEqual(['/orders', '/auth/refresh', '/orders'])
    expect(tokens.refresh()).toBe('r2')
  })

  it('clears the session and announces expiry when the refresh fails', async () => {
    signIn('old', 'r1')
    mockApi({ 'GET /orders': () => ({ status: 401 }), 'POST /auth/refresh': () => ({ status: 401 }) })
    const expired = vi.fn()
    window.addEventListener(SESSION_EXPIRED_EVENT, expired)
    await expect(http.get('/orders')).rejects.toMatchObject({ status: 401 })
    expect(expired).toHaveBeenCalledTimes(1)
    expect(tokens.access()).toBeNull()
    window.removeEventListener(SESSION_EXPIRED_EVENT, expired)
  })

  it('maps the backend error envelope and network failures', async () => {
    mockApi({ 'POST /orders': () => ({ status: 409, body: { error: { code: 'LISTING_UNAVAILABLE', message: 'Sold out' } } }) })
    await expect(http.post('/orders', {})).rejects.toMatchObject({ status: 409, code: 'LISTING_UNAVAILABLE', message: 'Sold out' })
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    const err = await http.get('/orders').catch((e: ApiError) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).isNetwork).toBe(true)
    expect((err as ApiError).code).toBe('NETWORK_ERROR')
  })

  it('shares one request between identical concurrent GETs', async () => {
    const { fetchMock } = mockApi({ 'GET /categories': () => ({ body: { items: [] } }) })
    await Promise.all([http.get('/categories'), http.get('/categories'), http.get('/categories')])
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('sends idempotency keys', async () => {
    const { calls } = mockApi({ 'POST /orders': () => ({ status: 201, body: {} }) })
    await http.post('/orders', { listing_id: 'l1', quantity: 1 }, { idempotencyKey: 'k-1' })
    expect(calls[0].headers['Idempotency-Key']).toBe('k-1')
  })
})

describe('mappers', () => {
  it('reads PascalCase search results', () => {
    const r = mapSearchResponse({ items: [searchItem('l1', 'Flat')], total: 1, has_next: true, next_cursor: 'c2' })
    expect(r.items[0]).toMatchObject({ listing_id: 'l1', title: 'Flat', price_amount: 85, store: { name: 'Gombe Stays' }, location: { neighborhood: 'Gombe' } })
    expect(r.next_cursor).toBe('c2')
  })

  it('reads listing media added next to the listing fields', () => {
    const l = mapListing({ ID: 'l1', Title: 'Flat', ListingType: 'real_estate', Status: 'published', Media: [{ URL: 'https://x/b.jpg', Primary: false }, { URL: 'https://x/a.jpg', Primary: true }] })
    expect(l.media[0].url).toBe('https://x/a.jpg')
    expect(l.listing_type).toBe('real_estate')
  })

  it('maps orders with totals in minor units', () => {
    const o = mapOrder({ ID: 'o1', Status: 'paid', TotalAmount: { AmountMinor: 17100, Currency: 'USD' }, Items: [{ ListingID: 'l1', Title: 'Flat', Quantity: 2 }] })
    expect(o).toMatchObject({ id: 'o1', total_minor: 17100, currency: 'USD', title: 'Flat', quantity: 2 })
  })

  it('maps conversation cards, confirmation tokens and created entities', () => {
    const reply = mapReply({
      conversation_id: 'c1', is_confirmation: true, requires_user_input: true,
      response: {
        text: 'Please confirm',
        cards: [{ id: 'l1', type: 'listing', title: 'Flat', description: '$85', data: { listing_id: 'l1', position: 1 } }],
        confirmation: { summary: 'Order 1 × Flat', details: { confirmation_token: 'confirm_x', action_type: 'CREATE_ORDER' } },
        metadata: { action_type: 'CREATE_ORDER', result_data: { order_id: 'o9' } },
      },
    })
    expect(reply.cards[0]).toMatchObject({ listing_id: 'l1', position: 1 })
    expect(reply.confirmation).toEqual({ summary: 'Order 1 × Flat', token: 'confirm_x', action_type: 'CREATE_ORDER' })
    expect(reply.refs.order_id).toBe('o9')
  })
})

describe('safety helpers', () => {
  it('only accepts same-site return paths', () => {
    expect(safeReturnTo('/orders/1')).toBe('/orders/1')
    expect(safeReturnTo('https://evil.example')).toBe('/')
    expect(safeReturnTo('//evil.example')).toBe('/')
    expect(safeReturnTo('/\\evil.example')).toBe('/')
    expect(safeReturnTo('/login?returnTo=/x')).toBe('/')
  })

  it('only renders http(s) media', () => {
    expect(safeImage('javascript:alert(1)')).toBeNull()
    expect(safeImage('data:image/svg+xml;base64,AAA')).toBeNull()
    expect(safeImage('https://cdn.example/a.jpg')).toBe('https://cdn.example/a.jpg')
  })
})

describe('conversation history on the server', () => {
  it('reads back the cards and results an answer showed', async () => {
    signIn('tok')
    mockApi({
      'GET /conversations/c-1/messages': () => ({ body: { items: [
        { id: 'm1', direction: 'inbound', text: 'Des ordinateurs Dell', created_at: 10 },
        { id: 'm2', direction: 'outbound', text: 'J’ai trouvé 1 offre.', created_at: 11, response: {
          cards: [{ id: 'lst-1', type: 'listing', title: 'Dell Latitude', description: '', data: { listing_id: 'lst-1', position: 1 } }],
          metadata: { action_type: 'SEARCH_LISTINGS', result_data: { items: [{ id: 'lst-1' }] }, suggestions: ['Moins cher'] },
        } },
      ] } }),
    })
    const messages = await conversationsApi.messages('c-1')
    expect(messages[0].reply).toBeUndefined()
    expect(messages[1].reply?.cards.map((c) => c.listing_id)).toEqual(['lst-1'])
    expect(messages[1].reply?.result_items).toHaveLength(1)
    expect(messages[1].reply?.suggestions).toEqual(['Moins cher'])
    expect(messages[1].reply?.confirmation).toBeUndefined()
  })

  it('renames, archives and deletes on the server', async () => {
    signIn('tok')
    const { calls } = mockApi({
      'PATCH /conversations/c-1': () => ({ body: { id: 'c-1' } }),
      'DELETE /conversations/c-1': () => ({ body: {} }),
    })
    await conversationsApi.update('c-1', { title: 'Achat bureau', archived: true })
    await conversationsApi.remove('c-1')
    expect(calls.map((c) => `${c.method} ${c.body ? JSON.stringify(c.body) : ''}`.trim())).toEqual([
      'PATCH {"title":"Achat bureau","archived":true}', 'DELETE',
    ])
  })
})

describe('personal credits', () => {
  it('reads the real personal balance and whether actions are charged', async () => {
    signIn('tok')
    mockApi({ 'GET /me/credits': () => ({ body: {
      period_id: '2026-11',
      personal: { opened: true, granted: 100, reserved: 5, consumed: 12, balance: 83 },
      ledger: [{ id: 'l1', action_type: 'SEARCH_LISTINGS', kind: 'consume', amount: -3, created_at: '2026-11-22T10:00:00Z' }],
      metering: { active: true, costs: [{ action_type: 'SEARCH_LISTINGS', channel: 'web', cost: 3 }] },
    } }) })
    const c = await creditsApi.mine()
    expect([c.balance, c.granted, c.reserved, c.consumed, c.opened, c.meteringActive]).toEqual([83, 100, 5, 12, true, true])
    expect(c.ledger[0]!.amount).toBe(-3)
    expect(c.costs).toEqual([{ actionType: 'SEARCH_LISTINGS', channel: 'web', cost: 3 }])
  })
})
