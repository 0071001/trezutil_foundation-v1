import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { analyticsBody } from '@/test/fixtures'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/*
 * The seller's Orders page carries an order through its whole lifecycle:
 * paid → confirmed → processing → ready → delivered → completed. Each order shows the one step its status allows;
 * the server enforces who and in which order (a 409 shows the truth, not the stale button).
 */

const gombe = {
  organization_id: 'org1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', role: 'owner',
  seller_profiles: [{ id: 'sp1', display_name: 'Gombe Stays', status: 'active', verification_status: 'verified', mine: true }],
  stores: [{ id: 's1', name: 'Gombe Stays', status: 'active' }],
}
const order = (id: string, status: string, title = `Wax fabric ${id}`) => ({
  id, status, total_amount: { amount_minor: 2500, currency: 'USD' }, currency: 'USD',
  items: [{ listing_id: 'l1', title, quantity: 1 }], created_at: 1790000000, updated_at: 1790000100,
})

function sellerWith(orders: ReturnType<typeof order>[], extra: Record<string, () => { status?: number; body?: unknown }> = {}) {
  signIn()
  return mockApi({
    'GET /auth/users/me': () => ({ body: me }),
    'GET /sellers/me': () => ({ body: { organizations: [gombe] } }),
    'GET /sellers/analytics': () => ({ body: analyticsBody(false) }),
    'GET /listings/manage': () => ({ body: { items: [] } }),
    'GET /reservations/incoming': () => ({ body: { items: [] } }),
    'GET /inventory': () => ({ body: { items: [] } }),
    'GET /orders/incoming': () => ({ body: { items: orders } }),
    ...extra,
  })
}

describe('seller orders: the lifecycle', () => {
  it.each([
    ['paid', 'To confirm', 'Confirm', '/orders/o1/confirm'],
    ['confirmed', 'In progress', 'Start preparing', '/orders/o1/process'],
    ['processing', 'In progress', 'Mark ready', '/orders/o1/ready'],
    ['ready_for_fulfillment', 'In progress', 'Mark delivered', '/orders/o1/fulfil'],
    ['fulfilled', 'In progress', 'Complete order', '/orders/o1/complete'],
  ])('offers exactly one step to an order that is %s, under "%s"', async (status, tab, label, path) => {
    const { calls } = sellerWith([order('o1', status)], { [`POST ${path}`]: () => ({ body: { order_id: 'o1', status: 'x' } }) })
    renderAt('/seller/orders')
    await userEvent.click(await screen.findByRole('tab', { name: new RegExp(tab) }))
    const card = (await screen.findByText(/Wax fabric o1/)).closest('article')!
    const buttons = within(card).getAllByRole('button')
    expect(buttons.map((b) => b.textContent?.trim())).toEqual([label])

    await userEvent.click(buttons[0])
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === path)).toBe(true))
  })

  it('shows nothing to do on an order that is waiting for someone else, or finished', async () => {
    sellerWith([order('o2', 'pending_payment'), order('o3', 'completed'), order('o4', 'cancelled')])
    renderAt('/seller/orders')
    for (const [tab, title] of [['Awaiting payment', 'Wax fabric o2'], ['Completed', 'Wax fabric o3'], ['Cancelled', 'Wax fabric o4']] as const) {
      await userEvent.click(await screen.findByRole('tab', { name: new RegExp(tab) }))
      const card = (await screen.findByText(title)).closest('article')!
      expect(within(card).queryAllByRole('button')).toHaveLength(0)
    }
  })

  it('keeps a delivered order in progress until it is completed, and a completed one under Completed', async () => {
    sellerWith([order('o5', 'fulfilled'), order('o6', 'completed')])
    renderAt('/seller/orders')
    expect(await screen.findByRole('tab', { name: /In progress\s*1/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /Completed\s*1/ })).toBeInTheDocument()
  })

  it('reloads the list after a step, so the order moves to where it now belongs', async () => {
    let status = 'confirmed'
    const { calls } = sellerWith([], {
      'GET /orders/incoming': () => ({ body: { items: [order('o7', status)] } }),
      'POST /orders/o7/process': () => { status = 'processing'; return { body: { order_id: 'o7', status } } },
    })
    renderAt('/seller/orders')
    await userEvent.click(await screen.findByRole('tab', { name: /In progress/ }))
    await userEvent.click(await screen.findByRole('button', { name: /Start preparing/ }))
    expect(await screen.findByRole('button', { name: /Mark ready/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Start preparing/ })).not.toBeInTheDocument()
    expect(calls.filter((c) => c.path.startsWith('/orders/incoming')).length).toBeGreaterThanOrEqual(2)
  })

  it('tells the seller when the order was changed elsewhere, and reloads instead of keeping a stale button', async () => {
    let status = 'confirmed'
    const { calls } = sellerWith([], {
      'GET /orders/incoming': () => ({ body: { items: [order('o8', status)] } }),
      // someone else already started it: the server answers a conflict
      'POST /orders/o8/process': () => { status = 'processing'; return { status: 409, body: { error: { code: 'INVALID_TRANSITION', message: 'The order is not in a state that allows this step' } } } },
    })
    renderAt('/seller/orders')
    await userEvent.click(await screen.findByRole('tab', { name: /In progress/ }))
    await userEvent.click(await screen.findByRole('button', { name: /Start preparing/ }))
    expect(await screen.findByText(/not in a state that allows this step/i)).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: /Mark ready/ })).toBeInTheDocument()
    expect(calls.filter((c) => c.path.startsWith('/orders/incoming')).length).toBeGreaterThanOrEqual(2)
  })
})
