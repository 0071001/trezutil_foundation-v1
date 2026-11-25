import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/* The Control Center (/admin): only the Trezutil team gets in, and every figure comes from the administration API. */

const staff = { ...me, id: 'admin-1', display_name: 'Gilio Kapapa', email: 'g@trezutil.com', roles: ['admin'] }

const overview = {
  generated_at: '2026-11-25T10:00:00Z',
  users: { total: 42, active: 40, suspended: 2, new_last_7_days: 5, by_role: { customer: 38, seller: 3, admin: 1 } },
  organizations: 4,
  listings_by_status: { published: 17, draft: 3 },
  orders: { total: 9, last_7_days: 2 },
  reservations: 1,
  conversations: { last_7_days: 31 },
}

const application = {
  organization_id: 'org-1', status: 'submitted', editable: false, missing: [], review_note: '', documents: [], submitted_at: '2026-11-24T08:00:00Z', reviewed_at: '',
  org_type: 'business', legal_name: 'Ciment du Congo SARL', display_name: 'Ciment du Congo', description: '', phone: '+243810000002', email: '', country: 'CD',
  province: 'Kinshasa', city: 'Gombe', address: '', legal_form: 'SARL', rccm: 'CD/KIN/RCCM/1', id_nat: '', nif: '', website: '', contact_name: '', contact_role: '',
  sectors: ['Matériaux'], capabilities: 'Ciment, 500 sacs par jour', min_order: '', lead_time_days: null, payment_terms: '', service_areas: ['Gombe'],
}

function staffRoutes(extra: Record<string, Parameters<typeof mockApi>[0][string]> = {}) {
  signIn()
  return mockApi({
    'GET /auth/users/me': () => ({ body: staff }),
    'GET /admin/overview': () => ({ body: overview }),
    'GET /admin/health': () => ({ body: { status: 'ok', checked_at: '2026-11-25T10:00:00Z', components: [{ name: 'postgres', kind: 'database', status: 'ok', latency_ms: 3 }], configured: {} } }),
    'GET /admin/operations': () => ({ body: { days: 7, stuck_after_hours: 24, counts: {}, recent: [] } }),
    'GET /admin/supplier-applications': () => ({ body: { items: [application] } }),
    'GET /admin/channels': () => ({ body: { items: [], configured: {} } }),
    'GET /admin/orders': () => ({ body: { items: [], total: 0, page: 1, page_size: 6 } }),
    'GET /exchange-rate': () => ({ body: { base: 'USD', quote: 'CDF', rate: null } }),
    ...extra,
  })
}

describe('Control Center', () => {
  it('keeps out an account without a team role', async () => {
    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }) })
    renderAt('/admin')
    expect(await screen.findByText(/réservé à l’équipe Trezutil/)).toBeInTheDocument()
    expect(screen.queryByText('Command Center')).not.toBeInTheDocument()
  })

  it('asks a visitor to sign in', async () => {
    mockApi({})
    renderAt('/admin')
    expect(await screen.findByRole('link', { name: 'Se connecter' })).toHaveAttribute('href', '/login?returnTo=%2Fadmin')
  })

  it('shows the real figures and what needs the team, nothing invented', async () => {
    staffRoutes()
    renderAt('/admin')
    expect(await screen.findByRole('heading', { name: 'Bonjour, Gilio' })).toBeInTheDocument()
    expect(await screen.findByText('42')).toBeInTheDocument()
    expect(screen.getByText('17')).toBeInTheDocument()
    expect(screen.getByText('31')).toBeInTheDocument()
    // One application waiting, no exchange rate: both are listed to handle.
    const todo = await screen.findByText('Dossiers fournisseurs à vérifier')
    expect(within(todo.closest('a')!).getByText('1')).toBeInTheDocument()
    expect(screen.getByText('Aucun taux CDF / USD saisi')).toBeInTheDocument()
    // No mock figures from the design.
    expect(screen.queryByText(/€/)).not.toBeInTheDocument()
  })

  it('verifies a supplier only on the team’s decision, and asks a reason to refuse', async () => {
    const { calls } = staffRoutes({
      'POST /admin/supplier-applications/org-1/review': (c) => ({ body: { ...application, status: (c.body as { decision: string }).decision === 'verify' ? 'verified' : 'rejected' } }),
    })
    renderAt('/admin/verification')
    await userEvent.click(await screen.findByRole('button', { name: /Ciment du Congo/ }))
    expect(screen.getByText('Ciment, 500 sacs par jour')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refuser' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: /Vérifier l’entreprise/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path.includes('/review'))).toBe(true))
    expect(calls.find((c) => c.path.includes('/review'))!.body).toMatchObject({ decision: 'verify' })
  })
})
