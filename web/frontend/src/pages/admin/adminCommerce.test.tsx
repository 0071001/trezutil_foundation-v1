import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

const operator = { ...me, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', phone: '', roles: ['operator'] }

const order = (id: string, extra: Record<string, unknown> = {}) => ({
  id, status: 'paid', total_minor: 2500, currency: 'USD', title: 'Wax fabric', item_count: 1,
  organization_id: 'org-1', organization_name: 'Gombe Stays', customer_id: 'c-1', customer_name: 'Marie Kalala',
  created_at: '2026-10-04T10:00:00Z', updated_at: '2026-10-04T11:00:00Z', ...extra,
})
const booking = (id: string, extra: Record<string, unknown> = {}) => ({
  id, status: 'confirmed', title: 'Double room', listing_id: 'l-1', organization_id: 'org-1', organization_name: 'Gombe Stays',
  customer_id: 'c-1', customer_name: 'Marie Kalala', start_at: '2026-11-01T14:00:00Z', end_at: '2026-11-03T11:00:00Z', quantity: 1, party_size: 2,
  created_at: '2026-10-04T10:00:00Z', ...extra,
})
const business = (id: string, extra: Record<string, unknown> = {}) => ({
  id, display_name: 'Gombe Stays', type: 'business', status: 'active', verification_status: 'verified', city: 'Kinshasa', country: 'CD',
  members: 2, stores: 1, published_listings: 7, orders: 12, created_at: '2026-09-01T10:00:00Z', ...extra,
})
const page = (items: Record<string, unknown>[], total = items.length) => ({ items, total, page: 1, page_size: 25 })

describe('administration: orders across the platform', () => {
  it('lists them with the business, the customer, the state and the total', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/orders': () => ({ body: page([order('o-aaaaaaaa-1', { status: 'processing', total_minor: 4000, item_count: 3 })]) }) })
    renderAt('/admin/orders')
    expect(await screen.findByRole('heading', { name: 'Orders' })).toBeInTheDocument()
    const row = (await screen.findByText('Wax fabric')).closest('tr')!
    expect(within(row).getByText('Gombe Stays')).toBeInTheDocument()
    expect(within(row).getByText('Marie Kalala')).toBeInTheDocument()
    expect(within(row).getByText('Processing')).toBeInTheDocument()
    expect(within(row).getByText('$40')).toBeInTheDocument()
    expect(within(row).getByText(/#o-aaaaaa/)).toBeInTheDocument()
    expect(within(row).getByText(/3 items/)).toBeInTheDocument()
  })

  it('searches and filters on the server, and starts again from the first page', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/orders': () => ({ body: page([order('o-1')], 80) }) })
    renderAt('/admin/orders')
    await screen.findByText('Wax fabric')
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(calls.some((c) => c.path.includes('page=2'))).toBe(true))

    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Status' }), 'paid')
    await waitFor(() => expect(calls.some((c) => c.path.includes('status=paid') && c.path.includes('page=1'))).toBe(true))
    await userEvent.type(screen.getByRole('searchbox'), 'marie')
    await waitFor(() => expect(calls.some((c) => c.path.includes('q=marie') && c.path.includes('status=paid'))).toBe(true))

    await userEvent.click(screen.getByRole('button', { name: 'Reset filters' }))
    await waitFor(() => expect(calls.filter((c) => c.path.startsWith('/admin/orders')).at(-1)!.path).toBe('/admin/orders?page=1&page_size=25'))
  })

  it('opens an order beside the list, with a way to its history', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: operator }),
      'GET /admin/orders': () => ({ body: page([order('o-aaaaaaaa-1')]) }),
      'GET /admin/audit': () => ({ body: { items: [], total: 0, page: 1, page_size: 50 } }),
    })
    renderAt('/admin/orders')
    await userEvent.click(await screen.findByText('Wax fabric'))
    const drawer = await screen.findByRole('dialog', { name: 'Wax fabric' })
    expect(within(drawer).getByText('o-aaaaaaaa-1')).toBeInTheDocument()
    expect(within(drawer).getByText('Marie Kalala')).toBeInTheDocument()
    expect(within(drawer).queryByText(/@|\+243/)).not.toBeInTheDocument() // no contact data
    const link = within(drawer).getByRole('link', { name: 'See what was done to it' })
    expect(link).toHaveAttribute('href', '/admin/audit?target_id=o-aaaaaaaa-1')
  })

  it('opens the audit trail already filtered on that record', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: operator }),
      'GET /admin/audit': () => ({ body: { items: [], total: 0, page: 1, page_size: 50 } }),
    })
    renderAt('/admin/audit?target_id=o-aaaaaaaa-1')
    await waitFor(() => expect(calls.some((c) => c.path.includes('target_id=o-aaaaaaaa-1'))).toBe(true))
    expect(screen.getByRole('searchbox')).toHaveValue('o-aaaaaaaa-1')
  })

  it('says so when there is nothing, and when the server fails', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/orders': () => ({ body: page([]) }) })
    const { unmount } = renderAt('/admin/orders')
    expect(await screen.findByText('No order has been placed yet.')).toBeInTheDocument()
    unmount()

    mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/orders': () => ({ status: 500, body: { error: { code: 'ADMIN_QUERY_FAILED', message: 'The list could not be loaded' } } }) })
    renderAt('/admin/orders')
    expect(await screen.findByRole('button', { name: /try again|retry/i })).toBeInTheDocument()
  })
})

describe('administration: bookings and businesses', () => {
  it('lists bookings with their dates and guests', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/reservations': () => ({ body: page([booking('r-1')]) }) })
    renderAt('/admin/reservations')
    expect(await screen.findByRole('heading', { name: 'Reservations' })).toBeInTheDocument()
    const row = (await screen.findByText('Double room')).closest('tr')!
    expect(within(row).getByText('Gombe Stays')).toBeInTheDocument()
    expect(within(row).getByText('Confirmed')).toBeInTheDocument()
    expect(within(row).getByText(/Nov 1, 2026 → Nov 3, 2026/)).toBeInTheDocument()
  })

  it('offers only the statuses a booking can have', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/reservations': () => ({ body: page([]) }) })
    renderAt('/admin/reservations')
    const select = await screen.findByRole('combobox', { name: 'Status' })
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['Every status', 'Pending', 'Confirmed', 'Completed', 'Cancelled'])
  })

  it('lists businesses with their activity and no contact data', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/organizations': () => ({ body: page([business('org-1')]) }) })
    renderAt('/admin/organizations')
    expect(await screen.findByRole('heading', { name: 'Organizations and sellers' })).toBeInTheDocument()
    const row = (await screen.findByText('Gombe Stays')).closest('tr')!
    expect(within(row).getByText('Kinshasa, CD')).toBeInTheDocument()
    expect(within(row).getByText('Active')).toBeInTheDocument()
    expect(within(row).getByText('Verified')).toBeInTheDocument()
    expect(within(row).getByText('7')).toBeInTheDocument() // live listings
    expect(within(row).getByText('12')).toBeInTheDocument() // orders
    expect(within(screen.getByRole('table')).queryByText(/@|\+243/)).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/admin/organizations'))).toBe(true)
  })
})

describe('administration: who may read these lists', () => {
  it('does not ask the server on behalf of a moderator, and shows no list', async () => {
    signIn()
    const moderator = { ...operator, id: 'm1', roles: ['moderator'] }
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: moderator }) })
    renderAt('/admin/orders')
    expect(await screen.findByText('Nothing to show yet')).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/admin/orders'))).toBe(false)
  })

  it('shows the three modules as built in the menu, not as planned', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: operator }),
      'GET /admin/overview': () => ({ body: { generated_at: '2026-10-04T12:00:00Z', users: { total: 1, active: 1, suspended: 0, new_last_7_days: 0, by_role: {} }, organizations: 0, listings_by_status: {}, orders: { total: 0, last_7_days: 0 }, reservations: 0, conversations: { last_7_days: 0 } } }),
    })
    renderAt('/admin')
    const nav = await screen.findByRole('navigation', { name: 'Administration menu' })
    for (const name of ['Orders', 'Reservations', 'Organizations and sellers', 'Audit']) {
      expect(within(nav).getByRole('link', { name: new RegExp(name) })).toBeInTheDocument()
    }
  })
})
