import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import { TrustStrip, TrustStripView } from '@/components/trust'
import { setLang } from '@/lib/i18n'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt, searchItem } from '@/test/utils'

beforeEach(() => { resetVerticals(); resetSiteContent() })

const served = (trust: unknown[], extra: Record<string, unknown> = {}) => ({ 'GET /site/content': () => ({ body: { trust, partners: [], sections: [], ...extra } }) })
const figure = (metric: string, value: number, extra: Record<string, unknown> = {}) => ({ metric, value, ...extra })

describe('TrustStrip (live figures)', () => {
  it('shows the figures the server counted, each with its words', async () => {
    mockApi(served([figure('live_listings', 197), figure('verified_sellers', 2), figure('cities_covered', 1), figure('deals_live', 7)]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    const region = await screen.findByRole('region', { name: 'Trezutil in numbers' })
    const rows = within(region).getAllByRole('listitem').map((li) => [li.querySelector('.trust-value')?.textContent, li.querySelector('.trust-copy')?.textContent])
    expect(rows).toEqual([['197', 'listings online'], ['2', 'verified sellers'], ['1', 'city covered'], ['7', 'listings on promotion']])
  })

  it('uses the singular for one, and French plural rules (0 and 1 are singular)', async () => {
    mockApi(served([figure('verified_sellers', 1)]))
    const first = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    expect(await screen.findByText('verified seller')).toBeInTheDocument()
    first.unmount()
    resetSiteContent()
    setLang('fr', false)
    mockApi(served([figure('live_listings', 1), figure('bookings_confirmed', 12)]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    expect(await screen.findByText('annonce en ligne')).toBeInTheDocument()
    expect(screen.getByText('réservations confirmées')).toBeInTheDocument()
  })

  it('groups thousands the way the language does', async () => {
    mockApi(served([figure('live_listings', 12345)]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    const value = (await screen.findByRole('region', { name: 'Trezutil in numbers' })).querySelector('.trust-value')!
    expect(value.textContent).toMatch(/^12[,   ]345$/)
  })

  it('uses the wording staff chose for the language, falling back to the default', async () => {
    mockApi(served([figure('verified_sellers', 5, { label_fr: 'boutiques vérifiées', label_en: 'checked shops' }), figure('live_listings', 9, { label_fr: 'offres' })]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    expect(await screen.findByText('checked shops')).toBeInTheDocument() // English wording, English page
    expect(screen.getByText('listings online')).toBeInTheDocument() // only French wording was given: the English default stays
    expect(screen.queryByText('offres')).not.toBeInTheDocument()
  })

  it('shows no strip when the server sends no figure', async () => {
    const { fetchMock } = mockApi(served([]))
    const { container } = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('shows no strip, and no stand-in numbers, when the figures cannot be loaded', async () => {
    const { fetchMock } = mockApi({ 'GET /site/content': () => ({ status: 503, body: { error: { code: 'DOWN', message: 'x' } } }) })
    const { container } = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByText(/\d/)).not.toBeInTheDocument()
  })

  it('has no links, buttons or marketing promises', async () => {
    mockApi(served([figure('live_listings', 197), figure('orders_completed', 40), figure('bookings_confirmed', 12)]))
    const { container } = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    await screen.findByRole('region', { name: 'Trezutil in numbers' })
    expect(container.querySelectorAll('a, button')).toHaveLength(0)
    for (const icon of container.querySelectorAll('.trust-icon')) expect(icon).toHaveAttribute('aria-hidden', 'true')
    expect(container.textContent).not.toMatch(/free shipping|money.?back|guarantee|24\s*\/\s*(24|7)|best|leading|trusted by/i)
  })

  it('does not ask for the figures again within a minute', async () => {
    const { calls } = mockApi(served([figure('live_listings', 197)]))
    const first = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    await screen.findByRole('region', { name: 'Trezutil in numbers' })
    first.unmount()
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    expect(await screen.findByRole('region', { name: 'Trezutil in numbers' })).toBeInTheDocument()
    expect(calls.filter((c) => c.path === '/site/content')).toHaveLength(1)
  })
})

describe('TrustStripView', () => {
  it('draws the items it is given, in order, without any request', () => {
    const { fetchMock } = mockApi({})
    render(<TrustStripView items={[{ metric: 'deals_live', value: 3, labelFr: '', labelEn: '' }, { metric: 'live_listings', value: 9, labelFr: '', labelEn: '' }]} />)
    expect(screen.getAllByRole('listitem').map((li) => li.getAttribute('data-metric'))).toEqual(['deals_live', 'live_listings'])
    expect(fetchMock).not.toHaveBeenCalled()
  })
})

describe('home page layout (managed by staff)', () => {
  const searchOnly = (extra: Record<string, unknown>) => mockApi({
    'POST /search': (c) => {
      const items = Array.from({ length: (c.body as { page_size?: number }).page_size ?? 4 }, (_, i) => searchItem(`p${i}`, `Item ${i}`))
      return { body: { items, total: items.length, has_next: false } }
    },
    ...extra,
  })
  // What each block of the page calls itself, in page order.
  const landmarks = () => [...screen.getByRole('main').children].map((el) => el.getAttribute('aria-labelledby') ?? el.getAttribute('aria-label') ?? el.className)

  it('tells the story in a fixed order, and shows every block when the settings cannot be loaded', async () => {
    searchOnly({}) // /site/content is not served here
    renderAt('/')
    await screen.findByRole('region', { name: 'Discover what exists around you' })
    // figures and partner logos come from the server: no settings, nothing to show for them
    expect(landmarks()).toEqual(['hero-title', 'needs-title', 'discover-title', 'how-title', 'business-title'])
  })

  it('lets staff hide a block, never move it', async () => {
    searchOnly(served([figure('live_listings', 197)], {
      partners: [{ id: 'a', name: 'A Bank', kind: 'bank', logo_url: '/partners/a.png' }],
      sections: [
        { key: 'categories', visible: false }, { key: 'nearby', visible: true }, { key: 'how', visible: true },
        { key: 'trust', visible: true }, { key: 'seller', visible: false }, { key: 'partners', visible: true },
      ],
    }))
    renderAt('/')
    await screen.findByRole('region', { name: 'Trezutil in numbers' })
    await screen.findByRole('region', { name: 'Our partners' })
    expect(landmarks()).toEqual(['hero-title', 'discover-title', 'how-title', 'Trezutil in numbers', 'partners-title'])
    expect(screen.queryByRole('region', { name: 'What can we help you with?' })).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Have something to sell or offer?' })).not.toBeInTheDocument()
  })

  it('ignores the order and the blocks of an older layout saved before the redesign', async () => {
    searchOnly(served([figure('live_listings', 197)], {
      partners: [{ id: 'a', name: 'A Bank', kind: 'bank', logo_url: '/partners/a.png' }],
      sections: [
        { key: 'partners', visible: true }, { key: 'trust', visible: true }, { key: 'showcase', visible: true }, { key: 'services', visible: true },
        { key: 'properties', visible: true }, { key: 'reservations', visible: true }, { key: 'whatsapp', visible: true },
        { key: 'how', visible: true }, { key: 'categories', visible: true }, { key: 'nearby', visible: true }, { key: 'seller', visible: true },
      ],
    }))
    renderAt('/')
    await screen.findByRole('region', { name: 'Our partners' })
    expect(landmarks()).toEqual(['hero-title', 'needs-title', 'discover-title', 'how-title', 'Trezutil in numbers', 'business-title', 'partners-title'])
  })
})

describe('the figures of the strip: what Trezutil enables, not how much it lists', () => {
  const enabled = [figure('conversations', 340), figure('connections', 41), figure('transactions', 920), figure('merchants', 38)]
  const optional = [figure('registered_users', 125), figure('partners', 13)]

  it('shows each with its own drawing, never a generic icon', async () => {
    mockApi(served([...enabled, ...optional]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    const region = await screen.findByRole('region', { name: 'Trezutil in numbers' })
    const rows = within(region).getAllByRole('listitem')
    expect(rows.map((li) => [li.querySelector('.trust-value')?.textContent, li.querySelector('.trust-copy')?.textContent])).toEqual([
      ['340', 'conversations started'], ['41', 'customer–business connections'], ['920', 'transactions'], ['38', 'merchants'], ['125', 'users'], ['13', 'partners'],
    ])
    for (const li of rows) {
      expect(li.querySelector('svg.trust-glyph')).not.toBeNull()
      expect(li.querySelector('svg.icon')).toBeNull()
      expect(li.querySelector('.trust-icon')).toHaveAttribute('aria-hidden', 'true') // the figure and its words say it all
    }
  })

  it('draws a different picture for every figure', async () => {
    mockApi(served([...enabled, ...optional]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    const region = await screen.findByRole('region', { name: 'Trezutil in numbers' })
    const drawings = [...region.querySelectorAll('svg.trust-glyph')].map((svg) => svg.innerHTML)
    expect(new Set(drawings).size).toBe(6)
  })

  it('uses the singular for one, and French plural rules (0 and 1 are singular)', async () => {
    mockApi(served([figure('conversations', 1), figure('connections', 1), figure('transactions', 1), figure('merchants', 1)]))
    const first = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    for (const word of ['conversation started', 'customer–business connection', 'transaction', 'merchant']) expect(await screen.findByText(word)).toBeInTheDocument()
    first.unmount()
    resetSiteContent()
    setLang('fr', false)
    mockApi(served([figure('conversations', 1), figure('connections', 1), figure('transactions', 1), figure('merchants', 1)]))
    const singular = render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    for (const word of ['conversation engagée', 'mise en relation', 'transaction', 'marchand']) expect(await screen.findByText(word)).toBeInTheDocument()
    singular.unmount()
    resetSiteContent()
    mockApi(served([...enabled, ...optional]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    for (const word of ['conversations engagées', 'mises en relation', 'transactions', 'marchands', 'utilisateurs', 'partenaires']) expect(await screen.findByText(word)).toBeInTheDocument()
  })

  it('keeps the older metrics, which still use the generic icons', async () => {
    mockApi(served([figure('live_listings', 197)]))
    render(<MemoryRouter><TrustStrip /></MemoryRouter>)
    const region = await screen.findByRole('region', { name: 'Trezutil in numbers' })
    expect(region.querySelector('svg.icon')).not.toBeNull()
    expect(region.querySelector('svg.trust-glyph')).toBeNull()
  })

  it('sits on a white band across the page, like “Discover what exists around you” (the card surface in dark mode)', () => {
    const css = readFileSync(resolve(__dirname, '../styles/home.css'), 'utf8')
    expect(css).toMatch(/\.app\.home \.trust-strip \{ background: var\(--card\);/)
    expect(css).toMatch(/\.home-discover \{ background: var\(--card\);/)
  })
})
