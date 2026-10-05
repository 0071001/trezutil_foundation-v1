import { describe, expect, it } from 'vitest'
import { gaps } from '@/portal/gaps'
import { adminGroups, adminModules, allows, sellerGroups, sellerModules, shellGroups, type PortalModule } from '@/portal/modules'
import { portalsFor } from '@/portal/portals'
import type { User } from '@/types'

const user = (roles: string[]): User => ({ id: 'u1', phone: '+243810000001', display_name: 'Test', status: 'active', roles, created_at: '2026-01-01T00:00:00Z' }) as User

describe.each([
  ['seller', sellerModules, sellerGroups, '/seller'],
  ['admin', adminModules, adminGroups, '/admin'],
] as const)('%s module registry', (_name, modules: PortalModule[], groups, prefix) => {
  it('gives every module a unique id and a unique route under the portal', () => {
    expect(new Set(modules.map((m) => m.id)).size).toBe(modules.length)
    expect(new Set(modules.map((m) => m.to)).size).toBe(modules.length)
    for (const m of modules) expect(m.to === prefix || m.to.startsWith(`${prefix}/`), m.to).toBe(true)
  })

  it('puts every module in a declared group, and declares no empty group', () => {
    const ids = new Set(groups.map((g) => g.id))
    for (const m of modules) expect(ids.has(m.group), `${m.id} → ${m.group}`).toBe(true)
    for (const g of groups) expect(modules.some((m) => m.group === g.id), g.id).toBe(true)
  })

  it('says what every module is for', () => {
    for (const m of modules) expect(m.purpose.length, m.id).toBeGreaterThan(10)
  })

  it('cites only gaps that are defined', () => {
    for (const m of modules) {
      const cited = m.status.kind === 'unavailable' ? m.status.gaps : m.status.kind === 'planned' ? (m.status.limits ?? []) : []
      for (const id of cited) expect(gaps, `${m.id} cites ${id}`).toHaveProperty(id)
      if (m.status.kind === 'unavailable') expect(m.status.gaps.length, `${m.id} names what is missing`).toBeGreaterThan(0)
    }
  })

  it('lists the endpoints a planned module will use, and only planned modules', () => {
    for (const m of modules) {
      if (m.status.kind === 'planned') expect(m.status.endpoints.length, m.id).toBeGreaterThan(0)
    }
  })

  it('keeps one ready dashboard at the portal root', () => {
    const root = modules.find((m) => m.to === prefix)
    expect(root?.status.kind).toBe('ready')
    expect(root?.end).toBe(true)
  })
})

describe('every defined gap is cited by a module', () => {
  it('has no orphan gap id', () => {
    const cited = new Set<string>()
    for (const m of [...sellerModules, ...adminModules]) {
      if (m.status.kind === 'unavailable') m.status.gaps.forEach((g) => cited.add(g))
      if (m.status.kind === 'planned') (m.status.limits ?? []).forEach((g) => cited.add(g))
    }
    expect(Object.keys(gaps).filter((id) => !cited.has(id))).toEqual([])
  })
})

describe('access convenience (the server decides; this only shapes menus)', () => {
  it('seller: members see everything, managers can operate', () => {
    expect(allows('member', {})).toBe(true)
    expect(allows('manager', { canManage: false })).toBe(false)
    expect(allows('manager', { canManage: true })).toBe(true)
  })

  it('admin: operator covers admin and operator, "admin" only the administrator', () => {
    expect(allows('operator', { roles: ['admin'] })).toBe(true)
    expect(allows('operator', { roles: ['operator'] })).toBe(true)
    expect(allows('operator', { roles: ['moderator'] })).toBe(false)
    expect(allows('operator', { roles: ['customer'] })).toBe(false)
    expect(allows('operator', {})).toBe(false)
    expect(allows('admin', { roles: ['operator'] })).toBe(false)
    expect(allows('admin', { roles: ['admin'] })).toBe(true)
  })

  it('seller rail lists every module but marks manager-only ones restricted for members', () => {
    const member = shellGroups('seller', { canManage: false }).flatMap((g) => g.items)
    const manager = shellGroups('seller', { canManage: true }).flatMap((g) => g.items)
    expect(member.map((i) => i.id)).toEqual(sellerModules.map((m) => m.id))
    expect(manager.some((i) => i.restricted)).toBe(false)
    const restricted = member.filter((i) => i.restricted).map((i) => i.id)
    expect(restricted).toEqual(sellerModules.filter((m) => m.requires === 'manager').map((m) => m.id))
    expect(restricted).toContain('finance')
    expect(restricted).not.toContain('listings')
  })

  it('admin rail drops what the role cannot use', () => {
    const ids = (roles: string[]) => shellGroups('admin', { roles }).flatMap((g) => g.items).map((i) => i.id)
    expect(ids(['admin'])).toContain('users')
    expect(ids(['operator'])).not.toContain('users')
    expect(ids(['operator'])).toContain('overview')
    expect(ids(['moderator'])).toEqual([])
    expect(ids(['customer'])).toEqual([])
    expect(shellGroups('admin', { roles: ['moderator'] })).toEqual([])
  })

  it('marks each rail item with its real status', () => {
    const items = shellGroups('admin', { roles: ['admin'] }).flatMap((g) => g.items)
    for (const item of items) expect(item.status).toBe(adminModules.find((m) => m.id === item.id)!.status.kind)
  })
})

describe('which portals a person holds', () => {
  const org = [{ organization_id: 'o1' }]

  it('gives a guest none', () => {
    expect(portalsFor(null, org)).toEqual([])
    expect(portalsFor(undefined, [])).toEqual([])
  })

  it('gives a customer only the customer portal', () => {
    expect(portalsFor(user(['customer']), [])).toEqual(['customer'])
  })

  it('adds the seller portal for a business member, whatever the platform role', () => {
    expect(portalsFor(user(['customer']), org)).toEqual(['customer', 'seller'])
  })

  it('adds administration for staff roles only', () => {
    expect(portalsFor(user(['admin']), [])).toEqual(['customer', 'admin'])
    expect(portalsFor(user(['operator']), [])).toEqual(['customer', 'admin'])
    expect(portalsFor(user(['moderator']), [])).toEqual(['customer', 'admin'])
    expect(portalsFor(user(['seller']), [])).toEqual(['customer'])
    expect(portalsFor(user(['delivery_partner']), [])).toEqual(['customer'])
  })

  it('orders them customer, seller, admin', () => {
    expect(portalsFor(user(['admin']), org)).toEqual(['customer', 'seller', 'admin'])
  })
})
