import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { authApi } from '@/api/auth'
import { SESSION_CHANGED_EVENT, tokens } from '@/api/client'
import { resetConversationChannel } from '@/api/conversations'
import type { User } from '@/types'

interface SessionState {
  /** undefined while the stored session is being checked. */
  user: User | null | undefined
  signedIn: boolean
  /** True after an explicit sign-out (protected pages then go home, not to sign-in). */
  signedOut: boolean
  refreshUser: () => Promise<void>
  signOut: () => void
}

const SessionContext = createContext<SessionState | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null | undefined>(() => (tokens.access() ? undefined : null))
  const [signedOut, setSignedOut] = useState(false)

  const refreshUser = useCallback(async () => {
    if (!tokens.access()) { setUser(null); return }
    try {
      setUser(await authApi.me())
      setSignedOut(false)
    } catch {
      // An expired session is renewed or cleared by the HTTP client.
      setUser(null)
    }
  }, [])

  useEffect(() => { void refreshUser() }, [refreshUser])

  useEffect(() => {
    const onChange = () => { if (!tokens.access()) setUser(null) }
    window.addEventListener(SESSION_CHANGED_EVENT, onChange)
    // Another tab signed in or out.
    const onStorage = (e: StorageEvent) => { if (e.key?.startsWith('trezutil_')) void refreshUser() }
    window.addEventListener('storage', onStorage)
    return () => {
      window.removeEventListener(SESSION_CHANGED_EVENT, onChange)
      window.removeEventListener('storage', onStorage)
    }
  }, [refreshUser])

  const signOut = useCallback(() => {
    authApi.signOut()
    try { localStorage.removeItem('trezutil_cart') } catch { /* ignore */ }
    window.dispatchEvent(new StorageEvent('storage', { key: 'trezutil_cart' }))
    resetConversationChannel()
    try { sessionStorage.removeItem('trezutil_thread') } catch { /* ignore */ }
    setSignedOut(true)
    setUser(null)
  }, [])

  const value = useMemo(() => ({ user, signedIn: !!user, signedOut, refreshUser, signOut }), [user, signedOut, refreshUser, signOut])
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): SessionState {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession outside SessionProvider')
  return ctx
}

/** Only same-site relative paths are accepted as post-login destinations. */
export function safeReturnTo(value: string | null | undefined, fallback = '/'): string {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return fallback
  if (value.startsWith('/login') || value.startsWith('/register')) return fallback
  return value
}
