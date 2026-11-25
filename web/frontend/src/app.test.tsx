import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { conversationStore } from '@/conversation/store'
import { addMemory, memoryAsPreferences, memoryStore } from '@/memory/memory'
import { me, mockApi, renderAt, searchItem, signIn, type MockCall } from '@/test/utils'

/*
 * The conversational interface against a mocked gateway. Every reply below is TEST DATA, shaped like the real
 * /conversations/message answer; nothing here is shown to a person.
 */

const searchReply = {
  conversation_id: 'conv-1',
  response: {
    text: 'J’ai trouvé 2 offres.',
    metadata: {
      action_type: 'SEARCH_LISTINGS',
      result_data: { items: [searchItem('l1', 'Ordinateur A'), searchItem('l2', 'Ordinateur B', { Seller: { SellerID: 's2', DisplayName: 'TechSource', TrustLevel: 0 } })] },
      suggestions: ['Moins cher'],
    },
  },
}

const confirmReply = {
  conversation_id: 'conv-1',
  is_confirmation: true,
  response: {
    text: 'Je prépare la commande.',
    confirmation: { summary: 'Commander 1 × Ordinateur A pour 85 $', details: { confirmation_token: 'tok-1', action_type: 'CREATE_ORDER' } },
    metadata: { action_type: 'CREATE_ORDER' },
  },
}

function signedInRoutes(extra: Record<string, (c: MockCall) => { status?: number; body?: unknown }>) {
  signIn()
  return mockApi({
    'GET /auth/users/me': () => ({ body: me }),
    'GET /missions': () => ({ body: { items: [], total: 0 } }),
    'GET /orders': () => ({ body: { items: [] } }),
    'GET /sellers/me': () => ({ body: { organizations: [] } }),
    'GET /conversations': () => ({ body: { items: [] } }),
    ...extra,
  })
}

async function fillPayer() {
  const set = async (label: RegExp, value: string) => {
    const input = await screen.findByLabelText(label)
    fireEvent.change(input, { target: { value } })
  }
  await screen.findByRole('radio', { name: /M-Pesa/ })
  if (screen.queryByLabelText(/Numéro mobile money/)) await set(/Numéro mobile money/, '0817127253')
  await set(/^Prénom/, 'Gilio')
  await set(/^Nom$/, 'Kapapa')
  await set(/^E-mail/, 'g@example.cd')
}

async function send(text: string) {
  const field = await screen.findByLabelText('Ton besoin')
  fireEvent.change(field, { target: { value: text } })
  fireEvent.keyDown(field, { key: 'Enter' })
}

beforeEach(() => {
  conversationStore.setOwner('__reset__')
  conversationStore.setOwner('')
  memoryStore.setOwner('__reset__')
  memoryStore.setOwner('')
})

describe('conversation', () => {
  it('opens on the question; a suggestion prepares the request without sending it', async () => {
    const { calls } = mockApi({})
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Qu’est-ce que tu souhaites')
    await userEvent.click(screen.getByRole('button', { name: /Acheter un équipement/ }))
    expect(screen.getByLabelText('Ton besoin')).toHaveValue('Je souhaite acheter ')
    expect(calls.filter((c) => c.path.startsWith('/conversations'))).toHaveLength(0)
  })

  it('sends with Enter on a thread of its own, never with Shift+Enter, and shows the offers as cards', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    const field = await screen.findByLabelText('Ton besoin')
    fireEvent.change(field, { target: { value: 'Je cherche 50 ordinateurs' } })
    fireEvent.keyDown(field, { key: 'Enter', shiftKey: true })
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0)
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(await screen.findByText('J’ai trouvé 2 offres.')).toBeInTheDocument()
    const body = calls.find((c) => c.path === '/conversations/message')!.body as Record<string, string>
    expect(body.message_text).toBe('Je cherche 50 ordinateurs')
    expect(body.sender_external_id).toMatch(/^web-/)
    expect(await screen.findByRole('article', { name: /Offre 1 : Ordinateur A/ })).toBeInTheDocument()
    expect(screen.getByText('Recherche de fournisseurs terminée')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Moins cher' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Je cherche 50 ordinateurs/ })).toBeInTheDocument() // in the sidebar
  })

  it('compares the offers of a reply and continues with the chosen one in the conversation', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    await send('Je cherche 50 ordinateurs')
    await userEvent.click(await screen.findByRole('button', { name: /Comparer ces offres/ }))
    const table = await screen.findByRole('table')
    expect(within(table).getByText('TechSource')).toBeInTheDocument()
    expect(within(table).getAllByText('Non communiqué').length).toBeGreaterThan(0)
    await userEvent.click(within(table).getAllByRole('button', { name: 'Sélectionner' })[0]!)
    await userEvent.click(screen.getByRole('button', { name: /Continuer avec l’offre 2/ }))
    await waitFor(() => expect(calls.filter((c) => c.path === '/conversations/message')).toHaveLength(2))
    expect((calls.at(-1)!.body as Record<string, string>).message_text).toContain('Je retiens l’offre 2')
  })

  it('asks a guest to sign in before approving, and never confirms on their behalf', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: confirmReply }) })
    renderAt('/')
    await send('Commande l’ordinateur A')
    expect(await screen.findByText('Commander 1 × Ordinateur A pour 85 $')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Se connecter pour approuver' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Approuver' })).not.toBeInTheDocument()
    expect(calls.some((c) => c.path.includes('/confirm'))).toBe(false)
  })

  it('sends the explicit approval of a signed-in person with the runtime’s token', async () => {
    const { calls } = signedInRoutes({
      'POST /conversations/message': () => ({ body: confirmReply }),
      'POST /conversations/conv-1/confirm': () => ({ body: { conversation_id: 'conv-1', response: { text: 'Commande créée.', metadata: { action_type: 'CREATE_ORDER', result_data: { order_id: 'o-1' } } } } }),
    })
    renderAt('/')
    await send('Commande l’ordinateur A')
    await userEvent.click(await screen.findByRole('button', { name: 'Approuver' }))
    expect(await screen.findByRole('link', { name: /Voir la commande/ })).toHaveAttribute('href', '/purchases/o-1')
    expect(calls.find((c) => c.path === '/conversations/conv-1/confirm')!.body).toEqual({ token: 'tok-1' })
  })

  it('sends a failed message again with the same idempotency key', async () => {
    let attempt = 0
    const { calls } = mockApi({ 'POST /conversations/message': () => (++attempt === 1 ? { status: 503, body: { error: { code: 'DOWN', message: 'Service indisponible' } } } : { body: searchReply }) })
    renderAt('/')
    await send('Je cherche 50 ordinateurs')
    await userEvent.click(await screen.findByRole('button', { name: 'Réessayer' }))
    expect(await screen.findByText('J’ai trouvé 2 offres.')).toBeInTheDocument()
    const keys = calls.filter((c) => c.path === '/conversations/message').map((c) => (c.body as Record<string, string>).message_idempotency_key)
    expect(keys).toHaveLength(2)
    expect(keys[0]).toBe(keys[1])
  })

  it('edits the first message on a fresh server conversation', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    await send('Je cherche 50 ordinateurs')
    await screen.findByText('J’ai trouvé 2 offres.')
    await userEvent.click(screen.getByRole('button', { name: 'Modifier le message' }))
    const box = screen.getByLabelText('Modifier le message')
    fireEvent.change(box, { target: { value: 'Je cherche 60 ordinateurs' } })
    await userEvent.click(screen.getByRole('button', { name: 'Envoyer' }))
    await waitFor(() => expect(calls.filter((c) => c.path === '/conversations/message')).toHaveLength(2))
    const [first, second] = calls.filter((c) => c.path === '/conversations/message').map((c) => c.body as Record<string, string>)
    expect(second!.message_text).toBe('Je cherche 60 ordinateurs')
    expect(second!.sender_external_id).not.toBe(first!.sender_external_id)
  })

  it('keeps chosen files on the device and says so: the message leaves without them', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    const input = (await screen.findByTestId('file-input')) as HTMLInputElement
    const file = new File(['%PDF'], 'devis.pdf', { type: 'application/pdf' })
    await act(async () => { fireEvent.change(input, { target: { files: [file] } }) })
    expect(screen.getByText('devis.pdf')).toBeInTheDocument()
    expect(screen.getByRole('note')).toHaveTextContent('ton message partira sans eux')
    await send('Voici mon devis')
    expect(await screen.findByText(/non transmis/)).toBeInTheDocument()
    const body = calls.find((c) => c.path === '/conversations/message')!.body as Record<string, unknown>
    expect(JSON.stringify(body)).not.toContain('devis.pdf')
  })
})

describe('server history and catalogue', () => {
  it('lists the conversations the server keeps for the account and reads one back when it is opened', async () => {
    const { calls } = signedInRoutes({
      'GET /conversations': () => ({ body: { items: [{ id: '55555555-5555-4555-8555-555555555555', channel: 'web', external_id: 'web-other-device', title: 'Traiteur pour 80 personnes', message_count: 2, created_at: 1700000000, last_activity_at: 1700000100 }] } }),
      'GET /conversations/55555555-5555-4555-8555-555555555555/messages': () => ({ body: { items: [
        { id: 'm1', direction: 'inbound', text: 'Traiteur pour 80 personnes', created_at: 1700000000 },
        { id: 'm2', direction: 'outbound', text: 'Voici trois traiteurs à Gombe.', created_at: 1700000001 },
      ] } }),
      'POST /conversations/message': () => ({ body: searchReply }),
    })
    renderAt('/')
    await userEvent.click(await screen.findByRole('link', { name: 'Traiteur pour 80 personnes' }))
    expect(await screen.findByText('Voici trois traiteurs à Gombe.')).toBeInTheDocument()
    expect(screen.getByText(/Conversation reprise depuis le serveur/)).toBeInTheDocument()
    // Continuing it goes to the same server conversation (its channel identity).
    await send('Et à Limete ?')
    await waitFor(() => expect(calls.some((c) => c.path === '/conversations/message')).toBe(true))
    expect((calls.find((c) => c.path === '/conversations/message')!.body as Record<string, string>).sender_external_id).toBe('web-other-device')
  })

  it('opens the detail of an offer from the catalogue beside the conversation', async () => {
    mockApi({
      'POST /conversations/message': () => ({ body: searchReply }),
      'GET /listings': () => ({ body: { ID: 'l1', Title: 'Ordinateur A', Description: 'Core i5, 16 Go, garantie constructeur.', PriceAmount: 85, PriceCurrency: 'USD', Status: 'published', Media: [] } }),
    })
    renderAt('/')
    await send('Je cherche 50 ordinateurs')
    const card = await screen.findByRole('article', { name: /Offre 1 : Ordinateur A/ })
    await userEvent.click(within(card).getByRole('button', { name: /Voir l’offre/ }))
    const panel = await screen.findByRole('dialog', { name: /Offre : Ordinateur A/ })
    expect(await within(panel).findByText('Core i5, 16 Go, garantie constructeur.')).toBeInTheDocument()
  })

  it('says plainly that online payment is not enabled when the backend has no provider', async () => {
    const order = { id: 'o-1', status: 'pending_payment', currency: 'USD', total_amount: { amount_minor: 8500, currency: 'USD' }, items: [{ listing_id: 'l1', title: 'Ordinateur A', quantity: 1, unit_price: { amount_minor: 8500 } }], created_at: 1700000000 }
    signedInRoutes({
      'GET /orders/o-1': () => ({ body: order }),
      'POST /payments': () => ({ status: 503, body: { error: { code: 'PAYMENT_PROVIDER_UNAVAILABLE', message: 'no provider' } } }),
    })
    renderAt('/purchases/o-1')
    await userEvent.click(await screen.findByRole('radio', { name: /M-Pesa/ }))
    await fillPayer()
    await userEvent.click(screen.getByRole('button', { name: /^Payer / }))
    expect(await screen.findByText(/paiement en ligne n’est pas encore activé/)).toBeInTheDocument()
  })

  it('pays by mobile money: the request goes to the phone, and the order is paid only once SasPay confirms it', async () => {
    const order = { id: 'o-2', organization_id: 'org-s', status: 'pending_payment', currency: 'CDF', total_amount: { amount_minor: 50000, currency: 'CDF' }, items: [{ listing_id: 'l1', title: 'Ciment', quantity: 1, unit_price: { amount_minor: 50000 } }], created_at: 1700000000 }
    let approved = false
    const { calls } = signedInRoutes({
      'GET /orders/o-2': () => ({ body: { ...order, status: approved ? 'paid' : 'pending_payment' } }),
      'POST /payments': () => ({ status: 201, body: { payment_id: 'pay-1', status: 'PENDING', client_action: { provider_code: 'saspay' } } }),
      'GET /payments/pay-1/status': () => ({ body: { payment_id: 'pay-1', status: approved ? 'CAPTURED' : 'PENDING' } }),
    })
    renderAt('/purchases/o-2')
    await fillPayer()
    await userEvent.click(screen.getByRole('button', { name: /^Payer / }))
    expect(await screen.findByText(/demande de paiement a été envoyée sur ton téléphone/)).toBeInTheDocument()
    const sent = calls.find((c) => c.method === 'POST' && c.path.startsWith('/payments'))!.body as Record<string, unknown>
    expect(sent).toMatchObject({ order_id: 'o-2', amount_minor: 50000, currency: 'CDF', payment_method_type: 'MOBILE_MONEY', payment_method_id: 'vodacom_cd' })
    expect(sent.metadata).toMatchObject({ phone: '0817127253', email: 'g@example.cd', first_name: 'Gilio', last_name: 'Kapapa' })
    // Not paid until SasPay says so: the first read-back is still pending.
    await waitFor(() => expect(calls.some((c) => c.path.startsWith('/payments/pay-1/status'))).toBe(true), { timeout: 6000 })
    expect(screen.getByText(/En attente de la confirmation/)).toBeInTheDocument()
    approved = true
    await waitFor(() => expect(screen.queryByText(/En attente de la confirmation/)).not.toBeInTheDocument(), { timeout: 6000 })
  }, 20000)
})

describe('memory', () => {
  it('refuses secrets and payment data, and goes with an operation only when it is on', () => {
    expect(addMemory('constraint', 'Mon code PIN est 1234')).toMatch(/ne mémorise pas/)
    expect(addMemory('constraint', 'Carte 4111 1111 1111 1111')).toMatch(/ne mémorise pas/)
    expect(addMemory('constraint', 'Livraison à Kinshasa')).toBeNull()
    expect(memoryAsPreferences()).toEqual({ memory_constraint: 'Livraison à Kinshasa' })
    memoryStore.set({ enabled: false })
    expect(memoryAsPreferences()).toEqual({})
  })
})

describe('operations', () => {
  it('pauses an operation through the mission lifecycle', async () => {
    const mission = { id: 'm1', type: 'EVENT', title: 'Conférence annuelle', objective: 'Conférence de 80 personnes', status: 'active', location: 'Kinshasa', created_at: '2026-11-21T10:00:00Z', updated_at: '2026-11-21T10:00:00Z' }
    const { calls } = signedInRoutes({
      'GET /missions/m1': () => ({ body: mission }),
      'GET /missions/m1/plan': () => ({ body: { mission_id: 'm1', plan_version: 0, status: 'none', plannable: true, steps: [] } }),
      'PATCH /missions/m1': (c) => ({ body: { ...mission, ...(c.body as object) } }),
    })
    renderAt('/operations/m1')
    expect(await screen.findByRole('heading', { name: 'Conférence annuelle' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Mettre en pause/ }))
    expect(await screen.findByRole('button', { name: /Reprendre/ })).toBeInTheDocument()
    expect(calls.find((c) => c.method === 'PATCH')!.body).toEqual({ status: 'paused' })
  })
})
