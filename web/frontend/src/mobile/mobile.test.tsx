import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetCatalog } from '@/mobile/catalog'
import { me, mockApi, renderAt, searchItem, signIn, type MockCall } from '@/test/utils'

/* The mobile layout (Design Trezutil Mobile App) on a phone-sized screen, fed with the real catalogue only. */

function phone() {
  vi.stubGlobal('matchMedia', (q: string) => ({
    matches: q.includes('max-width: 760px'), media: q, onchange: null,
    addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false,
  }))
}

const TOTALS: Record<string, number> = { REAL_ESTATE: 15, RESTAURANT: 12, RETAIL: 83, SERVICES: 0, HOTEL: 4, EVENTS: 0 }

const search = (call: MockCall) => {
  const body = call.body as { vertical?: string; page_size?: number; text?: string }
  if (body.vertical && body.page_size === 1) {
    const total = TOTALS[body.vertical] ?? 0
    return { body: { items: total ? [searchItem(`${body.vertical}-1`, `Une offre ${body.vertical}`)] : [], total } }
  }
  if (body.vertical === 'HOTEL') return { body: { items: [searchItem('h1', 'Makala City Hotel', { Vertical: 'HOTEL' })], total: 4 } }
  return { body: { items: [searchItem('c1', 'Collier oiseau en or', { Vertical: 'RETAIL', PriceAmount: 79.99 }), searchItem('r1', 'Noki Pizzeria', { Vertical: 'RESTAURANT' })], total: 2 } }
}

describe('the mobile layout', () => {
  beforeEach(() => { phone(); resetCatalog() })
  afterEach(() => vi.unstubAllGlobals())

  it('opens on the mobile home: the composer, the real kinds of offers and published offers, with the tab bar', async () => {
    mockApi({ 'POST /search': search })
    renderAt('/')
    expect(await screen.findByRole('heading', { name: /Que cherches-tu/ })).toBeInTheDocument()
    expect(screen.getByLabelText('Ton besoin')).toBeInTheDocument()
    const nav = screen.getByRole('navigation', { name: 'Navigation principale' })
    expect(within(nav).getByRole('button', { name: 'Accueil' })).toHaveAttribute('aria-current', 'page')
    // Only kinds with published offers are offered.
    expect(await screen.findByRole('button', { name: 'Immobilier' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Produits' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Services' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Expériences' })).not.toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Voir l’offre Collier oiseau en or' })).toBeInTheDocument()
    // No rating is shown: the backend has none.
    expect(screen.queryByText(/★|4\.\d/)).not.toBeInTheDocument()
  })

  it('starts a conversation from the home composer', async () => {
    const { calls } = mockApi({
      'POST /search': search,
      'POST /conversations/message': () => ({ body: { conversation_id: 'conv-1', response: { text: 'Voici des colliers.', metadata: { action_type: 'SEARCH_LISTINGS', result_data: { items: [] } } } } }),
    })
    renderAt('/')
    const field = await screen.findByLabelText('Ton besoin')
    fireEvent.change(field, { target: { value: 'un collier en or' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(await screen.findByText('Voici des colliers.')).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/conversations/message' && (c.body as { message_text: string }).message_text === 'un collier en or')).toBe(true)
    // The conversation has the mobile header.
    expect(screen.getByRole('button', { name: 'Retour à l’accueil' })).toBeInTheDocument()
    expect(screen.getByText('Disponible')).toBeInTheDocument()
  })

  it('discovers by kind with the real number of offers', async () => {
    mockApi({ 'POST /search': search })
    renderAt('/discover')
    expect(await screen.findByText('Tendances à Kinshasa')).toBeInTheDocument()
    // The busiest kind is trending and a collection, with its real count.
    expect(await screen.findAllByText('83 produits')).toHaveLength(2)
    await userEvent.click(screen.getByRole('button', { name: 'Hôtels', pressed: false }))
    expect(await screen.findByRole('button', { name: 'Voir l’offre Makala City Hotel' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hôtels', pressed: true })).toBeInTheDocument()
  })

  it('shows a guest how to see orders and bookings, and lists the account’s activity once signed in', async () => {
    mockApi({ 'POST /search': search })
    const { unmount } = renderAt('/activity')
    expect(await screen.findByRole('heading', { name: 'Historique' })).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: 'Se connecter' })).toBeInTheDocument()
    unmount()

    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /orders': () => ({ body: { items: [{ id: 'o1', status: 'pending_payment', currency: 'CDF', total_amount: { amount_minor: 4200000, currency: 'CDF' }, items: [{ listing_id: 'l1', title: 'Ciment CPJ 42.5', quantity: 1 }], created_at: 1700000000 }] } }),
      'GET /reservations': () => ({ body: { items: [] } }),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'GET /conversations': () => ({ body: { items: [] } }),
      'GET /sellers/me': () => ({ body: { organizations: [] } }),
    })
    renderAt('/activity')
    expect(await screen.findByText('Ciment CPJ 42.5')).toBeInTheDocument()
    expect(screen.getByText('Paiement attendu')).toBeInTheDocument()
    expect(screen.getByText('Aucune réservation.')).toBeInTheDocument()
    // Something waits for the person: they are not told they are up to date.
    expect(await screen.findByText('1 commande à payer')).toBeInTheDocument()
    expect(screen.queryByText('Tu es à jour')).not.toBeInTheDocument()
  })

  it('shows the signed-in profile with real counts and switches the theme', async () => {
    signIn()
    mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /orders': () => ({ body: { items: [] } }),
      'GET /reservations': () => ({ body: { items: [] } }),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'GET /conversations': () => ({ body: { items: [] } }),
      'GET /sellers/me': () => ({ body: { organizations: [] } }),
    })
    renderAt('/profile')
    expect(await screen.findByRole('heading', { name: 'Jean Mukendi' })).toBeInTheDocument()
    expect(screen.getByText('+243810000001')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('Achats').previousSibling).toHaveTextContent('0'))
    const theme = screen.getByRole('button', { name: /Mode sombre/ })
    await userEvent.click(theme)
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    await userEvent.click(screen.getByRole('button', { name: /Mode clair/ }))
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    expect(screen.getByRole('button', { name: /Se déconnecter/ })).toBeInTheDocument()
  })
})
