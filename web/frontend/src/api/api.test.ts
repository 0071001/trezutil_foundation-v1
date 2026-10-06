import { describe, expect, it, vi } from 'vitest'
import { http, SESSION_EXPIRED_EVENT, tokens, ApiError } from '@/api/client'
import { mapReply } from '@/api/conversations'
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
