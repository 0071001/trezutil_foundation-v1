import { isStaff } from '@/api/admin'
import { tx } from '@/lib/i18n'
import type { User } from '@/types'

/**
 * The three portals. Which ones a person can open is derived from two reads
 * the platform already offers (GET /auth/users/me → roles, GET /sellers/me →
 * business memberships). The result chooses menus and landing pages; it grants
 * nothing — every request is authorised again by the server.
 */
export type PortalKind = 'customer' | 'seller' | 'admin'

export interface PortalInfo {
  kind: PortalKind
  to: string
  label: string
  description: string
}

export const portalInfo: Record<PortalKind, PortalInfo> = {
  customer: { kind: 'customer', to: '/', label: tx('Trezutil'), description: tx('Discover, buy, book and follow your activity') },
  seller: { kind: 'seller', to: '/seller', label: tx('Seller workspace'), description: tx('Run your business on Trezutil') },
  admin: { kind: 'admin', to: '/admin', label: tx('Administration'), description: tx('Operate and govern Trezutil') },
}

/** The portals a signed-in person holds, in switcher order. A guest holds none. */
export function portalsFor(user: User | null | undefined, organizations: readonly unknown[]): PortalKind[] {
  if (!user) return []
  const out: PortalKind[] = ['customer']
  if (organizations.length > 0) out.push('seller')
  if (isStaff(user)) out.push('admin')
  return out
}
