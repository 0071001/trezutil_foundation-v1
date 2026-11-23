import { http, tokens } from '@/api/client'
import { forgetGoogleSelection } from '@/lib/google'
import { arr, obj, read, str, strOrNull } from '@/api/json'
import type { Session, User } from '@/types'

export function mapUser(value: unknown): User {
  const raw = obj(value)
  return {
    id: str(read(raw, 'id', 'ID')),
    phone: str(read(raw, 'phone', 'Phone')),
    email: strOrNull(read(raw, 'email', 'Email')),
    display_name: str(read(raw, 'display_name', 'DisplayName')),
    status: str(read(raw, 'status', 'Status')),
    roles: arr(read(raw, 'roles', 'Roles')).map(str).filter(Boolean),
    created_at: str(read(raw, 'created_at', 'CreatedAt')),
    verified_at: strOrNull(read(raw, 'verified_at', 'VerifiedAt')),
  }
}

export interface OtpChallenge {
  request_id: string
  expires_at: string
}

/** Phone-code sign-in (the only authentication method). */
export const authApi = {
  requestCode(phone: string): Promise<OtpChallenge> {
    return http.post<OtpChallenge>('/auth/request-otp', { phone })
  },

  /** Verifies the code and stores the session; returns the signed-in user. */
  async verifyCode(requestId: string, code: string): Promise<User> {
    const data = await http.post<Session>('/auth/verify-otp', { request_id: requestId, code })
    tokens.save(data)
    return mapUser(data.user)
  },

  async me(): Promise<User> {
    return mapUser(await http.get<unknown>('/auth/users/me'))
  },

  /** PATCH /auth/users/me: the display name and the e-mail address (the phone is the identity and does not change here). */
  async updateMe(patch: { display_name?: string; email?: string }): Promise<User> {
    return mapUser(await http.patch<unknown>('/auth/users/me', patch))
  },

  /** Which sign-in methods the server offers; Google's public client id when it is on. */
  async providers(): Promise<{ google: { enabled: boolean; clientId: string } }> {
    const raw = obj(await http.get<unknown>('/auth/providers'))
    const g = obj(raw.google)
    return { google: { enabled: g.enabled === true && !!str(g.client_id), clientId: str(g.client_id) } }
  },

  /** A one-time nonce for the next Google sign-in (the server checks it is in the ID token). */
  async googleNonce(): Promise<string> {
    return str(obj(await http.post<unknown>('/auth/google/nonce', {})).nonce)
  },

  /** Signs in (or creates the account) with a Google ID token and stores the session. */
  async signInWithGoogle(credential: string, nonce: string): Promise<User> {
    const data = await http.post<Session>('/auth/google', { credential, nonce })
    tokens.save(data)
    return mapUser(data.user)
  },

  /** The signed-in account's sign-in methods. */
  async identities(): Promise<{ phone: string; googleEnabled: boolean; google?: { email: string; name: string } }> {
    const raw = obj(await http.get<unknown>('/auth/identities'))
    const g = arr(raw.items).map(obj).find((i) => str(i.provider) === 'google')
    return { phone: str(raw.phone), googleEnabled: raw.google_enabled === true, ...(g ? { google: { email: str(g.email), name: str(g.name) } } : {}) }
  },

  /** Links a Google account to the signed-in account (a fresh Google sign-in proves it). */
  async linkGoogle(credential: string, nonce: string): Promise<void> {
    await http.post<unknown>('/auth/google/link', { credential, nonce })
  },

  async unlinkGoogle(): Promise<void> {
    await http.del<unknown>('/auth/identities/google')
  },

  /**
   * Signs out on this device. The backend issues stateless refresh tokens and
   * has no revocation endpoint, so this clears the stored tokens only.
   */
  signOut() {
    tokens.clear()
    forgetGoogleSelection()
  },
}
