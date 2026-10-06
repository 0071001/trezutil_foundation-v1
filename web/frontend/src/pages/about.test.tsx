import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes, RouteFocus } from '@/App'
import { I18nProvider, setLang } from '@/lib/i18n'
import { PrefsProvider } from '@/lib/prefs'
import { SessionProvider } from '@/lib/session'
import { ToastProvider } from '@/lib/toast'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt } from '@/test/utils'

/*
 * "Our company" lives in the footer, and only there: the primary menu is for
 * what a person came to do (docs/web/OUR_COMPANY.md). The footer's three links
 * lead to the About page, which says what Trezutil is without inventing
 * anything, and to its sections.
 */

const served = (extra: Record<string, unknown> = {}) => mockApi({
  'GET /site/content': () => ({ body: { trust: [{ metric: 'merchants', value: 20 }], partners: [{ id: 'rawbank', name: 'Rawbank', kind: 'bank', logo_url: '/partners/rawbank.png' }], sections: [], ...extra } }),
  'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }),
})

beforeEach(() => { setLang('en', false); resetVerticals(); resetSiteContent() })
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs() })

describe('the About page', () => {
  it('says what Trezutil is, with one title and the three things that set it apart', async () => {
    served()
    renderAt('/about')
    expect(await screen.findByRole('heading', { level: 1, name: 'Commerce through conversation.' })).toBeInTheDocument()
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    expect(document.querySelector('.about-eyebrow')).toHaveTextContent('Our company')
    expect(screen.getByText(/Trezutil currently serves Kinshasa\./)).toBeInTheDocument()
    expect(['You start from a need, not a catalogue', 'Offers come from real businesses', 'You can act on it'].map((name) => screen.getByRole('heading', { level: 2, name }))).toHaveLength(3)
    expect(document.title).toBe('About Trezutil — Trezutil')
  })

  it('claims nothing it cannot back: no team, no dates, no address, no careers or press', async () => {
    served()
    renderAt('/about')
    await screen.findByRole('heading', { level: 1 })
    const main = screen.getByRole('main')
    expect(main.textContent).not.toMatch(/\b(founded|since 20\d\d|our team|careers|press|headquarter|@)/i)
  })

  it('shows the five steps, which the menu entry “How it works” leads to', async () => {
    served()
    renderAt('/about')
    const how = await screen.findByRole('region', { name: 'From your need to the action' })
    expect(how).toHaveAttribute('id', 'how')
    expect(how.querySelectorAll('ol.loop > li')).toHaveLength(5)
  })

  it('has a contact section that offers the conversation, and WhatsApp only when a number is configured', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', '')
    served()
    const first = renderAt('/about')
    const contact = await screen.findByRole('region', { name: 'Contact us' })
    expect(contact).toHaveAttribute('id', 'contact')
    expect(within(contact).getByRole('link', { name: /Make a request/ })).toHaveAttribute('href', '/conversation')
    expect(within(contact).getByRole('link', { name: 'Start selling on Trezutil' })).toHaveAttribute('href', '/seller/onboarding')
    expect(within(contact).queryByRole('link', { name: 'Find us on WhatsApp' })).toBeNull()
    first.unmount()

    vi.stubEnv('VITE_WHATSAPP_NUMBER', '243812345678')
    renderAt('/about')
    const again = await screen.findByRole('region', { name: 'Contact us' })
    expect(within(again).getByRole('link', { name: 'Find us on WhatsApp' })).toHaveAttribute('href', expect.stringContaining('https://wa.me/243812345678'))
  })

  it('shows the live figures and the partners the administrators chose, and nothing they hid', async () => {
    served()
    const shown = renderAt('/about')
    expect(await screen.findByRole('region', { name: 'Trezutil in numbers' })).toBeInTheDocument()
    expect(await screen.findByRole('region', { name: 'Our partners' })).toBeInTheDocument()
    shown.unmount()

    resetSiteContent()
    served({ sections: [{ key: 'trust', visible: false }, { key: 'partners', visible: false }] })
    renderAt('/about')
    await screen.findByRole('region', { name: 'Contact us' })
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Trezutil in numbers' })).toBeNull())
    expect(screen.queryByRole('region', { name: 'Our partners' })).toBeNull()
  })

  it('speaks French when the site does', async () => {
    setLang('fr', false)
    served()
    renderAt('/about')
    expect(await screen.findByRole('heading', { level: 1, name: 'Le commerce par la conversation.' })).toBeInTheDocument()
    expect(document.querySelector('.about-eyebrow')).toHaveTextContent('Notre entreprise')
    expect(screen.getByRole('heading', { level: 2, name: 'Vous partez d’un besoin, pas d’un catalogue' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Nous contacter' })).toBeInTheDocument()
  })
})

describe('“Our company” in the footer', () => {
  it('is a column of three links, on the home page and on the About page', async () => {
    served()
    for (const path of ['/', '/about']) {
      const view = renderAt(path)
      const footer = await screen.findByRole('contentinfo')
      const column = within(footer).getByRole('navigation', { name: 'Our company' })
      expect(within(column).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
        ['About Trezutil', '/about'], ['How it works', '/about#how'], ['Contact us', '/about#contact'],
      ])
      view.unmount()
    }
  })

  it('is called Notre entreprise in French', async () => {
    setLang('fr', false)
    served()
    renderAt('/')
    const column = within(await screen.findByRole('contentinfo')).getByRole('navigation', { name: 'Notre entreprise' })
    expect(within(column).getAllByRole('link').map((a) => a.textContent)).toEqual(['À propos de Trezutil', 'Comment ça marche', 'Nous contacter'])
  })
})

describe('the primary menu stays about what you came to do', () => {
  it('has no company entry in the home page’s top bar or its small-screen menu', async () => {
    served()
    renderAt('/')
    const bar = await screen.findByRole('banner')
    expect(within(bar).queryByText(/Our company/)).toBeNull()
    expect(within(bar).queryByRole('link', { name: /About Trezutil|Contact us/ })).toBeNull()
    await userEvent.click(within(bar).getByRole('button', { name: 'Open menu' }))
    const menu = within(screen.getByRole('dialog', { name: 'Menu' })).getByRole('navigation', { name: 'Menu' }) // the same phone menu as every other page
    expect(within(menu).queryByText(/Our company/)).toBeNull()
    expect(within(menu).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
  })

  it('has none on the other pages either, in the bar or in the drawer', async () => {
    served()
    renderAt('/discover')
    const bar = await screen.findByRole('banner')
    expect(within(bar).queryByText(/Our company/)).toBeNull()
    await userEvent.click(within(bar).getByRole('button', { name: 'Open menu' }))
    expect(within(screen.getByRole('dialog', { name: 'Menu' })).queryByText(/Our company/)).toBeNull()
  })
})

/** As the app mounts it: the router, the providers, and the part that scrolls and moves focus when the place changes. */
function renderWithFocus(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <PrefsProvider><ToastProvider><SessionProvider><I18nProvider><RouteFocus /><AppRoutes /></I18nProvider></SessionProvider></ToastProvider></PrefsProvider>
    </MemoryRouter>,
  )
}

describe('links to a place on a page', () => {
  const scrolled = () => vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => {})
  beforeEach(() => { Element.prototype.scrollIntoView ??= () => {} })

  it('scroll to that section once the page has drawn it', async () => {
    served()
    const spy = scrolled()
    renderWithFocus('/about#contact')
    await screen.findByRole('region', { name: 'Contact us' })
    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect((spy.mock.contexts[0] as Element).id).toBe('contact')
    spy.mockRestore()
  })

  it('start at the top when the place does not exist', async () => {
    served()
    const spy = scrolled()
    const top = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    renderWithFocus('/about#nowhere')
    await screen.findByRole('heading', { level: 1 })
    await waitFor(() => expect(top).toHaveBeenCalledWith(0, 0), { timeout: 3000 })
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
    top.mockRestore()
  })

  it('leave an ordinary page link scrolling to the top', async () => {
    served()
    const top = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    renderWithFocus('/about')
    await screen.findByRole('heading', { level: 1 })
    expect(top).toHaveBeenCalledWith(0, 0)
    top.mockRestore()
  })
})
