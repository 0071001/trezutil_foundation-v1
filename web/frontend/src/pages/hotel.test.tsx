import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { dateInput, nightStart } from '@/api/hotels'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/* Hotel Commerce Pack in the web app: customer stay journey and seller rooms. */

const room = (id: string, title: string, extra: Record<string, unknown> = {}) => ({
  listing_id: id, organization_id: 'org1', store_id: 's1', title, description: 'Quiet, river view', status: 'published',
  nightly_price: 90, currency: 'USD', max_occupancy: 2, bed_configuration: '1 king bed', amenities: ['Wi-Fi', 'Breakfast'],
  units: 3, images: [], ...extra,
})
const property = { store_id: 's1', name: 'Hotel du Fleuve', city: 'Kinshasa', address: 'Avenue du Fleuve, Gombe', room_types: [room('r1', 'Deluxe room'), room('r2', 'Suite', { nightly_price: 150, max_occupancy: 4 })] }
const listing = { ID: 'r1', OrganizationID: 'org1', SellerID: 'sp1', StoreID: 's1', CategoryID: 'c1', Title: 'Deluxe room', ListingType: 'real_estate', Vertical: 'HOTEL', PricingModel: 'fixed', PriceAmount: 90, PriceCurrency: 'USD', Status: 'published', Metadata: {}, Media: [] }
const offer = (r: ReturnType<typeof room>, available: boolean, left: number) => ({ ...r, rooms: 1, available_rooms: left, available, fits_guests: true, nights: 2, total_price: r.nightly_price * 2, reason: available ? '' : 'insufficient capacity' })

beforeEach(() => resetVerticals())

describe('customer hotel stay', () => {
  it('checks every room type for the dates and guests, then reserves the chosen room', async () => {
    const { calls } = mockApi({
      'GET /listings': () => ({ body: listing }),
      'GET /stores': () => ({ body: { Name: 'Hotel du Fleuve' } }),
      'POST /search': () => ({ body: { items: [] } }),
      'GET /verticals': () => ({ status: 503, body: {} }),
      'GET /hotels/property': () => ({ body: property }),
      'GET /hotels/availability': () => ({ body: { check_in: dateInput(1), check_out: dateInput(3), nights: 2, guests: 2, offers: [offer(property.room_types[0], false, 0), offer(property.room_types[1], true, 2)] } }),
    })
    renderAt('/hotels/r1')
    expect(await screen.findByRole('heading', { level: 1, name: 'Hotel du Fleuve' })).toBeInTheDocument()
    expect(screen.getByText(/Avenue du Fleuve, Gombe/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^reserve$/i })).toBeDisabled()

    await userEvent.click(screen.getByRole('button', { name: /check availability/i }))
    expect(await screen.findByText(/1 room type available for 2 nights, 2 guests/i)).toBeInTheDocument()
    const q = calls.find((c) => c.path.startsWith('/hotels/availability'))!
    expect(q.path).toContain('store_id=s1')
    expect(q.path).toContain('guests=2')
    // The sold-out room type cannot be selected; the free one is selected for the guest.
    const deluxe = screen.getByRole('heading', { name: 'Deluxe room' }).closest('article')!
    expect(within(deluxe).getByRole('button', { name: /select room/i })).toBeDisabled()
    expect(within(deluxe).getByText(/insufficient capacity/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /reserve suite/i })).toBeEnabled()
  })

  it('refuses impossible stays before asking the server', async () => {
    const { calls } = mockApi({
      'GET /listings': () => ({ body: listing }),
      'GET /stores': () => ({ body: {} }),
      'POST /search': () => ({ body: { items: [] } }),
      'GET /verticals': () => ({ status: 503, body: {} }),
      'GET /hotels/property': () => ({ body: property }),
    })
    renderAt('/hotels/r1')
    const checkOut = await screen.findByLabelText('Check-out')
    await userEvent.clear(checkOut)
    await userEvent.type(checkOut, dateInput(1))
    await userEvent.click(screen.getByRole('button', { name: /check availability/i }))
    expect(await screen.findByText(/check-out must be after check-in/i)).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/hotels/availability'))).toBe(false)
  })

  it('books whole nights and several rooms, then pays through the stay order', async () => {
    signIn()
    const checkIn = dateInput(5), checkOut = dateInput(7)
    const reservation = { ID: 'res1', OrganizationID: 'org1', ListingID: 'r1', Status: 'confirmed', StartAt: nightStart(checkIn), EndAt: nightStart(checkOut), Quantity: 2, PartySize: 4 }
    const order = { ID: 'o1', OrganizationID: 'org1', Status: 'pending_payment', Currency: 'USD', TotalAmount: { AmountMinor: 36000, Currency: 'USD' }, Items: [{ ListingID: 'r1', Title: 'Deluxe room', Quantity: 4, UnitPrice: { AmountMinor: 9000 } }] }
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /listings': () => ({ body: listing }),
      'GET /hotels/availability': () => ({ body: { check_in: checkIn, check_out: checkOut, nights: 2, guests: 4, offers: [{ ...offer(property.room_types[0], true, 3), rooms: 2, total_price: 360 }] } }),
      'POST /reservations': () => ({ status: 201, body: { reservation_id: 'res1', status: 'confirmed' } }),
      'GET /reservations/res1': () => ({ body: reservation }),
      'POST /reservations/res1/checkout': () => ({ status: 201, body: { order_id: 'o1' } }),
      'GET /orders/o1': () => ({ body: order }),
    })
    renderAt(`/book/r1?check_in=${checkIn}&check_out=${checkOut}&guests=4&rooms=2`)
    expect(await screen.findByText(/3 rooms left for these nights/i)).toBeInTheDocument()
    expect(screen.getByText('$360')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /^reserve$/i }))
    expect(await screen.findByText('You’re booked!')).toBeInTheDocument()
    const body = calls.find((c) => c.method === 'POST' && c.path === '/reservations')!.body as Record<string, number | string>
    expect(body).toMatchObject({ listing_id: 'r1', start_at: nightStart(checkIn), end_at: nightStart(checkOut), quantity: 2, party_size: 4 })
    expect((body.start_at as number) % 86400).toBe(0)
    await userEvent.click(screen.getByRole('button', { name: /^pay now$/i }))
    expect(await screen.findByRole('button', { name: /pay securely/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /view reservation/i })).toHaveAttribute('href', '/reservations/res1')
  })
})

describe('seller hotel rooms', () => {
  const org = { organization_id: 'org1', display_name: 'Hotel du Fleuve', status: 'active', verification_status: 'verified', role: 'owner', seller_profiles: [{ id: 'sp1', display_name: 'Hotel du Fleuve', status: 'active', verification_status: 'verified', mine: true }], stores: [{ id: 's1', name: 'Hotel du Fleuve', status: 'active' }] }
  const nights = (capacity: number, reserved: number) => ({ nights: Array.from({ length: 14 }, (_, i) => ({ date: dateInput(i), capacity, reserved, available: capacity - reserved })) })

  it('creates a room type, publishes it and opens nights', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [org] } }),
      'GET /hotels/room-types': () => ({ body: { items: [] } }),
      'GET /categories': () => ({ body: { items: [{ id: 'c1', name: 'Hotels', slug: 'hotels' }] } }),
      'POST /hotels/room-types': () => ({ status: 201, body: room('r9', 'Deluxe room', { status: 'draft', units: 3 }) }),
      'POST /listings/publish': () => ({ body: { status: 'published' } }),
      'GET /hotels/room-types/r9/capacity': () => ({ body: nights(0, 0) }),
      'POST /hotels/room-types/r9/capacity': () => ({ body: nights(3, 0) }),
    })
    renderAt('/seller/hotel')
    await userEvent.click((await screen.findAllByRole('button', { name: /add room type/i }))[0])
    await userEvent.type(screen.getByLabelText('Room type name'), 'Deluxe room')
    await userEvent.selectOptions(screen.getByLabelText('Category'), 'c1')
    await userEvent.type(screen.getByLabelText('Nightly price'), '90')
    await userEvent.clear(screen.getByLabelText('Number of rooms'))
    await userEvent.type(screen.getByLabelText('Number of rooms'), '3')
    await userEvent.type(screen.getByLabelText('Amenities'), 'Wi-Fi, Breakfast')
    await userEvent.click(screen.getByRole('button', { name: /create room type/i }))
    await waitFor(() => expect(calls.some((c) => c.path === '/listings/publish?listing_id=r9')).toBe(true))
    const create = calls.find((c) => c.method === 'POST' && c.path === '/hotels/room-types')!.body as Record<string, unknown>
    expect(create).toMatchObject({ title: 'Deluxe room', category_id: 'c1', store_id: 's1', seller_id: 'sp1', price_amount: 90, units: 3, max_occupancy: 2, amenities: ['Wi-Fi', 'Breakfast'] })
    // The nights panel opens for the new room type.
    expect(await screen.findByRole('table', { name: /next 14 nights/i })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /set nights/i }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/hotels/room-types/r9/capacity')).toBe(true))
    const set = calls.find((c) => c.method === 'POST' && c.path === '/hotels/room-types/r9/capacity')!.body as Record<string, unknown>
    expect(set).toMatchObject({ from: dateInput(0), to: dateInput(14), rooms: 3 })
  })

  it('shows staff the rooms and nights without controls', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /sellers/me': () => ({ body: { organizations: [{ ...org, role: 'staff' }] } }),
      'GET /hotels/room-types': () => ({ body: { items: [room('r1', 'Deluxe room')] } }),
      'GET /hotels/room-types/r1/capacity': () => ({ body: nights(3, 1) }),
    })
    renderAt('/seller/hotel')
    await userEvent.click(await screen.findByRole('button', { name: /^nights$/i }))
    expect(await screen.findByRole('table', { name: /next 14 nights/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /add room type/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /set nights/i })).not.toBeInTheDocument()
    expect(screen.getByText(/only its owner, admins and managers can make changes/i)).toBeInTheDocument()
  })
})
