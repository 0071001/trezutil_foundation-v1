import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLang } from '@/lib/i18n'
import { MENU_NEEDS, NEED_GROUPS } from '@/lib/needs'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt } from '@/test/utils'

/*
 * The primary menu is the needs Trezutil helps with (docs/web/PRIMARY_MENU.md),
 * not the list of catalogue categories it used to be. The home page, the bar of
 * every other page and the mobile drawer all read the same list.
 */

beforeEach(() => { setLang('en', false); resetVerticals(); resetSiteContent() })

const ENTRIES = [
  ['Find a place', '/search?vertical=REAL_ESTATE'],
  ['Get something', '/search?vertical=RETAIL'],
  ['Find a service', '/search?vertical=SERVICES'],
  ['Go out', '/search?vertical=RESTAURANT'],
  ['Discover', '/discover'],
]
const entries = (nav: HTMLElement) => within(nav).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])

describe('the primary menu', () => {
  it('lists the needs, then Discover, with an icon on each, on every page of the product', async () => {
    mockApi({})
    renderAt('/discover')
    const nav = await screen.findByRole('navigation', { name: 'Primary' })
    expect(entries(nav)).toEqual(ENTRIES)
    for (const link of within(nav).getAllByRole('link')) expect(link.querySelector('svg.icon')).not.toBeNull()
  })

  it('no longer lists catalogue categories, and keeps "Reservations" with the person’s own pages', async () => {
    mockApi({})
    renderAt('/discover')
    const nav = await screen.findByRole('navigation', { name: 'Primary' })
    for (const old of ['Products', 'Services', 'Real Estate', 'Reservations']) expect(within(nav).queryByRole('link', { name: old })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    expect(within(screen.getByRole('dialog', { name: 'Menu' })).getByRole('link', { name: 'Reservations' })).toHaveAttribute('href', '/reservations')
  })

  it('is the same list on the home page, so nothing drifts between the two', async () => {
    mockApi({})
    const home = renderAt('/')
    const homeNav = await screen.findByRole('navigation', { name: 'Primary' })
    expect(entries(homeNav)).toEqual(ENTRIES)
    expect(within(screen.getByRole('banner')).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
    home.unmount()
    renderAt('/discover')
    expect(entries(await screen.findByRole('navigation', { name: 'Primary' }))).toEqual(ENTRIES)
  })

  it('is built from the shared list of needs: a need that needs stock to exist is left out of the menus', () => {
    expect(NEED_GROUPS.map((g) => g.id)).toEqual(['place', 'something', 'someone', 'out', 'around'])
    expect(MENU_NEEDS.map((g) => g.id)).toEqual(['place', 'something', 'someone', 'out']) // "Get around" shows only with live offers
  })

  it('has a call to action with a name for assistive technology, though its text is hidden at most widths', async () => {
    mockApi({})
    renderAt('/discover')
    const banner = await screen.findByRole('banner')
    expect(within(banner).getByRole('link', { name: 'Make a request' })).toHaveAttribute('href', '/conversation')
  })

  it('is named for what people do, in French too', async () => {
    setLang('fr', false)
    mockApi({})
    renderAt('/discover')
    const nav = await screen.findByRole('navigation', { name: 'Principale' })
    expect(within(nav).getAllByRole('link').map((a) => a.textContent)).toEqual(['Trouver un logement', 'Trouver un produit', 'Trouver un service', 'Sortir', 'Découvrir'])
  })
})

describe('the mobile drawer', () => {
  const open = async () => {
    mockApi({})
    renderAt('/discover')
    await userEvent.click(await screen.findByRole('button', { name: 'Open menu' }))
    return screen.getByRole('dialog', { name: 'Menu' })
  }

  it('opens with the request first, then the needs with the other searches under them, then the person’s own pages', async () => {
    const drawer = await open()
    const menu = within(drawer).getByRole('navigation', { name: 'Menu' }) // the brand link above it is not an entry
    expect(within(menu).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')]).slice(0, 8)).toEqual([
      ['Make a request', '/conversation'], ['Find a place', '/search?vertical=REAL_ESTATE'], ['Hotels', '/search?vertical=HOTEL'],
      ['Get something', '/search?vertical=RETAIL'], ['Find a service', '/search?vertical=SERVICES'], ['Go out', '/search?vertical=RESTAURANT'],
      ['Discover', '/discover'], ['Orders', '/orders'],
    ])
    for (const name of ['Reservations', 'Saved', 'Bag', 'For businesses']) expect(within(drawer).getByRole('link', { name })).toBeInTheDocument()
  })

  it('sits beside the header, never inside it: a blurred header would squeeze it into the height of the bar', async () => {
    const drawer = await open()
    expect(screen.getByRole('banner')).not.toContainElement(drawer)
  })

  it('closes with Escape and with its close button', async () => {
    const drawer = await open()
    expect(drawer).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: 'Menu' })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Open menu' }))
    await userEvent.click(screen.getByRole('button', { name: 'Close menu' }))
    expect(screen.queryByRole('dialog', { name: 'Menu' })).toBeNull()
  })
})
