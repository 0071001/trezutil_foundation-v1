import { mapUser } from '@/api/auth'
import { http, query, tokens } from '@/api/client'
import { arr, num, obj, read, str } from '@/api/json'
import type { Session, User } from '@/types'

/* Platform administration: staff sign in with an email and a password, and the
   console reads the platform's headline numbers and accounts. */

/** Roles that can open the administration area. */
export const STAFF_ROLES = ['admin', 'operator', 'moderator'] as const

export const isStaff = (user: User | null | undefined): boolean => !!user && user.roles.some((r) => (STAFF_ROLES as readonly string[]).includes(r))
export const isAdmin = (user: User | null | undefined): boolean => !!user && user.roles.includes('admin')
/** The overview is open to operators too; the account list (personal data) is for administrators. */
export const canSeeOverview = (user: User | null | undefined): boolean => !!user && (user.roles.includes('admin') || user.roles.includes('operator'))

export interface AdminOverview {
  generatedAt: string
  users: { total: number; active: number; suspended: number; newLast7Days: number; byRole: Record<string, number> }
  organizations: number
  listingsByStatus: Record<string, number>
  orders: { total: number; last7Days: number }
  reservations: number
  conversationsLast7Days: number
}

export interface AdminUser {
  id: string
  email: string
  phone: string
  displayName: string
  status: string
  roles: string[]
  createdAt: string
  lastLoginAt: string
}

export interface AdminUserPage {
  items: AdminUser[]
  total: number
  page: number
  pageSize: number
}

function counts(value: unknown): Record<string, number> {
  const out: Record<string, number> = {}
  for (const [k, v] of Object.entries(obj(value))) out[k] = num(v)
  return out
}

export function mapOverview(value: unknown): AdminOverview {
  const raw = obj(value)
  const users = obj(raw.users)
  const orders = obj(raw.orders)
  return {
    generatedAt: str(raw.generated_at),
    users: { total: num(users.total), active: num(users.active), suspended: num(users.suspended), newLast7Days: num(users.new_last_7_days), byRole: counts(users.by_role) },
    organizations: num(raw.organizations),
    listingsByStatus: counts(raw.listings_by_status),
    orders: { total: num(orders.total), last7Days: num(orders.last_7_days) },
    reservations: num(raw.reservations),
    conversationsLast7Days: num(obj(raw.conversations).last_7_days),
  }
}

export function mapAdminUser(value: unknown): AdminUser {
  const raw = obj(value)
  return {
    id: str(read(raw, 'id')),
    email: str(raw.email),
    phone: str(raw.phone),
    displayName: str(raw.display_name),
    status: str(raw.status),
    roles: arr(raw.roles).map(str).filter(Boolean),
    createdAt: str(raw.created_at),
    lastLoginAt: str(raw.last_login_at),
  }
}

export const adminApi = {
  /** Signs a staff member in with email and password and stores the session. */
  async login(email: string, password: string): Promise<User> {
    const data = await http.post<Session>('/auth/staff/login', { email: email.trim(), password })
    tokens.save(data)
    return mapUser(data.user)
  },

  async overview(): Promise<AdminOverview> {
    return mapOverview(await http.get<unknown>('/admin/overview'))
  },

  async users(params: { q?: string; page?: number; pageSize?: number } = {}): Promise<AdminUserPage> {
    const raw = obj(await http.get<unknown>(`/admin/users${query({ q: params.q, page: params.page, page_size: params.pageSize })}`))
    return { items: arr(raw.items).map(mapAdminUser), total: num(raw.total), page: num(raw.page) || 1, pageSize: num(raw.page_size) || 25 }
  },
}
