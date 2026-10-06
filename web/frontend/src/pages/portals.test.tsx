import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { I18nProvider } from '@/lib/i18n'
import { PrefsProvider } from '@/lib/prefs'
import { SessionProvider, useSession } from '@/lib/session'
import { ToastProvider } from '@/lib/toast'
import { adminModules, sellerModules } from '@/portal/modules'
import { PortalsProvider, usePortals } from '@/portal/PortalsProvider'
import { analyticsBody } from '@/test/fixtures'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/*
 * Portals Phase 1: who can open which console (the Security rows of
 * docs/web/PORTALS_INFORMATION_ARCHITECTURE.md §12) and how the shared shell
 * behaves (the Shell rows). The server is the authority; these tests prove the
 * UI never asks for more than the person holds and shows a refusal when the
 * server gives one.
 */

const org = (id: string, name: string, role = 'owner') => ({
  organization_id: id, display_name: name, status: 'active', verification_status: 'verified', role,
  seller_profiles: [{ id: `sp-${id}`, display_name: name, status: 'active', verification_status: 'verified', mine: true }],
  stores: [{ id: `s-${id}`, name, status: 'active' }],
})
const gombe = org('org1', 'Gombe Stays')
const kasa = org('org2', 'Kasa Market', 'manager')
const admin = { ...me, id: 'a1', display_name: 'Guilbert', email: 'guilbertmkap@gmail.com', phone: '', roles: ['admin'] }
const operator = { ...admin, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', roles: ['operator'] }
const moderator = { ...admin, id: 'm1', display_name: 'Moussa', email: 'moussa@example.com', roles: ['moderator'] }

const sellerApis = {
  'GET /sellers/analytics': () => ({ body: analyticsBody(false) }),
  'GET /listings/manage': () => ({ body: { items: [] } }),
  'GET /orders/incoming': () => ({ body: { items: [] } }),
  'GET /reservations/incoming': () => ({ body: { items: [] } }),
  'GET /inventory': () => ({ body: { items: [] } }),
}
const sellerSession = (orgs = [gombe], user: unknown = me) => ({
  'GET /auth/users/me': () => ({ body: user }),
  'GET /sellers/me': () => ({ body: { organizations: orgs } }),
  ...sellerApis,
})
const adminSession = (user: unknown = admin) => ({
  'GET /auth/users/me': () => ({ body: user }),
  'GET /sellers/me': () => ({ body: { organizations: [] } }),
  'GET /admin/overview': () => ({ body: { generated_at: '2026-10-04T12:00:00Z', users: { total: 1, active: 1, suspended: 0, new_last_7_days: 0, by_role: {} }, organizations: 0, listings_by_status: {}, orders: { total: 0, last_7_days: 0 }, reservations: 0, conversations: { last_7_days: 0 } } }),
})

const sellerNav = () => screen.findByRole('navigation', { name: 'Seller workspace' })
const adminNav = () => screen.findByRole('navigation', { name: 'Administration menu' })
const adminCalls = (calls: { path: string }[]) => calls.filter((c) => c.path.startsWith('/admin/'))

afterEach(() => { vi.unstubAllEnvs() })

describe('portal security: entry', () => {
  it('sends a guest from the seller workspace to sign-in and asks the server for nothing', async () => {
    const { calls } = mockApi({})
    renderAt('/seller/orders')
    expect(await screen.findByLabelText('Phone number')).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/sellers') || c.path.startsWith('/orders') || c.path.startsWith('/listings'))).toBe(false)
    expect(screen.queryByRole('navigation', { name: 'Seller workspace' })).not.toBeInTheDocument()
  })

  it('shows "no business yet" to a customer, with no console navigation and no seller data requested', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([]))
    renderAt('/seller')
    expect(await screen.findByText(/isn’t linked to a business yet/i)).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Seller workspace' })).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/listings/manage') || c.path.startsWith('/orders/incoming'))).toBe(false)
  })

  it('refuses a customer at every console address, even a deep one', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([]))
    renderAt('/admin/audit')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Administration menu' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Audit' })).not.toBeInTheDocument()
    expect(adminCalls(calls)).toEqual([])
  })

  it('refuses a seller at the administration, and never calls an administration endpoint', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([gombe]))
    renderAt('/admin')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Administration menu' })).not.toBeInTheDocument()
    expect(adminCalls(calls)).toEqual([])
  })

  it('treats a moderator as staff without an overview: the shell opens, no page does', async () => {
    signIn()
    const { calls } = mockApi(adminSession(moderator))
    renderAt('/admin')
    expect(await screen.findByText('Nothing to show yet')).toBeInTheDocument()
    expect(within(await adminNav()).queryAllByRole('link')).toHaveLength(0)
    expect(calls.some((c) => c.path === '/admin/overview')).toBe(false)
  })

  it('sends a guest from the administration to the staff sign-in, remembering where they were going', async () => {
    mockApi({})
    renderAt('/admin/organizations')
    expect(await screen.findByRole('heading', { name: 'Sign in to the administration' })).toBeInTheDocument()
  })
})

describe('portal security: seller scope', () => {
  it('ignores a remembered business the person is not a member of', async () => {
    signIn()
    localStorage.setItem('trezutil_seller_org', 'org-of-someone-else')
    const { calls } = mockApi(sellerSession([gombe]))
    renderAt('/seller/listings')
    await sellerNav()
    await waitFor(() => expect(calls.some((c) => c.path.startsWith('/listings/manage'))).toBe(true))
    const scoped = calls.filter((c) => c.path.startsWith('/listings/manage') || c.path.startsWith('/orders/incoming'))
    expect(scoped.length).toBeGreaterThan(0)
    for (const c of scoped) {
      expect(c.path).toContain('organization_id=org1')
      expect(c.path).not.toContain('org-of-someone-else')
    }
  })

  it('offers only the businesses the person belongs to, and scopes requests to the one chosen', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([gombe, kasa]))
    renderAt('/seller/listings')
    const picker = await screen.findByRole('combobox', { name: 'Business' })
    expect(within(picker).getAllByRole('option').map((o) => o.textContent)).toEqual(['Gombe Stays', 'Kasa Market'])

    await userEvent.selectOptions(picker, 'org2')
    await waitFor(() => expect(calls.some((c) => c.path.includes('/listings/manage') && c.path.includes('organization_id=org2'))).toBe(true))
    expect(localStorage.getItem('trezutil_seller_org')).toBe('org2')
    expect(screen.getAllByText('Manager', { selector: 'small' })).toHaveLength(2) // the role in the chosen business: rail and header
  })

  it('shows the server’s refusal inside the console when a business is not the person’s to read', async () => {
    signIn()
    mockApi({ ...sellerSession([gombe]), 'GET /listings/manage': () => ({ status: 403, body: { error: { code: 'FORBIDDEN', message: 'not a member' } } }) })
    renderAt('/seller/listings')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(await sellerNav()).toBeInTheDocument() // the frame stays; only the page is refused
  })

  it('shows a refusal, not an empty workspace, when the list of businesses itself is refused', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }), 'GET /sellers/me': () => ({ status: 403, body: { error: { code: 'FORBIDDEN', message: 'no' } } }) })
    renderAt('/seller')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Seller workspace' })).not.toBeInTheDocument()
  })

  it('keeps a member below manager read-only on a module that needs a manager, even by direct address', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([org('org1', 'Gombe Stays', 'staff')]))
    renderAt('/seller/finance')
    expect(await screen.findByRole('heading', { level: 1, name: 'Finance' })).toBeInTheDocument()
    expect(screen.getByText(/this page is for the owner, admins and managers of this business/i)).toBeInTheDocument()
    expect(within(await sellerNav()).getByRole('link', { name: 'Finance, managers only' })).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/console/payments') || c.path.includes('finance'))).toBe(false) // nor do they ask for money data
    expect(calls.some((c) => c.path.startsWith('/orders/incoming'))).toBe(false) // staff never ask for manager data
  })

  it('does not call manager-only endpoints for a staff member’s dashboard', async () => {
    signIn()
    const { calls } = mockApi(sellerSession([org('org1', 'Gombe Stays', 'staff')]))
    renderAt('/seller')
    await sellerNav()
    await waitFor(() => expect(calls.some((c) => c.path.startsWith('/listings/manage'))).toBe(true))
    expect(calls.some((c) => c.path.startsWith('/orders/incoming') || c.path.startsWith('/reservations/incoming'))).toBe(false)
  })
})

describe('portal security: menus and switcher', () => {
  it('lists administration modules by role: an operator does not see the user list', async () => {
    signIn()
    mockApi(adminSession(operator))
    renderAt('/admin')
    const nav = await adminNav()
    expect(within(nav).getByRole('link', { name: 'Overview' })).toBeInTheDocument()
    expect(within(nav).getByRole('link', { name: /^Orders/ })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: 'Users' })).not.toBeInTheDocument()
  })

  it('lists the user list for an administrator', async () => {
    signIn()
    mockApi(adminSession(admin))
    renderAt('/admin')
    expect(within(await adminNav()).getByRole('link', { name: 'Users' })).toBeInTheDocument()
  })

  it('lists only the portals a seller holds in the switcher', async () => {
    signIn()
    mockApi(sellerSession([gombe]))
    renderAt('/seller')
    await sellerNav()
    await userEvent.click(screen.getByRole('button', { name: /Seller workspace/ }))
    const menu = screen.getByRole('menu', { name: 'Switch portal' })
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      expect.stringContaining('Trezutil'),
      expect.stringContaining('Seller workspace'),
    ])
    expect(within(menu).queryByRole('menuitem', { name: /Administration/ })).not.toBeInTheDocument()
  })

  it('lists customer and administration for staff without a business', async () => {
    signIn()
    mockApi(adminSession(admin))
    renderAt('/admin')
    await adminNav()
    await userEvent.click(screen.getByRole('button', { name: /Administration/ }))
    const menu = screen.getByRole('menu', { name: 'Switch portal' })
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(2)
    expect(within(menu).queryByRole('menuitem', { name: /Seller workspace/ })).not.toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: /Administration/ })).toHaveAttribute('aria-current', 'true')
  })

  it('works from the keyboard: arrows move, Escape closes and returns to the button', async () => {
    signIn()
    mockApi(sellerSession([gombe], { ...me, roles: ['customer', 'admin'] }))
    renderAt('/seller')
    await sellerNav()
    const button = screen.getByRole('button', { name: /Seller workspace/ })
    button.focus()
    await userEvent.keyboard('{Enter}')
    const items = screen.getAllByRole('menuitem')
    expect(items).toHaveLength(3)
    expect(items[1]).toHaveFocus() // opens on the current portal
    await userEvent.keyboard('{ArrowDown}')
    expect(items[2]).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(items[0]).toHaveFocus() // wraps
    await userEvent.keyboard('{End}')
    expect(items[2]).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(button).toHaveFocus()
  })
})

describe('portal security: session', () => {
  it('signs out of the seller workspace: tokens are cleared and no business data stays on screen', async () => {
    signIn()
    mockApi(sellerSession([gombe]))
    renderAt('/seller')
    await sellerNav()
    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    await waitFor(() => expect(screen.queryByRole('navigation', { name: 'Seller workspace' })).not.toBeInTheDocument())
    expect(localStorage.getItem('trezutil_access_token')).toBeNull()
    expect(localStorage.getItem('trezutil_refresh_token')).toBeNull()
    expect(screen.queryByText('Gombe Stays')).not.toBeInTheDocument()
  })

  function Probe() {
    const { refreshUser, signOut } = useSession()
    const p = usePortals()
    return (
      <div>
        <p data-testid="who">{p.user?.display_name ?? 'nobody'}</p>
        <p data-testid="orgs">{p.orgs.map((o) => o.display_name).join(',') || 'none'}</p>
        <p data-testid="portals">{p.portals.join(',') || 'none'}</p>
        <button onClick={() => void refreshUser()}>refresh</button>
        <button onClick={signOut}>leave</button>
      </div>
    )
  }
  const renderProbe = () => render(
    <MemoryRouter>
      <PrefsProvider><ToastProvider><SessionProvider><I18nProvider><PortalsProvider><Probe /></PortalsProvider></I18nProvider></SessionProvider></ToastProvider></PrefsProvider>
    </MemoryRouter>,
  )

  it('drops everything when the person signs out', async () => {
    signIn()
    mockApi(sellerSession([gombe]))
    renderProbe()
    await waitFor(() => expect(screen.getByTestId('orgs')).toHaveTextContent('Gombe Stays'))
    expect(screen.getByTestId('portals')).toHaveTextContent('customer,seller')
    await userEvent.click(screen.getByRole('button', { name: 'leave' }))
    expect(screen.getByTestId('orgs')).toHaveTextContent('none')
    expect(screen.getByTestId('portals')).toHaveTextContent('none')
    expect(screen.getByTestId('who')).toHaveTextContent('nobody')
  })

  it('never shows one person’s businesses to the next person who signs in', async () => {
    signIn()
    let current: unknown = me
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: current }),
      'GET /sellers/me': () => ({ body: { organizations: (current as { id: string }).id === 'u1' ? [gombe] : [] } }),
    })
    renderProbe()
    await waitFor(() => expect(screen.getByTestId('orgs')).toHaveTextContent('Gombe Stays'))

    current = { ...me, id: 'u2', display_name: 'Another Person' }
    await userEvent.click(screen.getByRole('button', { name: 'refresh' }))
    await waitFor(() => expect(screen.getByTestId('who')).toHaveTextContent('Another Person'))
    expect(screen.getByTestId('orgs')).toHaveTextContent('none')
    expect(screen.getByTestId('portals')).toHaveTextContent(/^customer$/)
    await waitFor(() => expect(calls.filter((c) => c.path === '/sellers/me')).toHaveLength(2)) // asked again for the new person
  })

  it('does not read businesses for the public site: only a console asks', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: me }), 'GET /sellers/me': () => ({ body: { organizations: [gombe] } }) })
    render(
      <MemoryRouter>
        <PrefsProvider><ToastProvider><SessionProvider><I18nProvider><PortalsProvider><p>public page</p></PortalsProvider></I18nProvider></SessionProvider></ToastProvider></PrefsProvider>
      </MemoryRouter>,
    )
    await screen.findByText('public page')
    expect(calls.some((c) => c.path === '/sellers/me')).toBe(false)
  })
})

describe('portal shell', () => {
  it('gives the page one main landmark that route changes can focus', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    const main = screen.getByRole('main')
    expect(main).toHaveAttribute('id', 'main')
    expect(main).toHaveAttribute('tabindex', '-1')
    expect(screen.getAllByRole('main')).toHaveLength(1)
  })

  it('groups the rail by job and marks the current module', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller/listings')
    const nav = await sellerNav()
    expect(within(nav).getAllByRole('group').map((g) => g.getAttribute('aria-label'))).toEqual(['Run', 'Sell', 'Operate', 'Grow', 'Money', 'Business'])
    expect(within(nav).getByRole('link', { name: 'Listings' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: 'Dashboard' })).not.toHaveAttribute('aria-current')
  })

  it('tells assistive technology what is not built yet instead of leaving a dead link', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    const nav = await sellerNav()
    expect(within(nav).getByRole('link', { name: 'Marketing, not available yet' })).toBeInTheDocument()
    expect(within(nav).getByRole('link', { name: 'Inbox, not available yet' })).toBeInTheDocument()
    expect(within(nav).getByRole('link', { name: 'Orders' })).toBeInTheDocument() // a built module has no suffix
    await userEvent.click(within(nav).getByRole('link', { name: 'Inbox, not available yet' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Inbox' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Not available yet' })).toBeInTheDocument()
  })

  it('shows where you are: portal › module, and a generic crumb below a record', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller/finance')
    await sellerNav()
    const crumbs = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(within(crumbs).getByRole('link', { name: 'Seller workspace' })).toHaveAttribute('href', '/seller')
    expect(within(crumbs).getByText('Finance')).toHaveAttribute('aria-current', 'page')
  })

  it('switches theme and remembers it', async () => {
    signIn()
    mockApi(sellerSession())
    const { container } = renderAt('/seller')
    await sellerNav()
    const shell = container.querySelector('.app.portal')!
    expect(shell).not.toHaveClass('dark')
    await userEvent.click(screen.getByRole('button', { name: 'Switch to dark mode' }))
    expect(shell).toHaveClass('dark')
    expect(localStorage.getItem('trezutil_theme')).toContain('dark')
    await userEvent.click(screen.getByRole('button', { name: 'Switch to light mode' }))
    expect(shell).not.toHaveClass('dark')
  })

  it('keeps the seller light and the administration navy: one shell, two signatures', async () => {
    signIn()
    mockApi(sellerSession())
    const seller = renderAt('/seller')
    await sellerNav()
    expect(seller.container.querySelector('.app.portal')).toHaveClass('portal-seller')
    seller.unmount()

    mockApi(adminSession())
    const staff = renderAt('/admin')
    await adminNav()
    expect(staff.container.querySelector('.app.portal')).toHaveClass('portal-admin')
  })

  it('does not wrap the console in the customer navigation', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    expect(screen.queryByRole('link', { name: 'Skip to content' })).toBeNull() // no second skip target from the customer frame
    expect(screen.getAllByRole('banner')).toHaveLength(1)
  })

  it('shows the environment on the administration only, and never in production', async () => {
    signIn()
    vi.stubEnv('VITE_APP_ENV', 'staging')
    mockApi(adminSession())
    const staff = renderAt('/admin')
    await adminNav()
    expect(screen.getByTitle('Environment')).toHaveTextContent('staging')
    staff.unmount()

    mockApi(sellerSession())
    const seller = renderAt('/seller')
    await sellerNav()
    expect(screen.queryByTitle('Environment')).not.toBeInTheDocument()
    seller.unmount()

    vi.stubEnv('VITE_APP_ENV', 'production')
    mockApi(adminSession())
    renderAt('/admin')
    await adminNav()
    expect(screen.queryByTitle('Environment')).not.toBeInTheDocument()
  })

  it('says who is signed in and with which role', async () => {
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    const header = screen.getByRole('banner')
    expect(within(header).getByText('Jean Mukendi')).toBeInTheDocument()
    expect(within(header).getByText('Owner')).toBeInTheDocument()
  })
})

describe('portal shell: narrow screens', () => {
  const viewport = (narrow: boolean, phone: boolean) => vi.stubGlobal('matchMedia', (query: string) => ({
    matches: (query.includes('899') && narrow) || (query.includes('639') && phone),
    media: query, onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent: () => false,
  }))

  it('opens the rail as a modal dialog from the menu button, traps focus and returns it on Escape', async () => {
    viewport(true, false)
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    const opener = screen.getByRole('button', { name: 'Open menu' })
    expect(opener).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    await userEvent.click(opener)
    const rail = screen.getByRole('dialog', { name: 'Seller workspace' })
    expect(rail).toHaveAttribute('aria-modal', 'true')
    await waitFor(() => expect(rail).toContainElement(document.activeElement as HTMLElement))
    expect(opener).toHaveAttribute('aria-expanded', 'true')

    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(opener).toHaveFocus()
  })

  it('closes the rail after choosing a module', async () => {
    viewport(true, false)
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await userEvent.click(await screen.findByRole('button', { name: 'Open menu' }))
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('link', { name: /^Orders/ }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Orders' })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Open menu' })).toHaveAttribute('aria-expanded', 'false')
  })

  it('gives the seller a four-module tab bar plus "More" on phones', async () => {
    viewport(true, true)
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    const bar = screen.getByRole('navigation', { name: 'Quick navigation' })
    expect(within(bar).getAllByRole('link').map((l) => l.textContent)).toEqual(['Dashboard', 'Orders', 'Reservations', 'Listings'])
    await userEvent.click(within(bar).getByRole('button', { name: 'More' }))
    expect(screen.getByRole('dialog', { name: 'Seller workspace' })).toBeInTheDocument()
  })

  it('moves the language switch into the menu on phones so it stays reachable', async () => {
    viewport(true, true)
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    expect(within(screen.getByRole('banner')).queryByRole('group', { name: 'Language' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    expect(within(screen.getByRole('dialog')).getByRole('group', { name: 'Language' })).toBeInTheDocument()
    expect(screen.getAllByRole('group', { name: 'Language' })).toHaveLength(1)
  })

  it('keeps the language switch in the header on tablets and desktops', async () => {
    viewport(false, false)
    signIn()
    mockApi(sellerSession())
    renderAt('/seller')
    await sellerNav()
    expect(within(screen.getByRole('banner')).getByRole('group', { name: 'Language' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Quick navigation' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'More' })).not.toBeInTheDocument()
  })

  it('keeps the administration free of a bottom tab bar', async () => {
    viewport(true, true)
    signIn()
    mockApi(adminSession())
    renderAt('/admin')
    await adminNav()
    expect(screen.queryByRole('navigation', { name: 'Quick navigation' })).not.toBeInTheDocument()
  })
})

describe('module skeletons: every module has a route', () => {
  const planned = (list: typeof sellerModules) => list.filter((m) => m.status.kind !== 'ready')

  it.each(planned(sellerModules).map((m) => [m.label, m.to, m.status.kind] as const))('seller: %s (%s) renders its own page, not "not found"', async (label, to, kind) => {
    signIn()
    mockApi(sellerSession([gombe]))
    const { unmount } = renderAt(to)
    expect(await screen.findByRole('heading', { level: 1, name: label })).toBeInTheDocument()
    expect(screen.queryByText('This page doesn’t exist')).not.toBeInTheDocument()
    expect(screen.getByRole('region', { name: kind === 'planned' ? 'Coming soon' : 'Not available yet' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: new RegExp(`^${label}`) })).toHaveAttribute('aria-current', 'page')
    unmount()
  })

  it.each(planned(adminModules).map((m) => [m.label, m.to, m.status.kind] as const))('administration: %s (%s) renders its own page, not "not found"', async (label, to, kind) => {
    signIn()
    mockApi(adminSession(admin))
    const { unmount } = renderAt(to)
    expect(await screen.findByRole('heading', { level: 1, name: label })).toBeInTheDocument()
    expect(screen.queryByText('This page doesn’t exist')).not.toBeInTheDocument()
    expect(screen.getByRole('region', { name: kind === 'planned' ? 'Coming soon' : 'Not available yet' })).toBeInTheDocument()
    unmount()
  })

  it.each(sellerModules.filter((m) => m.status.kind === 'ready').map((m) => [m.label, m.to] as const))('seller: the built module %s (%s) is routed', async (_label, to) => {
    signIn()
    mockApi({ ...sellerSession([gombe]), 'GET /hotels/room-types': () => ({ body: { items: [] } }), 'GET /categories': () => ({ body: { items: [] } }) })
    const { unmount } = renderAt(to)
    await sellerNav()
    expect(screen.queryByText('This page doesn’t exist')).not.toBeInTheDocument()
    unmount()
  })

  it('shows no sample data on a module the platform cannot serve', async () => {
    signIn()
    mockApi(adminSession(admin))
    renderAt('/admin/settings')
    const state = await screen.findByRole('region', { name: 'Not available yet' })
    expect(within(state).getByText(/Platform settings/)).toBeInTheDocument()
    expect(within(state).getByText(/no platform settings or feature flags/i)).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /export|download|refund|delete/i })).not.toBeInTheDocument()
  })
})
