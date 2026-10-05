import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { resetSiteContent } from '@/lib/useSiteContent'
import { siteAdminBody } from '@/test/fixtures'
import { me, mockApi, renderAt, signIn, type MockCall } from '@/test/utils'

/*
 * Administration → Site content. Staff choose which live figures the home
 * page's trust strip shows (the numbers are counted by the platform and cannot
 * be typed), manage the partner logos, and choose which home page sections show.
 * Nothing reaches the public site until it is published, and a publish names
 * the version it read so two editors cannot overwrite each other.
 */

const admin = { ...me, id: 'a1', display_name: 'Guilbert', email: 'guilbertmkap@gmail.com', phone: '', roles: ['admin'] }
const operator = { ...admin, id: 'o1', display_name: 'Olivia', email: 'olivia@example.com', roles: ['operator'] }

interface SaveBody { expected_version: number; trust: Record<string, unknown>[]; partners: Record<string, unknown>[]; sections: { key: string; visible: boolean }[] }

/** A server that keeps what was published and bumps the version, like the real one. */
function siteServer(user: unknown = admin, options: { putStatus?: number; getStatus?: number } = {}) {
  let version = 0
  const saves: SaveBody[] = []
  const uploads: MockCall[] = []
  const api = mockApi({
    'GET /auth/users/me': () => ({ body: user }),
    'GET /sellers/me': () => ({ body: { organizations: [] } }),
    'GET /admin/overview': () => ({ body: { generated_at: '2026-10-04T12:00:00Z', users: { total: 1, active: 1, suspended: 0, new_last_7_days: 0, by_role: {} }, organizations: 0, listings_by_status: {}, orders: { total: 0, last_7_days: 0 }, reservations: 0, conversations: { last_7_days: 0 } } }),
    'GET /admin/site/content': () => (options.getStatus ? { status: options.getStatus, body: { error: { code: 'FORBIDDEN', message: 'This area is reserved for platform administrators' } } } : { body: siteAdminBody() }),
    'PUT /admin/site/content': (c) => {
      if (options.putStatus) return { status: options.putStatus, body: { error: { code: 'SITE_CONTENT_CONFLICT', message: 'Someone else saved changes since you opened this page. Reload to see them.' } } }
      const body = c.body as SaveBody
      if (body.expected_version !== version) return { status: 409, body: { error: { code: 'SITE_CONTENT_CONFLICT', message: 'Someone else saved changes since you opened this page. Reload to see them.' } } }
      saves.push(body)
      version += 1
      return { body: siteAdminBody({ content: { trust: body.trust, partners: body.partners, sections: body.sections }, version, stored: true, updated_at: '2026-10-04T14:02:00Z', updated_by_name: 'Guilbert' }) }
    },
    'POST /admin/site/assets': (c) => { uploads.push(c); return { status: 201, body: { id: '5e11e700-0000-4000-8000-0000000000a1', url: '/api/v1/site/assets/5e11e700-0000-4000-8000-0000000000a1', content_type: 'image/png', size: 100 } } },
  })
  return { ...api, saves, uploads }
}

const png = (name = 'equity.png', size = 100) => new File([new Uint8Array(size)], name, { type: 'image/png' })
const row = (name: string | RegExp) => screen.getByRole('switch', { name }).closest('li') as HTMLElement
const publish = async () => {
  await userEvent.click(screen.getByRole('button', { name: 'Publish changes' }))
  await userEvent.click(within(await screen.findByRole('dialog', { name: 'Publish these changes?' })).getByRole('button', { name: 'Publish' }))
}
const open = async () => {
  signIn()
  renderAt('/admin/site')
  await screen.findByRole('heading', { level: 1, name: 'Site content' })
}

beforeEach(() => resetSiteContent())

describe('who can manage the site content', () => {
  it('lists the module for administrators', async () => {
    signIn()
    siteServer()
    renderAt('/admin')
    const nav = await screen.findByRole('navigation', { name: 'Administration menu' })
    expect(within(nav).getByRole('link', { name: 'Site content' })).toHaveAttribute('href', '/admin/site')
  })

  it('does not list it for an operator, and the server refuses the address anyway', async () => {
    signIn()
    siteServer(operator, { getStatus: 403 })
    renderAt('/admin/site')
    expect(await screen.findByText('You don’t have access')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { level: 1, name: 'Site content' })).not.toBeInTheDocument()
    expect(within(screen.getByRole('navigation', { name: 'Administration menu' })).queryByRole('link', { name: 'Site content' })).not.toBeInTheDocument()
  })

  it('refuses a guest: the staff sign-in comes first', async () => {
    mockApi({})
    renderAt('/admin/site')
    expect(await screen.findByRole('heading', { name: 'Sign in to the administration' })).toBeInTheDocument()
  })
})

describe('live figures', () => {
  it('shows the real figure behind every metric, and what it does on the site today', async () => {
    siteServer()
    await open()
    expect(screen.getByText('Nothing has been published yet: the site shows the built-in defaults.')).toBeInTheDocument()
    const list = screen.getByRole('list', { name: 'Live figures' })
    const rows = within(list).getAllByRole('listitem')
    expect(rows).toHaveLength(13)
    // what Trezutil enables leads the list, in the order of the strip
    expect(rows.slice(0, 4).map((r) => r.querySelector('[role="switch"]')?.closest('label')?.textContent)).toEqual(['conversations started', 'customer–business connections', 'transactions', 'merchants'])
    expect(within(row('conversations started')).getByText(/Today: 340/)).toBeInTheDocument()
    expect(within(row('conversations started')).getByText('Shown on the site')).toBeInTheDocument()
    expect(within(row('customer–business connections')).getByText(/Today: 41/)).toBeInTheDocument()
    expect(within(row('transactions')).getByText(/Today: 12/)).toBeInTheDocument()
    expect(within(row('merchants')).getByText(/Today: 20/)).toBeInTheDocument()
    expect(within(row('users')).getByText(/Today: 38/)).toBeInTheDocument()
    expect(within(row('users')).getByText('Off')).toBeInTheDocument() // users and partners are available, off by default
    expect(within(row('listings online')).getByText(/Today: 197/)).toBeInTheDocument()
    expect(within(row('listings online')).getByText('Off')).toBeInTheDocument() // so are the older metrics: listings are not the story
    await userEvent.click(screen.getByRole('switch', { name: 'confirmed bookings' }))
    expect(within(row('confirmed bookings')).getByText(/Today: 0/)).toBeInTheDocument()
    expect(within(row('confirmed bookings')).getByText('Hidden: below the minimum')).toBeInTheDocument() // enabled, but 0 < 1
  })

  it('offers no way to type a number: only whether, from when, and in what words', async () => {
    siteServer()
    await open()
    const fields = within(row('conversations started'))
    expect(fields.getByLabelText('Show conversations started from')).toHaveAttribute('type', 'number')
    expect(fields.getByLabelText('French wording for conversations started')).toHaveAttribute('maxlength', '40')
    expect(fields.getByLabelText('English wording for conversations started')).toHaveAttribute('maxlength', '40')
    // The figure itself is text, never an input.
    expect(fields.queryByDisplayValue('340')).not.toBeInTheDocument()
  })

  it('previews the strip with today’s real figures, and the draft changes it before anything is published', async () => {
    const { saves } = siteServer()
    await open()
    const preview = screen.getByRole('region', { name: 'Preview' })
    const shown = () => within(preview).getAllByRole('listitem').map((li) => li.querySelector('.trust-value')?.textContent + ' ' + li.querySelector('.trust-copy')?.textContent)
    expect(shown()).toEqual(['340 conversations started', '41 customer–business connections', '12 transactions', '20 merchants'])

    await userEvent.click(screen.getByRole('switch', { name: 'orders completed' }))
    expect(shown()).toContain('12 orders completed')
    await userEvent.clear(within(row('conversations started')).getByLabelText('Show conversations started from'))
    await userEvent.type(within(row('conversations started')).getByLabelText('Show conversations started from'), '500')
    expect(shown()).not.toContain('340 conversations started') // 340 < 500: held back until it is worth showing
    expect(within(row('conversations started')).getByText('Hidden: below the minimum')).toBeInTheDocument()
    expect(saves).toHaveLength(0)
  })

  it('counts the partners figure from the draft list, since the partners are content, not platform activity', async () => {
    siteServer()
    await open()
    await userEvent.click(screen.getByRole('switch', { name: 'partners' })) // off by default: the band below the strip already shows them
    const preview = screen.getByRole('region', { name: 'Preview' })
    const partners = () => within(preview).getAllByRole('listitem').map((li) => li.querySelector('.trust-value')?.textContent + ' ' + li.querySelector('.trust-copy')?.textContent).find((text) => /partner/.test(text))
    expect(partners()).toBe('2 partners') // Rawbank and TMB are shown, Stripe is hidden
    await userEvent.click(screen.getByRole('tab', { name: /Partners/ }))
    await userEvent.click(screen.getByRole('switch', { name: /Stripe/ })) // showing the third one…
    await userEvent.click(screen.getByRole('tab', { name: /Live figures/ }))
    expect(partners()).toBe('3 partners') // …changes the figure at once, before anything is published
  })

  it('says nothing is shown when nothing qualifies, rather than inventing a figure', async () => {
    siteServer()
    await open()
    for (const name of ['conversations started', 'customer–business connections', 'transactions', 'merchants']) await userEvent.click(screen.getByRole('switch', { name }))
    expect(within(screen.getByRole('region', { name: 'Preview' })).getByText('No figure to show yet: the strip is hidden.')).toBeInTheDocument()
    expect(screen.getByText('0 figures are switched on.')).toBeInTheDocument()
  })

  it('publishes exactly what was edited, naming the version it read', async () => {
    const { saves } = siteServer()
    await open()
    await userEvent.type(within(row('conversations started')).getByLabelText('French wording for conversations started'), 'conversations ouvertes')
    await userEvent.click(screen.getByRole('switch', { name: 'orders completed' }))
    await userEvent.click(screen.getByRole('button', { name: 'Move merchants up' }))
    await publish()

    await waitFor(() => expect(saves).toHaveLength(1))
    const sent = saves[0]!
    expect(sent.expected_version).toBe(0)
    expect(sent.trust.map((t) => t.metric)).toEqual(['conversations', 'connections', 'merchants', 'transactions', 'registered_users', 'partners', 'live_listings', 'verified_sellers', 'deals_live', 'cities_covered', 'bookings_confirmed', 'orders_completed', 'verticals_live'])
    expect(sent.trust.find((t) => t.metric === 'conversations')).toEqual({ metric: 'conversations', enabled: true, min_value: 1, label_fr: 'conversations ouvertes', label_en: '' })
    expect(sent.trust.find((t) => t.metric === 'orders_completed')).toMatchObject({ enabled: true })
    // No figure travels to the server, in any form.
    expect(JSON.stringify(sent)).not.toMatch(/197|340|"value"|"stats"|"partners":\s*2/)
    expect(await screen.findByText('Site content published. Visitors see it now.')).toBeInTheDocument()
    expect(screen.getByText(/Last published .* by Guilbert/)).toBeInTheDocument()
  })

  it('names the new version on the next publish', async () => {
    const { saves } = siteServer()
    await open()
    await userEvent.click(screen.getByRole('switch', { name: 'orders completed' }))
    await publish()
    await screen.findByText('Site content published. Visitors see it now.')
    await userEvent.click(await screen.findByRole('switch', { name: 'verticals with live listings' }).catch(() => screen.getByRole('switch', { name: /sectors with live listings/ })))
    await publish()
    await waitFor(() => expect(saves).toHaveLength(2))
    expect(saves.map((s) => s.expected_version)).toEqual([0, 1])
  })

  it('keeps Publish off until something changed, and Discard puts everything back', async () => {
    siteServer()
    await open()
    const button = screen.getByRole('button', { name: 'Publish changes' })
    expect(button).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Discard changes' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('switch', { name: 'orders completed' }))
    expect(button).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect(screen.getByRole('switch', { name: 'orders completed' })).not.toBeChecked()
    expect(button).toBeDisabled()
  })

  it('refuses a minimum a server would refuse, and says which figure', async () => {
    siteServer()
    await open()
    const field = within(row('listings online')).getByLabelText('Show listings online from')
    await userEvent.clear(field)
    await userEvent.type(field, '2000000')
    expect(await screen.findByText(/“listings online”: the minimum must be a whole number from 0 to 1,000,000\./)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Publish changes' })).toBeDisabled()
    await userEvent.clear(field)
    await userEvent.type(field, '25')
    expect(screen.getByRole('button', { name: 'Publish changes' })).toBeEnabled()
  })

  it('stops an editor overwriting somebody else’s publish', async () => {
    const { saves } = siteServer(admin, { putStatus: 409 })
    await open()
    await userEvent.click(screen.getByRole('switch', { name: 'orders completed' }))
    await publish()
    const dialog = await screen.findByRole('dialog', { name: 'Publish these changes?' })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Someone else saved changes since you opened this page. Reload to see them.')
    expect(await screen.findByText(/Someone else published changes while you were editing/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument()
    expect(saves).toHaveLength(0)
  })
})

describe('partners', () => {
  const openPartners = async () => { await open(); await userEvent.click(screen.getByRole('tab', { name: /Partners/ })) }

  it('lists the partners with their kind, website and visibility', async () => {
    siteServer()
    await openPartners()
    expect(screen.getByRole('tab', { name: /Partners/ })).toHaveTextContent('3')
    const list = screen.getByRole('list', { name: 'Partners' })
    const rows = within(list).getAllByRole('listitem')
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent('Rawbank')
    expect(rows[0]).toHaveTextContent('Bank')
    expect(rows[1]).toHaveTextContent('tmb.cd')
    expect(rows[2]).toHaveTextContent('Stripe')
    expect(rows[2]).toHaveTextContent('Hidden')
  })

  it('previews the band with the visible partners only', async () => {
    siteServer()
    await openPartners()
    const band = within(screen.getByRole('region', { name: 'Preview' })).getByRole('region', { name: 'Our partners' })
    expect(within(band).getAllByRole('img').map((i) => i.getAttribute('alt'))).toEqual(['Rawbank', 'TMB']) // Stripe is switched off
    await userEvent.click(screen.getByRole('switch', { name: 'Show Stripe' }))
    expect(within(band).getAllByRole('img').map((i) => i.getAttribute('alt'))).toEqual(['Rawbank', 'TMB', 'Stripe'])
  })

  it('adds a partner with an uploaded logo and publishes it', async () => {
    const { saves, uploads } = siteServer()
    await openPartners()
    await userEvent.click(screen.getByRole('button', { name: 'Add a partner' }))
    const drawer = await screen.findByRole('dialog', { name: 'Add a partner' })
    const add = within(drawer).getByRole('button', { name: 'Add partner' })
    expect(add).toBeDisabled() // no name, no logo yet

    await userEvent.type(within(drawer).getByLabelText('Name'), 'Equity BCDC')
    await userEvent.selectOptions(within(drawer).getByLabelText('Kind'), 'bank')
    expect(add).toBeDisabled() // still no logo
    await userEvent.upload(within(drawer).getByLabelText('Upload a logo'), png())
    await waitFor(() => expect(uploads).toHaveLength(1))
    expect((uploads[0]!.body as { file: File }).file.name).toBe('equity.png')
    expect(await within(drawer).findByRole('img', { name: 'Logo of Equity BCDC' })).toHaveAttribute('src', '/api/v1/site/assets/5e11e700-0000-4000-8000-0000000000a1')
    await waitFor(() => expect(add).toBeEnabled())
    await userEvent.click(add)

    expect(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('listitem')).toHaveLength(4)
    await publish()
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]!.partners[3]).toEqual({ id: 'equity-bcdc', name: 'Equity BCDC', kind: 'bank', logo_url: '/api/v1/site/assets/5e11e700-0000-4000-8000-0000000000a1', link_url: '', visible: true })
  })

  it('refuses a logo that is not a PNG, JPEG or WebP, or is too large, before sending it', async () => {
    const { uploads } = siteServer()
    await openPartners()
    await userEvent.click(screen.getByRole('button', { name: 'Add a partner' }))
    const drawer = await screen.findByRole('dialog', { name: 'Add a partner' })
    const input = within(drawer).getByLabelText('Upload a logo')
    await userEvent.upload(input, new File(['<svg/>'], 'logo.svg', { type: 'image/svg+xml' }), { applyAccept: false })
    expect(await within(drawer).findByText('Use a PNG, JPEG or WebP image.')).toBeInTheDocument()
    await userEvent.upload(input, png('big.png', 600 * 1024))
    expect(await within(drawer).findByText('The logo must be 512 KB or smaller.')).toBeInTheDocument()
    expect(uploads).toHaveLength(0)
  })

  it('only accepts an https website', async () => {
    siteServer()
    await openPartners()
    await userEvent.click(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('button', { name: /Edit/ })[0]!)
    const drawer = await screen.findByRole('dialog', { name: 'Edit partner' })
    const site = within(drawer).getByLabelText('Website (optional)')
    await userEvent.type(site, 'http://rawbank.cd')
    expect(within(drawer).getByText('The website must start with https://')).toBeInTheDocument()
    expect(within(drawer).getByRole('button', { name: 'Apply' })).toBeDisabled()
    await userEvent.clear(site)
    await userEvent.type(site, 'https://www.rawbank.cd')
    expect(within(drawer).getByRole('button', { name: 'Apply' })).toBeEnabled()
  })

  it('removes a partner only after asking, and only from the draft until published', async () => {
    const { saves } = siteServer()
    await openPartners()
    await userEvent.click(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('button', { name: /Remove/ })[1]!)
    const dialog = await screen.findByRole('alertdialog', { name: 'Remove this partner?' })
    expect(dialog).toHaveTextContent('“TMB” leaves the draft. It disappears from the site only when you publish.')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('listitem')).toHaveLength(3)

    await userEvent.click(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('button', { name: /Remove/ })[1]!)
    await userEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Remove' }))
    expect(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('listitem')).toHaveLength(2)
    expect(saves).toHaveLength(0) // nothing is public yet
    await publish()
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]!.partners.map((p) => p.name)).toEqual(['Rawbank', 'Stripe'])
  })

  it('reorders partners, and shows the empty band honestly', async () => {
    const { saves } = siteServer()
    await openPartners()
    await userEvent.click(screen.getByRole('button', { name: 'Move TMB up' }))
    expect(within(screen.getByRole('list', { name: 'Partners' })).getAllByRole('listitem')[0]).toHaveTextContent('TMB')
    expect(screen.getByRole('button', { name: 'Move TMB up' })).toBeDisabled() // already first
    for (const name of ['Rawbank', 'TMB']) await userEvent.click(screen.getByRole('switch', { name: `Show ${name}` }))
    expect(within(screen.getByRole('region', { name: 'Preview' })).getByText('No partners to show: the band is hidden.')).toBeInTheDocument()
    await publish()
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]!.partners.map((p) => [p.name, p.visible])).toEqual([['TMB', false], ['Rawbank', false], ['Stripe', false]])
  })

  it('warns that a logo is a trademark', async () => {
    siteServer()
    await openPartners()
    await userEvent.click(screen.getByRole('button', { name: 'Add a partner' }))
    expect(await screen.findByText(/A logo is a trademark\. Add only organizations Trezutil really works with/)).toBeInTheDocument()
  })
})

describe('home layout', () => {
  it('lists the seven blocks in the order of the story, with nothing to move: the welcome banner stays first', async () => {
    siteServer()
    await open()
    await userEvent.click(screen.getByRole('tab', { name: 'Home layout' }))
    expect(screen.getByText(/The home page always starts with its welcome banner/)).toBeInTheDocument()
    const rows = within(screen.getByRole('list', { name: 'Home page sections' })).getAllByRole('listitem')
    expect(rows.map((row) => row.textContent?.replace(/^\d+/, ''))).toEqual([
      'What can we help you with', 'Discover around you', 'From need to action', 'WhatsApp', 'Trezutil in numbers', 'For businesses', 'Partner logos',
    ])
    expect(screen.queryByRole('button', { name: /^Move / })).toBeNull() // the order is the product narrative, kept in code
  })

  it('hides a block and publishes the new layout, keeping the order', async () => {
    const { saves } = siteServer()
    await open()
    await userEvent.click(screen.getByRole('tab', { name: 'Home layout' }))
    await userEvent.click(screen.getByRole('switch', { name: 'Discover around you' }))
    await publish()
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]!.sections).toEqual([
      { key: 'categories', visible: true }, { key: 'nearby', visible: false }, { key: 'how', visible: true },
      { key: 'whatsapp', visible: true }, { key: 'trust', visible: true }, { key: 'seller', visible: true }, { key: 'partners', visible: true },
    ])
  })
})
