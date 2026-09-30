import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

const raw = (id: string, title: string, price: number, store = 's1', currency = 'USD') =>
  ({ ID: id, OrganizationID: `org-${store}`, StoreID: store, Title: title, ListingType: 'physical_product', PriceAmount: price, PriceCurrency: currency, Status: 'published', Media: [] })
const catalogue: Record<string, ReturnType<typeof raw>> = {
  a: raw('a', 'Item A', 10), b: raw('b', 'Item B', 20), c: raw('c', 'Item C', 5),
  z: raw('z', 'Other seller item', 8, 's2'),
}
const bagItem = (id: string, quantity: number, seller = 'Shop one') =>
  ({ listing_id: id, title: catalogue[id].Title, price_amount: catalogue[id].PriceAmount, price_currency: catalogue[id].PriceCurrency, image: null, seller, quantity })
const seedBag = (...items: ReturnType<typeof bagItem>[]) => localStorage.setItem('trezutil_bag', JSON.stringify(items))

const orderDto = { order_id: 'o1', status: 'pending_payment', total: { AmountMinor: 2000, Currency: 'USD' }, currency: 'USD', customer_id: 'u1', items: 3 }
const orderRead = { ID: 'o1', OrganizationID: 'org-s1', Status: 'pending_payment', TotalAmount: { AmountMinor: 2000, Currency: 'USD' },
  Items: [{ ListingID: 'a', Title: 'Item A', Quantity: 2, UnitPrice: { AmountMinor: 1000 } }, { ListingID: 'b', Title: 'Item B', Quantity: 1, UnitPrice: { AmountMinor: 2000 } }] }

const api = (extra: Record<string, (c: never) => { status?: number; body?: unknown }> = {}) => mockApi({
  'GET /auth/users/me': () => ({ body: me }),
  'GET /listings': (c: { path: string }) => ({ body: catalogue[new URLSearchParams(c.path.split('?')[1]).get('listing_id') ?? ''] }),
  'POST /orders': () => ({ status: 201, body: orderDto }),
  'GET /orders/o1': () => ({ body: orderRead }),
  'POST /payments': () => ({ status: 503, body: { error: { code: 'PAYMENT_PROVIDER_UNAVAILABLE', message: 'x' } } }),
  ...extra,
} as never)

describe('bag checkout', () => {
  beforeEach(() => { signIn() })

  it('places the whole bag of one seller as a single order with one idempotency key', async () => {
    seedBag(bagItem('a', 2), bagItem('b', 1))
    const { calls } = api()
    renderAt('/checkout?bag=1')
    await userEvent.click(await screen.findByRole('button', { name: /place order/i }))
    expect(await screen.findByRole('heading', { name: 'Payment' })).toBeInTheDocument()
    const posts = calls.filter((c) => c.method === 'POST' && c.path === '/orders')
    expect(posts).toHaveLength(1)
    expect(posts[0].body).toEqual({ items: [{ listing_id: 'a', quantity: 2 }, { listing_id: 'b', quantity: 1 }] })
    expect(posts[0].headers['Idempotency-Key']).toBeTruthy()
    // The ordered items leave the bag.
    await waitFor(() => expect(JSON.parse(localStorage.getItem('trezutil_bag') ?? '[]')).toHaveLength(0))
  })

  it('sends the edited quantities', async () => {
    seedBag(bagItem('a', 2), bagItem('b', 1))
    const { calls } = api()
    renderAt('/checkout?bag=1')
    await userEvent.click(await screen.findByRole('button', { name: 'Increase quantity of Item B' }))
    await userEvent.click(screen.getByRole('button', { name: /place order/i }))
    await screen.findByRole('heading', { name: 'Payment' })
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')!.body).toEqual({ items: [{ listing_id: 'a', quantity: 2 }, { listing_id: 'b', quantity: 2 }] })
  })

  it('asks which seller to order from when the bag spans stores, and orders only that seller', async () => {
    seedBag(bagItem('a', 1), bagItem('z', 3, 'Shop two'))
    const { calls } = api()
    renderAt('/checkout?bag=1')
    expect(await screen.findByRole('heading', { name: 'Choose a seller' })).toBeInTheDocument()
    await userEvent.click(screen.getAllByRole('button', { name: 'Order these' })[1])
    await userEvent.click(await screen.findByRole('button', { name: /place order/i }))
    await screen.findByRole('heading', { name: 'Payment' })
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')!.body).toEqual({ items: [{ listing_id: 'z', quantity: 3 }] })
    // The other seller's item stays in the bag.
    expect(JSON.parse(localStorage.getItem('trezutil_bag') ?? '[]').map((b: { listing_id: string }) => b.listing_id)).toEqual(['a'])
  })

  it('shows the server’s refusal and keeps the bag when the order is rejected', async () => {
    seedBag(bagItem('a', 1), bagItem('b', 1))
    api({ 'POST /orders': () => ({ status: 409, body: { error: { code: 'LISTING_UNAVAILABLE', message: 'x' } } }) })
    renderAt('/checkout?bag=1')
    await userEvent.click(await screen.findByRole('button', { name: /place order/i }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(JSON.parse(localStorage.getItem('trezutil_bag') ?? '[]')).toHaveLength(2)
  })

  it('keeps single-listing checkout unchanged', async () => {
    const { calls } = api()
    renderAt('/checkout?listing_id=a&quantity=2')
    await userEvent.click(await screen.findByRole('button', { name: /place order/i }))
    await screen.findByRole('heading', { name: 'Payment' })
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')!.body).toEqual({ listing_id: 'a', quantity: 2 })
  })
})
