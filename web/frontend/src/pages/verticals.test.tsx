import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

/* The web app follows the backend vertical catalogue (GET /verticals). */

const catalogue = {
  items: [
    { code: 'RETAIL', name: 'Retail', description: '', flows: ['BUY'], capabilities: ['ORDERING'], listing_types: ['physical_product', 'vehicle', 'food', 'other'] },
    { code: 'HOTEL', name: 'Hotels & accommodation', description: '', flows: ['BOOK'], capabilities: ['RESERVATION', 'ORDERING'], listing_types: ['real_estate', 'service', 'other'] },
    { code: 'RESTAURANT', name: 'Restaurants', description: '', flows: ['BOOK', 'BUY'], capabilities: ['RESERVATION', 'ORDERING'], listing_types: ['food', 'service', 'other'] },
    { code: 'SERVICES', name: 'Services', description: '', flows: ['BOOK'], capabilities: ['RESERVATION'], listing_types: ['service', 'other'] },
  ],
}

const listing = (vertical: string, type: string) => ({
  ID: 'l1', OrganizationID: 'org1', SellerID: 'sp1', StoreID: 's1', CategoryID: 'cat1', Title: 'Deluxe room',
  Description: 'Quiet room', ListingType: type, Vertical: vertical, PricingModel: 'fixed', PriceAmount: 90,
  PriceCurrency: 'USD', Status: 'published', Metadata: {}, Media: [],
})

function detail(vertical: string, type: string, verticals: 'ok' | 'down' = 'ok') {
  mockApi({
    'GET /listings': () => ({ body: listing(vertical, type) }),
    'GET /stores': () => ({ body: { Name: 'Gombe Stays', City: 'Kinshasa', Status: 'active' } }),
    'POST /search': () => ({ body: { items: [] } }),
    'GET /verticals': () => (verticals === 'ok' ? { body: catalogue } : { status: 503, body: { error: { code: 'DOWN', message: 'down' } } }),
  })
}

beforeEach(() => resetVerticals())

describe('vertical-aware listing detail', () => {
  it('opens the hotel page for a hotel room type, whatever its listing type', async () => {
    mockApi({
      'GET /listings': () => ({ body: listing('HOTEL', 'real_estate') }),
      'GET /stores': () => ({ body: {} }),
      'POST /search': () => ({ body: { items: [] } }),
      'GET /verticals': () => ({ body: catalogue }),
      'GET /hotels/property': () => ({ body: { store_id: 's1', name: 'Hotel du Fleuve', city: 'Kinshasa', address: '', room_types: [{ listing_id: 'l1', title: 'Deluxe room', nightly_price: 90, currency: 'USD', max_occupancy: 2, amenities: [], images: [], units: 3 }] } }),
    })
    renderAt('/hotels/l1')
    expect(await screen.findByRole('heading', { name: 'Choose your room' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /check availability/i })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /order instead/i })).not.toBeInTheDocument()
  })

  it('offers both flows for a restaurant: reserve a table, or order', async () => {
    detail('RESTAURANT', 'food')
    renderAt('/restaurants/l1')
    expect(await screen.findByRole('button', { name: /reserve a table/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /order instead/i })).toBeInTheDocument()
  })

  it('sells a retail product', async () => {
    detail('RETAIL', 'physical_product')
    renderAt('/products/l1')
    expect(await screen.findByRole('button', { name: /add to bag/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /buy now/i })).toBeInTheDocument()
  })

  it('falls back to the listing type when the catalogue cannot be loaded', async () => {
    // Degraded: flows come from the legacy listing-type guess (food → book only).
    detail('RESTAURANT', 'food', 'down')
    renderAt('/restaurants/l1')
    expect(await screen.findByRole('button', { name: /reserve a table/i })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /order instead/i })).not.toBeInTheDocument()
  })
})

describe('vertical-aware search', () => {
  it('filters by vertical and location, and links results to their vertical page', async () => {
    const { calls } = mockApi({
      'GET /verticals': () => ({ body: catalogue }),
      'GET /categories': () => ({ body: { items: [] } }),
      'POST /search': () => ({ body: { items: [searchItem('r1', 'Deluxe room', { Vertical: 'HOTEL' })], total: 1 } }),
    })
    renderAt('/search?vertical=HOTEL&where=Gombe')
    const link = (await screen.findAllByRole('link', { name: 'Deluxe room' }))[0]
    expect(link.getAttribute('href')).toBe('/hotels/r1')
    const body = calls.find((c) => c.method === 'POST' && c.path === '/search')!.body as Record<string, unknown>
    expect(body).toMatchObject({ vertical: 'HOTEL', location: 'Gombe' })
    expect(await screen.findByRole('radio', { name: 'Hotels & accommodation' })).toBeChecked()
  })

  it('maps old ?type= links onto verticals', async () => {
    const { calls } = mockApi({
      'GET /verticals': () => ({ body: catalogue }),
      'GET /categories': () => ({ body: { items: [] } }),
      'POST /search': () => ({ body: { items: [], total: 0 } }),
    })
    renderAt('/search?type=food')
    await waitFor(() => expect(calls.some((c) => c.path === '/search')).toBe(true))
    expect((calls.find((c) => c.path === '/search')!.body as Record<string, unknown>).vertical).toBe('RESTAURANT')
  })
})

describe('seller chooses a vertical', () => {
  const org = { organization_id: 'org1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', role: 'owner', seller_profiles: [{ id: 'sp1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', mine: true }], stores: [{ id: 's1', name: 'Gombe Stays', status: 'active' }] }

  it('sends the chosen vertical and a listing kind that vertical accepts', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /verticals': () => ({ body: catalogue }),
      'GET /categories': () => ({ body: { items: [{ id: 'cat1', name: 'Dining', slug: 'dining' }] } }),
      'POST /listings': () => ({ status: 201, body: { ...listing('RESTAURANT', 'food'), ID: 'new1', Status: 'draft' } }),
    })
    renderAt('/seller/listings/new')
    await userEvent.type(await screen.findByLabelText('Listing name'), 'Grilled fish')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    // A vertical is required before continuing.
    await userEvent.selectOptions(await screen.findByLabelText('Category'), 'cat1')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    expect(await screen.findByText(/choose the business vertical/i)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: 'Restaurants' }))
    expect(screen.getByLabelText('Kind of listing')).toHaveValue('food')
    await userEvent.click(screen.getByRole('button', { name: /save and continue/i }))
    await userEvent.type(await screen.findByLabelText('Price'), '12')
    for (let i = 0; i < 3; i++) await userEvent.click(await screen.findByRole('button', { name: /save and continue/i }))
    await userEvent.click(await screen.findByRole('button', { name: /save as draft/i }))
    expect(await screen.findByText('Draft saved')).toBeInTheDocument()
    const create = calls.find((c) => c.method === 'POST' && c.path === '/listings')!.body as { listing: Record<string, unknown> }
    expect(create.listing).toMatchObject({ Title: 'Grilled fish', Vertical: 'RESTAURANT', ListingType: 'food' })
  })
})
