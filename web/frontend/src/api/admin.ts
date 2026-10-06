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

export interface AuditEvent {
  id: number
  at: string
  actorId: string
  /** The actor's display name; empty for the system or an account that no longer exists. */
  actorName: string
  action: string
  targetType: string
  targetId: string
  requestId: string
  metadata: Record<string, unknown>
}

export interface AuditPage {
  items: AuditEvent[]
  total: number
  page: number
  pageSize: number
}

export interface AuditFilters {
  /** The start of the action name: "order." lists every order event. */
  action?: string
  actorId?: string
  targetType?: string
  targetId?: string
  /** Plain dates (2026-10-05); "to" includes the whole day. */
  from?: string
  to?: string
}

export function mapAuditEvent(value: unknown): AuditEvent {
  const raw = obj(value)
  return {
    id: num(raw.id),
    at: str(raw.at),
    actorId: str(raw.actor_id),
    actorName: str(raw.actor_name),
    action: str(raw.action),
    targetType: str(raw.target_type),
    targetId: str(raw.target_id),
    requestId: str(raw.request_id),
    metadata: obj(raw.metadata),
  }
}

export interface AdminPage<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface AdminListQuery {
  q?: string
  status?: string
  page?: number
  pageSize?: number
}

export interface AdminOrder {
  id: string
  status: string
  totalMinor: number
  currency: string
  title: string
  itemCount: number
  organizationId: string
  organizationName: string
  customerId: string
  customerName: string
  createdAt: string
  updatedAt: string
}

export interface AdminReservation {
  id: string
  status: string
  title: string
  listingId: string
  organizationId: string
  organizationName: string
  customerId: string
  customerName: string
  startAt: string
  endAt: string
  quantity: number
  partySize: number
  createdAt: string
}

export interface AdminOrganization {
  id: string
  displayName: string
  type: string
  status: string
  verificationStatus: string
  city: string
  country: string
  members: number
  stores: number
  publishedListings: number
  orders: number
  createdAt: string
}

export function mapAdminOrder(value: unknown): AdminOrder {
  const raw = obj(value)
  return {
    id: str(raw.id), status: str(raw.status), totalMinor: num(raw.total_minor), currency: str(raw.currency), title: str(raw.title), itemCount: num(raw.item_count),
    organizationId: str(raw.organization_id), organizationName: str(raw.organization_name), customerId: str(raw.customer_id), customerName: str(raw.customer_name),
    createdAt: str(raw.created_at), updatedAt: str(raw.updated_at),
  }
}

export function mapAdminReservation(value: unknown): AdminReservation {
  const raw = obj(value)
  return {
    id: str(raw.id), status: str(raw.status), title: str(raw.title), listingId: str(raw.listing_id),
    organizationId: str(raw.organization_id), organizationName: str(raw.organization_name), customerId: str(raw.customer_id), customerName: str(raw.customer_name),
    startAt: str(raw.start_at), endAt: str(raw.end_at), quantity: num(raw.quantity), partySize: num(raw.party_size), createdAt: str(raw.created_at),
  }
}

export function mapAdminOrganization(value: unknown): AdminOrganization {
  const raw = obj(value)
  return {
    id: str(raw.id), displayName: str(raw.display_name), type: str(raw.type), status: str(raw.status), verificationStatus: str(raw.verification_status),
    city: str(raw.city), country: str(raw.country), members: num(raw.members), stores: num(raw.stores), publishedListings: num(raw.published_listings),
    orders: num(raw.orders), createdAt: str(raw.created_at),
  }
}

async function adminList<T>(path: string, params: AdminListQuery, map: (v: unknown) => T): Promise<AdminPage<T>> {
  const raw = obj(await http.get<unknown>(`${path}${query({ q: params.q, status: params.status, page: params.page, page_size: params.pageSize })}`))
  return { items: arr(raw.items).map(map), total: num(raw.total), page: num(raw.page) || 1, pageSize: num(raw.page_size) || 25 }
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

  /** Every order on the platform, newest first (administrators and operators). Names and states, no contact data. */
  orders: (params: AdminListQuery = {}) => adminList('/admin/orders', params, mapAdminOrder),
  /** Every booking on the platform, newest first. */
  reservations: (params: AdminListQuery = {}) => adminList('/admin/reservations', params, mapAdminReservation),
  /** Every business on the platform, newest first, with its counts. */
  organizations: (params: AdminListQuery = {}) => adminList('/admin/organizations', params, mapAdminOrganization),

  /** The durable audit trail, newest first. Filters narrow it; nothing here can change an entry. */
  async audit(filters: AuditFilters = {}, page = 1, pageSize = 50): Promise<AuditPage> {
    const raw = obj(await http.get<unknown>(`/admin/audit${query({
      action: filters.action, actor_id: filters.actorId, target_type: filters.targetType, target_id: filters.targetId,
      from: filters.from, to: filters.to, page, page_size: pageSize,
    })}`))
    return { items: arr(raw.items).map(mapAuditEvent), total: num(raw.total), page: num(raw.page) || 1, pageSize: num(raw.page_size) || pageSize }
  },

  /** Suspends an account (administrators only). The reason, 3 to 300 characters, goes to the audit trail. */
  async suspendUser(id: string, reason: string): Promise<void> {
    await http.post(`/admin/users/${encodeURIComponent(id)}/suspend`, { reason })
  },

  /** Gives a suspended account back. */
  async reactivateUser(id: string, reason = ''): Promise<void> {
    await http.post(`/admin/users/${encodeURIComponent(id)}/reactivate`, reason ? { reason } : {})
  },

  async users(params: { q?: string; page?: number; pageSize?: number } = {}): Promise<AdminUserPage> {
    const raw = obj(await http.get<unknown>(`/admin/users${query({ q: params.q, page: params.page, page_size: params.pageSize })}`))
    return { items: arr(raw.items).map(mapAdminUser), total: num(raw.total), page: num(raw.page) || 1, pageSize: num(raw.page_size) || 25 }
  },
}
