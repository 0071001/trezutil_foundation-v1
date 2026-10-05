import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { analyticsBody } from '@/test/fixtures'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

const admin = { ...me, id: 'a1', display_name: 'Guilbert', email: 'guilbertmkap@gmail.com', phone: '', roles: ['admin'] }
const operator = { ...admin, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', roles: ['operator'] }

const overview = {
  generated_at: '2026-10-04T12:00:00Z',
  users: { total: 7, active: 6, suspended: 1, new_last_7_days: 3, by_role: { customer: 5, admin: 1, seller: 1 } },
  organizations: 2,
  listings_by_status: { published: 197, draft: 3 },
  orders: { total: 12, last_7_days: 4 },
  reservations: 5,
  conversations: { last_7_days: 30 },
}

const page = (items: Record<string, unknown>[], total = items.length) => ({ items, total, page: 1, page_size: 25 })
const row = (id: string, name: string, email: string, roles: string[], extra: Record<string, unknown> = {}) => ({
  id, email, phone: '', display_name: name, status: 'active', roles, created_at: '2026-10-01T10:00:00Z', ...extra,
})

describe('administration sign-in', () => {
  it('sends a guest to the staff sign-in page', async () => {
    mockApi({})
    renderAt('/admin')
    expect(await screen.findByRole('heading', { name: 'Sign in to the administration' })).toBeInTheDocument()
  })

  it('signs in with an email and a password and opens the console', async () => {
    const { calls } = mockApi({
      'POST /auth/staff/login': () => ({ body: { user: admin, tokens: { access_token: 'acc', refresh_token: 'ref', expires_at: '' } } }),
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/overview': () => ({ body: overview }),
    })
    renderAt('/admin/login')
    await userEvent.type(await screen.findByLabelText('Email address'), '  guilbertmkap@gmail.com ')
    await userEvent.type(screen.getByLabelText('Password'), 'Kap0071001')
    await userEvent.click(screen.getByRole('button', { name: /^sign in/i }))

    expect(await screen.findByRole('heading', { name: 'Overview' })).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/auth/staff/login')!.body).toEqual({ email: 'guilbertmkap@gmail.com', password: 'Kap0071001' })
    expect(localStorage.getItem('trezutil_access_token')).toBe('acc')
    expect(await screen.findByText('Active accounts')).toBeInTheDocument()
    expect(screen.getByText('200')).toBeInTheDocument() // 197 published + 3 draft
    expect(screen.getByText(/Administrator · guilbertmkap@gmail.com/)).toBeInTheDocument()
  })

  it('says the same thing for every wrong answer and clears the password', async () => {
    mockApi({ 'POST /auth/staff/login': () => ({ status: 401, body: { error: { code: 'INVALID_CREDENTIALS', message: 'Email or password is incorrect' } } }) })
    renderAt('/admin/login')
    await userEvent.type(await screen.findByLabelText('Email address'), 'guilbertmkap@gmail.com')
    await userEvent.type(screen.getByLabelText('Password'), 'not-the-password')
    await userEvent.click(screen.getByRole('button', { name: /^sign in/i }))
    expect(await screen.findByText('Email or password is incorrect.')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toHaveValue('')
    expect(localStorage.getItem('trezutil_access_token')).toBeNull()
  })

  it('explains a rate limit', async () => {
    mockApi({ 'POST /auth/staff/login': () => ({ status: 429, body: { error: { code: 'RATE_LIMITED', message: 'x' } } }) })
    renderAt('/admin/login')
    await userEvent.type(await screen.findByLabelText('Email address'), 'a@b.cd')
    await userEvent.type(screen.getByLabelText('Password'), 'whatever')
    await userEvent.click(screen.getByRole('button', { name: /^sign in/i }))
    expect(await screen.findByText(/too many attempts/i)).toBeInTheDocument()
  })

  it('asks for both fields before calling the server', async () => {
    const { calls } = mockApi({})
    renderAt('/admin/login')
    await userEvent.click(await screen.findByRole('button', { name: /^sign in/i }))
    expect(await screen.findByText('Enter your email address and your password.')).toBeInTheDocument()
    expect(calls.filter((c) => c.path === '/auth/staff/login')).toHaveLength(0)
  })

  it('explains a malformed email address instead of the generic refusal, and sends nothing', async () => {
    const { calls } = mockApi({})
    renderAt('/admin/login')
    for (const bad of ['guilbert', 'guilbertmkap@gmail', 'guilbert mkap@gmail.com', '@gmail.com']) {
      await userEvent.clear(await screen.findByLabelText('Email address'))
      await userEvent.type(screen.getByLabelText('Email address'), bad)
      await userEvent.type(screen.getByLabelText('Password'), 'Whatever-1234')
      await userEvent.click(screen.getByRole('button', { name: /^sign in/i }))
      expect(await screen.findByText('Enter a valid email address, like name@company.com.')).toBeInTheDocument()
      await userEvent.clear(screen.getByLabelText('Password'))
    }
    expect(calls.filter((c) => c.path === '/auth/staff/login')).toHaveLength(0)
  })

  it('reveals the password only on request', async () => {
    mockApi({})
    renderAt('/admin/login')
    const field = await screen.findByLabelText('Password')
    expect(field).toHaveAttribute('type', 'password')
    await userEvent.click(screen.getByRole('button', { name: 'Show password' }))
    expect(field).toHaveAttribute('type', 'text')
  })

  it('does not let a customer into the console', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }) })
    renderAt('/admin')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Overview' })).not.toBeInTheDocument()
  })

  it('tells a customer on the sign-in page that this is not their door', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }) })
    renderAt('/admin/login')
    expect(await screen.findByText('This account is not an administration account.')).toBeInTheDocument()
  })

  it('goes straight to the console when already signed in as staff', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: admin }), 'GET /admin/users': () => ({ body: page([row('u1', 'Marie', 'marie@example.com', ['customer'])]) }) })
    renderAt('/admin/login?returnTo=%2Fadmin%2Fusers')
    // The saved destination inside the administration is honoured.
    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
    expect(await screen.findByText('marie@example.com')).toBeInTheDocument()
  })

  it('never follows a returnTo outside the administration area', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: admin }), 'GET /admin/overview': () => ({ body: overview }) })
    renderAt('/admin/login?returnTo=%2Forders%2Fsecret')
    expect(await screen.findByRole('heading', { name: 'Overview' })).toBeInTheDocument()
  })
})

describe('administration console', () => {
  it('shows the platform numbers to an operator but keeps the user list for administrators', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: operator }), 'GET /admin/overview': () => ({ body: overview }) })
    renderAt('/admin')
    expect(await screen.findByText('Active accounts')).toBeInTheDocument()
    expect(screen.getByText(/Operator · olivia@example.com/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Users' })).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/admin/users'))).toBe(false)
  })

  it('refuses the user list to an operator even by address', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: operator }) })
    renderAt('/admin/users')
    expect(await screen.findByText('Administrators only')).toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/admin/users'))).toBe(false)
  })

  it('lists accounts and searches by name, email or phone', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/users': (c) => (c.path.includes('q=marie')
        ? { body: page([row('u2', 'Marie Kabila', 'marie@example.com', ['customer', 'seller'])], 1) }
        : { body: page([row('u1', 'Guilbert', 'guilbertmkap@gmail.com', ['admin'], { last_login_at: '2026-10-04T08:00:00Z' }), row('u2', 'Marie Kabila', 'marie@example.com', ['customer', 'seller']), row('u3', 'Suspended One', 'x@example.com', ['customer'], { status: 'suspended' })], 3) }),
    })
    renderAt('/admin/users')
    const table = await screen.findByRole('table')
    expect(within(table).getByText('guilbertmkap@gmail.com')).toBeInTheDocument()
    expect(within(table).getAllByText('Administrator').length).toBeGreaterThan(0)
    expect(within(table).getByText('Suspended')).toBeInTheDocument()
    expect(screen.getByText('3 accounts')).toBeInTheDocument()
    expect(calls.find((c) => c.path.startsWith('/admin/users'))!.path).toBe('/admin/users?page=1&page_size=25')

    await userEvent.type(screen.getByLabelText('Search users'), 'marie')
    await waitFor(() => expect(calls.some((c) => c.path === '/admin/users?q=marie&page=1&page_size=25')).toBe(true))
    await waitFor(() => expect(screen.getByText('1 accounts')).toBeInTheDocument())
    expect(within(screen.getByRole('table')).queryByText('guilbertmkap@gmail.com')).not.toBeInTheDocument()
  })

  it('says so when nothing matches, and pages through long lists', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/users': (c) => (c.path.includes('q=zzz') ? { body: page([], 0) } : { body: { items: [row('u1', 'One', 'one@example.com', ['customer'])], total: 60, page: 1, page_size: 25 } }),
    })
    renderAt('/admin/users')
    expect(await screen.findByText('Page 1 of 3')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled()
    await userEvent.type(screen.getByLabelText('Search users'), 'zzz')
    expect(await screen.findByText('No account matches')).toBeInTheDocument()
  })

  it('shows a retryable error when the server fails', async () => {
    signIn()
    let fail = true
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/overview': () => (fail ? { status: 500, body: { error: { code: 'ADMIN_QUERY_FAILED', message: 'The overview could not be loaded' } } } : { body: overview }),
      'GET /admin/analytics': () => ({ body: analyticsBody(true) }), // the activity block has its own state: only the snapshot fails here
    })
    renderAt('/admin')
    expect(await screen.findByText('The overview could not be loaded')).toBeInTheDocument()
    fail = false
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('Active accounts')).toBeInTheDocument()
  })

  it('signs out and returns to the sign-in page', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: admin }), 'GET /admin/overview': () => ({ body: overview }) })
    renderAt('/admin')
    await screen.findByText('Active accounts')
    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(await screen.findByRole('heading', { name: 'Sign in to the administration' })).toBeInTheDocument()
    expect(localStorage.getItem('trezutil_access_token')).toBeNull()
  })
})
