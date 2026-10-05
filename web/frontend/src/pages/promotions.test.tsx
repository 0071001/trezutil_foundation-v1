import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

/* Promotions: the sale on cards, search, detail and the seller console. The home page has no promotions section. */

type SearchBody = { vertical?: string; on_promotion?: boolean; sort_field?: string; page_size?: number }

const sale = (id: string, title: string, price: number, compareAt: number) => searchItem(id, title, { PriceAmount: price, CompareAtAmount: compareAt, DiscountPercent: 99 })
const regular = (id: string, title: string) => searchItem(id, title)

/** A server that honours on_promotion like the real one: only sale items for the deals request. */
function searchServer(deals: unknown[] = [sale('d1', 'Smart TV', 300, 400), sale('d2', 'Sofa', 90, 100), sale('d3', 'Kettle', 18, 20), sale('d4', 'Lamp', 40, 50)]) {
  return mockApi({
    'POST /search': (c) => {
      const body = c.body as SearchBody
      if (body.on_promotion) return { body: { items: deals, total: deals.length, has_next: false } }
      // Like the real server: as many products as were asked for.
      return { body: { items: Array.from({ length: body.page_size ?? 4 }, (_, i) => regular(`${body.vertical ?? 'new'}-${i + 1}`, `${body.vertical ?? 'New'} item ${i + 1}`)), total: 40, has_next: true } }
    },
  })
}

beforeEach(() => resetVerticals())

describe('homepage has no slideshow and no promotions section', () => {
  it('goes from the welcome straight to what we can help with, with no carousel', async () => {
    searchServer()
    renderAt('/')
    await screen.findByRole('region', { name: 'What can we help you with?' })
    expect(document.querySelector('.carousel, [aria-roledescription="carousel"]')).toBeNull()
    expect(screen.queryByRole('button', { name: /slide/i })).toBeNull()
  })

  it('has no promotions section, and does not ask the server for sales to build one', async () => {
    const { calls } = searchServer()
    renderAt('/')
    await screen.findByRole('region', { name: 'Discover what exists around you' })
    expect(screen.queryByRole('region', { name: 'On promotion' })).toBeNull()
    expect(screen.queryByText(/up to .*% off/i)).toBeNull()
    expect(screen.queryByRole('link', { name: 'See all deals' })).toBeNull()
    expect(calls.some((c) => (c.body as SearchBody)?.on_promotion)).toBe(false)
  })
})

describe('search with promotions', () => {
  it('asks only for sales when opened from the deals link, ordered by saving', async () => {
    const { calls } = searchServer()
    renderAt('/search?promo=1&sort=discount')
    expect(await screen.findByRole('heading', { name: 'Smart TV' })).toBeInTheDocument()
    const asked = calls.find((c) => c.method === 'POST' && c.path.startsWith('/search'))!.body as SearchBody
    expect(asked).toMatchObject({ on_promotion: true, sort_field: 'discount' })
    expect(screen.getByRole('checkbox', { name: 'On promotion' })).toBeChecked()
    expect(screen.getByRole('combobox', { name: /sort by/i })).toHaveValue('discount')
    expect(screen.getAllByRole('img', { name: /% off/ })).toHaveLength(4)
    expect(await screen.findByText(/I found 4 options for deals/)).toBeInTheDocument()
  })

  it('turns the filter on and off from the filter panel', async () => {
    const { calls } = searchServer()
    renderAt('/search')
    await screen.findByRole('heading', { name: 'New item 1' })
    expect(screen.getByRole('checkbox', { name: 'On promotion' })).not.toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: 'On promotion' }))
    expect(await screen.findByRole('heading', { name: 'Smart TV' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'New item 1' })).toBeNull()
    await userEvent.click(screen.getByRole('checkbox', { name: 'On promotion' }))
    expect(await screen.findByRole('heading', { name: 'New item 1' })).toBeInTheDocument()
    const last = [...calls].reverse().find((c) => c.method === 'POST' && c.path.startsWith('/search'))!.body as SearchBody
    expect(last.on_promotion).toBeUndefined()
  })

  it('says so when there are no deals', async () => {
    searchServer([])
    renderAt('/search?promo=1')
    expect(await screen.findByText('No deals right now')).toBeInTheDocument()
  })

  it('never lists a regular-price product under deals', async () => {
    searchServer([regular('r1', 'Plain phone'), sale('d1', 'Smart TV', 300, 400)])
    renderAt('/search?promo=1')
    expect(await screen.findByRole('heading', { name: 'Smart TV' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Plain phone' })).toBeNull()
  })
})

describe('listing page', () => {
  const retail = (price: number, metadata: unknown) => ({
    ID: 'l1', OrganizationID: 'org1', SellerID: 'sp1', StoreID: 's1', CategoryID: 'cat1', Title: 'Smart TV',
    Description: '4K', ListingType: 'physical_product', Vertical: 'RETAIL', PricingModel: 'fixed', PriceAmount: price,
    PriceCurrency: 'USD', Status: 'published', Metadata: metadata, Media: [],
  })
  const server = (listing: unknown) => mockApi({
    'GET /listings': () => ({ body: listing }),
    'GET /stores': () => ({ body: { Name: 'Gombe Shop', City: 'Kinshasa', Status: 'active' } }),
    'POST /search': () => ({ body: { items: [] } }),
    'GET /verticals': () => ({ body: { items: [{ code: 'RETAIL', name: 'Retail', description: '', flows: ['BUY'], capabilities: ['ORDERING'], listing_types: ['physical_product'] }] } }),
  })

  it('shows the sale price, the struck-through regular price and the saving', async () => {
    server(retail(300, { promotion: { compare_at_amount: 400 } }))
    renderAt('/products/l1')
    expect(await screen.findByRole('heading', { name: 'Smart TV', level: 1 })).toBeInTheDocument()
    const row = document.querySelector('.detail-price-row')!
    expect(row.querySelector('.price')).toHaveTextContent('$300')
    expect(row.querySelector('s.was-price')).toHaveTextContent('$400')
    expect(within(row as HTMLElement).getByRole('img', { name: '25% off' })).toBeInTheDocument()
  })

  it('shows a plain price when there is no sale', async () => {
    server(retail(300, {}))
    renderAt('/products/l1')
    await screen.findByRole('heading', { name: 'Smart TV', level: 1 })
    const row = document.querySelector('.detail-price-row')!
    expect(row.querySelector('.price')).toHaveTextContent('$300')
    expect(row.querySelector('s')).toBeNull()
    expect(within(row as HTMLElement).queryByRole('img')).toBeNull()
  })
})

describe('seller promotion panel', () => {
  const org = { organization_id: 'org1', display_name: 'Gombe Shop', status: 'active', verification_status: 'verified', role: 'owner', seller_profiles: [{ id: 'sp1', display_name: 'Gombe Shop', status: 'active', verification_status: 'verified', mine: true }], stores: [{ id: 's1', name: 'Gombe Shop', status: 'active' }] }
  const listing = (price: number, metadata: unknown = {}, status = 'published') => ({ ID: 'l1', OrganizationID: 'org1', StoreID: 's1', Title: 'Smart TV', ListingType: 'physical_product', PriceAmount: price, PriceCurrency: 'USD', Status: status, Metadata: metadata, Media: [] })

  function sellerServer(initial: ReturnType<typeof listing>) {
    let current = initial
    const api = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /listings/manage': () => ({ body: { items: [current] } }),
      'POST /listings/promotion': (c) => {
        const sale = (c.body as { sale_price: number }).sale_price
        current = listing(sale, { promotion: { compare_at_amount: 400 } })
        return { body: current }
      },
      'POST /listings/promotion/end': () => { current = listing(400); return { body: current } },
    })
    return api
  }

  it('puts a live listing on sale and ends the sale', async () => {
    signIn()
    const { calls } = sellerServer(listing(400))
    renderAt('/seller/listings/l1')
    expect(await screen.findByText(/not on promotion/i)).toBeInTheDocument()

    const input = screen.getByLabelText(/sale price/i)
    const start = screen.getByRole('button', { name: 'Start promotion' })
    expect(start).toBeDisabled()

    // Not a saving: refused before anything is sent.
    await userEvent.type(input, '400')
    expect(start).toBeDisabled()
    expect(await screen.findByText(/below \$400/i)).toBeInTheDocument()

    await userEvent.clear(input)
    await userEvent.type(input, '300')
    expect(start).toBeEnabled()
    await userEvent.click(start)
    expect(await screen.findByText('The promotion is live.')).toBeInTheDocument()
    const call = calls.find((c) => c.path.startsWith('/listings/promotion') && !c.path.includes('/end'))!
    expect(call.path).toBe('/listings/promotion?listing_id=l1')
    expect(call.body).toEqual({ sale_price: 300 })

    // The panel now shows the sale, and the sale can be ended.
    expect(await screen.findByText(/25% off the regular price of \$400/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'End promotion' }))
    expect(await screen.findByText(/the regular price is back/i)).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/listings/promotion/end?listing_id=l1')).toBe(true)
    await waitFor(() => expect(screen.getByText(/not on promotion/i)).toBeInTheDocument())
  })

  it('does not offer a sale on a listing that is sold or closed', async () => {
    signIn()
    sellerServer(listing(400, {}, 'closed'))
    renderAt('/seller/listings/l1')
    await screen.findByRole('heading', { name: 'Smart TV', level: 1 })
    expect(screen.queryByRole('button', { name: 'Start promotion' })).toBeNull()
    expect(screen.queryByText(/not on promotion/i)).toBeNull()
  })

  it('shows staff the promotion read-only', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [{ ...org, role: 'staff' }] } }),
      'GET /listings/manage': () => ({ body: { items: [listing(300, { promotion: { compare_at_amount: 400 } })] } }),
    })
    renderAt('/seller/listings/l1')
    expect(await screen.findByText(/25% off the regular price of \$400/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /promotion/i })).toBeNull()
    expect(screen.queryByLabelText(/sale price/i)).toBeNull()
  })

  it('shows the server’s refusal instead of pretending it worked', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /listings/manage': () => ({ body: { items: [listing(400)] } }),
      'POST /listings/promotion': () => ({ status: 400, body: { error: { code: 'INVALID_PARAMETER', message: 'sale price must be below the regular price' } } }),
    })
    renderAt('/seller/listings/l1')
    await userEvent.type(await screen.findByLabelText(/sale price/i), '300')
    await userEvent.click(screen.getByRole('button', { name: 'Start promotion' }))
    expect(await screen.findByText(/sale price must be below the regular price/i)).toBeInTheDocument()
    expect(screen.queryByText('The promotion is live.')).toBeNull()
  })
})
