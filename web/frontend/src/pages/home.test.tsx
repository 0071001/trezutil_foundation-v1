import { existsSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setLang } from '@/lib/i18n'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

/*
 * The home page is the place to say what you need, not a catalogue
 * (docs/web/HOME_PAGE_REDESIGN.md). These tests pin that: the welcome and the
 * composer, what each block promises, that offers appear only as the supply
 * layer, and that the page stays understandable with none at all.
 */

type SearchBody = { vertical?: string; page_size?: number; sort_field?: string }

interface Supply { hotels?: unknown[]; tables?: unknown[]; fresh?: unknown[]; around?: unknown[] }

/** A server with the offers given; the conversation answers with a fixed sentence and records what it was asked. */
function server(supply: Supply = {}, extra: Record<string, () => { status?: number; body?: unknown }> = {}) {
  const sent: string[] = []
  const api = mockApi({
    'POST /search': (c) => {
      const body = c.body as SearchBody
      const items = (body.vertical === 'HOTEL' ? supply.hotels : body.vertical === 'RESTAURANT' ? supply.tables
        : body.vertical === 'VEHICLE_RENTAL' || body.vertical === 'TRANSPORT' ? supply.around : supply.fresh) ?? []
      return { body: { items, total: items.length, has_next: false } }
    },
    'POST /conversations/message': (c) => {
      sent.push((c.body as { message_text: string }).message_text)
      return { body: { conversation_id: 'c1', response: { text: 'Here is what I found.' } } }
    },
    ...extra,
  })
  return { ...api, sent }
}

const hotel = (id: string, title: string) => searchItem(id, title, { Vertical: 'HOTEL' })
const table = (id: string, title: string) => searchItem(id, title, { Vertical: 'RESTAURANT' })
const fresh = (id: string, title: string) => searchItem(id, title, { Vertical: 'RETAIL' })

const region = (name: string) => screen.findByRole('region', { name })
const field = () => screen.getByRole('textbox', { name: 'Your need' })

beforeEach(() => { setLang('en', false); resetVerticals(); resetSiteContent() })
afterEach(() => vi.unstubAllGlobals())

describe('the welcome', () => {
  it('asks what you need, in a few warm words, with a place to say it', async () => {
    server()
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you need today?' })).toBeInTheDocument()
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    expect(screen.getByText('Tell us what you need. We’ll help you find it.')).toBeInTheDocument()
    expect(field()).toHaveAttribute('placeholder', 'I need a furnished apartment in Gombe under $1,000.')
    expect(screen.getByText('You don’t have to search everywhere. Just tell Trezutil what you need.')).toBeInTheDocument()
    expect(document.title).toBe('Trezutil — Tell us what you need')
  })

  it('has a clean top bar: brand, the needs and Discover, For businesses, language, theme and sign-in, and no search field', async () => {
    server()
    renderAt('/')
    const bar = await screen.findByRole('banner')
    expect(within(bar).getByRole('link', { name: 'Trezutil home' })).toBeInTheDocument()
    const nav = within(bar).getByRole('navigation', { name: 'Primary' })
    expect(within(nav).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([['Find a place', '/search?vertical=REAL_ESTATE'], ['Get something', '/search?vertical=RETAIL'], ['Find a service', '/search?vertical=SERVICES'], ['Go out', '/search?vertical=RESTAURANT'], ['Discover', '/discover']])
    expect(within(bar).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
    expect(within(bar).getByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/login')
    expect(within(bar).queryByRole('searchbox')).toBeNull()
    expect(within(bar).queryByRole('textbox')).toBeNull()
  })

  it('shows the account instead of “Sign in” once signed in', async () => {
    signIn()
    server({}, { 'GET /auth/users/me': () => ({ body: me }) })
    renderAt('/')
    expect(await screen.findByRole('link', { name: 'Your account' })).toHaveAttribute('href', '/account')
    expect(within(screen.getByRole('banner')).queryByRole('link', { name: 'Sign in' })).toBeNull()
  })

  it('opens a menu on small screens with every way in, including “Make a request”', async () => {
    server()
    renderAt('/')
    await userEvent.click(await screen.findByRole('button', { name: 'Open menu' }))
    const menu = screen.getByRole('navigation', { name: 'Menu' })
    expect(within(menu).getByRole('link', { name: 'Make a request' })).toHaveAttribute('href', '/conversation')
    expect(within(menu).getByRole('link', { name: 'For businesses' })).toHaveAttribute('href', '/seller')
    await userEvent.click(screen.getByRole('button', { name: 'Close menu' }))
    expect(screen.queryByRole('navigation', { name: 'Menu' })).toBeNull()
  })

  it('does not put a second chat on top of the page that is itself a chat entry', async () => {
    server()
    renderAt('/')
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByRole('button', { name: 'Open the chat with Trezutil' })).toBeNull()
  })
})

describe('the composer', () => {
  it('cannot send nothing, and sends what was written to the conversation', async () => {
    const { sent } = server()
    renderAt('/')
    const send = await screen.findByRole('button', { name: 'Send' })
    expect(send).toBeDisabled()
    await userEvent.type(field(), '   ')
    expect(send).toBeDisabled() // blanks are not a need
    await userEvent.type(field(), 'A mechanic in Limete')
    expect(send).toBeEnabled()
    await userEvent.click(send)
    expect(await screen.findByText('Here is what I found.')).toBeInTheDocument()
    expect(sent).toEqual(['A mechanic in Limete'])
  })

  it('sends with Enter, and keeps a new line for Shift+Enter', async () => {
    const { sent } = server()
    renderAt('/')
    await screen.findByRole('heading', { level: 1 })
    await userEvent.type(field(), 'one{Shift>}{Enter}{/Shift}two')
    expect(field()).toHaveValue('one\ntwo')
    expect(sent).toEqual([])
    await userEvent.type(field(), '{Enter}')
    expect(await screen.findByText('Here is what I found.')).toBeInTheDocument()
    expect(sent).toEqual(['one\ntwo'])
  })

  it('starts the conversation from an example sentence in one tap', async () => {
    const { sent } = server()
    renderAt('/')
    const examples = within(await screen.findByRole('list', { name: 'For example' }))
    expect(examples.getAllByRole('button').map((b) => b.textContent)).toEqual([
      'I’m looking for a furnished apartment in Gombe.', 'I need a mechanic nearby.', 'Find me a hotel for this weekend.',
      'I’m looking for an iPhone 15 in Kinshasa.', 'I’m looking for someone to install a camera.', 'I want to book a restaurant tonight.',
    ])
    await userEvent.click(examples.getByRole('button', { name: 'I need a mechanic nearby.' }))
    expect(await screen.findByText('Here is what I found.')).toBeInTheDocument()
    expect(sent).toEqual(['I need a mechanic nearby.'])
  })

  it('dictates into the field when the browser can, and says so when it cannot', async () => {
    server()
    const first = renderAt('/')
    await userEvent.click(await screen.findByRole('button', { name: 'Dictate' }))
    expect(await screen.findByText('Voice input is not available in this browser.')).toBeInTheDocument()
    first.unmount()

    class Recognition {
      static last: Recognition | null = null
      lang = ''
      interimResults = false
      onresult: ((e: unknown) => void) | null = null
      onend: (() => void) | null = null
      onerror: (() => void) | null = null
      constructor() { Recognition.last = this }
      start() {}
      stop() { this.onend?.() }
    }
    vi.stubGlobal('webkitSpeechRecognition', Recognition)
    renderAt('/')
    const mic = await screen.findByRole('button', { name: 'Dictate' })
    expect(mic).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(mic)
    expect(screen.getByRole('button', { name: 'Listening… tap to stop' })).toHaveAttribute('aria-pressed', 'true')
    act(() => Recognition.last!.onresult!({ results: [[{ transcript: 'a hotel in Gombe' }]] }))
    expect(field()).toHaveValue('a hotel in Gombe')
    await userEvent.click(screen.getByRole('button', { name: 'Listening… tap to stop' }))
    expect(screen.getByRole('button', { name: 'Dictate' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('offers only what works: no photo or location button that could only pretend', async () => {
    server()
    renderAt('/')
    const hero = await region('What do you need today?')
    for (const name of [/photo/i, /picture/i, /image/i, /location/i, /attach/i, /^camera$/i]) {
      expect(within(hero).queryByRole('button', { name })).toBeNull()
    }
  })
})

describe('what can we help you with', () => {
  it('names needs, not categories: four groups that always show', async () => {
    server()
    renderAt('/')
    const section = await region('What can we help you with?')
    expect(within(section).getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual(['Find a place', 'Get something', 'Find a service', 'Go out'])
    const links = within(section).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])
    expect(links).toEqual(expect.arrayContaining([
      ['Homes and apartments', '/search?vertical=REAL_ESTATE'], ['Hotels', '/search?vertical=HOTEL'], ['Products', '/search?vertical=RETAIL'],
      ['Services and professionals', '/search?vertical=SERVICES'], ['Restaurants', '/search?vertical=RESTAURANT'],
    ]))
  })

  it('sits on a white band across the page, with its content kept at the width of the page', async () => {
    server()
    renderAt('/')
    const section = await region('What can we help you with?')
    expect(section).toHaveClass('home-needs') // the band: white in light mode, the card surface in dark mode (styles/home.css)
    expect([...section.children].some((child) => child.classList.contains('home-section'))).toBe(true)
    const css = readFileSync(join(__dirname, '../styles/home.css'), 'utf8')
    expect(css).toMatch(/\.home-needs \{ background: var\(--card\);/)
    expect(css).toMatch(/\.home-needs \+ \.home-discover \{ border-top: 0; \}/) // two white bands in a row share one rule
  })

  it('turns a suggested need into a conversation', async () => {
    const { sent } = server()
    renderAt('/')
    const section = await region('What can we help you with?')
    await userEvent.click(within(section).getByRole('button', { name: /I’m looking for an iPhone 15 in Kinshasa/ }))
    expect(await screen.findByText('Here is what I found.')).toBeInTheDocument()
    expect(sent).toEqual(['I’m looking for an iPhone 15 in Kinshasa.'])
  })

  it('shows “Get around” only when cars or transport are really on offer', async () => {
    server()
    const none = renderAt('/')
    const section = await region('What can we help you with?')
    await waitFor(() => expect(within(section).getAllByRole('heading', { level: 3 })).toHaveLength(4))
    expect(within(section).queryByRole('heading', { name: 'Get around' })).toBeNull()
    none.unmount()

    resetSiteContent()
    server({ around: [searchItem('c1', 'Toyota RAV4', { Vertical: 'VEHICLE_RENTAL' })] })
    renderAt('/')
    const withCars = await region('What can we help you with?')
    expect(await within(withCars).findByRole('heading', { name: 'Get around' })).toBeInTheDocument()
    expect(within(withCars).getByRole('link', { name: 'Cars' })).toHaveAttribute('href', '/search?vertical=VEHICLE_RENTAL')
    expect(within(withCars).getByRole('link', { name: 'Transport' })).toHaveAttribute('href', '/search?vertical=TRANSPORT')
  })
})

describe('discover what exists around you', () => {
  it('shows a few real offers in three short lists, never a catalogue', async () => {
    const { calls } = server({
      hotels: [hotel('h1', 'Grand Hotel'), hotel('h2', 'Riverside Stay'), hotel('h3', 'Lake View')],
      tables: [table('r1', 'Chez Maman')],
      fresh: [fresh('h1', 'Grand Hotel'), fresh('n1', 'Solar lamp'), fresh('n2', 'Office chair'), fresh('n3', 'Work boots'), fresh('n4', 'Desk fan')],
    })
    renderAt('/')
    const section = await region('Discover what exists around you')
    const column = (title: string) => within(within(section).getByRole('heading', { name: title }).closest('.disc-col') as HTMLElement)
    expect(column('To sleep').getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Grand Hotel', 'Riverside Stay', 'Lake View'])
    expect(column('To eat').getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Chez Maman'])
    // “New” never repeats what the other lists already show, and holds three at most
    expect(column('New on Trezutil').getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Solar lamp', 'Office chair', 'Work boots'])
    expect(within(section).getAllByRole('listitem')).toHaveLength(7)
    expect(column('To sleep').getByRole('link', { name: 'See more places to stay' })).toHaveAttribute('href', '/search?vertical=HOTEL')
    expect(column('To eat').getByRole('link', { name: 'See more places to eat' })).toHaveAttribute('href', '/search?vertical=RESTAURANT')
    expect(column('New on Trezutil').getByRole('link', { name: 'See what’s new' })).toHaveAttribute('href', '/search?sort=newest')
    const asked = calls.filter((c) => c.path === '/search').map((c) => c.body as SearchBody)
    expect(asked).toEqual(expect.arrayContaining([
      expect.objectContaining({ vertical: 'HOTEL', page_size: 3 }), expect.objectContaining({ vertical: 'RESTAURANT', page_size: 3 }),
      expect.objectContaining({ sort_field: 'newest', page_size: 9 }),
    ]))
    // a row is a name, a place and a price: a link, not a card
    expect(within(section).queryAllByRole('article')).toHaveLength(0)
    expect(within(section).getByRole('link', { name: /Grand Hotel/ })).toHaveAttribute('href', '/hotels/h1')
  })

  it('sits on a white band across the page, with its content kept at the width of the page', async () => {
    server({ hotels: [hotel('h1', 'Grand Hotel')] })
    renderAt('/')
    const section = await region('Discover what exists around you')
    expect(section).toHaveClass('home-discover') // the band: white in light mode, the card surface in dark mode (styles/home.css)
    expect([...section.children].some((child) => child.classList.contains('home-section'))).toBe(true) // the content sits inside, as in the other sections
  })

  it('leaves out a list with nothing in it, and the whole block when there is nothing to discover', async () => {
    server({ tables: [table('r1', 'Chez Maman')] })
    const some = renderAt('/')
    const section = await region('Discover what exists around you')
    expect(within(section).queryByRole('heading', { name: 'To sleep' })).toBeNull()
    expect(within(section).getByRole('heading', { name: 'To eat' })).toBeInTheDocument()
    some.unmount()

    resetSiteContent()
    server()
    renderAt('/')
    await region('From your need to the action')
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Discover what exists around you' })).toBeNull())
  })
})

describe('from need to action', () => {
  it('lets a stranger follow the loop from you, to Trezutil, to the offers, to you, to the business', async () => {
    server()
    renderAt('/')
    const section = await region('From your need to the action')
    const steps = [...section.querySelectorAll('ol.loop > li')]
    expect(steps.map((li) => [li.querySelector('.loop-who')?.textContent, li.querySelector('h3')?.textContent])).toEqual([
      ['You', 'You say what you need'], ['Trezutil', 'We understand it'], ['The offers', 'You see what fits'],
      ['You', 'You act'], ['The business', 'It confirms'],
    ])
  })

  it('draws two parties on one line: what you do, and what Trezutil and the business do', async () => {
    server()
    renderAt('/')
    const section = await region('From your need to the action')
    expect([...section.querySelectorAll('ol.loop > li')].map((li) => li.getAttribute('data-side'))).toEqual(['you', 'us', 'us', 'you', 'us'])
  })

  it('lists what you can do, and is honest about what is not there yet', async () => {
    server()
    renderAt('/')
    const section = await region('From your need to the action')
    const actions = within(within(section).getByRole('list', { name: 'Then you can' })).getAllByRole('listitem')
    expect(actions.map((li) => li.querySelector('span')?.textContent)).toEqual(['Buy', 'Book', 'Pay online', 'Contact the seller', 'Request a quote'])
    expect(actions.map((li) => li.querySelector('small')?.textContent ?? null)).toEqual([null, null, null, 'Soon', 'Soon'])
  })
})

describe('for businesses', () => {
  it('invites a business without crowding the page, with real next steps', async () => {
    server()
    renderAt('/')
    const section = await region('Have something to sell or offer?')
    expect(within(section).getByRole('link', { name: 'Start selling on Trezutil' })).toHaveAttribute('href', '/seller')
    expect(within(section).getByRole('link', { name: 'See how it works' })).toHaveAttribute('href', '/seller/onboarding')
    expect(within(section).getAllByRole('listitem').map((li) => li.querySelector('strong')?.textContent)).toEqual(['Products', 'Services', 'Orders', 'Bookings', 'Availability', 'Statistics'])
  })
})

describe('the story the page tells', () => {
  it('stays understandable with no offer at all: removing every listing leaves the idea intact', async () => {
    server() // no hotel, restaurant, car or new listing; no site settings either
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1, name: 'What do you need today?' })).toBeInTheDocument() // I can explain what I need
    await region('What can we help you with?') // Trezutil can help with a product, a service, a place or a solution
    await region('From your need to the action') // and I can buy, book, contact or request
    await region('Have something to sell or offer?')
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Discover what exists around you' })).toBeNull())
    expect(screen.queryAllByRole('article')).toHaveLength(0)
    expect(document.querySelector('a[href^="/products/"], a[href^="/hotels/"], a[href^="/restaurants/"]')).toBeNull()
  })

  it('never shows a listing card, even when offers exist: offers are rows inside one block', async () => {
    server({ hotels: [hotel('h1', 'Grand Hotel')], tables: [table('r1', 'Chez Maman')], fresh: [fresh('n1', 'Solar lamp')] })
    renderAt('/')
    await region('Discover what exists around you')
    expect(screen.queryAllByRole('article')).toHaveLength(0)
    expect(document.querySelectorAll('.result-card, .product-card, .listing-card')).toHaveLength(0)
  })

  it('does not look like an assistant product: no AI wording, no robot, no sparkles in the page body', async () => {
    server({ hotels: [hotel('h1', 'Grand Hotel')] })
    renderAt('/')
    await region('Discover what exists around you')
    const words = /\bAI\b|\bIA\b|artificial|intelligen|assistant|copilot|chatbot|robot|powered by/i
    expect(document.body.textContent).not.toMatch(words)
    const labels = [...document.querySelectorAll('[aria-label], [alt], [placeholder], [title]')].map((el) => ['aria-label', 'alt', 'placeholder', 'title'].map((a) => el.getAttribute(a) ?? '').join(' '))
    expect(labels.join(' ')).not.toMatch(words)
    // the sparkle glyph is the logo mark in the bar and footer only, never a decoration of the content
    const spark = 'path[d^="m12 3 1.7 4.3"]'
    expect(screen.getByRole('main').querySelector(spark)).toBeNull()
  })

  it('tells the same story in French', async () => {
    setLang('fr', false)
    server({ hotels: [hotel('h1', 'Grand Hotel')] })
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1, name: 'De quoi avez-vous besoin aujourd’hui ?' })).toBeInTheDocument()
    expect(screen.getByText('Dites-nous ce qu’il vous faut. Nous vous aidons à le trouver.')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Votre besoin' })).toBeInTheDocument()
    await region('Comment pouvons-nous vous aider ?')
    await region('De votre besoin à l’action')
    await region('Vous avez quelque chose à vendre ou à proposer ?')
    expect(document.body.textContent).not.toMatch(/\bIA\b|intelligence artificielle|assistant|chatbot|robot/i)
  })
})

describe('the pictures', () => {
  /** Every address a picture or video of the page points to (src, poster and srcset). */
  const addresses = (root: ParentNode) => [...root.querySelectorAll('img, source, video')]
    .flatMap((el) => [el.getAttribute('src'), el.getAttribute('poster'), ...(el.getAttribute('srcset') ?? '').split(',').map((part) => part.trim().split(/\s+/)[0])])
    .filter((src): src is string => !!src)

  it('puts a photograph behind the welcome, as decoration, in sizes for desktop and phones', async () => {
    server()
    renderAt('/')
    const hero = await region('What do you need today?')
    const background = hero.querySelector('picture.home-hero-bg')!
    expect(background).toHaveAttribute('aria-hidden', 'true')
    expect(background.querySelector('img')).toHaveAttribute('alt', '')
    expect(background.querySelector('img')?.getAttribute('srcset')).toMatch(/mall-900\.webp 900w.*mall-1600\.webp 1600w.*mall-2400\.webp 2400w/)
    expect(background.querySelector('source')?.getAttribute('srcset')).toMatch(/mall-portrait-700\.webp 700w.*mall-portrait-1100\.webp 1100w/)
    expect(hero.querySelector('video')).toBeNull() // a still picture: nothing moves, nothing to pause
    expect(within(hero).queryAllByRole('img')).toHaveLength(0) // nothing for a screen reader to describe
  })

  it('draws the needs as vector art, hidden from assistive technology', async () => {
    server()
    renderAt('/')
    const section = await region('What can we help you with?')
    const art = section.querySelectorAll('svg.need-art')
    expect(art).toHaveLength(4)
    for (const drawing of art) expect(drawing).toHaveAttribute('aria-hidden', 'true')
    expect(within(section).queryAllByRole('img')).toHaveLength(0)
  })

  it('shows the business photograph from the site itself, with a description', async () => {
    server()
    renderAt('/')
    const section = await region('Have something to sell or offer?')
    const photo = within(section).getByRole('img', { name: 'A shop owner at work' })
    expect(photo.getAttribute('src')).toMatch(/^\/images\//)
  })

  it('bundles every picture it uses: nothing is loaded from a third party, and every file exists', async () => {
    server()
    renderAt('/')
    const hero = await region('What do you need today?')
    const business = await region('Have something to sell or offer?')
    const own = [...addresses(hero), ...addresses(business)]
    expect(own.length).toBeGreaterThanOrEqual(8)
    for (const src of own) {
      expect(src).toMatch(/^\/(hero|images)\//)
      expect(existsSync(join(__dirname, '../../public', src)), src).toBe(true)
    }
  })

  it('keeps the welcome within a weight budget: no picture over 400 KB, and a phone never needs more than 150 KB', () => {
    const size = (file: string) => statSync(join(__dirname, '../../public/hero', file)).size
    for (const file of ['mall-900.webp', 'mall-1600.webp', 'mall-2400.webp', 'mall-portrait-700.webp', 'mall-portrait-1100.webp']) expect(size(file), file).toBeLessThan(400 * 1024)
    expect(size('mall-portrait-700.webp')).toBeLessThan(150 * 1024)
  })
})
