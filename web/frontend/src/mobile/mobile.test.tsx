import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { conversationStore } from '@/conversation/store'
import { me, mockApi, renderAt, searchItem, signIn, type MockCall } from '@/test/utils'

/* The mobile layout ("Concevoir l’application mobile Trezutil") on a phone-sized screen, on real API shapes. */

function phone() {
  vi.stubGlobal('matchMedia', (q: string) => ({
    matches: q.includes('max-width: 760px'), media: q, onchange: null,
    addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {}, dispatchEvent: () => false,
  }))
}

const noSite = () => ({ body: { trust: [], partners: [], sections: [] } })
const searchReply = {
  conversation_id: 'conv-1',
  response: {
    text: 'Voici des offres.',
    metadata: { action_type: 'SEARCH_LISTINGS', result_data: { items: [searchItem('a', 'Café A', { PriceAmount: 90 }), searchItem('b', 'Café B', { PriceAmount: 70 })] } },
  },
}
const account = (extra: Record<string, (c: MockCall) => { status?: number; body?: unknown }> = {}) => {
  signIn()
  return mockApi({
    'GET /site/content': noSite,
    'GET /auth/users/me': () => ({ body: me }),
    'GET /orders': () => ({ body: { items: [] } }),
    'GET /reservations': () => ({ body: { items: [] } }),
    'GET /missions': () => ({ body: { items: [], total: 0 } }),
    'GET /conversations': () => ({ body: { items: [] } }),
    'GET /sellers/me': () => ({ body: { organizations: [] } }),
    'GET /me/credits': () => ({ body: { period_id: '2026-11', personal: { opened: true, granted: 100, reserved: 0, consumed: 20, balance: 80 }, ledger: [], metering: { active: true, costs: [] } } }),
    ...extra,
  })
}

async function send(text: string) {
  const field = await screen.findByLabelText('Ton besoin')
  fireEvent.change(field, { target: { value: text } })
  fireEvent.keyDown(field, { key: 'Enter' })
}

describe('the mobile layout', () => {
  beforeEach(() => { phone(); conversationStore.setOwner('__reset__'); conversationStore.setOwner('') })
  afterEach(() => vi.unstubAllGlobals())

  it('opens on the design home: logo, question, three suggestions, composer; no sidebar and no tab bar', async () => {
    mockApi({ 'GET /site/content': noSite })
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Que souhaites-tu acheter,vendre ou faire réaliser ?')
    expect(screen.getByRole('button', { name: 'Trouver un fournisseur' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Comparer des offres' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Négocier un prix' })).toBeInTheDocument()
    expect(screen.getByText('Commerce OS')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Navigation principale' })).not.toBeInTheDocument()
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
    // No partner is shown unless the team published one.
    expect(screen.queryByLabelText('Partenaires')).not.toBeInTheDocument()
  })

  it('shows the Trezutil logo and Trezy on the home', async () => {
    mockApi({ 'GET /site/content': noSite })
    renderAt('/')
    expect(await screen.findByRole('img', { name: 'Trezy vous souhaite la bienvenue' })).toHaveAttribute('src', expect.stringContaining('/trezy/trezy-accueil'))
    expect(screen.getAllByRole('img', { name: 'Trezutil' }).length).toBeGreaterThan(0)
  })

  it('offers every menu of the sidebar in the drawer, with the account menu', async () => {
    account()
    renderAt('/')
    await userEvent.click(await screen.findByRole('button', { name: 'Ouvrir le menu' }))
    const drawer = screen.getByRole('dialog', { name: 'Menu' })
    const menu = within(drawer).getByRole('navigation', { name: 'Espace commercial' })
    for (const name of ['Vue d’ensemble', 'Opérations', 'Négociations', 'Achats et commandes', 'Réservations et organisation', 'Fournisseurs', 'Alertes et opportunités']) {
      expect(within(menu).getByRole('button', { name: new RegExp(name) })).toBeInTheDocument()
    }
    expect(within(menu).getByText('Automatisations').closest('[aria-disabled]')).toHaveAttribute('aria-disabled', 'true')
    expect(within(drawer).getByRole('button', { name: 'Paramètres' })).toBeInTheDocument()
    await userEvent.click(await within(drawer).findByRole('button', { name: 'Compte : Jean Mukendi' }))
    for (const name of ['Mon profil', 'Crédits et usage', 'Devenir fournisseur', 'Informations légales', 'Changer de compte', 'Se déconnecter']) {
      expect(within(drawer).getByRole('menuitem', { name })).toBeInTheDocument()
    }
    expect(within(drawer).getByRole('radiogroup', { name: 'Thème' })).toBeInTheDocument()
    await userEvent.click(within(menu).getByRole('button', { name: /Négociations/ }))
    expect(screen.queryByRole('dialog', { name: 'Menu' })).not.toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Négociations' })).toBeInTheDocument()
  })

  it('shows only the partners published on the site', async () => {
    mockApi({ 'GET /site/content': () => ({ body: { trust: [], sections: [], partners: [{ id: 'p1', name: 'Partenaire Test', kind: 'other', logo_url: 'https://images.example.com/partenaire.png', link_url: '' }] } }) })
    renderAt('/')
    expect(await screen.findByRole('img', { name: 'Partenaire Test' })).toBeInTheDocument()
  })

  it('answers with the progress, the cheapest offer first and the comparison', async () => {
    const { calls } = mockApi({ 'GET /site/content': noSite, 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    await send('du café pour le bureau')
    expect(await screen.findByText('Voici des offres.')).toBeInTheDocument()
    expect(screen.getByText('Besoin analysé')).toBeInTheDocument()
    expect(screen.getByText('2 offres pertinentes identifiées')).toBeInTheDocument()
    const card = screen.getByRole('article', { name: 'Offre 2 : Café B' })
    expect(within(card).getByText('Meilleur prix')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Comparer les 2 offres/ }))
    expect(await screen.findByRole('heading', { name: '2 offres' })).toBeInTheDocument()
    expect(screen.getByText('MEILLEUR PRIX')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Continuer avec cette offre' }))
    await waitFor(() => expect(calls.filter((c) => c.path === '/conversations/message')).toHaveLength(2))
    expect((calls.at(-1)!.body as { message_text: string }).message_text).toContain('Je retiens l’offre 2')
  })

  it('opens the drawer with the history, and renames and archives from it', async () => {
    mockApi({ 'GET /site/content': noSite, 'POST /conversations/message': () => ({ body: searchReply }) })
    renderAt('/')
    await send('du café')
    await screen.findByText('Voici des offres.')
    await userEvent.click(screen.getByRole('button', { name: 'Ouvrir le menu' }))
    const drawer = screen.getByRole('dialog', { name: 'Menu' })
    expect(within(drawer).getByRole('button', { name: 'du café' })).toBeInTheDocument()
    await userEvent.click(within(drawer).getByRole('button', { name: 'Actions pour du café' }))
    await userEvent.click(within(drawer).getByRole('menuitem', { name: 'Archiver' }))
    // Archiving the open conversation leaves it: the home, with the drawer closed.
    expect(screen.queryByRole('dialog', { name: 'Menu' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Ouvrir le menu' }))
    expect(within(screen.getByRole('dialog', { name: 'Menu' })).queryByRole('button', { name: 'du café' })).not.toBeInTheDocument()
  })

  it('adds files from the bottom sheet and says they stay on the device', async () => {
    mockApi({ 'GET /site/content': noSite })
    renderAt('/')
    await userEvent.click(await screen.findByRole('button', { name: 'Ajouter des fichiers ou une photo' }))
    const sheet = screen.getByRole('dialog', { name: 'Ajouter à la conversation' })
    expect(within(sheet).getByRole('button', { name: /Prendre une photo/ })).toBeInTheDocument()
    expect(within(sheet).getByRole('button', { name: /Photos/ })).toBeInTheDocument()
    fireEvent.change(screen.getByTestId('file-input'), { target: { files: [new File(['x'], 'devis.pdf', { type: 'application/pdf' })] } })
    expect(await screen.findByText('devis.pdf')).toBeInTheDocument()
    expect(screen.getByRole('note')).toHaveTextContent('ton message partira sans eux')
  })

  it('shows the profile with the real credits and the settings of the design', async () => {
    account()
    renderAt('/profile')
    expect(await screen.findByRole('heading', { name: 'Jean Mukendi' })).toBeInTheDocument()
    expect(await screen.findByText('80 crédits disponibles')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Paramètres/ }))
    expect(await screen.findByText('APPARENCE ET APPAREIL')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Notifications' })).toHaveAttribute('aria-checked', 'true')
    await userEvent.click(screen.getByRole('button', { name: 'Android' }))
    expect(document.querySelector('.mx-app')).toHaveClass('android')
    await userEvent.click(screen.getByRole('switch', { name: 'Mode sombre' }))
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('shows the credit balance without a purchase button that does nothing', async () => {
    account()
    renderAt('/settings/credits')
    expect(await screen.findByText('Solde disponible')).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: '80' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Ajouter des crédits/ })).not.toBeInTheDocument()
  })

  it('filters the missions', async () => {
    account({ 'GET /missions': () => ({ body: { items: [{ id: 'm1', title: 'Négociation Kawa', status: 'active', objective: 'o', updated_at: '2026-11-21T10:00:00Z' }, { id: 'm2', title: 'Réservation salle', status: 'completed', objective: 'o', updated_at: '2026-11-20T10:00:00Z' }], total: 2 } }) })
    renderAt('/operations')
    expect(await screen.findByText('Négociation Kawa')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Terminées' }))
    expect(screen.queryByText('Négociation Kawa')).not.toBeInTheDocument()
    expect(screen.getByText('Réservation salle')).toBeInTheDocument()
  })

  it('shows the design sign-in, then the phone form', async () => {
    mockApi({ 'GET /auth/providers': () => ({ body: { google: { enabled: false } } }) })
    renderAt('/login')
    expect(await screen.findByRole('heading', { name: 'Bienvenue sur Trezutil' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Continuer avec un numéro' }))
    expect(screen.getByLabelText('Numéro de téléphone')).toBeInTheDocument()
  })
})
