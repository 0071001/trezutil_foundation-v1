import { http, tokens } from '@/api/client'
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

  /**
   * Signs out on this device. The backend issues stateless refresh tokens and
   * has no revocation endpoint, so this clears the stored tokens only.
   */
  signOut() {
    tokens.clear()
  },
}
