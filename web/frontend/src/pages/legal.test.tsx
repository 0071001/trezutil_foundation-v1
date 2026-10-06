import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { setLang } from '@/lib/i18n'
import { LEGAL, LEGAL_DOCS } from '@/lib/legal'
import { resetSiteContent } from '@/lib/useSiteContent'
import { resetVerticals } from '@/lib/verticals'
import { mockApi, renderAt } from '@/test/utils'

/*
 * The legal pages exist before the company's facts do, and before a lawyer has
 * read them. These tests pin what keeps that honest: the draft banner, the
 * highlighted missing facts (never an invented value), the places the pages are
 * linked from and the sign-in line that only appears once they are reviewed.
 */

const pristine = JSON.parse(JSON.stringify(LEGAL)) as typeof LEGAL

beforeEach(() => {
  setLang('en', false)
  resetVerticals()
  resetSiteContent()
  Object.assign(LEGAL, JSON.parse(JSON.stringify(pristine)))
  vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', '')
  mockApi({ 'GET /site/content': () => ({ body: { trust: [], partners: [], sections: [] } }), 'POST /search': () => ({ body: { items: [], total: 0, has_next: false } }) })
})
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); Object.assign(LEGAL, JSON.parse(JSON.stringify(pristine))) })

const doc = async () => screen.findByRole('article')

describe('the legal documents', () => {
  it('has four, each on its own address, with the list of them beside it', async () => {
    renderAt('/legal/terms')
    await doc()
    const nav = screen.getByRole('navigation', { name: 'Legal documents' })
    expect(within(nav).getAllByRole('link').map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Terms of use', '/legal/terms'], ['Privacy policy', '/legal/privacy'], ['Cancellations and refunds', '/legal/refunds'], ['Legal notice', '/legal/notice'],
    ])
    expect(within(nav).getByRole('link', { name: 'Terms of use' })).toHaveAttribute('aria-current', 'page')
  })

  it('opens the terms at /legal and for an unknown document', async () => {
    renderAt('/legal')
    expect(await screen.findByRole('heading', { level: 1, name: 'Terms of use' })).toBeInTheDocument()
  })

  for (const [path, title] of [['terms', 'Terms of use'], ['privacy', 'Privacy policy'], ['refunds', 'Cancellations and refunds'], ['notice', 'Legal notice']] as const) {
    it(`shows “${title}” with one title, numbered sections and a table of contents that leads to them`, async () => {
      renderAt(`/legal/${path}`)
      expect(await screen.findByRole('heading', { level: 1, name: title })).toBeInTheDocument()
      expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
      const toc = screen.getByRole('navigation', { name: 'On this page' })
      const entries = within(toc).getAllByRole('link')
      expect(entries.length).toBeGreaterThanOrEqual(4)
      for (const link of entries) {
        const id = link.getAttribute('href')!.split('#')[1]!
        expect(document.getElementById(id)).not.toBeNull() // every entry has its section
      }
      expect(document.title).toBe(`${title} — Trezutil`)
    })
  }
})

describe('a draft says it is a draft', () => {
  it('carries a banner until the texts are marked reviewed', async () => {
    renderAt('/legal/privacy')
    await doc()
    expect(screen.getByRole('note')).toHaveTextContent('This text is a draft: a lawyer has not reviewed it yet and it is not final.')
  })

  it('drops the banner once reviewed', async () => {
    LEGAL.reviewed = true
    renderAt('/legal/privacy')
    await doc()
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('highlights every fact it does not have and never invents one', async () => {
    renderAt('/legal/notice')
    const article = await doc()
    const marks = [...article.querySelectorAll('mark.legal-todo')].map((m) => m.textContent)
    expect(marks).toEqual(expect.arrayContaining(['[To complete: company name]', '[To complete: legal form]', '[To complete: registered address]', '[To complete: registration numbers]', '[To complete: host (name, address, country)]']))
    expect(article.textContent).not.toMatch(/SARL|SA |RCCM|Kinshasa, /) // no company, no number, no street was made up
  })

  it('shows a fact as plain text once it is given, in place of its marker', async () => {
    LEGAL.entity.name = 'Trezutil Congo SARL'
    LEGAL.entity.email = 'legal@trezutil.example'
    renderAt('/legal/terms')
    const article = await doc()
    expect(article.textContent).toContain('operated by Trezutil Congo SARL')
    expect(article.textContent).toContain('legal@trezutil.example')
    expect(article.querySelectorAll('mark.legal-todo')).not.toHaveLength(0) // others are still missing
    expect([...article.querySelectorAll('mark.legal-todo')].map((m) => m.textContent)).not.toContain('[To complete: company name]')
  })
})

describe('what the texts say about the product is what the product does', () => {
  it('terms: Trezutil is not the seller, nothing is ordered without confirmation, cards never reach Trezutil, "soon" is not promised', async () => {
    renderAt('/legal/terms')
    const text = (await doc()).textContent!
    expect(text).toMatch(/Trezutil is not the seller/)
    expect(text).toMatch(/No order or booking is made unless you have expressly confirmed it/)
    expect(text).toMatch(/Trezutil neither receives nor keeps them/)
    expect(text).toMatch(/announced as “soon” .* are not available yet/)
    expect(text).toMatch(/one-time code is sent to you on WhatsApp/)
  })

  it('privacy: names the recipients (WhatsApp, the payment provider, AI providers), says there is no self-service deletion and no advertising cookie', async () => {
    renderAt('/legal/privacy')
    const text = (await doc()).textContent!
    for (const who of ['WhatsApp (Meta)', 'Stripe', 'Anthropic or Google']) expect(text).toContain(who)
    expect(text).toMatch(/Self-service account deletion does not exist yet/)
    expect(text).toMatch(/no advertising cookie/)
    expect(text).toMatch(/No card number is kept by us/)
  })

  it('refunds: a refund cannot exceed what was paid, and the delays are left for the company to set', async () => {
    renderAt('/legal/refunds')
    const article = await doc()
    expect(article.textContent).toMatch(/It cannot exceed the amount paid/)
    expect([...article.querySelectorAll('mark.legal-todo')].map((m) => m.textContent)).toEqual(expect.arrayContaining(['[To complete: time limit to ask for a refund]', '[To complete: refund delay]']))
  })

  it('privacy and terms agree on who the operator is and where to write', async () => {
    LEGAL.entity.name = 'Trezutil Congo SARL'
    LEGAL.entity.email = 'legal@trezutil.example'
    for (const path of ['terms', 'privacy', 'refunds', 'notice']) {
      const view = renderAt(`/legal/${path}`)
      const text = (await doc()).textContent!
      expect(text).toContain('legal@trezutil.example')
      view.unmount()
    }
  })
})

describe('in French', () => {
  it('is written in French, with the same sections, and its own markers', async () => {
    setLang('fr', false)
    renderAt('/legal/terms')
    expect(await screen.findByRole('heading', { level: 1, name: 'Conditions d’utilisation' })).toBeInTheDocument()
    const article = await doc()
    expect(article.textContent).toMatch(/Trezutil n’est pas le vendeur/)
    expect(article.textContent).toMatch(/Aucune commande ni réservation n’est faite sans que vous l’ayez confirmée expressément/)
    expect(article.querySelector('mark.legal-todo')?.textContent).toMatch(/^\[À compléter : /)
    expect(screen.getByRole('note')).toHaveTextContent('Ce texte est un projet')
    expect(screen.getByText(/Dernière mise à jour : 5 octobre 2026/)).toBeInTheDocument()
  })

  it('has as many sections in French as in English, for every document', async () => {
    const counts: Record<string, number[]> = {}
    for (const lang of ['en', 'fr'] as const) {
      setLang(lang, false)
      for (const d of LEGAL_DOCS) {
        const view = renderAt(`/legal/${d}`)
        await doc()
        ;(counts[d] ??= []).push(document.querySelectorAll('article section').length)
        view.unmount()
      }
    }
    for (const d of LEGAL_DOCS) expect(counts[d]![0]).toBe(counts[d]![1])
  })
})

describe('where the pages are linked from', () => {
  const footerLinks = async () => {
    const footer = await screen.findByRole('contentinfo')
    return within(footer).queryByRole('navigation', { name: 'Legal' })
  }

  it('not from the footer of the live site while they are drafts', async () => {
    renderAt('/')
    expect(await footerLinks()).toBeNull()
  })

  it('from the footer on staging, where placeholders are on', async () => {
    vi.stubEnv('VITE_CHANNEL_PLACEHOLDERS', 'true')
    renderAt('/')
    const nav = await footerLinks()
    expect(nav).not.toBeNull()
    expect(within(nav!).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/legal/terms', '/legal/privacy', '/legal/refunds', '/legal/notice'])
  })

  it('from the footer of the live site once the texts are reviewed', async () => {
    LEGAL.reviewed = true
    renderAt('/')
    expect(await footerLinks()).not.toBeNull()
  })
})

describe('the sign-in form', () => {
  it('says the code comes on WhatsApp, not by text message', async () => {
    renderAt('/login')
    expect(await screen.findByText(/We’ll send you a 6-digit code on WhatsApp/)).toBeInTheDocument()
    expect(screen.getByText('By continuing you agree to receive a one-time code on WhatsApp.')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/SMS|text you/i)
  })

  it('does not tell anyone they accept a draft', async () => {
    renderAt('/login')
    await screen.findByText(/We’ll send you a 6-digit code/)
    expect(screen.queryByText(/you accept the/i)).toBeNull()
  })

  it('asks for acceptance of the terms and the privacy policy once they are reviewed, with links to them', async () => {
    LEGAL.reviewed = true
    renderAt('/login')
    const line = (await screen.findByText(/By continuing you accept the/)).closest('p')!
    expect(within(line).getByRole('link', { name: 'Terms of use' })).toHaveAttribute('href', '/legal/terms')
    expect(within(line).getByRole('link', { name: 'Privacy policy' })).toHaveAttribute('href', '/legal/privacy')
    await userEvent.click(within(line).getByRole('link', { name: 'Privacy policy' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Privacy policy' })).toBeInTheDocument()
  })
})
