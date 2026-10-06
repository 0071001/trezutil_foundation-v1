import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { analyticsBody, quietAnalytics } from '@/test/fixtures'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/*
 * The dashboards. Every figure and every chart point is a number the server
 * computed from its own tables (GET /admin/analytics, GET /sellers/analytics);
 * these tests check that the screens show those numbers faithfully, ask for the
 * window the person picked, scope a business's figures to that business, keep a
 * failing block from taking the page down, and never fill a quiet period with
 * invented activity.
 */

const admin = { ...me, id: 'a1', display_name: 'Guilbert', email: 'guilbertmkap@gmail.com', phone: '', roles: ['admin'] }
const operator = { ...admin, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', roles: ['operator'] }

const overview = {
  generated_at: '2026-10-04T12:00:00Z',
  users: { total: 7, active: 6, suspended: 1, new_last_7_days: 3, by_role: { customer: 5, admin: 1, seller: 1 } },
  organizations: 2, listings_by_status: { published: 197, draft: 3 }, orders: { total: 12, last_7_days: 4 }, reservations: 5, conversations: { last_7_days: 30 },
}

const adminApis = (user: unknown = admin, analytics: () => { status?: number; body?: unknown } = () => ({ body: analyticsBody(true) })) => ({
  'GET /auth/users/me': () => ({ body: user }),
  'GET /sellers/me': () => ({ body: { organizations: [] } }),
  'GET /admin/overview': () => ({ body: overview }),
  'GET /admin/analytics': analytics,
})

const kpi = (label: string) => screen.getByText(label, { selector: 'small' }).closest('.tzc-kpi') as HTMLElement
const card = (name: string) => screen.getByRole('region', { name })
const hasCard = (name: string) => screen.queryByRole('region', { name }) !== null
/** What a tile's comparison says, as one sentence. */
const said = (tile: HTMLElement) => tile.querySelector('.tzc-delta')?.textContent?.replace(/\s+/g, ' ').trim()
/** The key figures are tiles: pick one and its chart takes the panel under the board. */
const open = (label: string) => userEvent.click(kpi(label))

describe('administration dashboard', () => {
  it('shows what the platform holds today and what happened over the period', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    expect(await screen.findByRole('heading', { level: 2, name: 'Platform today' })).toBeInTheDocument()
    expect(await screen.findByText('Active accounts')).toBeInTheDocument() // the snapshot
    expect(await screen.findByRole('heading', { level: 2, name: 'Activity over the last 30 days' })).toBeInTheDocument()

    // The headline figures are the server's, formatted: nothing is computed in the browser but the layout.
    await screen.findByText('Orders placed')
    expect(within(kpi('Revenue')).getByText(/\$4,734/)).toBeInTheDocument()
    expect(within(kpi('Orders placed')).getByText('5')).toBeInTheDocument()
    expect(within(kpi('Bookings')).getByText('3')).toBeInTheDocument()
    expect(within(kpi('Buyers')).getByText('3')).toBeInTheDocument()
    expect(within(kpi('Buyers')).getByText('Returning buyers: 1')).toBeInTheDocument()
    expect(within(kpi('New accounts')).getByText('2')).toBeInTheDocument()
    expect(within(kpi('Conversations')).getByText('9')).toBeInTheDocument()
  })

  it('compares each figure with the period before it, in words and numbers', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    expect(said(kpi('Orders placed'))).toBe('▲ +150 % vs the previous 3 days') // 5 against 2
    expect(said(kpi('Bookings'))).toBe('▲ +200 % vs the previous 3 days') // 3 against 1
    expect(kpi('Orders placed').querySelector('.tzc-delta')).toHaveClass('good')
  })

  it('does not invent a percentage when the period before had nothing', async () => {
    signIn()
    mockApi(adminApis(admin, () => ({ body: analyticsBody(true, { previous: { orders: 0, reservations: 0, conversations: 0, new_users: 0, new_listings: 0, customers: 0, returning_customers: 0, revenue: {} } }) })))
    renderAt('/admin')
    await screen.findByText('Orders placed')
    expect(said(kpi('Orders placed'))).toBe('▲ +5 more than the previous 3 days (none then)')
    expect(kpi('Orders placed').textContent).not.toMatch(/Infinity|NaN/)
  })

  it('asks the server for the window the person picks', async () => {
    signIn()
    const { calls } = mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    expect(calls.find((c) => c.path.startsWith('/admin/analytics'))!.path).toBe('/admin/analytics?days=30')
    await userEvent.click(screen.getByRole('button', { name: '7 days' }))
    await waitFor(() => expect(calls.some((c) => c.path === '/admin/analytics?days=7')).toBe(true))
    await userEvent.click(screen.getByRole('button', { name: '90 days' }))
    await waitFor(() => expect(calls.some((c) => c.path === '/admin/analytics?days=90')).toBe(true))
    expect(screen.getByRole('button', { name: '90 days' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('opens one chart at a time, the one of the figure picked, and lets it be read as a table of the same numbers', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    // Revenue leads: its chart is the one under the board, and it is the selected tile.
    expect(hasCard('Revenue per day')).toBe(true)
    expect(kpi('Revenue')).toHaveAttribute('aria-pressed', 'true')
    expect(hasCard('Orders per day')).toBe(false)

    for (const [figure, title] of [['Orders placed', 'Orders per day'], ['Bookings', 'Bookings per day'], ['New accounts', 'New accounts per day'], ['Conversations', 'Conversations per day'], ['New listings', 'New listings per day']] as const) {
      await open(figure)
      expect(kpi(figure)).toHaveAttribute('aria-pressed', 'true')
      expect(card(title)).toBeInTheDocument()
      expect(screen.getAllByRole('region').filter((r) => /per day$/.test(r.getAttribute('aria-labelledby') ? r.querySelector('h2')?.textContent ?? '' : ''))).toHaveLength(1) // never two at once
    }
    expect(kpi('Revenue')).toHaveAttribute('aria-pressed', 'false')

    await open('Orders placed')
    const orders = card('Orders per day')
    expect(within(orders).getByRole('group')).toHaveAccessibleName(/Orders per day\. From Oct 2 to Oct 4\. Orders: total 5, highest on Oct 4 \(4\)/)
    await userEvent.click(within(orders).getByRole('button', { name: 'View as a table' }))
    const rows = within(within(orders).getByRole('table')).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell').map((c) => c.textContent))
    expect(rows).toEqual([[expect.stringMatching(/Oct 2, 2026/), '0'], [expect.stringMatching(/Oct 3, 2026/), '1'], [expect.stringMatching(/Oct 4, 2026/), '4']])
  })

  it('keeps the figure picked when the period changes', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    await open('Bookings')
    await userEvent.click(screen.getByRole('button', { name: '7 days' }))
    await waitFor(() => expect(screen.getByRole('button', { name: '7 days' })).toHaveAttribute('aria-pressed', 'true'))
    await screen.findByText('Orders placed')
    expect(kpi('Bookings')).toHaveAttribute('aria-pressed', 'true')
    expect(card('Bookings per day')).toBeInTheDocument()
  })

  it('leaves a figure with no daily series (buyers) as a plain figure, not a button', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    expect(kpi('Buyers').tagName).toBe('DIV')
    expect(kpi('Orders placed').tagName).toBe('BUTTON')
    expect(kpi('Orders placed')).toHaveAttribute('aria-controls')
  })

  it('exports the figures on screen as a CSV file, and only once there are figures', async () => {
    signIn()
    mockApi(adminApis())
    const urls: Blob[] = []
    const url = URL as unknown as { createObjectURL?: (b: Blob) => string; revokeObjectURL?: (u: string) => void }
    const [create, revoke] = [url.createObjectURL, url.revokeObjectURL]
    url.createObjectURL = (b: Blob) => { urls.push(b); return 'blob:csv' }
    url.revokeObjectURL = () => {}
    const clicked: string[] = []
    const real = HTMLAnchorElement.prototype.click
    HTMLAnchorElement.prototype.click = function (this: HTMLAnchorElement) { clicked.push(this.download) }
    try {
      renderAt('/admin')
      await screen.findByText('Orders placed')
      await userEvent.click(screen.getByRole('button', { name: 'Export' }))
      expect(clicked).toEqual(['trezutil-platform-2026-10-02-to-2026-10-04.csv'])
      const text = await urls[0]!.text()
      expect(text).toContain('Day,Orders,Bookings,New listings,New accounts,Conversations,Revenue (USD),Revenue (CDF)')
      expect(text).toContain('2026-10-04,4,2,0,2,4,4734.00,50000.00')
      expect(text).toContain('Total,5,3,1,2,9,4734.00,50000.00') // the server's own totals
    } finally {
      HTMLAnchorElement.prototype.click = real
      url.createObjectURL = create
      url.revokeObjectURL = revoke
    }
  })

  it('keeps each currency on its own: a switch, never a sum', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    const revenue = card('Revenue per day')
    const group = within(revenue).getByRole('group', { name: 'Currency' })
    expect(within(group).getByRole('button', { name: 'USD' })).toHaveAttribute('aria-pressed', 'true') // USD first
    expect(revenue).toHaveTextContent('in USD')
    await userEvent.click(within(revenue).getByRole('button', { name: 'View as a table' }))
    expect(within(revenue).getByRole('table')).toHaveTextContent(/\$4,734/)
    expect(within(revenue).getByRole('table')).not.toHaveTextContent(/50,000/)

    await userEvent.click(within(group).getByRole('button', { name: 'CDF' }))
    expect(within(group).getByRole('button', { name: 'CDF' })).toHaveAttribute('aria-pressed', 'true')
    expect(revenue).toHaveTextContent('in CDF')
    expect(within(revenue).getByRole('table')).toHaveTextContent(/50,000/)
    expect(within(revenue).getByRole('table')).not.toHaveTextContent(/4,734/)
    // The headline names both, each in its own currency.
    expect(within(kpi('Revenue')).getByText('$4,734')).toBeInTheDocument() // the main currency, in whole units
    expect(within(kpi('Revenue')).getByText(/Also .*50,000, counted separately/)).toBeInTheDocument() // the other one, never added to it
  })

  it('shows the breakdowns with counts and shares as text', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    const status = within(card('Orders by status')).getByRole('list', { name: 'Orders by status' })
    expect(status).toHaveTextContent('Paid')
    expect(status).toHaveTextContent('3')
    expect(status).toHaveTextContent('60 %') // 3 of 5
    expect(within(card('Live listings by sector')).getByRole('list')).toHaveTextContent('Retail')
    expect(within(card('Bookings by status')).getByRole('list', { name: 'Bookings by status' })).toHaveTextContent('Confirmed')
  })

  it('ranks the businesses with the most orders', async () => {
    signIn()
    mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    const ranking = within(card('Top businesses')).getByRole('list', { name: 'Businesses with the most orders' })
    const rows = within(ranking).getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('Gombe Stays')
    expect(rows[0]).toHaveTextContent('4 orders')
    expect(rows[0]).toHaveTextContent(/\$3,500 \+ .*50,000/) // each currency on its own
    expect(rows[1]).toHaveTextContent('Kasa Market')
    expect(rows[1]).toHaveTextContent('1 order')
  })

  it('says a quiet period was quiet: real zeros, plain sentences, no invented points', async () => {
    signIn()
    mockApi(adminApis(admin, () => ({ body: quietAnalytics(true) })))
    renderAt('/admin')
    await screen.findByText('Orders placed')
    expect(within(kpi('Orders placed')).getByText('0')).toBeInTheDocument()
    expect(within(kpi('Orders placed')).getByText('No activity in either period')).toBeInTheDocument()
    expect(within(card('Revenue per day')).getByText('No revenue in this period.')).toBeInTheDocument()
    expect(within(card('Top businesses')).getByText('No orders in this period.')).toBeInTheDocument()
    expect(within(card('Orders by status')).getByText('Nothing to show for this period.')).toBeInTheDocument()
    await open('Orders placed')
    expect(within(card('Orders per day')).getByText('Nothing in this period.')).toBeInTheDocument()
    expect(within(card('Orders per day')).queryByRole('group')).not.toBeInTheDocument() // no chart drawn for nothing
    expect(within(card('Orders per day')).queryByRole('button', { name: 'View as a table' })).not.toBeInTheDocument()
  })

  it('keeps the snapshot when the activity fails, and retries only the activity', async () => {
    signIn()
    let fail = true
    const { calls } = mockApi(adminApis(admin, () => (fail ? { status: 500, body: { error: { code: 'ANALYTICS_FAILED', message: 'The analytics could not be loaded' } } } : { body: analyticsBody(true) })))
    renderAt('/admin')
    expect(await screen.findByText('Active accounts')).toBeInTheDocument()
    expect(await screen.findByText('The analytics could not be loaded')).toBeInTheDocument()
    const overviewCalls = calls.filter((c) => c.path === '/admin/overview').length
    fail = false
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('Orders placed')).toBeInTheDocument()
    expect(calls.filter((c) => c.path === '/admin/overview').length).toBe(overviewCalls) // the snapshot was not reloaded
  })

  it('refreshes both blocks together', async () => {
    signIn()
    const { calls } = mockApi(adminApis())
    renderAt('/admin')
    await screen.findByText('Orders placed')
    await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(calls.filter((c) => c.path === '/admin/overview').length).toBe(2))
    expect(calls.filter((c) => c.path.startsWith('/admin/analytics')).length).toBe(2)
  })

  it('is open to operators, who still do not get the account list', async () => {
    signIn()
    mockApi(adminApis(operator))
    renderAt('/admin')
    expect(await screen.findByText('Orders placed')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Users' })).not.toBeInTheDocument()
  })
})

/* ---------------------------------------------------------------- seller */

const org = (id: string, name: string, role = 'owner') => ({
  organization_id: id, display_name: name, status: 'active', verification_status: 'verified', role,
  seller_profiles: [{ id: `sp-${id}`, display_name: name, status: 'active', verification_status: 'verified', mine: true }],
  stores: [{ id: `s-${id}`, name, status: 'active' }],
})

const sellerApis = (orgs = [org('org1', 'Gombe Stays')], analytics: (path: string) => { status?: number; body?: unknown } = () => ({ body: analyticsBody(false) })) => ({
  'GET /auth/users/me': () => ({ body: me }),
  'GET /sellers/me': () => ({ body: { organizations: orgs } }),
  'GET /sellers/analytics': (c: { path: string }) => analytics(c.path),
  'GET /listings/manage': () => ({ body: { items: [] } }),
  'GET /orders/incoming': () => ({ body: { items: [] } }),
  'GET /reservations/incoming': () => ({ body: { items: [] } }),
  'GET /inventory': () => ({ body: { items: [] } }),
})

describe('seller dashboard', () => {
  it('shows a manager the platform’s own counts of this business', async () => {
    signIn()
    const { calls } = mockApi(sellerApis())
    renderAt('/seller')
    expect(await screen.findByText('Orders placed')).toBeInTheDocument()
    expect(calls.find((c) => c.path.startsWith('/sellers/analytics'))!.path).toBe('/sellers/analytics?organization_id=org1&days=30')
    expect(within(kpi('Sales')).getByText('$4,734')).toBeInTheDocument()
    expect(within(kpi('Orders placed')).getByText('5')).toBeInTheDocument()
    expect(within(kpi('Bookings')).getByText('3')).toBeInTheDocument()
    expect(within(kpi('Customers')).getByText('3')).toBeInTheDocument()
    expect(within(kpi('Customers')).getByText('Returning buyers: 1')).toBeInTheDocument()
    expect(within(kpi('Live listings')).getByText('4')).toBeInTheDocument() // published, from the same count
    expect(within(kpi('Live listings')).getByText('1 draft')).toBeInTheDocument()
    expect(screen.getByText(/Counted by the platform from your orders, bookings and listings, Oct 2 to Oct 4\./)).toBeInTheDocument()
  })

  it('lists the best sellers and the orders by status', async () => {
    signIn()
    mockApi(sellerApis())
    renderAt('/seller')
    await screen.findByText('Orders placed')
    const best = within(card('Best sellers')).getByRole('list', { name: 'Best sellers' })
    expect(within(best).getAllByRole('listitem')[0]).toHaveTextContent('Café Mokili')
    expect(within(best).getAllByRole('listitem')[0]).toHaveTextContent('3 units sold')
    expect(within(card('Orders by status')).getByRole('list', { name: 'Orders by status' })).toHaveTextContent('Paid')
  })

  it('asks for the period picked, always for this business', async () => {
    signIn()
    const { calls } = mockApi(sellerApis())
    renderAt('/seller')
    await screen.findByText('Orders placed')
    await userEvent.click(screen.getByRole('button', { name: '90 days' }))
    await waitFor(() => expect(calls.some((c) => c.path === '/sellers/analytics?organization_id=org1&days=90')).toBe(true))
    for (const c of calls.filter((x) => x.path.startsWith('/sellers/analytics'))) expect(c.path).toContain('organization_id=org1')
  })

  it('never shows one business’s figures under another’s name', async () => {
    signIn()
    const { calls } = mockApi(sellerApis([org('org1', 'Gombe Stays'), org('org2', 'Kasa Market', 'manager')], (path) => ({
      body: analyticsBody(false, path.includes('org2') ? { current: { orders: 11, reservations: 0, conversations: 0, new_users: 0, new_listings: 0, customers: 4, returning_customers: 0, revenue: { USD: 99900 } }, previous: { orders: 0, reservations: 0, conversations: 0, new_users: 0, new_listings: 0, customers: 0, returning_customers: 0, revenue: {} } } : {}),
    })))
    renderAt('/seller')
    await screen.findByText('Orders placed')
    expect(within(kpi('Orders placed')).getByText('5')).toBeInTheDocument()
    await userEvent.selectOptions(await screen.findByRole('combobox', { name: 'Business' }), 'org2')
    await waitFor(() => expect(calls.some((c) => c.path.includes('organization_id=org2'))).toBe(true))
    await waitFor(() => expect(within(kpi('Orders placed')).getByText('11')).toBeInTheDocument())
    expect(kpi('Sales').textContent).toMatch(/\$999/)
  })

  it('gives a member below manager the catalogue only, and asks for no analytics', async () => {
    signIn()
    const { calls } = mockApi(sellerApis([org('org1', 'Gombe Stays', 'staff')]))
    renderAt('/seller')
    expect(await screen.findByText('Only managers see orders and bookings.')).toBeInTheDocument()
    expect(screen.queryByText('Orders placed')).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Period' })).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/sellers/analytics'))).toBe(false)
    expect(screen.getByText('Live listings')).toBeInTheDocument()
  })

  it('shows the server’s refusal in place and keeps the rest of the dashboard', async () => {
    signIn()
    mockApi(sellerApis([org('org1', 'Gombe Stays')], () => ({ status: 403, body: { error: { code: 'FORBIDDEN', message: 'no' } } })))
    renderAt('/seller')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Needs your attention' })).toBeInTheDocument()
  })

  it('retries a failed analytics read without reloading the lists', async () => {
    signIn()
    let fail = true
    const { calls } = mockApi(sellerApis([org('org1', 'Gombe Stays')], () => (fail ? { status: 500, body: { error: { code: 'ANALYTICS_FAILED', message: 'The analytics could not be loaded' } } } : { body: analyticsBody(false) })))
    renderAt('/seller')
    expect(await screen.findByText('The analytics could not be loaded')).toBeInTheDocument()
    fail = false
    const listCalls = calls.filter((c) => c.path.startsWith('/listings/manage')).length
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('Orders placed')).toBeInTheDocument()
    expect(calls.filter((c) => c.path.startsWith('/listings/manage')).length).toBe(listCalls)
  })

  it('shows a quiet business real zeros', async () => {
    signIn()
    mockApi(sellerApis([org('org1', 'Gombe Stays')], () => ({ body: quietAnalytics(false) })))
    renderAt('/seller')
    await screen.findByText('Orders placed')
    expect(within(kpi('Sales')).getByText('No activity in either period')).toBeInTheDocument()
    expect(within(card('Best sellers')).getByText('No itemised sales in this period yet.')).toBeInTheDocument()
    expect(within(card('Revenue per day')).getByText('No revenue in this period.')).toBeInTheDocument()
  })
})

describe('seller analytics module', () => {
  it('is a built module for managers, with everything the dashboard summarises', async () => {
    signIn()
    const { calls } = mockApi(sellerApis())
    renderAt('/seller/analytics')
    expect(await screen.findByRole('heading', { level: 1, name: 'Analytics' })).toBeInTheDocument()
    expect(await screen.findByText('Orders placed')).toBeInTheDocument()
    expect(calls.find((c) => c.path.startsWith('/sellers/analytics'))!.path).toBe('/sellers/analytics?organization_id=org1&days=30')
    for (const title of ['Revenue per day', 'Orders by status', 'Best sellers', 'Bookings by status', 'Listings by status', 'Live listings by sector']) {
      expect(card(title)).toBeInTheDocument()
    }
    for (const [figure, title] of [['Orders placed', 'Orders per day'], ['Bookings', 'Bookings per day'], ['New listings', 'New listings per day']] as const) {
      await open(figure)
      expect(card(title)).toBeInTheDocument()
    }
    expect(within(kpi('New listings')).getByText('1')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Analytics' })).toHaveAttribute('aria-current', 'page') // no "not available yet" any more
  })

  it('tells a member below manager it is for managers, and asks for nothing', async () => {
    signIn()
    const { calls } = mockApi(sellerApis([org('org1', 'Gombe Stays', 'staff')]))
    renderAt('/seller/analytics')
    expect(await screen.findByText('Managers only')).toBeInTheDocument()
    expect(screen.getByText('Analytics are for the owner, admins and managers of this business.')).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/sellers/analytics'))).toBe(false)
  })

  it('changes the window and reads the new one', async () => {
    signIn()
    const { calls } = mockApi(sellerApis())
    renderAt('/seller/analytics')
    await screen.findByText('Orders placed')
    await userEvent.click(screen.getByRole('button', { name: '7 days' }))
    await waitFor(() => expect(calls.some((c) => c.path === '/sellers/analytics?organization_id=org1&days=7')).toBe(true))
  })
})
