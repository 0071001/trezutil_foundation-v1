import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { vi } from 'vitest'
import { AppRoutes } from '@/App'
import { I18nProvider } from '@/lib/i18n'
import { SessionProvider } from '@/lib/session'

export interface MockCall { method: string; path: string; body: unknown; headers: Record<string, string> }
type Handler = (call: MockCall) => { status?: number; body?: unknown } | Promise<{ status?: number; body?: unknown }>

/**
 * Routes fetch() by "METHOD /path" (path without the /api/v1 prefix and query).
 * Unmatched requests fail loudly so tests never hit an unexpected endpoint.
 */
export function mockApi(routes: Record<string, Handler>) {
  const calls: MockCall[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const method = (init?.method ?? 'GET').toUpperCase()
    const path = url.pathname.replace(/^\/api\/v1/, '')
    const headers = Object.fromEntries(Object.entries((init?.headers ?? {}) as Record<string, string>))
    // JSON bodies arrive parsed; a file upload arrives as its form fields (the file itself under its field name).
    const raw = init?.body
    const requestBody = raw instanceof FormData ? Object.fromEntries(raw.entries()) : raw ? JSON.parse(String(raw)) : undefined
    const call = { method, path: `${path}${url.search}`, body: requestBody, headers }
    calls.push(call)
    const handler = routes[`${method} ${path}`]
    if (!handler) return new Response(JSON.stringify({ error: { code: 'UNMOCKED', message: `${method} ${path}` } }), { status: 599 })
    const { status = 200, body = {} } = await handler(call)
    return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { calls, fetchMock }
}

export function signIn(token = 'access-1', refresh = 'refresh-1') {
  localStorage.setItem('trezutil_access_token', token)
  localStorage.setItem('trezutil_refresh_token', refresh)
}

export const me = { id: 'u1', phone: '+243810000001', display_name: 'Jean Mukendi', status: 'active', roles: ['customer'], created_at: '2026-01-01T00:00:00Z' }

export function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nProvider>
        <SessionProvider><AppRoutes /></SessionProvider>
      </I18nProvider>
    </MemoryRouter>,
  )
}

export const searchItem = (id: string, title: string, extra: Record<string, unknown> = {}) => ({
  ListingID: id, Title: title, PriceAmount: 85, PriceCurrency: 'USD', AvailabilityStatus: 'in_stock',
  Store: { StoreID: 's1', Name: 'Gombe Stays' }, Seller: { SellerID: 'sp1', DisplayName: 'Gombe Stays', TrustLevel: 1 },
  Location: { City: 'Kinshasa', Neighborhood: 'Gombe' }, PrimaryMediaURL: 'https://images.example.com/a.jpg', ...extra,
})
