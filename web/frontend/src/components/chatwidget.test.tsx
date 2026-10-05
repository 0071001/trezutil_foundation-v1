import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ChatWidget, ChatWidgetProvider, useChatWidget } from '@/components/ChatWidget'
import { whatsappHref } from '@/lib/config'
import { I18nProvider } from '@/lib/i18n'
import { PrefsProvider } from '@/lib/prefs'
import { SessionProvider } from '@/lib/session'
import { ToastProvider } from '@/lib/toast'
import { mockApi } from '@/test/utils'

// The default page is a customer page where the chat is offered (the home page has its own place to say what you need).
function renderWidget(path = '/discover', extra?: ReactNode) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <PrefsProvider>
        <ToastProvider>
          <SessionProvider>
            <I18nProvider>
              <ChatWidgetProvider>
                {extra}
                <ChatWidget />
              </ChatWidgetProvider>
            </I18nProvider>
          </SessionProvider>
        </ToastProvider>
      </PrefsProvider>
    </MemoryRouter>,
  )
}

const reply = (text: string, extra: Record<string, unknown> = {}) => ({ body: { conversation_id: 'c1', response: { text, metadata: { action_type: 'SEARCH_LISTINGS', ...extra } } } })

/** A page-level "Ask Trezutil about this" button. */
function AskButton({ prompt }: { prompt: string }) {
  const widget = useChatWidget()
  return <button type="button" onClick={() => widget.show(prompt)}>Ask about this page</button>
}

afterEach(() => {
  vi.unstubAllEnvs()
  sessionStorage.clear()
})

describe('floating chat', () => {
  it('opens from the button, sends a message to the assistant and shows the reply with quick replies', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => reply('I found 2 homes in Gombe.', { suggestions: ['Cheapest first'] }) })
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    expect(screen.getByRole('dialog', { name: 'Chat with Trezutil' })).toBeInTheDocument()
    expect(screen.getByText('Hello! Tell me what you need.')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Message Trezutil'), 'A flat in Gombe{Enter}')
    expect(await screen.findByText('I found 2 homes in Gombe.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cheapest first' })).toBeInTheDocument()
    const call = calls.find((c) => c.path === '/conversations/message')!
    expect(call.body).toMatchObject({ message_text: 'A flat in Gombe', channel: 'web', locale: 'en' })
  })

  it('starts from a suggestion', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => reply('Here are some restaurants.') })
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    await userEvent.click(screen.getByRole('button', { name: 'A good restaurant in Gombe' }))
    expect(await screen.findByText('Here are some restaurants.')).toBeInTheDocument()
    expect((calls.find((c) => c.path === '/conversations/message')!.body as { message_text: string }).message_text).toBe('A good restaurant in Gombe')
  })

  it('sends the question raised by a page exactly once and opens the panel', async () => {
    const { calls } = mockApi({ 'POST /conversations/message': () => reply('Happy to help with that listing.') })
    renderWidget('/discover', <AskButton prompt="Tell me about Riverside apartment" />)
    await userEvent.click(screen.getByRole('button', { name: 'Ask about this page' }))
    expect(await screen.findByText('Happy to help with that listing.')).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    const sent = calls.filter((c) => c.path === '/conversations/message')
    expect(sent).toHaveLength(1)
    expect((sent[0].body as { message_text: string }).message_text).toBe('Tell me about Riverside apartment')
  })

  it('keeps the thread for the full conversation page', async () => {
    mockApi({ 'POST /conversations/message': () => reply('Noted.') })
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    await userEvent.type(screen.getByLabelText('Message Trezutil'), 'Hello there{Enter}')
    await screen.findByText('Noted.')
    const saved = JSON.parse(sessionStorage.getItem('trezutil_thread')!) as { conversationId: string; turns: { role: string; text: string }[] }
    expect(saved.conversationId).toBe('c1')
    expect(saved.turns.map((x) => [x.role, x.text])).toEqual([['user', 'Hello there'], ['assistant', 'Noted.']])
  })

  it('closes with Escape and gives the focus back to the button', async () => {
    mockApi({})
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Open the chat with Trezutil' })).toHaveFocus()
  })

  it('is not shown on the full conversation page or the sign-in pages', () => {
    mockApi({})
    for (const path of ['/conversation', '/login', '/register']) {
      const { unmount } = renderWidget(path)
      expect(screen.queryByRole('button', { name: 'Open the chat with Trezutil' })).not.toBeInTheDocument()
      unmount()
    }
  })

  it('is not shown on the home page, which is itself the place to say what you need', () => {
    mockApi({})
    renderWidget('/')
    expect(screen.queryByRole('button', { name: 'Open the chat with Trezutil' })).not.toBeInTheDocument()
  })

  it('stays out of the seller and administration consoles, which have their own frame', () => {
    mockApi({})
    for (const path of ['/seller', '/seller/orders', '/admin', '/admin/users']) {
      const { unmount } = renderWidget(path)
      expect(screen.queryByRole('button', { name: 'Open the chat with Trezutil' })).not.toBeInTheDocument()
      unmount()
    }
    renderWidget('/sellers-guide') // a customer page that only shares a prefix keeps the button
    expect(screen.getByRole('button', { name: 'Open the chat with Trezutil' })).toBeInTheDocument()
  })

  it('never shows the server’s technical wording when a detail is missing', async () => {
    mockApi({
      'POST /conversations/message': () => ({
        body: {
          conversation_id: 'c1',
          is_clarification: true,
          missing_params: [
            { name: 'listing_id', description: "required parameter 'listing_id' is missing for action GET_LISTING" },
            { name: 'party_size', description: "required parameter 'party_size' is missing for action CHECK_AVAILABILITY" },
          ],
          response: { text: 'Which one do you mean?', metadata: {} },
        },
      }),
    })
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    await userEvent.type(screen.getByLabelText('Message Trezutil'), 'Tell me more{Enter}')
    expect(await screen.findByText('Which one do you mean?')).toBeInTheDocument()
    expect(screen.queryByText(/required parameter/i)).not.toBeInTheDocument()
    expect(screen.getByText('Please tell me the party size.')).toBeInTheDocument()
    expect(screen.queryByText('Please tell me the listing id.')).not.toBeInTheDocument()
  })

  it('tells the person when the assistant cannot be reached and lets them retry', async () => {
    let fail = true
    mockApi({ 'POST /conversations/message': () => (fail ? { status: 503, body: { error: { code: 'UNAVAILABLE', message: 'Assistant is busy' } } } : reply('Back online.')) })
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    await userEvent.type(screen.getByLabelText('Message Trezutil'), 'Anything{Enter}')
    expect(await screen.findByText('Assistant is busy')).toBeInTheDocument()
    fail = false
    await userEvent.click(screen.getByRole('button', { name: /try again/i }))
    expect(await screen.findByText('Back online.')).toBeInTheDocument()
  })
})

describe('WhatsApp button', () => {
  it('opens a real chat with the configured number and a ready-made greeting', () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', '+243 81 234 5678')
    mockApi({})
    renderWidget()
    const link = screen.getByRole('link', { name: 'Chat with Trezutil on WhatsApp' })
    expect(link).toHaveAttribute('href', 'https://wa.me/243812345678?text=Hello%20Trezutil!%20I%20need%20some%20help.')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
  })

  it('is not shown, and links nowhere, when no number is configured', () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', '')
    mockApi({})
    renderWidget()
    expect(screen.queryByRole('link', { name: /whatsapp/i })).not.toBeInTheDocument()
    expect(whatsappHref('hi')).toBeNull()
  })

  it('is offered inside the chat panel too', async () => {
    vi.stubEnv('VITE_WHATSAPP_NUMBER', '243812345678')
    mockApi({})
    renderWidget()
    await userEvent.click(screen.getByRole('button', { name: 'Open the chat with Trezutil' }))
    expect(screen.getByRole('link', { name: /prefer whatsapp/i })).toHaveAttribute('href', expect.stringContaining('https://wa.me/243812345678'))
  })
})
