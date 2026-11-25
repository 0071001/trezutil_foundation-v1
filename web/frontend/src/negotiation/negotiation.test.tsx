import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

/* Negotiation in the web app: the buyer sets a mandate and decides; the supplier answers without seeing it. */

const common = {
  'GET /auth/users/me': () => ({ body: me }),
  'GET /orders': () => ({ body: { items: [] } }),
  'GET /conversations': () => ({ body: { items: [] } }),
  'GET /missions': () => ({ body: { items: [], total: 0 } }),
  'GET /sellers/me': () => ({ body: { organizations: [] } }),
}

const negotiation = (extra: Record<string, unknown> = {}) => ({
  id: 'n1', title: 'Ciment CPJ 42.5', quantity: 10, currency: 'USD', status: 'awaiting_decision', deadline: Math.floor(Date.now() / 1000) + 90000, order_id: '', created_at: 1700000000,
  mandate: { target_unit_minor: 4000, max_unit_minor: 4600, max_rounds: 2, auto_counter: true },
  participants: [{ id: 'p1', supplier: 'Quincaillerie A', listing_id: 'l1', listing_title: 'Ciment CPJ 42.5', list_unit_minor: 5000, status: 'responded', round: 1 }],
  offers: [{ id: 'o1', participant_id: 'p1', round: 1, author: 'supplier', source: 'portal', unit_minor: 3900, quantity: 10, status: 'open', delivery_days: 2, created_at: 1700000100 }],
  events: [{ kind: 'supplier.offer', actor: 'supplier', participant_id: 'p1', data: { unit_minor: 3900, supplier: 'Quincaillerie A' }, created_at: 1700000100 }],
  messages: [{ participant_id: 'p1', channel: 'whatsapp', body: 'x', state: 'skipped', last_error: 'WhatsApp is not configured for suppliers (no approved template)', created_at: 1700000000 }],
  ...extra,
})

const reply = { conversation_id: 'c1', response: { text: 'Voici une offre.', metadata: { action_type: 'SEARCH_LISTINGS', result_data: { items: [searchItem('l1', 'Ciment CPJ 42.5', { Vertical: 'RETAIL', PriceAmount: 50 })] } } } }

describe('negotiating', () => {
  it('sends the buyer’s mandate from an offer and opens the negotiation', async () => {
    signIn()
    const { calls } = mockApi({
      ...common,
      'POST /conversations/message': () => ({ body: reply }),
      'GET /listings': () => ({ body: { ID: 'l1', Title: 'Ciment CPJ 42.5', PriceAmount: 50, PriceCurrency: 'USD', Status: 'published', Media: [], Metadata: {} } }),
      'POST /negotiations': () => ({ status: 201, body: negotiation({ status: 'soliciting', offers: [] }) }),
      'GET /negotiations/n1': () => ({ body: negotiation({ status: 'soliciting', offers: [] }) }),
    })
    renderAt('/')
    const field = await screen.findByLabelText('Ton besoin')
    await userEvent.type(field, 'ciment{Enter}')
    await userEvent.click(within(await screen.findByRole('article', { name: /Offre 1/ })).getByRole('button', { name: /Voir l’offre/ }))
    await userEvent.click(await screen.findByRole('button', { name: /Négocier le prix/ }))
    const dialog = await screen.findByRole('dialog', { name: /Quincaillerie|Gombe Stays/ })
    expect(within(dialog).getByText(/Ton plafond reste privé/)).toBeInTheDocument()
    const target = within(dialog).getByLabelText(/Prix cible par unité/)
    const ceiling = within(dialog).getByLabelText(/Plafond par unité/)
    await userEvent.clear(target); await userEvent.type(target, '40')
    await userEvent.clear(ceiling); await userEvent.type(ceiling, '38')
    expect(within(dialog).getByRole('button', { name: /Envoyer la demande de prix/ })).toBeDisabled()
    await userEvent.clear(ceiling); await userEvent.type(ceiling, '46')
    await userEvent.click(within(dialog).getByRole('button', { name: /Envoyer la demande de prix/ }))
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/negotiations')).toBe(true))
    expect(calls.find((c) => c.method === 'POST' && c.path === '/negotiations')!.body).toMatchObject({ listing_ids: ['l1'], quantity: 1, target_price: 40, max_price: 46, deadline_hours: 48, max_rounds: 2, auto_counter: true })
    expect(await screen.findByRole('heading', { name: 'Ciment CPJ 42.5' })).toBeInTheDocument()
  })

  it('lets the buyer accept an offer only after confirming, and says why WhatsApp did not leave', async () => {
    signIn()
    const { calls } = mockApi({
      ...common,
      'GET /negotiations/n1': () => ({ body: negotiation() }),
      'POST /negotiations/n1/accept': () => ({ body: negotiation({ status: 'accepted', order_id: 'o-9', accepted_offer_id: 'o1' }) }),
    })
    renderAt('/negotiations/n1')
    const card = await screen.findByRole('article', { name: /Offre de Quincaillerie A/ })
    expect(screen.getByText(/modèle de message n’est pas encore approuvé/)).toBeInTheDocument()
    await userEvent.click(within(card).getByRole('button', { name: 'Accepter' }))
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
    await userEvent.click(within(card).getByRole('button', { name: 'Accepter et commander' }))
    await waitFor(() => expect(calls.find((c) => c.path === '/negotiations/n1/accept')?.body).toEqual({ offer_id: 'o1' }))
    expect(await screen.findByRole('link', { name: 'Voir et payer la commande' })).toHaveAttribute('href', '/purchases/o-9')
  })

  it('shows a supplier the request without the buyer’s prices and sends their offer', async () => {
    signIn()
    const request = negotiation({
      status: 'soliciting', offers: [], events: [], messages: [], mandate: { max_rounds: 2 },
      participants: [{ id: 'p1', supplier: 'Quincaillerie A', listing_id: 'l1', listing_title: 'Ciment CPJ 42.5', list_unit_minor: 5000, status: 'invited', round: 1 }],
    })
    const { calls } = mockApi({
      ...common,
      'GET /supplier-requests/p1': () => ({ body: request }),
      'POST /supplier-requests/p1/offer': () => ({ body: request }),
    })
    renderAt('/supplier/requests/p1')
    expect(await screen.findByText(/te demande ton meilleur prix pour 10 ×/)).toBeInTheDocument()
    expect(screen.queryByText(/Plafond/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Prix cible/)).not.toBeInTheDocument()
    await userEvent.type(screen.getByLabelText(/Prix par unité/), '45,5')
    await userEvent.type(screen.getByLabelText(/Livraison/), '3')
    await userEvent.click(screen.getByRole('button', { name: 'Envoyer mon offre' }))
    await waitFor(() => expect(calls.find((c) => c.path === '/supplier-requests/p1/offer')?.body).toEqual({ unit_price: 45.5, delivery_days: 3, valid_days: 7, note: '' }))
  })
})
