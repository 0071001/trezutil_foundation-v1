import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

const listing = { ID: 'l1', OrganizationID: 'org1', StoreID: 's1', Title: 'Riverside apartment', ListingType: 'physical_product', PriceAmount: 85, PriceCurrency: 'USD', Status: 'published', Media: [] }

describe('authentication', () => {
  it('signs in with a phone code and returns to the original page', async () => {
    const { calls } = mockApi({
      'POST /auth/request-otp': () => ({ body: { request_id: 'req-1', expires_at: new Date(Date.now() + 300000).toISOString() } }),
      'POST /auth/verify-otp': () => ({ body: { user: me, tokens: { access_token: 'a', refresh_token: 'r', expires_at: '' } } }),
      'GET /auth/users/me': () => ({ body: me }),
      'GET /orders': () => ({ body: { items: [] } }),
    })
    renderAt('/login?returnTo=%2Forders')
    await userEvent.type(await screen.findByLabelText('Phone number'), '0812345678')
    await userEvent.click(screen.getByRole('button', { name: /send code/i }))
    expect(await screen.findByText('Check your phone')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/auth/request-otp')!.body).toEqual({ phone: '+243812345678' })
    fireEvent.paste(screen.getByLabelText('Digit 1'), { clipboardData: { getData: () => '123456' } })
    expect(await screen.findByRole('heading', { name: 'Orders' })).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/auth/verify-otp')!.body).toEqual({ request_id: 'req-1', code: '123456' })
    expect(localStorage.getItem('trezutil_access_token')).toBe('a')
  })

  it('explains rate limits and wrong codes', async () => {
    let attempts = 0
    mockApi({
      'POST /auth/request-otp': () => (attempts++ === 0 ? { status: 429, body: { error: { code: 'RATE_LIMITED', message: 'x' } } } : { body: { request_id: 'r', expires_at: new Date(Date.now() + 300000).toISOString() } }),
      'POST /auth/verify-otp': () => ({ status: 401, body: { error: { code: 'INVALID_CREDENTIALS', message: 'x' } } }),
    })
    renderAt('/login')
    await userEvent.type(await screen.findByLabelText('Phone number'), '+243812345678')
    await userEvent.click(screen.getByRole('button', { name: /send code/i }))
    expect(await screen.findByText(/too many codes requested/i)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /send code/i }))
    fireEvent.paste(await screen.findByLabelText('Digit 1'), { clipboardData: { getData: () => '000000' } })
    expect(await screen.findByText(/code is incorrect or has expired/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /resend code in/i })).toBeDisabled()
  })

  it('sends guests to sign-in with a return path', async () => {
    mockApi({})
    renderAt('/orders/o1')
    expect(await screen.findByText('Sign in to Trezutil')).toBeInTheDocument()
  })
})

describe('search & discovery', () => {
  it('shows live results, loads more with the cursor, and links to details', async () => {
    const { calls } = mockApi({
      'GET /categories': () => ({ body: { items: [] } }),
      'POST /search': (c) => ((c.body as { cursor?: string }).cursor
        ? { body: { items: [searchItem('l3', 'Garden studio')], total: 3, has_next: false } }
        : { body: { items: [searchItem('l1', 'Riverside apartment'), searchItem('l2', 'City loft')], total: 3, has_next: true, next_cursor: 'c2' } }),
    })
    renderAt('/search?q=apartment')
    expect(await screen.findByText('Riverside apartment')).toBeInTheDocument()
    expect(screen.getByText(/I found 3 options/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /show more results/i }))
    expect(await screen.findByText('Garden studio')).toBeInTheDocument()
    expect(calls.filter((c) => c.path === '/search').map((c) => (c.body as { text: string; cursor?: string }).cursor)).toEqual([undefined, 'c2'])
    expect(screen.getAllByRole('link', { name: 'Riverside apartment' })[0]).toHaveAttribute('href', '/products/l1')
  })

  it('shows an empty state and a retryable error', async () => {
    let fail = true
    mockApi({
      'GET /categories': () => ({ body: { items: [] } }),
      'POST /search': () => (fail ? { status: 503, body: { error: { code: 'SEARCH_FAILED', message: 'Search is temporarily unavailable' } } } : { body: { items: [], total: 0 } }),
    })
    renderAt('/search?q=zzz')
    expect(await screen.findByText('Search is temporarily unavailable')).toBeInTheDocument()
    fail = false
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('No results yet')).toBeInTheDocument()
  })
})

describe('conversational commerce', () => {
  it('shows structured results, keeps context and confirms an order with the token', async () => {
    signIn()
    const sent: string[] = []
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'POST /conversations/message': (c) => {
        const text = (c.body as { message_text: string }).message_text
        sent.push(text)
        if (text.startsWith('Find')) return { body: { conversation_id: 'c1', response: { text: 'Here is what I found (2):', metadata: { action_type: 'SEARCH_LISTINGS', result_data: { items: [searchItem('l1', 'Riverside apartment'), searchItem('l2', 'City loft')] } } } } }
        if (text === 'Show me the second one') return { body: { conversation_id: 'c1', response: { text: 'City loft — $85\nSay "book it" to reserve dates or "buy it" to order.', metadata: { action_type: 'GET_LISTING', result_data: { ID: 'l2', Title: 'City loft' } } } } }
        return { body: { conversation_id: 'c1', is_confirmation: true, response: { text: 'I need your confirmation before I proceed.', confirmation: { summary: 'Order City loft', details: { confirmation_token: 'confirm_1' } } } } }
      },
      'POST /conversations/c1/confirm': (c) => { expect(c.body).toEqual({ token: 'confirm_1' }); return { body: { conversation_id: 'c1', response: { text: 'Your order is created.', metadata: { action_type: 'CREATE_ORDER', result_data: { order_id: 'o7' } } } } } },
    })
    renderAt('/conversation?q=Find%20me%20a%20furnished%20apartment%20in%20Gombe%20under%20%24800')
    expect(await screen.findByText('Riverside apartment')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Open the second one' }))
    expect(await screen.findByText(/say "book it"/i)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Buy it' }))
    const confirmBox = await screen.findByRole('group', { name: /confirmation required/i })
    await userEvent.click(within(confirmBox).getByRole('button', { name: /confirm/i }))
    expect(await screen.findByRole('link', { name: /view order/i })).toHaveAttribute('href', '/orders/o7')
    expect(sent).toEqual(['Find me a furnished apartment in Gombe under $800', 'Show me the second one', 'Buy it'])
  })

  it('retries a failed message with the same idempotency key', async () => {
    const keys: string[] = []
    let fail = true
    mockApi({
      'POST /conversations/message': (c) => {
        keys.push((c.body as { message_idempotency_key: string }).message_idempotency_key)
        if (fail) { fail = false; return { status: 500, body: { error: { code: 'PROCESSING_FAILED', message: 'The assistant could not process this message' } } } }
        return { body: { conversation_id: 'c1', response: { text: 'Hello again' } } }
      },
    })
    renderAt('/conversation')
    await userEvent.type(await screen.findByLabelText('Message Trezutil'), 'hello{Enter}')
    expect(await screen.findByText('The assistant could not process this message')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('Hello again')).toBeInTheDocument()
    expect(keys[0]).toBe(keys[1])
  })
})

describe('order and checkout', () => {
  it('places an order then explains when online payment is not enabled', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /listings': () => ({ body: listing }),
      'POST /orders': () => ({ status: 201, body: { ID: 'o1', OrganizationID: 'org1', Status: 'pending_payment', TotalAmount: { AmountMinor: 17000, Currency: 'USD' }, Items: [{ ListingID: 'l1', Title: 'Riverside apartment', Quantity: 2 }] } }),
      'POST /payments': () => ({ status: 503, body: { error: { code: 'PAYMENT_PROVIDER_UNAVAILABLE', message: 'x' } } }),
    })
    renderAt('/checkout?listing_id=l1&quantity=2')
    await userEvent.click(await screen.findByRole('button', { name: /place order/i }))
    expect(await screen.findByRole('heading', { name: 'Payment' })).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/orders')!.body).toEqual({ listing_id: 'l1', quantity: 2 })
    expect(calls.find((c) => c.path === '/orders')!.headers['Idempotency-Key']).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: /pay securely/i }))
    expect(await screen.findByText(/online payment isn’t enabled for this seller yet/i)).toBeInTheDocument()
    const pay = calls.find((c) => c.path.startsWith('/payments'))!
    expect(pay.path).toBe('/payments?organization_id=org1')
    expect(pay.body).toMatchObject({ order_id: 'o1', amount_minor: 17000, currency: 'USD', idempotency_key: 'pay-o1' })
  })
})

describe('seller workspace', () => {
  const org = { organization_id: 'org1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', role: 'owner', seller_profiles: [{ id: 'sp1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', mine: true }], stores: [{ id: 's1', name: 'Gombe Stays', status: 'active' }] }

  it('lists the organization’s listings and publishes a draft', async () => {
    signIn()
    let published = false
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /listings/manage': () => ({ body: { items: [{ ...listing, Status: published ? 'published' : 'draft' }] } }),
      'POST /listings/publish': () => { published = true; return { body: { status: 'published' } } },
    })
    renderAt('/seller/listings')
    await userEvent.click(await screen.findByRole('tab', { name: /drafts/i }))
    await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))
    expect(await screen.findByText(/is live and searchable/i)).toBeInTheDocument()
    expect(calls.find((c) => c.path.startsWith('/listings/manage'))!.path).toContain('organization_id=org1')
    expect(calls.find((c) => c.path.startsWith('/listings/publish'))!.path).toBe('/listings/publish?listing_id=l1')
  })

  it('hides management actions from staff (the server enforces it too)', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [{ ...org, role: 'staff' }] } }),
      'GET /listings/manage': () => ({ body: { items: [listing] } }),
    })
    renderAt('/seller/listings')
    expect(await screen.findByText(/only its owner, admins and managers can make changes/i)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /create listing/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Pause' })).not.toBeInTheDocument()
  })

  it('shows onboarding to accounts without a business', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }), 'GET /sellers/me': () => ({ body: { organizations: [] } }) })
    renderAt('/seller')
    expect(await screen.findByText(/isn’t linked to a business yet/i)).toBeInTheDocument()
  })

  it('creates a listing through the six steps', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /categories': () => ({ body: { items: [{ id: 'cat1', name: 'Apartments', slug: 'apartments' }] } }),
      'POST /listings': () => ({ status: 201, body: { ...listing, ID: 'new1', Status: 'draft' } }),
      'POST /listings/publish': () => ({ body: { status: 'published' } }),
    })
    renderAt('/seller/listings/new')
    await userEvent.type(await screen.findByLabelText('Listing name'), 'Sea view flat')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    await userEvent.selectOptions(await screen.findByLabelText('Category'), 'cat1')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    await userEvent.type(await screen.findByLabelText('Price'), '120')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    await userEvent.type(await screen.findByLabelText('Image links'), 'javascript:alert(1)')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    expect(await screen.findByText(/must be public https/i)).toBeInTheDocument()
    await userEvent.clear(screen.getByLabelText('Image links'))
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    await userEvent.click(await screen.findByRole('button', { name: /save and continue/i }))
    await userEvent.click(await screen.findByRole('button', { name: /publish listing/i }))
    expect(await screen.findByText('Your listing is live')).toBeInTheDocument()
    const create = calls.find((c) => c.method === 'POST' && c.path === '/listings')!.body as { listing: Record<string, unknown> }
    expect(create.listing).toMatchObject({ Title: 'Sea view flat', CategoryID: 'cat1', PriceAmount: 120, StoreID: 's1', SellerID: 'sp1' })
    expect(create.listing).not.toHaveProperty('OrganizationID')
    expect(create.listing).not.toHaveProperty('Status')
  })
})

describe('failure handling and navigation', () => {
  it('renders a 404 page for unknown routes', async () => {
    mockApi({})
    renderAt('/nope')
    expect(await screen.findByText('This page doesn’t exist')).toBeInTheDocument()
  })

  it('shows a forbidden state when the API refuses access', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }), 'GET /orders/o9': () => ({ status: 403, body: { error: { code: 'FORBIDDEN', message: 'no' } } }) })
    renderAt('/orders/o9')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
  })

  it('keeps saved items and the bag on the device', async () => {
    mockApi({ 'GET /listings': () => ({ body: listing }), 'GET /stores': () => ({ body: { Name: 'Gombe Stays' } }), 'POST /search': () => ({ body: { items: [] } }) })
    renderAt('/products/l1')
    await userEvent.click(await screen.findByRole('button', { name: /add to bag/i }))
    expect(await screen.findByText(/added to your bag/i)).toBeInTheDocument()
    await waitFor(() => expect(JSON.parse(localStorage.getItem('trezutil_bag') ?? '[]')).toHaveLength(1))
  })
})
