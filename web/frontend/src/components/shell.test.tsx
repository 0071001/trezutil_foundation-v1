import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { setLang } from '@/lib/i18n'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, signIn } from '@/test/utils'

/*
 * The primary navigation (docs/web/PRIMARY_MENU.md): "How it works" and three menus that open under their word.
 *
 *   How it works    Discover ▾    Organise ▾    Activities ▾
 *
 * The header of every page, the home page's header and the phone's menu all read the same list (src/lib/nav.ts).
 */

beforeEach(() => { setLang('en', false); resetVerticals(); resetSiteContent() })

const DISCOVER = [
  ['Products', 'Find what you need', '/search?vertical=RETAIL'],
  ['Services', 'Find someone to do it', '/search?vertical=SERVICES'],
  ['Places and outings', 'Restaurants, places and things to do', '/search?vertical=RESTAURANT'],
  ['Housing', 'Stays and accommodation', '/search?vertical=HOTEL'],
  ['Offers and recommendations', 'What Trezutil suggests for you', '/discover'],
]
const sentence = (start: string) => `/organize?q=${encodeURIComponent(start)}`
const ORGANISE = [
  ['Trip', 'Transport, accommodation, activities', sentence('I want to organise a trip to ')],
  ['Event', 'Party, birthday, ceremony', sentence('I want to organise an event for ')],
  ['Stay', 'Accommodation, transport, activities', sentence('I want to organise a stay in ')],
  ['Move', 'Housing, transport, help', sentence('I want to organise a move to ')],
  ['Transport', 'Getting around, transfer, driver', sentence('I want to organise transport for ')],
  ['Another project', 'Tell me what you want to organise', '/organize'],
]
const ACTIVITIES = [
  ['My missions', 'What I am organising', '/organize#missions'],
  ['My requests', 'What I asked Trezutil', '/conversation'],
  ['My orders', 'My purchases in progress', '/orders'],
  ['My reservations', 'My bookings', '/reservations'],
  ['My favourites', 'What I saved', '/wishlist'],
  ['History', 'My past activity', '/notifications'],
]

const primary = async () => screen.findByRole('navigation', { name: 'Primary' })
const trigger = (nav: HTMLElement, name: string) => within(nav).getByRole('button', { name })
/** The entries of an open menu: label, the line under it, where it leads. */
const panelEntries = (nav: HTMLElement, name: string) =>
  within(within(nav).getByRole('group', { name })).getAllByRole('link').filter((a) => a.querySelector('strong')).map((a) => [a.querySelector('strong')?.textContent, a.querySelector('small')?.textContent, a.getAttribute('href')])

describe('the primary navigation', () => {
  it('is “How it works” and three menus, nothing more', async () => {
    mockApi({})
    renderAt('/discover')
    const nav = await primary()
    const how = within(nav).getByRole('link', { name: 'How it works' })
    expect(how).toHaveAttribute('href', '/about#how')
    expect(within(nav).getAllByRole('link')).toHaveLength(1) // closed menus show no links
    const buttons = within(nav).getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).toEqual(['Discover', 'Organise', 'Activities'])
    for (const b of buttons) expect(b).toHaveAttribute('aria-expanded', 'false')
    // the order on the page
    expect([how, ...buttons].every((el, i, all) => i === 0 || all[i - 1]!.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true)
  })

  it('Discover is what exists: five entries, each with the line under it, leading to places that exist', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Discover'))
    expect(trigger(nav, 'Discover')).toHaveAttribute('aria-expanded', 'true')
    expect(panelEntries(nav, 'Discover')).toEqual(DISCOVER)
    for (const link of within(within(nav).getByRole('group', { name: 'Discover' })).getAllByRole('link')) expect(link.querySelector('svg.icon')).not.toBeNull()
  })

  it('Organise leads into the Organiser, as the beginning of a sentence: no flow of its own', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Organise'))
    expect(panelEntries(nav, 'Organise')).toEqual(ORGANISE)
    for (const [, , href] of ORGANISE) expect(href.startsWith('/organize')).toBe(true)
  })

  it('Activities are the person’s own pages, at the places they already have', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Activities'))
    expect(panelEntries(nav, 'Activities')).toEqual(ACTIVITIES)
  })

  it('tells a guest where their activity will be, and says nothing of the kind once signed in', async () => {
    mockApi({})
    const guest = renderAt('/about')
    let nav = await primary()
    await userEvent.click(trigger(nav, 'Activities'))
    const note = within(nav).getByText('Sign in to find what you started here.')
    expect(within(note.closest('p')!).getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
    await userEvent.click(trigger(nav, 'Discover'))
    expect(within(nav).queryByText('Sign in to find what you started here.')).toBeNull() // only the Activities menu carries it
    guest.unmount()

    signIn()
    mockApi({ 'GET /auth/users/me': () => ({ body: me }) })
    renderAt('/about')
    nav = await primary()
    await screen.findByRole('link', { name: 'Your account' })
    await userEvent.click(trigger(nav, 'Activities'))
    expect(within(nav).queryByText('Sign in to find what you started here.')).toBeNull()
  })

  it('opens one menu at a time, and closes it with the same button', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Discover'))
    await userEvent.click(trigger(nav, 'Organise'))
    expect(trigger(nav, 'Discover')).toHaveAttribute('aria-expanded', 'false')
    expect(trigger(nav, 'Organise')).toHaveAttribute('aria-expanded', 'true')
    expect(within(nav).queryByRole('group', { name: 'Discover' })).toBeNull()
    await userEvent.click(trigger(nav, 'Organise'))
    expect(within(nav).queryByRole('group')).toBeNull()
  })

  it('closes with Escape and gives the focus back to its button', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Discover'))
    await userEvent.keyboard('{Escape}')
    expect(within(nav).queryByRole('group')).toBeNull()
    expect(trigger(nav, 'Discover')).toHaveFocus()
  })

  it('closes on a click elsewhere and when the focus leaves it', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Discover'))
    await userEvent.click(document.body)
    expect(within(nav).queryByRole('group')).toBeNull()
    await userEvent.click(trigger(nav, 'Organise'))
    await userEvent.tab() // into the panel
    await userEvent.tab({ shift: true })
    await userEvent.tab({ shift: true })
    await userEvent.tab({ shift: true }) // out of the menu, backwards
    expect(within(nav).queryByRole('group', { name: 'Organise' })).toBeNull()
  })

  it('is used with the keyboard: arrows open and move, Home and End jump, Escape leaves', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    trigger(nav, 'Discover').focus()
    await userEvent.keyboard('{ArrowDown}')
    const links = () => within(within(nav).getByRole('group', { name: 'Discover' })).getAllByRole('link')
    await waitFor(() => expect(links()[0]).toHaveFocus())
    await userEvent.keyboard('{ArrowDown}')
    expect(links()[1]).toHaveFocus()
    await userEvent.keyboard('{End}')
    expect(links()[4]).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}') // wraps
    expect(links()[0]).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}')
    expect(links()[4]).toHaveFocus()
    await userEvent.keyboard('{Home}')
    expect(links()[0]).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    expect(trigger(nav, 'Discover')).toHaveFocus()
    expect(within(nav).queryByRole('group')).toBeNull()
    // from the button, Enter opens it too, and ArrowUp lands on the last entry
    await userEvent.keyboard('{ArrowUp}')
    await waitFor(() => expect(within(within(nav).getByRole('group', { name: 'Discover' })).getAllByRole('link')[4]).toHaveFocus())
  })

  it('opens when a mouse rests on it and closes shortly after it leaves', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.hover(trigger(nav, 'Organise'))
    expect(within(nav).getByRole('group', { name: 'Organise' })).toBeInTheDocument()
    await userEvent.unhover(trigger(nav, 'Organise'))
    await waitFor(() => expect(within(nav).queryByRole('group')).toBeNull())
  })

  it('closes when a link is followed, and goes to where it said', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Activities'))
    await userEvent.click(within(nav).getByRole('link', { name: /My requests/ }))
    expect(await screen.findByRole('heading', { name: /Ask Trezutil|what do you need/i })).toBeInTheDocument()
    expect(within(await primary()).queryByRole('group')).toBeNull()
  })

  it('shows which menu the page belongs to', async () => {
    mockApi({})
    renderAt('/wishlist')
    let nav = await primary()
    await waitFor(() => expect(trigger(nav, 'Activities')).toHaveClass('active'))
    expect(trigger(nav, 'Discover')).not.toHaveClass('active')
    expect(trigger(nav, 'Organise')).not.toHaveClass('active')
    cleanup()
    mockApi({ 'GET /missions': () => ({ body: { items: [], total: 0 } }) })
    renderAt('/organize')
    nav = await primary()
    await waitFor(() => expect(trigger(nav, 'Organise')).toHaveClass('active'))
    expect(trigger(nav, 'Activities')).not.toHaveClass('active') // “My missions” opens this page, which is Organise’s
  })

  it('is the same navigation on the home page, so nothing drifts between the two', async () => {
    mockApi({})
    const home = renderAt('/')
    const homeNav = await primary()
    expect(within(homeNav).getByRole('link', { name: 'How it works' })).toHaveAttribute('href', '/about#how')
    expect(within(homeNav).getAllByRole('button').map((b) => b.textContent)).toEqual(['Discover', 'Organise', 'Activities'])
    await userEvent.click(trigger(homeNav, 'Organise'))
    expect(panelEntries(homeNav, 'Organise')).toEqual(ORGANISE)
    await userEvent.click(trigger(homeNav, 'Discover'))
    expect(panelEntries(homeNav, 'Discover')).toEqual(DISCOVER)
    await userEvent.click(trigger(homeNav, 'Activities'))
    expect(panelEntries(homeNav, 'Activities')).toEqual(ACTIVITIES)
    expect(within(screen.getByRole('banner')).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
    home.unmount()
  })

  it('keeps one conversation button in the header, and no other primary entry', async () => {
    mockApi({})
    renderAt('/discover')
    const banner = await screen.findByRole('banner')
    expect(within(banner).getByRole('link', { name: 'Make a request' })).toHaveAttribute('href', '/conversation')
    const nav = await primary()
    expect(within(nav).getAllByRole('button')).toHaveLength(3)
    for (const old of ['Find a place', 'Get something', 'Find a service', 'Go out']) expect(screen.queryByRole('link', { name: old })).toBeNull()
  })

  it('puts a picture that says what it is about before each of the four words', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    const entries = [within(nav).getByRole('link', { name: 'How it works' }), ...within(nav).getAllByRole('button')]
    expect(entries).toHaveLength(4)
    for (const entry of entries) expect(entry.firstElementChild?.matches('svg.icon')).toBe(true) // the picture comes first
    await userEvent.click(trigger(nav, 'Activities'))
    await userEvent.click(within(nav).getByRole('button', { name: 'Activities' })) // closes again: the picture is not a second target
    expect(within(nav).queryByRole('group')).toBeNull()
  })

  it('is set next to the brand in one row, not centred on the page where it would run into the actions', () => {
    const css = readFileSync(join(process.cwd(), 'src/styles/nav.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '')
    expect(css).toMatch(/\.nav-inner > \.desktop-nav \{[^}]*position: static[^}]*transform: none/)
    // the link and the three buttons share one box: same height, padding, size
    expect(css).toMatch(/\.site-nav > \.action-link, \.site-nav-trigger \{[^}]*height: 2\.6rem[^}]*padding: 0 0\.9rem/)
  })

  it('is named in French, with the words of the brief', async () => {
    setLang('fr', false)
    mockApi({})
    renderAt('/about')
    const nav = await screen.findByRole('navigation', { name: 'Principale' })
    expect(within(nav).getByRole('link', { name: 'Comment ça marche' })).toBeInTheDocument()
    expect(within(nav).getAllByRole('button').map((b) => b.textContent)).toEqual(['Découvrir', 'Organiser', 'Activités'])
    await userEvent.click(trigger(nav, 'Découvrir'))
    expect(panelEntries(nav, 'Découvrir').map(([a, b]) => [a, b])).toEqual([
      ['Produits', 'Trouver ce dont tu as besoin'], ['Services', 'Trouver quelqu’un pour le faire'], ['Lieux & sorties', 'Restaurants, lieux et activités'],
      ['Logements', 'Séjours et hébergements'], ['Offres & recommandations', 'Ce que Trezutil te suggère'],
    ])
    await userEvent.click(trigger(nav, 'Organiser'))
    expect(panelEntries(nav, 'Organiser').map(([a, b]) => [a, b])).toEqual([
      ['Voyage', 'Transport, logement, activités'], ['Événement', 'Fête, anniversaire, cérémonie'], ['Séjour', 'Logement, transport, activités'],
      ['Déménagement', 'Logement, transport, aide'], ['Transport', 'Déplacement, transfert, chauffeur'], ['Autre projet', 'Dis-moi ce que tu veux organiser'],
    ])
    await userEvent.click(trigger(nav, 'Activités'))
    expect(panelEntries(nav, 'Activités').map(([a, b]) => [a, b])).toEqual([
      ['Mes missions', 'Ce que j’organise'], ['Mes demandes', 'Ce que j’ai demandé à Trezutil'], ['Mes commandes', 'Mes achats en cours'],
      ['Mes réservations', 'Mes réservations'], ['Mes favoris', 'Ce que j’ai sauvegardé'], ['Historique', 'Mes activités passées'],
    ])
  })

  it('has an entry in the Organiser that finishes a sentence in the person’s language', async () => {
    setLang('fr', false)
    mockApi({})
    renderAt('/about')
    const nav = await screen.findByRole('navigation', { name: 'Principale' })
    await userEvent.click(trigger(nav, 'Organiser'))
    const trip = within(nav).getByRole('link', { name: /Voyage/ })
    expect(decodeURIComponent(trip.getAttribute('href')!.split('q=')[1]!)).toBe('Je veux organiser un voyage à ')
  })
})

describe('one header for every page', () => {
  /** What a person can use in the top bar, in order: every link and button, by its visible or accessible name. */
  const controls = (bar: HTMLElement) =>
    [...bar.querySelectorAll('a, button')].map((el) => `${el.tagName.toLowerCase()}:${el.getAttribute('aria-label') ?? el.textContent}`.replace(/\s+/g, ' ').trim())

  it('has the same primary navigation and the same actions on the home page and on every other page', async () => {
    const seen: string[][] = []
    for (const path of ['/', '/discover', '/about', '/legal/terms', '/wishlist']) {
      mockApi({})
      renderAt(path)
      seen.push(controls(await screen.findByRole('banner')))
      cleanup()
    }
    expect(seen[0]).toContain('button:Discover')
    expect(seen[0]).toContain('a:Make a request')
    for (const list of seen.slice(1)) expect(list).toEqual(seen[0])
  })

  it('has the same phone menu on the home page and on every other page', async () => {
    const menus: string[][] = []
    for (const path of ['/', '/discover', '/about']) {
      mockApi({})
      renderAt(path)
      await userEvent.click(await screen.findByRole('button', { name: 'Open menu' }))
      const dialog = screen.getByRole('dialog', { name: 'Menu' })
      menus.push(within(dialog).getAllByRole('link').map((a) => `${a.textContent}|${a.getAttribute('href')}`))
      cleanup()
    }
    expect(menus[0]!.length).toBeGreaterThan(20)
    for (const m of menus.slice(1)) expect(m).toEqual(menus[0])
  })

  it('shows the same bar at the bottom of a phone on every page', async () => {
    const bars: string[][] = []
    for (const path of ['/', '/discover', '/about']) {
      mockApi({})
      renderAt(path)
      const bar = await screen.findByRole('navigation', { name: 'Mobile navigation' })
      bars.push(within(bar).getAllByRole('link').map((a) => `${a.textContent}|${a.getAttribute('href')}`))
      cleanup()
    }
    for (const b of bars.slice(1)) expect(b).toEqual(bars[0])
  })

  it('keeps For businesses, the bag, the account and the request button in it, on the home page too', async () => {
    mockApi({})
    renderAt('/')
    const bar = await screen.findByRole('banner')
    expect(within(bar).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
    expect(within(bar).getByRole('link', { name: /^Bag/ })).toHaveAttribute('href', '/cart')
    expect(within(bar).getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
    expect(within(bar).getByRole('link', { name: 'Make a request' })).toHaveAttribute('href', '/conversation')
    expect(within(bar).getByRole('button', { name: 'Switch to dark mode' })).toBeInTheDocument()
  })
})

describe('no bar across the bottom of a page', () => {
  it('has no “Ask Trezutil about this page” bar; the pages that have a question of their own offer it inline', async () => {
    mockApi({ 'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }), 'GET /verticals': () => ({ status: 503, body: {} }) })
    const { container } = renderAt('/search?q=laptop')
    await screen.findByRole('heading', { level: 1, name: 'Find what you need.' })
    expect(container.querySelector('.conversation-dock')).toBeNull()
    expect(screen.queryByText('Ask Trezutil anything about this page')).toBeNull()
    expect(screen.queryByText('Compare, refine or find something nearby')).toBeNull()
    // the header's request button is the way into the conversation
    expect(within(screen.getByRole('banner')).getByRole('link', { name: 'Make a request' })).toHaveAttribute('href', '/conversation')
  })
})

describe('the Organiser entries', () => {
  it('lead to the Organiser with the sentence begun, and the cursor at its end', async () => {
    mockApi({ 'GET /missions': () => ({ body: { items: [], total: 0 } }) })
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Organise'))
    await userEvent.click(within(nav).getByRole('link', { name: /Event/ }))
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you want to organise?' })).toBeInTheDocument()
    const field = screen.getByLabelText('I want to organise…') as HTMLTextAreaElement
    expect(field).toHaveValue('I want to organise an event for ')
    await waitFor(() => expect(field).toHaveFocus())
    expect(field.selectionStart).toBe(field.value.length)
  })

  it('“Another project” opens the Organiser with an empty field', async () => {
    mockApi({})
    renderAt('/about')
    const nav = await primary()
    await userEvent.click(trigger(nav, 'Organise'))
    await userEvent.click(within(nav).getByRole('link', { name: /Another project/ }))
    expect(await screen.findByLabelText('I want to organise…')).toHaveValue('')
  })
})

describe('the mobile drawer', () => {
  const open = async () => {
    mockApi({})
    renderAt('/discover')
    await userEvent.click(await screen.findByRole('button', { name: 'Open menu' }))
    return screen.getByRole('dialog', { name: 'Menu' })
  }

  it('opens with the request, then the same words as the header, grouped, then the person’s account', async () => {
    const drawer = await open()
    const menu = within(drawer).getByRole('navigation', { name: 'Menu' }) // the brand link above it is not an entry
    const links = within(menu).getAllByRole('link').map((a) => [a.querySelector('strong')?.textContent ?? a.textContent, a.getAttribute('href')])
    expect(links.slice(0, 2)).toEqual([['Make a request', '/conversation'], ['How it works', '/about#how']])
    const group = (name: string) => within(within(menu).getByRole('region', { name })).getAllByRole('link').filter((a) => a.querySelector('strong')).map((a) => [a.querySelector('strong')?.textContent, a.getAttribute('href')])
    expect(group('Discover')).toEqual(DISCOVER.map(([label, , href]) => [label, href]))
    expect(group('Organise')).toEqual(ORGANISE.map(([label, , href]) => [label, href]))
    expect(group('Activities')).toEqual(ACTIVITIES.map(([label, , href]) => [label, href]))
    expect(within(menu).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['Discover', 'Organise', 'Activities'])
    for (const name of ['Bag', 'For businesses']) expect(within(drawer).getByRole('link', { name })).toBeInTheDocument()
    expect(within(drawer).getAllByRole('link', { name: 'Sign in' }).map((a) => a.getAttribute('href'))).toEqual(['/login', '/login']) // the guest note in Activities, and the account entry
    // the person's pages are in Activities, once
    expect(within(drawer).getAllByRole('link', { name: /My orders/ })).toHaveLength(1)
    expect(within(drawer).queryByRole('link', { name: 'Reservations' })).toBeNull()
  })

  it('reads in French too', async () => {
    setLang('fr', false)
    mockApi({})
    renderAt('/discover')
    await userEvent.click(await screen.findByRole('button', { name: 'Ouvrir le menu' }))
    const menu = within(screen.getByRole('dialog', { name: 'Menu' })).getByRole('navigation', { name: 'Menu' })
    expect(within(menu).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['Découvrir', 'Organiser', 'Activités'])
    expect(within(menu).getByRole('link', { name: 'Comment ça marche' })).toHaveAttribute('href', '/about#how')
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
