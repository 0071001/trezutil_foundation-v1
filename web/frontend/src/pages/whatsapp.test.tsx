import { screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setLang } from '@/lib/i18n'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt } from '@/test/utils'

/*
 * The small WhatsApp invitation on the home page, and the footer's
 * "Available on WhatsApp, PlayStore and AppStore", while the channels are not
 * connected yet: placeholders, shown only when VITE_CHANNEL_PLACEHOLDERS is
 * "true" (staging). With a real number or store address the real thing shows
 * instead; with neither, nothing is claimed. See docs/web/OUR_COMPANY.md.
 */

const WA = '243812345678'
const PLAY = 'https://play.google.com/store/apps/details?id=cd.trezutil'

const content = (sections: unknown[] = []) => mockApi({
  'GET /site/content': () => ({ body: { trust: [], partners: [], sections } }),
  'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }),
})
const region = () => screen.queryByRole('region', { name: 'Trezutil, right in your WhatsApp.' })
const findRegion = () => screen.findByRole('region', { name: 'Trezutil, right in your WhatsApp.' })

beforeEach(() => {
  setLang('en', false)
  resetVerticals()
  resetSiteContent()
  vi.stubEnv('VITE_WHATSAPP_NUMBER', '')
  vi.stubEnv('VITE_PLAY_STORE_URL', '')
  vi.stubEnv('VITE_APP_STORE_URL', '')
  vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', '')
})
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals() })

describe('the WhatsApp invitation on the home page', () => {
  it('is not drawn when the channel does not exist and placeholders are off: nothing is promised', async () => {
    content()
    renderAt('/')
    await screen.findByRole('region', { name: 'What can we help you with?' })
    expect(region()).toBeNull()
  })

  it('only the exact word "true" switches placeholders on', async () => {
    for (const value of ['false', '1', 'yes', 'TRUEISH']) {
      vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', value)
      content()
      const view = renderAt('/')
      await screen.findByRole('region', { name: 'What can we help you with?' })
      expect(region()).toBeNull()
      view.unmount()
      resetSiteContent()
    }
  })

  it('is a placeholder while the number is missing and placeholders are on: same layout, a disabled button, a QR frame', async () => {
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    content()
    renderAt('/')
    const section = await findRegion()
    expect(section).toHaveAttribute('id', 'whatsapp')
    expect(within(section).getByText('On WhatsApp')).toBeInTheDocument()
    expect(within(section).getByText(/Say what you need in a message, the way you would to a friend/)).toBeInTheDocument()
    expect(within(section).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['Nothing to install', 'No sign-up form: your number is enough', 'In French or English'])
    const button = within(section).getByRole('button', { name: 'Write to Trezutil on WhatsApp' })
    expect(button).toBeDisabled() // not a dead link: it is plainly not available yet
    expect(button).toHaveAccessibleDescription('The WhatsApp number is coming soon.')
    expect(within(section).queryByRole('link')).toBeNull() // nothing leads anywhere
    expect(within(section).getByRole('img', { name: 'QR code coming soon' })).toHaveClass('qr-placeholder')
    expect(within(section).getByText('QR code coming soon', { selector: 'figcaption' })).toBeInTheDocument()
  })

  it('is the real thing once the number is set: a link to the chat with a greeting, and a real QR code', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    content()
    renderAt('/')
    const section = await findRegion()
    const link = within(section).getByRole('link', { name: 'Write to Trezutil on WhatsApp' })
    expect(link).toHaveAttribute('href', `https://wa.me/${WA}?text=Hello%20Trezutil!%20I%20need%20some%20help.`)
    expect(link).toHaveAttribute('target', '_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
    expect(within(section).queryByRole('button')).toBeNull()
    const qr = within(section).getByRole('img', { name: 'QR code that opens the WhatsApp chat with Trezutil' })
    expect(qr).not.toHaveClass('qr-placeholder')
    expect(qr.querySelector('path')?.getAttribute('d')?.length).toBeGreaterThan(100)
    expect(within(section).queryByText(/coming soon/i)).toBeNull()
    expect(within(section).getByText('Scan to chat with Trezutil on WhatsApp')).toBeInTheDocument()
  })

  it('comes after “From your need to the action” and before the figures, in the story of the page', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    mockApi({
      'GET /site/content': () => ({ body: { trust: [{ metric: 'merchants', value: 20 }], partners: [], sections: [] } }),
      'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }),
    })
    renderAt('/')
    await screen.findByRole('region', { name: 'Trezutil in numbers' })
    const main = screen.getByRole('main')
    const names = [...main.querySelectorAll('section[aria-labelledby], section[aria-label]')].map((s) => s.getAttribute('aria-label') ?? s.getAttribute('aria-labelledby'))
    expect(names.slice(names.indexOf('how-title'), names.indexOf('how-title') + 3)).toEqual(['how-title', 'whatsapp-title', 'Trezutil in numbers'])
  })

  it('stays hidden when an administrator hides the block, even with a real number', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    content([{ key: 'whatsapp', visible: false }])
    renderAt('/')
    await screen.findByRole('region', { name: 'What can we help you with?' })
    await vi.waitFor(() => expect(region()).toBeNull())
  })

  it('speaks French', async () => {
    setLang('fr', false)
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    content()
    renderAt('/')
    const section = await screen.findByRole('region', { name: 'Trezutil, directement dans votre WhatsApp.' })
    expect(within(section).getByText('Sur WhatsApp')).toBeInTheDocument()
    expect(within(section).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['Rien à installer', 'Pas de formulaire d’inscription : votre numéro suffit', 'En français ou en anglais'])
    expect(within(section).getByRole('button', { name: 'Écrire à Trezutil sur WhatsApp' })).toBeDisabled()
    expect(within(section).getByText('Le numéro WhatsApp arrive bientôt.')).toBeInTheDocument()
  })

  it('promises nothing the platform does not do: no speed, price, support, delivery, payment or AI wording', async () => {
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    content()
    renderAt('/')
    const text = (await findRegion()).textContent ?? ''
    expect(text).not.toMatch(/\b(24\/7|instant|immediate|free|gratuit|delivery|livraison|refund|support|pay|paiement|AI|IA|assistant|intelligen)/i)
  })
})

describe('the footer, while the channels are placeholders', () => {
  const area = async () => (await screen.findByRole('contentinfo')).querySelector('.footer-brand') as HTMLElement

  it('says “Available on WhatsApp, PlayStore and AppStore.” with the names as text, not links, and a QR frame', async () => {
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    content()
    renderAt('/')
    const footer = await area()
    const line = footer.querySelector('.footer-available') as HTMLElement
    expect(line.textContent).toBe('Available on WhatsApp, PlayStore and AppStore.')
    expect(line.querySelectorAll('a')).toHaveLength(0) // nothing to click yet
    expect([...line.querySelectorAll('[data-placeholder]')].map((n) => n.textContent)).toEqual(['WhatsApp', 'PlayStore', 'AppStore'])
    expect(within(footer).getByRole('img', { name: 'QR code coming soon' })).toHaveClass('qr-placeholder')
    expect(footer.querySelector('figcaption')?.textContent).toBe('QR code coming soon')
    expect(within(footer).queryByRole('link', { name: 'Find us on WhatsApp' })).toBeNull() // that button needs a number
  })

  it('connects each channel as its real value arrives, leaving the others as placeholders', async () => {
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    vi.stubEnv('VITE_PLAY_STORE_URL', PLAY)
    content()
    renderAt('/')
    const footer = await area()
    const line = footer.querySelector('.footer-available') as HTMLElement
    expect(line.textContent).toBe('Available on WhatsApp, PlayStore and AppStore.')
    expect([...line.querySelectorAll('a')].map((a) => [a.textContent, a.getAttribute('href')])).toEqual([['WhatsApp', `https://wa.me/${WA}`], ['PlayStore', PLAY]])
    expect([...line.querySelectorAll('[data-placeholder]')].map((n) => n.textContent)).toEqual(['AppStore'])
    expect(within(footer).getByRole('img', { name: 'QR code that opens the WhatsApp chat with Trezutil' })).not.toHaveClass('qr-placeholder')
  })

  it('is silent in a production build: no placeholders, so nothing is named that does not exist', async () => {
    content()
    renderAt('/')
    const footer = await area()
    expect(footer.querySelector('.footer-available')).toBeNull()
    expect(footer.querySelector('.qr')).toBeNull()
  })

  it('says it in French', async () => {
    setLang('fr', false)
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    content()
    renderAt('/')
    const footer = await area()
    expect(footer.querySelector('.footer-available')?.textContent).toBe('Disponible sur WhatsApp, PlayStore et AppStore.')
    expect(footer.querySelector('figcaption')?.textContent).toBe('Code QR bientôt disponible')
  })
})
