import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

const admin = { ...me, id: 'a1', display_name: 'Guilbert', email: 'guilbertmkap@gmail.com', phone: '', roles: ['admin'] }
const operator = { ...admin, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', roles: ['operator'] }
const moderator = { ...admin, id: 'm1', display_name: 'Moussa', email: 'moussa@example.com', roles: ['moderator'] }

const event = (id: number, action: string, extra: Record<string, unknown> = {}) => ({
  id, at: '2026-10-05T14:30:00Z', actor_id: 'a1', actor_name: 'Guilbert', action, target_type: 'order', target_id: 'order-0000-1111-2222',
  request_id: `req-${id}`, metadata: {}, ...extra,
})
const trail = (items: Record<string, unknown>[], total = items.length, extra: Record<string, unknown> = {}) => ({ items, total, page: 1, page_size: 50, ...extra })

describe('administration: the audit trail', () => {
  it('lists what was done, by whom and to what, in plain words', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ body: trail([
        event(3, 'order.confirmed', { metadata: { previous_status: 'paid', new_status: 'confirmed' } }),
        event(2, 'user.suspended', { target_type: 'user', target_id: 'user-9', metadata: { previous_status: 'active', new_status: 'suspended', reason: 'repeated chargebacks' } }),
        event(1, 'order.created', { actor_id: '', actor_name: '' }),
      ]) }),
    })
    renderAt('/admin/audit')
    expect(await screen.findByRole('heading', { name: 'Audit' })).toBeInTheDocument()
    const table = await screen.findByRole('table')
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(3)
    expect(within(rows[0]).getByText('Order confirmed')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Paid → Confirmed')).toBeInTheDocument()
    expect(within(rows[0]).getByRole('button', { name: 'Guilbert' })).toBeInTheDocument()
    expect(within(rows[1]).getByText('Account suspended')).toBeInTheDocument()
    expect(within(rows[1]).getByText(/Active → Suspended · “repeated chargebacks”/)).toBeInTheDocument()
    expect(within(rows[2]).getByText('The platform')).toBeInTheDocument()
  })

  it('sends the filters to the server and starts again from the first page', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: operator }),
      'GET /admin/audit': () => ({ body: trail([event(1, 'order.created')]) }),
    })
    renderAt('/admin/audit')
    await screen.findByRole('table')
    const first = calls.find((c) => c.path.startsWith('/admin/audit'))!
    expect(first.path).toBe('/admin/audit?page=1&page_size=50')

    await userEvent.selectOptions(screen.getByRole('combobox', { name: 'What' }), 'order.')
    await waitFor(() => expect(calls.some((c) => c.path.includes('action=order.'))).toBe(true))
    await userEvent.type(screen.getByLabelText('From'), '2026-10-01')
    await waitFor(() => expect(calls.some((c) => c.path.includes('from=2026-10-01'))).toBe(true))
    await userEvent.type(screen.getByRole('searchbox'), 'order-0000')
    await waitFor(() => expect(calls.some((c) => c.path.includes('target_id=order-0000'))).toBe(true))
    const last = calls.filter((c) => c.path.startsWith('/admin/audit')).at(-1)!
    expect(last.path).toContain('action=order.')
    expect(last.path).toContain('from=2026-10-01')
    expect(last.path).toContain('page=1')

    await userEvent.click(screen.getByRole('button', { name: 'Reset filters' }))
    await waitFor(() => expect(calls.filter((c) => c.path.startsWith('/admin/audit')).at(-1)!.path).toBe('/admin/audit?page=1&page_size=50'))
  })

  it('narrows to one person with a click on their name, and lets go with a click on the chip', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ body: trail([event(1, 'order.confirmed')]) }),
    })
    renderAt('/admin/audit')
    await userEvent.click(await screen.findByRole('button', { name: 'Guilbert' }))
    await waitFor(() => expect(calls.some((c) => c.path.includes('actor_id=a1'))).toBe(true))
    await userEvent.click(await screen.findByRole('button', { name: /Only Guilbert/ }))
    await waitFor(() => expect(calls.filter((c) => c.path.startsWith('/admin/audit')).at(-1)!.path).not.toContain('actor_id'))
  })

  it('pages through the trail from the server', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ body: trail([event(1, 'order.created')], 120) }),
    })
    renderAt('/admin/audit')
    await userEvent.click(await screen.findByRole('button', { name: 'Next' }))
    await waitFor(() => expect(calls.some((c) => c.path.includes('page=2'))).toBe(true))
  })

  it('opens an entry beside the list, with its request id and every detail', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ body: trail([event(5, 'listing.paused', { target_type: 'listing', metadata: { previous_status: 'published', new_status: 'paused', note: 'hello' } })]) }),
    })
    renderAt('/admin/audit')
    await userEvent.click(await screen.findByText('Listing paused'))
    const drawer = await screen.findByRole('dialog', { name: 'Listing paused' })
    expect(within(drawer).getByText('req-5')).toBeInTheDocument()
    expect(within(drawer).getByText('listing.paused')).toBeInTheDocument()
    expect(within(drawer).getByText(/"note": "hello"/)).toBeInTheDocument()
  })

  it('exports the page on screen as a CSV file, nothing more', async () => {
    signIn()
    let blob: Blob | null = null
    const originalCreate = URL.createObjectURL
    const originalRevoke = URL.revokeObjectURL
    URL.createObjectURL = (b: Blob | MediaSource) => { blob = b as Blob; return 'blob:audit' }
    URL.revokeObjectURL = () => {}
    try {
      mockApi({
        'GET /auth/users/me': () => ({ body: admin }),
        'GET /admin/audit': () => ({ body: trail([event(7, 'order.confirmed', { metadata: { new_status: 'confirmed' } })]) }),
      })
      renderAt('/admin/audit')
      await userEvent.click(await screen.findByRole('button', { name: /Export this page/ }))
      expect(blob).not.toBeNull()
      const text = await blob!.text()
      expect(text).toContain('order.confirmed')
      expect(text).toContain('req-7')
      expect(text.split('\r\n').filter(Boolean)).toHaveLength(2) // the header and one entry
    } finally {
      URL.createObjectURL = originalCreate
      URL.revokeObjectURL = originalRevoke
    }
  })

  it('says so when nothing matches, and when the server refuses', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ body: trail([]) }),
    })
    const { unmount } = renderAt('/admin/audit')
    expect(await screen.findByText('Nothing has been recorded yet.')).toBeInTheDocument()
    unmount()

    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/audit': () => ({ status: 500, body: { error: { code: 'ADMIN_QUERY_FAILED', message: 'The audit trail could not be loaded' } } }),
    })
    renderAt('/admin/audit')
    expect(await screen.findByRole('button', { name: /try again|retry/i })).toBeInTheDocument()
  })

  it('does not ask the server for the trail on behalf of someone who may not read it', async () => {
    signIn()
    const { calls } = mockApi({ 'GET /auth/users/me': () => ({ body: moderator }) })
    renderAt('/admin/audit')
    // a moderator is staff (the shell opens) but not an administrator or operator: the shell shows no page
    expect(await screen.findByText('Nothing to show yet')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Audit' })).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.startsWith('/admin/audit'))).toBe(false)
  })
})

const user = (id: string, name: string, roles: string[], status = 'active') => ({
  id, email: `${name.toLowerCase()}@example.com`, phone: '', display_name: name, status, roles, created_at: '2026-10-01T10:00:00Z',
})
const users = (items: Record<string, unknown>[]) => ({ items, total: items.length, page: 1, page_size: 25 })

describe('administration: suspending and reactivating an account', () => {
  const list = () => users([
    user('a1', 'Guilbert', ['admin']),
    user('o9', 'Olivia', ['operator']),
    user('c1', 'Marie', ['customer']),
    user('c2', 'Paul', ['customer'], 'suspended'),
    user('c3', 'Nadia', ['customer'], 'pending_verification'),
  ])

  it('offers the right step to the right accounts only', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: admin }), 'GET /admin/users': () => ({ body: list() }) })
    renderAt('/admin/users')
    await screen.findByText('marie@example.com')
    expect(screen.getByRole('button', { name: /Suspend Marie/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Reactivate Paul/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Guilbert/ })).not.toBeInTheDocument() // yourself
    expect(screen.queryByRole('button', { name: /Olivia/ })).not.toBeInTheDocument() // staff
    expect(screen.queryByRole('button', { name: /Nadia/ })).not.toBeInTheDocument() // not verified yet
  })

  it('asks for a reason, sends it, and shows the account as suspended after reloading', async () => {
    signIn()
    let suspended = false
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/users': () => ({ body: users([user('c1', 'Marie', ['customer'], suspended ? 'suspended' : 'active')]) }),
      'POST /admin/users/c1/suspend': () => { suspended = true; return { body: { user_id: 'c1', status: 'suspended' } } },
    })
    renderAt('/admin/users')
    await userEvent.click(await screen.findByRole('button', { name: /Suspend Marie/ }))
    const dialog = await screen.findByRole('alertdialog', { name: 'Suspend Marie?' })
    expect(within(dialog).getByText(/up to 15 minutes/)).toBeInTheDocument()

    // no reason: refused in place, nothing sent
    await userEvent.click(within(dialog).getByRole('button', { name: 'Suspend account' }))
    expect(await within(dialog).findByText('Give a reason of at least 3 characters.')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'POST')).toBe(false)

    await userEvent.type(within(dialog).getByRole('textbox'), 'repeated chargebacks')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Suspend account' }))
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ reason: 'repeated chargebacks' }))
    expect(await screen.findByRole('button', { name: /Reactivate Marie/ })).toBeInTheDocument()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('reactivates without asking for a reason', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/users': () => ({ body: users([user('c2', 'Paul', ['customer'], 'suspended')]) }),
      'POST /admin/users/c2/reactivate': () => ({ body: { user_id: 'c2', status: 'active' } }),
    })
    renderAt('/admin/users')
    await userEvent.click(await screen.findByRole('button', { name: /Reactivate Paul/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Reactivate Paul?' })
    await userEvent.click(within(dialog).getByRole('button', { name: 'Reactivate account' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/admin/users/c2/reactivate')).toBe(true))
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({})
  })

  it('shows the server’s refusal in the dialog and keeps it open', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: admin }),
      'GET /admin/users': () => ({ body: users([user('c1', 'Marie', ['customer'])]) }),
      'POST /admin/users/c1/suspend': () => ({ status: 409, body: { error: { code: 'INVALID_STATE', message: 'the account is not in a state that allows this step' } } }),
    })
    renderAt('/admin/users')
    await userEvent.click(await screen.findByRole('button', { name: /Suspend Marie/ }))
    const dialog = await screen.findByRole('alertdialog')
    await userEvent.type(within(dialog).getByRole('textbox'), 'a reason')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Suspend account' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(/not in a state/)
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })
})
