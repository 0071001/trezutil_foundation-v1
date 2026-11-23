/*
 * Google Identity Services ("Sign in with Google"). The official button is drawn by Google's own script, with
 * Google's logo and branding; it hands back a Google ID token, which the server verifies (signature, this client,
 * expiry and the nonce it issued). The browser never holds any secret: the client id is public.
 */

import { isDark } from '@/lib/theme'

interface GoogleCredentialResponse { credential?: string }
interface GoogleIdApi {
  initialize(config: Record<string, unknown>): void
  renderButton(parent: HTMLElement, options: Record<string, unknown>): void
  disableAutoSelect(): void
}
declare global { interface Window { google?: { accounts?: { id?: GoogleIdApi } } } }

const SCRIPT = 'https://accounts.google.com/gsi/client'
let loading: Promise<GoogleIdApi> | null = null

/** Loads Google's script once. */
export function loadGoogle(): Promise<GoogleIdApi> {
  if (window.google?.accounts?.id) return Promise.resolve(window.google.accounts.id)
  loading ??= new Promise<GoogleIdApi>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = SCRIPT
    script.async = true
    script.defer = true
    script.onload = () => (window.google?.accounts?.id ? resolve(window.google.accounts.id) : reject(new Error('Google n’a pas pu être chargé.')))
    script.onerror = () => { loading = null; reject(new Error('Google n’a pas pu être chargé.')) }
    document.head.appendChild(script)
  })
  return loading
}

/**
 * Draws Google's button in `parent` for one sign-in: `nonce` comes from the server (POST /auth/google/nonce) and
 * `onCredential` receives the ID token to send back with it.
 */
export async function renderGoogleButton(parent: HTMLElement, opts: { clientId: string; nonce: string; text?: 'continue_with' | 'signin_with'; onCredential: (credential: string) => void }) {
  const api = await loadGoogle()
  api.initialize({
    client_id: opts.clientId,
    nonce: opts.nonce,
    callback: (res: GoogleCredentialResponse) => { if (res.credential) opts.onCredential(res.credential) },
    ux_mode: 'popup',
    auto_select: false,
    cancel_on_tap_outside: true,
    itp_support: true,
    use_fedcm_for_button: true,
  })
  parent.replaceChildren()
  api.renderButton(parent, {
    type: 'standard', theme: isDark() ? 'filled_black' : 'outline', size: 'large', shape: 'pill', text: opts.text ?? 'continue_with',
    logo_alignment: 'left', locale: 'fr', width: Math.min(400, Math.max(220, parent.clientWidth || 320)),
  })
}

/** After signing out: Google does not sign the person straight back in. */
export function forgetGoogleSelection() {
  try { window.google?.accounts?.id?.disableAutoSelect() } catch { /* not loaded */ }
}
