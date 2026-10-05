import { screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { qrModules } from '@/components/QrCode'
import { setLang } from '@/lib/i18n'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt } from '@/test/utils'

/*
 * The footer says where Trezutil can be reached: "Available on WhatsApp,
 * PlayStore and AppStore", and a QR code for the WhatsApp chat. Each channel
 * appears only when it exists (a number, a published store page), so the footer
 * never promises an app that is not published nor points a QR code at a number
 * nobody set. See docs/web/OUR_COMPANY.md.
 */

const WA = '243812345678'
const PLAY = 'https://play.google.com/store/apps/details?id=cd.trezutil'
const APP = 'https://apps.apple.com/app/trezutil/id000000000'

const footer = async () => screen.findByRole('contentinfo')
const brand = (el: HTMLElement) => el.querySelector('.footer-brand') as HTMLElement

beforeEach(() => {
  setLang('en', false)
  resetVerticals()
  resetSiteContent()
  mockApi({ 'GET /site/content': () => ({ body: { trust: [], partners: [], sections: [] } }), 'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }) })
})
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals() })

describe('where Trezutil can be reached, in the footer', () => {
  it('says nothing, and shows no QR code, while nothing is configured: no claim without a real channel', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', '')
    vi.stubEnv('VITE_PLAY_STORE_URL', '')
    vi.stubEnv('VITE_APP_STORE_URL', '')
    renderAt('/')
    const area = brand(await footer())
    expect(area.textContent).toContain('Commerce through conversation.')
    expect(within(area).queryByText(/Available on/)).toBeNull()
    expect(area.querySelector('svg.qr')).toBeNull()
  })

  it('says “Available on WhatsApp, PlayStore and AppStore.” right after the tagline, each name a link', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    vi.stubEnv('VITE_PLAY_STORE_URL', PLAY)
    vi.stubEnv('VITE_APP_STORE_URL', APP)
    renderAt('/')
    const area = brand(await footer())
    const line = area.querySelector('.footer-available') as HTMLElement
    expect(line.textContent).toBe('Available on WhatsApp, PlayStore and AppStore.')
    expect(line.previousElementSibling?.textContent).toBe('Commerce through conversation.Built around what you need.') // right after the tagline
    expect([...line.querySelectorAll('a')].map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['WhatsApp', `https://wa.me/${WA}`], ['PlayStore', PLAY], ['AppStore', APP],
    ])
    for (const a of line.querySelectorAll('a')) expect(a.getAttribute('rel')).toContain('noopener')
  })

  it('says it in French', async () => {
    setLang('fr', false)
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    vi.stubEnv('VITE_PLAY_STORE_URL', PLAY)
    vi.stubEnv('VITE_APP_STORE_URL', APP)
    renderAt('/')
    const area = brand(await footer())
    expect(area.querySelector('.footer-available')?.textContent).toBe('Disponible sur WhatsApp, PlayStore et AppStore.')
    expect(area.querySelector('figcaption')?.textContent).toBe('Scannez pour discuter avec Trezutil sur WhatsApp')
    expect(within(area).getByRole('img', { name: 'Code QR qui ouvre la conversation WhatsApp avec Trezutil' })).toBeInTheDocument()
  })

  it('names only the channels that exist', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    vi.stubEnv('VITE_PLAY_STORE_URL', '')
    vi.stubEnv('VITE_APP_STORE_URL', '')
    const only = renderAt('/')
    expect(brand(await footer()).querySelector('.footer-available')?.textContent).toBe('Available on WhatsApp.')
    only.unmount()

    vi.stubEnv('VITE_WHATSAPP_NUMBER', '')
    vi.stubEnv('VITE_PLAY_STORE_URL', PLAY)
    vi.stubEnv('VITE_APP_STORE_URL', APP)
    renderAt('/')
    const area = brand(await footer())
    expect(area.querySelector('.footer-available')?.textContent).toBe('Available on PlayStore and AppStore.')
    expect(area.querySelector('svg.qr')).toBeNull() // no WhatsApp number, so no QR code for it
  })

  it('ignores a store address that is not https, instead of linking it', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    vi.stubEnv('VITE_PLAY_STORE_URL', 'javascript:alert(1)')
    vi.stubEnv('VITE_APP_STORE_URL', 'http://apps.example.com/trezutil')
    renderAt('/')
    const line = brand(await footer()).querySelector('.footer-available') as HTMLElement
    expect(line.textContent).toBe('Available on WhatsApp.')
    expect(line.innerHTML).not.toContain('javascript')
  })

  it('shows a QR code for the WhatsApp chat, with a caption and an accessible name', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', WA)
    renderAt('/')
    const area = brand(await footer())
    const qr = within(area).getByRole('img', { name: 'QR code that opens the WhatsApp chat with Trezutil' })
    expect(qr.tagName.toLowerCase()).toBe('svg')
    expect(area.querySelector('figure.footer-qr figcaption')?.textContent).toBe('Scan to chat with Trezutil on WhatsApp')
    expect(qr.querySelector('path')?.getAttribute('d')?.length).toBeGreaterThan(100)
  })
})

describe('the QR code itself', () => {
  it('has a quiet zone of four modules on every side and the three finder squares', () => {
    const { size, path } = qrModules(`https://wa.me/${WA}`)
    const modules = size - 8
    expect([21, 25, 29, 33]).toContain(modules) // a short address fits the smallest versions: big modules, easy to scan
    // the top-left finder pattern starts with a row of seven dark modules, right after the quiet zone
    expect(path.startsWith('M4 4h7v1h-7z')).toBe(true)
    // the top-right one ends at the right edge of the code, the quiet zone beyond it
    expect(path).toContain(`M${size - 4 - 7} 4h7v1h-7z`)
  })

  it('encodes the text it is given, and a different text gives a different code', () => {
    expect(qrModules('https://wa.me/243812345678').path).not.toBe(qrModules('https://wa.me/243812345679').path)
    expect(qrModules('https://wa.me/243812345678').path).toBe(qrModules('https://wa.me/243812345678').path)
  })
})
