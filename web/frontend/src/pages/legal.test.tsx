import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { LEGAL, LEGAL_DOCS, type LegalDoc } from '@/lib/legal'
import { mockApi, renderAt } from '@/test/utils'

describe('legal pages', () => {
  it.each(Object.keys(LEGAL_DOCS) as LegalDoc[])('%s renders, says it is a draft, and marks what is missing', async (doc) => {
    mockApi({})
    renderAt(`/legal/${doc}`)
    expect(await screen.findByRole('heading', { level: 1, name: LEGAL_DOCS[doc] })).toBeTruthy()
    expect(screen.getByRole('note').textContent).toMatch(/projet en cours de validation/)
    if (doc !== 'cookies') expect(document.querySelectorAll('.legal-missing').length).toBeGreaterThan(0)
  })

  it('never invents a company name: every unknown fact is a marker', async () => {
    mockApi({})
    renderAt('/legal/mentions')
    await screen.findByRole('heading', { level: 1, name: 'Mentions légales' })
    expect(LEGAL.entity.name).toBe('')
    expect(screen.getAllByText(/À compléter : nom de la société/, { selector: 'mark', exact: false }).length).toBeGreaterThan(0)
    expect(screen.getAllByText(/Contabo GmbH/).length).toBeGreaterThan(0) // the one fact known from the server
  })

  it('lists every document on the index, and an unknown document falls back to it', async () => {
    mockApi({})
    renderAt('/legal/inconnu')
    expect(await screen.findByRole('heading', { level: 1, name: 'Informations légales' })).toBeTruthy()
    for (const title of Object.values(LEGAL_DOCS)) expect(screen.getAllByRole('link', { name: title }).length).toBeGreaterThan(0)
  })

  it('the sign-in screen links the texts without claiming they were accepted while they are drafts', async () => {
    mockApi({ 'GET /auth/providers': () => ({ body: { google: { enabled: false } } }) })
    renderAt('/login')
    const legal = (await screen.findByText(/Consulte les/)).closest('p')!
    expect(legal.textContent).not.toMatch(/tu acceptes/)
    expect(screen.getByRole('link', { name: 'politique de confidentialité' }).getAttribute('href')).toBe('/legal/confidentialite')
  })
})
