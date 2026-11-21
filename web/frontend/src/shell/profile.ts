import { useEffect, useState } from 'react'
import { sellerApi, type SellerOrganization } from '@/api/seller'
import { createLocalStore } from '@/lib/localStore'

/*
 * Who the person acts as: themselves, or one of the real organizations of their account (GET /sellers/me). The choice
 * is kept on this device; the backend still derives identity from the access token only.
 */

export interface Profile { kind: 'personal' | 'company'; organizationId: string }

export const profileStore = createLocalStore<Profile>('profile', { kind: 'personal', organizationId: '' })

export function useOrganizations(signedIn: boolean): { orgs: SellerOrganization[]; loaded: boolean; failed: boolean } {
  const [orgs, setOrgs] = useState<SellerOrganization[]>([])
  const [loaded, setLoaded] = useState(false)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    if (!signedIn) { setOrgs([]); setLoaded(true); return }
    let alive = true
    setLoaded(false)
    sellerApi.context()
      .then((list) => { if (alive) { setOrgs(list); setFailed(false) } })
      .catch(() => { if (alive) { setOrgs([]); setFailed(true) } })
      .finally(() => { if (alive) setLoaded(true) })
    return () => { alive = false }
  }, [signedIn])
  return { orgs, loaded, failed }
}
