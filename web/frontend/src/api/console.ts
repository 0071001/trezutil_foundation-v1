import { http, query } from '@/api/client'
import { arr, num, obj, str, type Json } from '@/api/json'

/*
 * The seller workspace's money, customers, fulfilment, availability, usage, integrations and team, and the
 * administration console's listings, payments, operations, providers, channels, security, AI, monetization and
 * health. Every call is authorised by the server; nothing here decides who may see what.
 */

export interface Paged<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

function paged<T>(value: unknown, map: (v: unknown) => T): Paged<T> {
  const raw = obj(value)
  return { items: arr(raw.items).map(map), total: num(raw.total), page: num(raw.page) || 1, pageSize: num(raw.page_size) || 25 }
}

const bool = (v: unknown) => v === true
const counts = (v: unknown): Record<string, number> => Object.fromEntries(Object.entries(obj(v)).map(([k, n]) => [k, num(n)]))
const strings = (v: unknown): string[] => arr(v).map(str).filter(Boolean)

// --- money ---------------------------------------------------------------------------------------------------

export interface Payment {
  id: string
  orderId: string
  amountMinor: number
  currency: string
  method: string
  provider: string
  status: string
  refundedMinor: number
  failureReason: string
  createdAt: string
  capturedAt: string
  organizationId: string
  organizationName: string
}

export interface MoneyTotals {
  currency: string
  capturedMinor: number
  refundedMinor: number
  netMinor: number
  payments: number
  failed: number
  earnedMinor: number
  orders: number
}

export interface Finance {
  days: number
  from: string
  to: string
  totals: MoneyTotals[]
  payments: Paged<Payment>
}

export function mapPayment(v: unknown): Payment {
  const r = obj(v)
  return {
    id: str(r.id), orderId: str(r.order_id), amountMinor: num(r.amount_minor), currency: str(r.currency), method: str(r.method), provider: str(r.provider),
    status: str(r.status), refundedMinor: num(r.refunded_minor), failureReason: str(r.failure_reason), createdAt: str(r.created_at), capturedAt: str(r.captured_at),
    organizationId: str(r.organization_id), organizationName: str(r.organization_name),
  }
}

function mapTotals(v: unknown): MoneyTotals {
  const r = obj(v)
  return {
    currency: str(r.currency), capturedMinor: num(r.captured_minor), refundedMinor: num(r.refunded_minor), netMinor: num(r.net_minor),
    payments: num(r.payments), failed: num(r.failed), earnedMinor: num(r.earned_minor), orders: num(r.orders),
  }
}

export function mapFinance(v: unknown): Finance {
  const r = obj(v)
  return { days: num(r.days), from: str(r.from), to: str(r.to), totals: arr(r.totals).map(mapTotals), payments: paged(r.payments, mapPayment) }
}

// --- customers, fulfilment, availability ---------------------------------------------------------------------

export interface Customer {
  customerId: string
  displayName: string
  orders: number
  reservations: number
  spent: Record<string, number>
  firstAt: string
  lastAt: string
}

export function mapCustomer(v: unknown): Customer {
  const r = obj(v)
  return { customerId: str(r.customer_id), displayName: str(r.display_name), orders: num(r.orders), reservations: num(r.reservations), spent: counts(r.spent), firstAt: str(r.first_at), lastAt: str(r.last_at) }
}

export interface Fulfilment {
  id: string
  reference: string
  type: string
  status: string
  orderId: string
  reservationId: string
  title: string
  orderStatus: string
  customerName: string
  trackingRef: string
  assignedTo: string
  assignedName: string
  createdAt: string
  updatedAt: string
}

export function mapFulfilment(v: unknown): Fulfilment {
  const r = obj(v)
  return {
    id: str(r.id), reference: str(r.reference), type: str(r.type), status: str(r.status), orderId: str(r.order_id), reservationId: str(r.reservation_id),
    title: str(r.title), orderStatus: str(r.order_status), customerName: str(r.customer_name), trackingRef: str(r.tracking_ref),
    assignedTo: str(r.assigned_to), assignedName: str(r.assigned_name), createdAt: str(r.created_at), updatedAt: str(r.updated_at),
  }
}

/** The next step of a fulfilment, by status (the server's table: PENDING → ACCEPTED → IN_PROGRESS → READY → COMPLETED). */
export const FULFILMENT_NEXT: Record<string, string> = { PENDING: 'ACCEPTED', ACCEPTED: 'IN_PROGRESS', IN_PROGRESS: 'READY', READY: 'COMPLETED' }
/** Where a fulfilment can be stopped from. */
export const FULFILMENT_STOP: Record<string, 'CANCELLED' | 'FAILED'> = { PENDING: 'CANCELLED', ACCEPTED: 'CANCELLED', IN_PROGRESS: 'FAILED', READY: 'CANCELLED' }

export interface CapacitySlot {
  id: string
  startAt: string
  endAt: string
  capacity: number
  reserved: number
}

export interface CapacityResource {
  inventoryId: string
  listingId: string
  listingTitle: string
  listingType: string
  resourceType: string
  total: number
  reserved: number
  consumed: number
  windows: CapacitySlot[]
}

export interface Capacity {
  from: string
  days: number
  items: CapacityResource[]
}

export function mapCapacity(v: unknown): Capacity {
  const r = obj(v)
  return {
    from: str(r.from), days: num(r.days),
    items: arr(r.items).map((x) => {
      const c = obj(x)
      return {
        inventoryId: str(c.inventory_id), listingId: str(c.listing_id), listingTitle: str(c.listing_title), listingType: str(c.listing_type),
        resourceType: str(c.resource_type), total: num(c.total), reserved: num(c.reserved), consumed: num(c.consumed),
        windows: arr(c.windows).map((w) => { const s = obj(w); return { id: str(s.id), startAt: str(s.start_at), endAt: str(s.end_at), capacity: num(s.capacity), reserved: num(s.reserved) } }),
      }
    }),
  }
}

// --- usage, plans, integrations ------------------------------------------------------------------------------

export interface Subscription {
  id: string
  organizationName: string
  status: string
  periodId: string
  cancelAtPeriodEnd: boolean
  startedAt: string
  plan: { id: string; name: string; slug: string; priceMinor: number; currency: string; billingCycle: string; quota: number }
}

export function mapSubscription(v: unknown): Subscription | null {
  if (!v) return null
  const r = obj(v)
  const p = obj(r.plan)
  return {
    id: str(r.id), organizationName: str(r.organization_name), status: str(r.status), periodId: str(r.period_id), cancelAtPeriodEnd: bool(r.cancel_at_period_end), startedAt: str(r.started_at),
    plan: { id: str(p.id), name: str(p.name), slug: str(p.slug), priceMinor: num(p.price_minor), currency: str(p.currency), billingCycle: str(p.billing_cycle), quota: num(p.quota) },
  }
}

export interface UsagePolicy {
  actionType: string
  channel: string
  actorType: string
  cost: number
  active: boolean
}

const mapPolicy = (v: unknown): UsagePolicy => {
  const r = obj(v)
  return { actionType: str(r.action_type), channel: str(r.channel), actorType: str(r.actor_type), cost: num(r.cost), active: bool(r.active) }
}

export interface Usage {
  accounts: { accountId: string; ownerType: string; periodId: string; granted: number; reserved: number; consumed: number; balance: number; updatedAt: string }[]
  ledger: { id: string; actionType: string; kind: string; amount: number; balanceAfter: number; periodId: string; createdAt: string }[]
  subscription: Subscription | null
  policies: UsagePolicy[]
}

export function mapUsage(v: unknown): Usage {
  const r = obj(v)
  return {
    accounts: arr(r.accounts).map((x) => {
      const a = obj(x)
      return { accountId: str(a.account_id), ownerType: str(a.owner_type), periodId: str(a.period_id), granted: num(a.granted), reserved: num(a.reserved), consumed: num(a.consumed), balance: num(a.balance), updatedAt: str(a.updated_at) }
    }),
    ledger: arr(r.ledger).map((x) => {
      const e = obj(x)
      return { id: str(e.id), actionType: str(e.action_type), kind: str(e.kind), amount: num(e.amount), balanceAfter: num(e.balance_after), periodId: str(e.period_id), createdAt: str(e.created_at) }
    }),
    subscription: mapSubscription(r.subscription),
    policies: arr(r.policies).map(mapPolicy),
  }
}

export interface Provider {
  id: string
  organizationId: string
  organizationName: string
  code: string
  name: string
  type: string
  status: string
  healthStatus: string
  capabilities: string[]
  priority: number
  updatedAt: string
  attempts: number
  failures: number
  p50Ms: number
  p95Ms: number
  lastError: string
  lastFailureAt: string
}

export function mapProvider(v: unknown): Provider {
  const r = obj(v)
  const e = obj(r.executions)
  return {
    id: str(r.id), organizationId: str(r.organization_id), organizationName: str(r.organization_name), code: str(r.code), name: str(r.name), type: str(r.type),
    status: str(r.status), healthStatus: str(r.health_status), capabilities: strings(r.capabilities), priority: num(r.priority), updatedAt: str(r.updated_at),
    attempts: num(e.attempts), failures: num(e.failures), p50Ms: num(e.p50_ms), p95Ms: num(e.p95_ms), lastError: str(e.last_error), lastFailureAt: str(e.last_failure_at),
  }
}

export interface ChannelAccount {
  id: string
  channel: string
  externalId: string
  status: string
}

// --- team ----------------------------------------------------------------------------------------------------

export interface TeamMember {
  userId: string
  displayName: string
  phone: string
  role: string
  status: string
  joinedAt: string
  you: boolean
}

export interface Invitation {
  id: string
  phone: string
  role: string
  invitedBy: string
  createdAt: string
  expiresAt: string
}

export interface Team {
  members: TeamMember[]
  invitations: Invitation[]
  canManage: boolean
  yourRole: string
}

export function mapTeam(v: unknown): Team {
  const r = obj(v)
  return {
    members: arr(r.members).map((x) => {
      const m = obj(x)
      return { userId: str(m.user_id), displayName: str(m.display_name), phone: str(m.phone), role: str(m.role), status: str(m.status), joinedAt: str(m.joined_at), you: bool(m.you) }
    }),
    invitations: arr(r.invitations).map((x) => {
      const i = obj(x)
      return { id: str(i.id), phone: str(i.phone), role: str(i.role), invitedBy: str(i.invited_by), createdAt: str(i.created_at), expiresAt: str(i.expires_at) }
    }),
    canManage: bool(r.can_manage),
    yourRole: str(r.your_role),
  }
}

/** Roles an invitation or a role change may give (the server refuses owner; only an owner gives admin). */
export const INVITABLE_ROLES = ['admin', 'manager', 'staff', 'accountant', 'sales', 'operations'] as const

// --- business details ----------------------------------------------------------------------------------------

export interface BusinessDetails {
  displayName: string
  legalName: string
  description: string
  phone: string
  email: string
  province: string
  city: string
  address: string
  country: string
}

/** The organization record as GET /organizations returns it (Go field names). */
export function mapBusiness(v: unknown): BusinessDetails {
  const r = obj(v) as Json
  const s = (k: string) => str(r[k])
  return {
    displayName: s('DisplayName'), legalName: s('LegalName'), description: s('Description'), phone: s('Phone'), email: s('Email'),
    province: s('Province'), city: s('City'), address: s('Address'), country: s('Country'),
  }
}

export interface StoreDetails {
  id: string
  name: string
  description: string
  phone: string
  email: string
  province: string
  city: string
  address: string
}

export function mapStore(v: unknown): StoreDetails {
  const r = obj(v) as Json
  const s = (k: string) => str(r[k])
  return { id: s('ID'), name: s('Name'), description: s('Description'), phone: s('Phone'), email: s('Email'), province: s('Province'), city: s('City'), address: s('Address') }
}

// --- administration ------------------------------------------------------------------------------------------

export interface AdminListing {
  id: string
  title: string
  status: string
  vertical: string
  listingType: string
  priceAmount: number
  currency: string
  organizationId: string
  organizationName: string
  category: string
  images: number
  flags: string[]
  createdAt: string
  updatedAt: string
}

export function mapAdminListing(v: unknown): AdminListing {
  const r = obj(v)
  return {
    id: str(r.id), title: str(r.title), status: str(r.status), vertical: str(r.vertical), listingType: str(r.listing_type), priceAmount: num(r.price_amount),
    currency: str(r.currency), organizationId: str(r.organization_id), organizationName: str(r.organization_name), category: str(r.category),
    images: num(r.images), flags: strings(r.flags), createdAt: str(r.created_at), updatedAt: str(r.updated_at),
  }
}

export interface ExceptionRow {
  kind: string
  id: string
  reference: string
  status: string
  detail: string
  business: string
  at: string
}

export interface Operations {
  days: number
  stuckAfterHours: number
  counts: Record<string, number>
  recent: ExceptionRow[]
}

export function mapOperations(v: unknown): Operations {
  const r = obj(v)
  return {
    days: num(r.days), stuckAfterHours: num(r.stuck_after_hours), counts: counts(r.counts),
    recent: arr(r.recent).map((x) => { const e = obj(x); return { kind: str(e.kind), id: str(e.id), reference: str(e.reference), status: str(e.status), detail: str(e.detail), business: str(e.business), at: str(e.at) } }),
  }
}

export interface ChannelHealth {
  channel: string
  events24h: number
  failed24h: number
  pending: number
  lastEventAt: string
  lastFailureAt: string
  lastError: string
  accounts: number
  conversations7d: number
}

export interface Channels {
  items: ChannelHealth[]
  configured: Record<string, boolean>
}

const flags = (v: unknown): Record<string, boolean> => Object.fromEntries(Object.entries(obj(v)).map(([k, b]) => [k, b === true]))

export function mapChannels(v: unknown): Channels {
  const r = obj(v)
  return {
    configured: flags(r.configured),
    items: arr(r.items).map((x) => {
      const c = obj(x)
      return {
        channel: str(c.channel), events24h: num(c.events_24h), failed24h: num(c.failed_24h), pending: num(c.pending), lastEventAt: str(c.last_event_at),
        lastFailureAt: str(c.last_failure_at), lastError: str(c.last_error), accounts: num(c.accounts), conversations7d: num(c.conversations_7d),
      }
    }),
  }
}

export interface Security {
  staff: { userId: string; displayName: string; email: string; roles: string[]; status: string; failedAttempts: number; locked: boolean; lockedUntil: string; lastLoginAt: string; passwordSetAt: string }[]
  events: { id: number; at: string; action: string; actorId: string; actorName: string; targetType: string; targetId: string }[]
}

export function mapSecurity(v: unknown): Security {
  const r = obj(v)
  return {
    staff: arr(r.staff).map((x) => {
      const s = obj(x)
      return {
        userId: str(s.user_id), displayName: str(s.display_name), email: str(s.email), roles: strings(s.roles), status: str(s.status), failedAttempts: num(s.failed_attempts),
        locked: bool(s.locked), lockedUntil: str(s.locked_until), lastLoginAt: str(s.last_login_at), passwordSetAt: str(s.password_set_at),
      }
    }),
    events: arr(r.events).map((x) => {
      const e = obj(x)
      return { id: num(e.id), at: str(e.at), action: str(e.action), actorId: str(e.actor_id), actorName: str(e.actor_name), targetType: str(e.target_type), targetId: str(e.target_id) }
    }),
  }
}

export interface AIActivity {
  days: number
  conversations: number
  activeCustomers: number
  byChannel: Record<string, number>
  byState: Record<string, number>
  messagesIn: number
  messagesOut: number
  confirmations: Record<string, number>
  resultsByType: Record<string, number>
  perDay: { date: string; count: number }[]
  aiExecutionRecords: number
  configured: Record<string, boolean>
}

export function mapAI(v: unknown): AIActivity {
  const r = obj(v)
  return {
    days: num(r.days), conversations: num(r.conversations), activeCustomers: num(r.active_customers), byChannel: counts(r.by_channel), byState: counts(r.by_state),
    messagesIn: num(r.messages_in), messagesOut: num(r.messages_out), confirmations: counts(r.confirmations), resultsByType: counts(r.results_by_type),
    perDay: arr(r.per_day).map((x) => { const d = obj(x); return { date: str(d.date), count: num(d.count) } }),
    aiExecutionRecords: num(r.ai_execution_records), configured: flags(r.configured),
  }
}

export interface Plan {
  id: string
  name: string
  slug: string
  priceMinor: number
  currency: string
  billingCycle: string
  quota: number
  creditPack: number
  status: string
  subscribers: number
}

export interface Monetization {
  days: number
  plans: Plan[]
  subscriptionsByStatus: Record<string, number>
  subscriptions: Subscription[]
  credits: { accounts: number; granted: number; consumed: number; reserved: number }
  ledgerByType: Record<string, number>
  policies: UsagePolicy[]
}

export function mapMonetization(v: unknown): Monetization {
  const r = obj(v)
  const c = obj(r.credits)
  return {
    days: num(r.days),
    plans: arr(r.plans).map((x) => {
      const p = obj(x)
      return { id: str(p.id), name: str(p.name), slug: str(p.slug), priceMinor: num(p.price_minor), currency: str(p.currency), billingCycle: str(p.billing_cycle), quota: num(p.quota), creditPack: num(p.credit_pack), status: str(p.status), subscribers: num(p.subscribers) }
    }),
    subscriptionsByStatus: counts(r.subscriptions_by_status),
    subscriptions: arr(r.subscriptions).map(mapSubscription).filter((s): s is Subscription => s !== null),
    credits: { accounts: num(c.accounts), granted: num(c.granted), consumed: num(c.consumed), reserved: num(c.reserved) },
    ledgerByType: counts(r.ledger_by_type),
    policies: arr(r.policies).map(mapPolicy),
  }
}

export interface HealthComponent {
  name: string
  kind: string
  status: string
  latencyMs: number
  detail: string
}

export interface Health {
  status: string
  checkedAt: string
  components: HealthComponent[]
  providersByHealth: Record<string, number>
  configured: Record<string, boolean>
}

export function mapHealth(v: unknown): Health {
  const r = obj(v)
  return {
    status: str(r.status), checkedAt: str(r.checked_at), providersByHealth: counts(r.providers_by_health), configured: flags(r.configured),
    components: arr(r.components).map((x) => { const c = obj(x); return { name: str(c.name), kind: str(c.kind), status: str(c.status), latencyMs: num(c.latency_ms), detail: str(c.detail) } }),
  }
}

export interface Category {
  id: string
  name: string
  slug: string
  parentId: string
}

export function mapCategory(v: unknown): Category {
  const r = obj(v)
  return { id: str(r.id ?? r.ID), name: str(r.name ?? r.Name), slug: str(r.slug ?? r.Slug), parentId: str(r.parent_id ?? r.ParentID) }
}

// --- calls ---------------------------------------------------------------------------------------------------

const org = (organizationId: string) => ({ organization_id: organizationId })

export const consoleApi = {
  finance: async (organizationId: string, p: { days?: number; status?: string; page?: number } = {}) =>
    mapFinance(await http.get(`/sellers/finance${query({ ...org(organizationId), days: p.days, status: p.status, page: p.page })}`)),
  customers: async (organizationId: string, p: { q?: string; page?: number } = {}) =>
    paged(await http.get(`/sellers/customers${query({ ...org(organizationId), q: p.q, page: p.page })}`), mapCustomer),
  fulfilments: async (organizationId: string, p: { status?: string; page?: number } = {}) =>
    paged(await http.get(`/sellers/fulfilments${query({ ...org(organizationId), status: p.status, page: p.page })}`), mapFulfilment),
  capacity: async (organizationId: string, from?: string, days?: number) => mapCapacity(await http.get(`/sellers/capacity${query({ ...org(organizationId), from, days })}`)),
  usage: async (organizationId: string) => mapUsage(await http.get(`/sellers/usage${query(org(organizationId))}`)),
  integrations: async (organizationId: string) => {
    const r = obj(await http.get(`/sellers/integrations${query(org(organizationId))}`))
    return {
      providers: arr(r.providers).map(mapProvider),
      channels: arr(r.channels).map((x): ChannelAccount => { const c = obj(x); return { id: str(c.id), channel: str(c.channel), externalId: str(c.external_id), status: str(c.status) } }),
    }
  },
  team: async (organizationId: string) => mapTeam(await http.get(`/sellers/team${query(org(organizationId))}`)),
  invite: (organizationId: string, phone: string, role: string) => http.post(`/sellers/team/invitations${query(org(organizationId))}`, { phone, role }),
  revokeInvitation: (organizationId: string, id: string) => http.del(`/sellers/team/invitations/${encodeURIComponent(id)}${query(org(organizationId))}`),
  changeRole: (organizationId: string, userId: string, role: string) => http.patch(`/sellers/team/members/${encodeURIComponent(userId)}${query(org(organizationId))}`, { role }),
  removeMember: (organizationId: string, userId: string) => http.del(`/sellers/team/members/${encodeURIComponent(userId)}${query(org(organizationId))}`),

  /** Fulfilment steps run on the commerce service and are scoped by the business (tenant_id). */
  fulfilmentStep: (organizationId: string, id: string, target: string) =>
    http.post(`/fulfillments/${encodeURIComponent(id)}/transition${query({ tenant_id: organizationId })}`, { target_status: target, idempotency_key: crypto.randomUUID() }),
  takeFulfilment: (organizationId: string, id: string) => http.post(`/fulfillments/${encodeURIComponent(id)}/assign${query({ tenant_id: organizationId })}`, { idempotency_key: crypto.randomUUID() }),
  trackFulfilment: (organizationId: string, id: string, trackingRef: string) =>
    http.post(`/fulfillments/${encodeURIComponent(id)}/tracking${query({ tenant_id: organizationId })}`, { tracking_ref: trackingRef, idempotency_key: crypto.randomUUID() }),
  /** Changes the total stock of a listing by delta (a restock or a write-off), with a reason. */
  adjustStock: (organizationId: string, listingId: string, delta: number, reason: string) =>
    http.post(`/inventory/adjust${query({ tenant_id: organizationId, listing_id: listingId })}`, { delta_total: delta, reason, idempotency_key: crypto.randomUUID() }),
  /** Takes a listing off sale for good. */
  closeListing: (listingId: string) => http.post(`/listings/close${query({ listing_id: listingId })}`),

  business: async (organizationId: string) => mapBusiness(await http.get(`/organizations${query(org(organizationId))}`)),
  updateBusiness: (organizationId: string, patch: Partial<Record<keyof BusinessDetails, string>>) => http.patch(`/organizations${query(org(organizationId))}`, snake(patch)),
  store: async (storeId: string) => mapStore(await http.get(`/stores${query({ store_id: storeId })}`)),
  updateStore: (storeId: string, patch: Partial<Record<keyof StoreDetails, string>>) => http.patch(`/stores${query({ store_id: storeId })}`, snake(patch)),
  updateSellerProfile: (sellerId: string, patch: { display_name?: string; bio?: string; phone?: string; email?: string }) => http.patch(`/sellers${query({ seller_id: sellerId })}`, patch),
  updateMe: (patch: { display_name?: string; email?: string }) => http.patch<unknown>('/auth/users/me', patch),
}

/** displayName → display_name for the edit routes. */
function snake(patch: Record<string, string | undefined>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(patch)) if (v !== undefined) out[k.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`)] = v
  return out
}

export const adminConsoleApi = {
  listings: async (p: { q?: string; status?: string; vertical?: string; flag?: string; page?: number } = {}) =>
    paged(await http.get(`/admin/listings${query({ q: p.q, status: p.status, vertical: p.vertical, flag: p.flag, page: p.page })}`), mapAdminListing),
  suspendListing: (id: string, reason: string) => http.post(`/listings/suspend${query({ listing_id: id })}`, { reason }),
  reinstateListing: (id: string, reason = '') => http.post(`/listings/reinstate${query({ listing_id: id })}`, reason ? { reason } : {}),
  payments: async (p: { q?: string; status?: string; days?: number; page?: number } = {}) =>
    mapFinance(await http.get(`/admin/payments${query({ q: p.q, status: p.status, days: p.days, page: p.page })}`)),
  operations: async (days = 7) => mapOperations(await http.get(`/admin/operations${query({ days })}`)),
  providers: async (days = 7) => arr(obj(await http.get(`/admin/providers${query({ days })}`)).items).map(mapProvider),
  enableProvider: (id: string) => http.post(`/admin/providers/${encodeURIComponent(id)}/enable`),
  disableProvider: (id: string, reason: string) => http.post(`/admin/providers/${encodeURIComponent(id)}/disable`, { reason }),
  channels: async () => mapChannels(await http.get('/admin/channels')),
  security: async () => mapSecurity(await http.get('/admin/security')),
  ai: async (days = 30) => mapAI(await http.get(`/admin/ai${query({ days })}`)),
  monetization: async (days = 30) => mapMonetization(await http.get(`/admin/monetization${query({ days })}`)),
  health: async () => mapHealth(await http.get('/admin/health')),
  categories: async () => {
    const raw = await http.get<unknown>('/categories')
    const list = Array.isArray(raw) ? raw : arr(obj(raw).items ?? obj(raw).categories ?? obj(raw).data)
    return list.map(mapCategory)
  },
  createCategory: (name: string, slug: string, parentId?: string) => http.post('/categories', { name, slug, ...(parentId ? { parent_id: parentId } : {}) }),
}
