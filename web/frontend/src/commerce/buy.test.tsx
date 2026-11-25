import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { cart } from '@/commerce/cart'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

/* An offer in the conversation opens on a click, and a product is bought at once or put in the cart. */

const reply = {
  conversation_id: 'conv-1',
  response: {
    text: 'J’ai trouvé 3 offres.',
    metadata: {
      action_type: 'SEARCH_LISTINGS',
      result_data: {
        items: [
          searchItem('l1', 'Ciment CPJ 42.5', { Vertical: 'RETAIL', PriceAmount: 42000, PriceCurrency: 'CDF', Store: { StoreID: 'st-q', Name: 'Quincaillerie' }, Seller: { SellerID: 'sp-q', DisplayName: 'Quincaillerie Limete (démo)', TrustLevel: 0 } }),
          searchItem('l2', 'Fer à béton 12 mm', { Vertical: 'RETAIL', PriceAmount: 38000, PriceCurrency: 'CDF', Store: { StoreID: 'st-q', Name: 'Quincaillerie' }, Seller: { SellerID: 'sp-q', DisplayName: 'Quincaillerie Limete (démo)', TrustLevel: 0 } }),
          searchItem('l3', 'Plomberie express', { Vertical: 'SERVICES', PriceAmount: 35 }),
        ],
      },
    },
  },
}
const listing = { ID: 'l1', Title: 'Ciment CPJ 42.5', Description: 'Sac de 50 kg.', PriceAmount: 42000, PriceCurrency: 'CDF', Status: 'published', Media: [] }
const order = (id: string, items: unknown[]) => ({ id, organization_id: 'org-q', status: 'pending_payment', currency: 'CDF', total_amount: { amount_minor: 8400000, currency: 'CDF' }, items, created_at: 1700000000 })

async function search() {
  const field = await screen.findByLabelText('Ton besoin')
  fireEvent.change(field, { target: { value: 'ciment' } })
  fireEvent.keyDown(field, { key: 'Enter' })
  return screen.findByRole('article', { name: /Offre 1 : Ciment/ })
}

describe('buying from an offer', () => {
  beforeEach(() => cart.clear())

  it('opens the offer when the card itself is clicked', async () => {
    mockApi({ 'POST /conversations/message': () => ({ body: reply }), 'GET /listings': () => ({ body: listing }) })
    renderAt('/')
    const card = await search()
    await userEvent.click(within(card).getByText('Ciment CPJ 42.5'))
    expect(await screen.findByRole('dialog', { name: /Offre : Ciment CPJ 42.5/ })).toBeInTheDocument()
  })

  it('buys a product at once in the chosen quantity, then lands on the order to pay it', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'POST /conversations/message': () => ({ body: reply }),
      'GET /listings': () => ({ body: listing }),
      'POST /orders': () => ({ status: 201, body: { order_id: 'o-9', status: 'pending_payment' } }),
      'GET /orders/o-9': () => ({ body: order('o-9', [{ listing_id: 'l1', title: 'Ciment CPJ 42.5', quantity: 2, unit_price: { amount_minor: 4200000 } }]) }),
      'GET /orders': () => ({ body: { items: [] } }),
      'GET /conversations': () => ({ body: { items: [] } }),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'GET /sellers/me': () => ({ body: { organizations: [] } }),
    })
    renderAt('/')
    await userEvent.click(within(await search()).getByRole('button', { name: /Voir l’offre/ }))
    const panel = await screen.findByRole('dialog', { name: /Offre : Ciment/ })
    await userEvent.click(within(panel).getByRole('button', { name: 'Ajouter un' }))
    await userEvent.click(within(panel).getByRole('button', { name: /Acheter maintenant/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/orders')).toBe(true))
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')!.body).toEqual({ listing_id: 'l1', quantity: 2 })
    expect(await screen.findByRole('radio', { name: /M-Pesa/ })).toBeInTheDocument()
  })

  it('puts products in the cart and orders them together from one shop', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'POST /conversations/message': () => ({ body: reply }),
      'POST /orders': () => ({ status: 201, body: { order_id: 'o-10', status: 'pending_payment' } }),
      'GET /orders/o-10': () => ({ body: order('o-10', []) }),
      'GET /orders': () => ({ body: { items: [] } }),
      'GET /conversations': () => ({ body: { items: [] } }),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'GET /sellers/me': () => ({ body: { organizations: [] } }),
    })
    renderAt('/')
    const first = await search()
    await userEvent.click(within(first).getByRole('button', { name: /Ajouter Ciment CPJ 42.5 au panier/ }))
    expect(within(first).getByText(/Dans le panier \(1\)/)).toBeInTheDocument()
    await userEvent.click(within(first).getByRole('button', { name: /Ajouter Ciment CPJ 42.5 au panier/ }))
    await userEvent.click(screen.getByRole('button', { name: /Ajouter Fer à béton 12 mm au panier/ }))
    // A service is asked for, not put in a cart.
    expect(screen.queryByRole('button', { name: /Ajouter Plomberie express au panier/ })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Panier, 3 article/ }))
    const drawer = await screen.findByRole('dialog', { name: 'Panier' })
    expect(within(drawer).getByText('Quincaillerie Limete (démo)')).toBeInTheDocument()
    await userEvent.click(within(drawer).getByRole('button', { name: /Commander et payer/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/orders')).toBe(true))
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')!.body).toEqual({ items: [{ listing_id: 'l1', quantity: 2 }, { listing_id: 'l2', quantity: 1 }] })
    await waitFor(() => expect(cart.count()).toBe(0))
  })

  it('keeps a visitor’s product in the cart and asks them to sign in', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: reply }), 'GET /listings': () => ({ body: listing }) })
    renderAt('/')
    await userEvent.click(within(await search()).getByRole('button', { name: /Voir l’offre/ }))
    const panel = await screen.findByRole('dialog', { name: /Offre : Ciment/ })
    await userEvent.click(within(panel).getByRole('button', { name: /Se connecter pour acheter/ }))
    expect(await screen.findByRole('heading', { name: /Connecte-toi à Trezutil/ })).toBeInTheDocument()
    expect(cart.items().map((i) => [i.listingId, i.quantity])).toEqual([['l1', 1]])
    expect(calls.some((c) => c.method === 'POST' && c.path === '/orders')).toBe(false)
  })
})
