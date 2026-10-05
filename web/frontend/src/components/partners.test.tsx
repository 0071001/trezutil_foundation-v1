import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { beforeEach, describe, expect, it } from 'vitest'
import { PartnersBand, PartnersBandView } from '@/components/partners'
import { resetSiteContent } from '@/lib/useSiteContent'
import { mockApi } from '@/test/utils'

// Tests run in a DOM environment where import.meta.url is not a file URL, so paths hang off this file's folder.
const frontend = resolve(__dirname, '../..')

const partner = (id: string, name: string, extra: Record<string, unknown> = {}) => ({ id, name, kind: 'bank', logo_url: `/partners/${id}.png`, ...extra })
const content = (partners: unknown[]) => ({ 'GET /site/content': () => ({ body: { trust: [], partners, sections: [] } }) })

beforeEach(() => resetSiteContent())

describe('PartnersBand (what the server sends)', () => {
  it('lists each partner once for assistive technology: the second loop is a hidden copy', async () => {
    mockApi(content([partner('rawbank', 'Rawbank'), partner('tmb', 'TMB')]))
    const { container } = render(<PartnersBand />)
    const section = await screen.findByRole('region', { name: 'Our partners' })
    expect(within(section).getAllByRole('img').map((i) => i.getAttribute('alt'))).toEqual(['Rawbank', 'TMB'])
    const tracks = container.querySelectorAll('.partners-track')
    expect(tracks).toHaveLength(2)
    expect(tracks[1]).toHaveAttribute('aria-hidden', 'true')
    expect(tracks[0]).not.toHaveAttribute('aria-hidden')
    for (const img of tracks[1]!.querySelectorAll('img')) expect(img).toHaveAttribute('alt', '')
    expect(screen.getByText('Banks, mobile money, shops, hotels and innovation')).toBeInTheDocument()
  })

  it('draws the logos the server names, from this site, and nothing else', async () => {
    mockApi(content([partner('rawbank', 'Rawbank')]))
    const { container } = render(<PartnersBand />)
    await screen.findByRole('region', { name: 'Our partners' })
    const sources = [...container.querySelectorAll('img')].map((i) => i.getAttribute('src'))
    expect(sources).toEqual(['/partners/rawbank.png', '/partners/rawbank.png'])
  })

  it('shows no band, and invents no logo, when there are no partners', async () => {
    const { fetchMock } = mockApi(content([]))
    const { container } = render(<PartnersBand />)
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('shows no band when the content cannot be loaded', async () => {
    const { fetchMock } = mockApi({ 'GET /site/content': () => ({ status: 500, body: { error: { code: 'X', message: 'down' } } }) })
    const { container } = render(<PartnersBand />)
    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('opens a partner’s website in a new tab, safely, only from the visible loop', async () => {
    mockApi(content([partner('tmb', 'TMB', { link_url: 'https://tmb.cd' }), partner('rawbank', 'Rawbank')]))
    const { container } = render(<PartnersBand />)
    const link = await screen.findByRole('link', { name: 'TMB (opens its website)' })
    expect(link).toHaveAttribute('href', 'https://tmb.cd')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    // The copy used for the seamless scroll is hidden and can never take focus.
    const copy = container.querySelectorAll('.partners-track')[1]!.querySelector('a')!
    expect(copy).toHaveAttribute('tabindex', '-1')
    expect(screen.queryByRole('link', { name: /Rawbank/ })).not.toBeInTheDocument() // no website, no link
  })

  it('gives a tall or square logo more height than a wide one', async () => {
    mockApi(content([partner('tmb', 'TMB'), partner('rawbank', 'Rawbank')]))
    render(<PartnersBand />)
    const [tmb, rawbank] = await screen.findAllByRole('img')
    for (const [img, w, h] of [[tmb!, 813, 931], [rawbank!, 1024, 236]] as const) {
      Object.defineProperty(img, 'naturalWidth', { value: w, configurable: true })
      Object.defineProperty(img, 'naturalHeight', { value: h, configurable: true })
      fireEvent.load(img)
    }
    expect(tmb!.closest('li')).toHaveAttribute('data-tall', 'true')
    expect(rawbank!.closest('li')).not.toHaveAttribute('data-tall')
  })

  it('can be paused and resumed, because moving content must be stoppable', async () => {
    mockApi(content([partner('tmb', 'TMB')]))
    const { container } = render(<PartnersBand />)
    const marquee = (await screen.findByRole('region', { name: 'Our partners' })).querySelector('.partners-marquee')!
    expect(marquee).not.toHaveAttribute('data-paused')
    const pause = screen.getByRole('button', { name: 'Pause scrolling' })
    expect(pause).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(pause)
    expect(marquee).toHaveAttribute('data-paused', 'true')
    const resume = screen.getByRole('button', { name: 'Resume scrolling' })
    expect(resume).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(resume)
    expect(marquee).not.toHaveAttribute('data-paused')
    expect(container.querySelectorAll('.partners-track button, .partners-track [tabindex]:not([tabindex="-1"])')).toHaveLength(0)
  })
})

describe('PartnersBandView (the editor preview)', () => {
  it('draws exactly the partners it is given, with no request', () => {
    const { fetchMock } = mockApi({})
    render(<PartnersBandView partners={[{ id: 'a', name: 'A Bank', kind: 'bank', logoUrl: '/partners/a.png', linkUrl: '' }]} />)
    expect(screen.getByRole('img', { name: 'A Bank' })).toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
  })
})

describe('reduced motion', () => {
  it('stops scrolling for people who ask for it', () => {
    const css = readFileSync(resolve(frontend, 'src/styles/app.css'), 'utf8')
    const reduced = css.slice(css.indexOf('/* Reduced motion: no scrolling.'))
    expect(reduced).toMatch(/prefers-reduced-motion: reduce/)
    expect(reduced).toMatch(/\.partners-track \{ animation: none;/)
    expect(reduced).toMatch(/\.partners-track\[aria-hidden="true"\] \{ display: none; \}/)
  })
})
