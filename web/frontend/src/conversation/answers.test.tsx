import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { similarTo } from '@/commerce/OfferExtras'
import { toOffer } from '@/commerce/offers'
import { mapSearchResult } from '@/api/search'
import { CountAnswer, DateAnswer } from '@/conversation/Answers'
import { asDateField, fieldDate, inDays, nextSaturday } from '@/lib/dates'
import { me, mockApi, renderAt, searchItem, signIn } from '@/test/utils'

describe('answering with pickers', () => {
  it('sends the picked date as the person would type it, and refuses a past one', async () => {
    const onSend = vi.fn()
    render(<DateAnswer param="start_at" disabled={false} onSend={onSend} />)
    const field = screen.getByLabelText(/Date et heure de début/)
    fireEvent.change(field, { target: { value: '2001-01-01T10:00' } })
    expect(screen.getByRole('button', { name: /Valider la date/ })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Demain' }))
    await userEvent.click(screen.getByRole('button', { name: /Valider la date/ }))
    expect(onSend).toHaveBeenCalledWith(fieldDate(inDays(1), 14).replace('T', ' '))
  })

  it('keeps the end after the start', async () => {
    const onSend = vi.fn()
    const start = fieldDate(inDays(5), 14)
    render(<DateAnswer param="end_at" after={start} disabled={false} onSend={onSend} />)
    fireEvent.change(screen.getByLabelText(/Date et heure de fin/), { target: { value: fieldDate(inDays(4), 11) } })
    expect(screen.getByRole('alert')).toHaveTextContent('La fin doit venir après le début.')
    await userEvent.click(screen.getByRole('button', { name: '2 nuits' }))
    await userEvent.click(screen.getByRole('button', { name: /Valider la date/ }))
    expect(onSend).toHaveBeenCalledWith(fieldDate(inDays(2, new Date(start)), 11).replace('T', ' '))
  })

  it('counts nights', async () => {
    const onSend = vi.fn()
    render(<CountAnswer param="nights" disabled={false} onSend={onSend} />)
    await userEvent.click(screen.getByRole('button', { name: 'Ajouter un' }))
    await userEvent.click(screen.getByRole('button', { name: /Valider/ }))
    expect(onSend).toHaveBeenCalledWith('2')
  })

  it('reads only whole dates, and finds the next Saturday', () => {
    expect(asDateField('2026-12-01 14:00')).toBe('2026-12-01T14:00')
    expect(asDateField('pour 2 personnes')).toBe('')
    expect(nextSaturday(new Date('2026-11-28T09:00:00')).getDate()).toBe(5)
  })

  it('shows the date picker when the assistant asks when to start, and sends the date', async () => {
    signIn()
    const { calls } = mockApi({
      'GET /auth/users/me': () => ({ body: me }),
      'GET /orders': () => ({ body: { items: [] } }),
      'GET /conversations': () => ({ body: { items: [] } }),
      'GET /missions': () => ({ body: { items: [], total: 0 } }),
      'POST /conversations/message': () => ({ body: { conversation_id: 'c1', is_clarification: true, requires_user_input: true, missing_params: [{ name: 'start_at', description: '' }], response: { text: 'Quand voulez-vous commencer ?', metadata: { action_type: 'CREATE_RESERVATION' } } } }),
    })
    renderAt('/')
    await userEvent.type(await screen.findByLabelText('Ton besoin'), 'réserve une chambre{Enter}')
    await userEvent.click(await screen.findByRole('button', { name: /Valider la date/ }))
    const sent = calls.filter((c) => c.path === '/conversations/message')
    expect((sent.at(-1)!.body as { message_text: string }).message_text).toMatch(/^\d{4}-\d{2}-\d{2} 14:00$/)
  })
})

describe('similar offers', () => {
  const offer = (id: string, price: number, currency = 'USD') => toOffer(mapSearchResult(searchItem(id, id, { PriceAmount: price, PriceCurrency: currency })), 0)

  it('leaves out the offer itself and puts the closest prices of the same currency first', () => {
    const list = similarTo(offer('me', 100), [offer('far', 400), offer('me', 100), offer('francs', 100, 'CDF'), offer('near', 110)])
    expect(list.map((o) => o.id)).toEqual(['near', 'far', 'francs'])
  })
})
