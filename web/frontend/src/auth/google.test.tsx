import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { me, mockApi, renderAt } from '@/test/utils'

/* Sign in with Google on the login screen, with Google's script played by a stub (the real one draws the button). */

let onCredential: ((r: { credential?: string }) => void) | null = null
let initialized: Record<string, unknown> | null = null

function stubGoogle() {
  window.google = { accounts: { id: {
    initialize: (config: Record<string, unknown>) => { initialized = config; onCredential = config.callback as typeof onCredential },
    renderButton: (parent: HTMLElement) => { const b = document.createElement('button'); b.textContent = 'Continuer avec Google'; b.onclick = () => onCredential?.({ credential: 'google-id-token' }); parent.appendChild(b) },
    disableAutoSelect: () => {},
  } } }
}

afterEach(() => { delete window.google; onCredential = null; initialized = null })

describe('sign in with Google', () => {
  it('is hidden when the server does not offer it', async () => {
    mockApi({ 'GET /auth/providers': () => ({ body: { phone: { enabled: true }, google: { enabled: false, client_id: '' } } }) })
    renderAt('/login')
    await screen.findByText(/Connecte-toi à Trezutil/)
    await waitFor(() => expect(document.querySelector('.google-signin')).toBeNull())
  })

  it('sends the Google credential with the server nonce and opens the session', async () => {
    stubGoogle()
    const { calls } = mockApi({
      'GET /auth/providers': () => ({ body: { google: { enabled: true, client_id: 'cid.apps.googleusercontent.com' } } }),
      'POST /auth/google/nonce': () => ({ body: { nonce: 'nonce-1' } }),
      'POST /auth/google': () => ({ body: { user: me, tokens: { access_token: 'acc', refresh_token: 'ref' } } }),
      'GET /auth/users/me': () => ({ body: me }),
    })
    renderAt('/login')
    fireEvent.click(await screen.findByText('Continuer avec Google'))
    await waitFor(() => expect(localStorage.getItem('trezutil_access_token')).toBe('acc'))
    expect(initialized?.client_id).toBe('cid.apps.googleusercontent.com')
    expect(initialized?.nonce).toBe('nonce-1')
    const post = calls.find((c) => c.method === 'POST' && c.path === '/auth/google')!
    expect(post.body).toEqual({ credential: 'google-id-token', nonce: 'nonce-1' })
    expect(post.headers.Authorization).toBeUndefined() // a sign-in never carries an old token
  })

  it('explains how to link when the email already belongs to an account', async () => {
    stubGoogle()
    mockApi({
      'GET /auth/providers': () => ({ body: { google: { enabled: true, client_id: 'cid' } } }),
      'POST /auth/google/nonce': () => ({ body: { nonce: 'n' } }),
      'POST /auth/google': () => ({ status: 409, body: { error: { code: 'ACCOUNT_EXISTS', message: 'exists' } } }),
    })
    renderAt('/login')
    fireEvent.click(await screen.findByText('Continuer avec Google'))
    expect(await screen.findByText(/lie Google dans Paramètres → Sécurité/)).toBeTruthy()
    expect(localStorage.getItem('trezutil_access_token')).toBeNull()
  })
})
