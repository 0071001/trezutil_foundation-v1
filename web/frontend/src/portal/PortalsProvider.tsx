import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { toApiError, type ApiError } from '@/api/client'
import { sellerApi, type SellerOrganization } from '@/api/seller'
import { useSession } from '@/lib/session'
import { portalsFor, type PortalKind } from '@/portal/portals'
import type { User } from '@/types'

/**
 * Which portals the signed-in person can open.
 *
 * The businesses a user belongs to come from `GET /sellers/me`. The call is
 * made once per session and only when a portal shell asks for it, so the
 * public site and the customer pages never pay for it. Signing out (or another
 * account signing in) drops everything: no portal data survives a change of
 * person.
 */
export type OrgsStatus = 'idle' | 'loading' | 'ready' | 'error'

export interface PortalsState {
  /** undefined while the stored session is being checked; null for a guest. */
  user: User | null | undefined
  orgs: SellerOrganization[]
  orgsStatus: OrgsStatus
  orgsError: ApiError | null
  /** Portals held, in switcher order; empty for a guest. Advisory: the server decides. */
  portals: PortalKind[]
  reload: () => void
  /** Called by portal shells; loads the businesses once. */
  request: () => void
}

const PortalsContext = createContext<PortalsState | null>(null)

interface Loaded {
  userId: string
  status: OrgsStatus
  orgs: SellerOrganization[]
  error: ApiError | null
}

export function PortalsProvider({ children }: { children: ReactNode }) {
  const { user } = useSession()
  const [wanted, setWanted] = useState(false)
  const [nonce, setNonce] = useState(0)
  const [loaded, setLoaded] = useState<Loaded | null>(null)
  const userId = user?.id ?? ''

  useEffect(() => {
    if (!userId || !wanted) return
    let live = true
    // A reload keeps what is on screen (same person) so the workspace does not flash.
    setLoaded((prev) => ({ userId, status: 'loading', orgs: prev?.userId === userId ? prev.orgs : [], error: null }))
    sellerApi.context()
      .then((orgs) => { if (live) setLoaded({ userId, status: 'ready', orgs, error: null }) })
      .catch((err: unknown) => { if (live) setLoaded({ userId, status: 'error', orgs: [], error: toApiError(err) }) })
    return () => { live = false }
  }, [userId, wanted, nonce])

  // Another person, or nobody: never show the previous person's businesses.
  const current = loaded && loaded.userId === userId && userId ? loaded : null
  const orgs = useMemo(() => current?.orgs ?? [], [current])
  const reload = useCallback(() => setNonce((n) => n + 1), [])
  const request = useCallback(() => setWanted(true), [])

  const value = useMemo<PortalsState>(() => ({
    user,
    orgs,
    orgsStatus: current?.status ?? 'idle',
    orgsError: current?.error ?? null,
    portals: portalsFor(user, orgs),
    reload,
    request,
  }), [user, orgs, current, reload, request])

  return <PortalsContext.Provider value={value}>{children}</PortalsContext.Provider>
}

/**
 * Portal resolution for the signed-in user. Calling it asks for the businesses
 * to be loaded (once per session); pass `false` to read without asking, for
 * example while the caller already knows the person may not enter.
 */
export function usePortals(load = true): PortalsState {
  const ctx = useContext(PortalsContext)
  if (!ctx) throw new Error('usePortals outside PortalsProvider')
  const { request } = ctx
  useEffect(() => { if (load) request() }, [load, request])
  return ctx
}
