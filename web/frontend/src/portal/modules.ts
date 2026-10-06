import type { IconName } from '@/components/ui'
import { tx } from '@/lib/i18n'
import type { GapId } from '@/portal/gaps'

/*
 * The seller and administration modules, declared once.
 *
 * Navigation, route skeletons and the implementation map all follow this
 * list. A module is one of:
 *   ready        a screen exists today and talks to a real API
 *   planned      the API exists; the screen is not built yet (endpoints listed)
 *   unavailable  the platform lacks the capability; the screen says so and
 *                cites the gap (docs/web/PORTALS_BACKEND_GAP_ANALYSIS.md)
 * Nothing here grants access: every request is authorised by the server.
 */

export type ModuleStatus =
  | { kind: 'ready' }
  | { kind: 'planned'; endpoints: string[]; limits?: GapId[] }
  | { kind: 'unavailable'; gaps: GapId[]; note?: string; link?: { to: string; label: string } }

/**
 * Who the module is meant for. Seller: any member, or owner/admin/manager.
 * Administration: platform operator (admin or operator), or admin only.
 */
export type Requirement = 'member' | 'manager' | 'operator' | 'admin'

export interface PortalModule {
  id: string
  group: string
  label: string
  to: string
  icon: IconName
  end?: boolean
  requires: Requirement
  status: ModuleStatus
  purpose: string
}

export interface ModuleGroup {
  id: string
  label: string
}

export const sellerGroups: ModuleGroup[] = [
  { id: 'run', label: tx('Run') },
  { id: 'sell', label: tx('Sell') },
  { id: 'operate', label: tx('Operate') },
  { id: 'grow', label: tx('Grow') },
  { id: 'money', label: tx('Money') },
  { id: 'business', label: tx('Business') },
]

export const adminGroups: ModuleGroup[] = [
  { id: 'overview', label: tx('Overview') },
  { id: 'people', label: tx('People and businesses') },
  { id: 'catalog', label: tx('Catalogue') },
  { id: 'commerce', label: tx('Commerce') },
  { id: 'platform', label: tx('Platform') },
  { id: 'operations', label: tx('Operations') },
  { id: 'settings', label: tx('Settings') },
]

const ready: ModuleStatus = { kind: 'ready' }
const unavailable = (gaps: GapId[], extra: { note?: string; link?: { to: string; label: string } } = {}): ModuleStatus => ({ kind: 'unavailable', gaps, ...extra })

export const sellerModules: PortalModule[] = [
  // Run
  { id: 'dashboard', group: 'run', label: tx('Dashboard'), to: '/seller', icon: 'home', end: true, requires: 'member', status: ready, purpose: tx('What needs you now, and how your business is doing.') },
  { id: 'inbox', group: 'run', label: tx('Inbox'), to: '/seller/inbox', icon: 'inbox', requires: 'manager', status: unavailable(['G-B8']), purpose: tx('Conversations between your customers and your business, with the listing, order or booking in view.') },
  // Sell
  { id: 'listings', group: 'sell', label: tx('Listings'), to: '/seller/listings', icon: 'shop', requires: 'member', status: ready, purpose: tx('Everything you sell: create, publish, pause and price it.') },
  { id: 'media', group: 'sell', label: tx('Categories and media'), to: '/seller/media', icon: 'attach', requires: 'manager', status: unavailable(['G-C6', 'G-A10']), purpose: tx('Your image library and the categories your listings are published under.') },
  // Operate
  { id: 'orders', group: 'operate', label: tx('Orders'), to: '/seller/orders', icon: 'orders', requires: 'manager', status: ready, purpose: tx('Incoming orders to confirm and follow.') },
  { id: 'reservations', group: 'operate', label: tx('Reservations'), to: '/seller/reservations', icon: 'calendar', requires: 'manager', status: ready, purpose: tx('Bookings for your rooms, tables, services and places.') },
  { id: 'inventory', group: 'operate', label: tx('Inventory'), to: '/seller/inventory', icon: 'briefcase', requires: 'manager', status: ready, purpose: tx('What you have in stock, reserved and sold.') },
  { id: 'availability', group: 'operate', label: tx('Availability'), to: '/seller/availability', icon: 'clock', requires: 'manager', status: ready, purpose: tx('What can still be booked, day by day, for every room, table, slot or item you track.') },
  { id: 'hotel', group: 'operate', label: tx('Hotel rooms'), to: '/seller/hotel', icon: 'building', requires: 'manager', status: ready, purpose: tx('Room types, rooms, rates and nightly capacity.') },
  { id: 'fulfilment', group: 'operate', label: tx('Fulfilment'), to: '/seller/fulfilment', icon: 'check', requires: 'manager', status: ready, purpose: tx('Every delivery, pickup and appointment to carry out, from acceptance to hand-over.') },
  // Grow
  { id: 'marketing', group: 'grow', label: tx('Marketing'), to: '/seller/marketing', icon: 'spark', requires: 'manager', status: unavailable(['G-C4'], { link: { to: '/seller/listings', label: tx('Put a listing on sale from Listings') } }), purpose: tx('Campaigns, audiences and automations to bring customers back.') },
  { id: 'analytics', group: 'grow', label: tx('Analytics'), to: '/seller/analytics', icon: 'chart', requires: 'manager', status: ready, purpose: tx('Sales, orders, bookings and listing performance over time.') },
  { id: 'customers', group: 'grow', label: tx('Customers'), to: '/seller/customers', icon: 'users', requires: 'manager', status: ready, purpose: tx('The people who buy and book from you, and their history.') },
  // Money
  { id: 'finance', group: 'money', label: tx('Finance'), to: '/seller/finance', icon: 'wallet', requires: 'manager', status: ready, purpose: tx('Payments, refunds, settlements and balances.') },
  { id: 'usage', group: 'money', label: tx('Usage and credits'), to: '/seller/usage', icon: 'pulse', requires: 'manager', status: ready, purpose: tx('Your plan, credit balance and what the assistant has used.') },
  // Business
  { id: 'business', group: 'business', label: tx('Business profile'), to: '/seller/business', icon: 'building', requires: 'member', status: ready, purpose: tx('Your business, its stores and its verification.') },
  { id: 'verticals', group: 'business', label: tx('Verticals'), to: '/seller/verticals', icon: 'layers', requires: 'member', status: ready, purpose: tx('The kinds of business you run on Trezutil and what each one supports.') },
  { id: 'team', group: 'business', label: tx('Team'), to: '/seller/team', icon: 'users', requires: 'member', status: ready, purpose: tx('Who works in your business, and with which role.') },
  { id: 'integrations', group: 'business', label: tx('Integrations'), to: '/seller/integrations', icon: 'plug', requires: 'manager', status: ready, purpose: tx('The payment, messaging and provider connections your business uses.') },
  { id: 'settings', group: 'business', label: tx('Settings'), to: '/seller/settings', icon: 'settings', requires: 'member', status: ready, purpose: tx('Your name and email on Trezutil, and how you sign in.') },
]

export const adminModules: PortalModule[] = [
  // Overview
  { id: 'overview', group: 'overview', label: tx('Overview'), to: '/admin', icon: 'home', end: true, requires: 'operator', status: ready, purpose: tx('The platform at a glance.') },
  // People and businesses
  { id: 'users', group: 'people', label: tx('Users'), to: '/admin/users', icon: 'user', requires: 'admin', status: ready, purpose: tx('Search and review accounts.') },
  { id: 'organizations', group: 'people', label: tx('Organizations and sellers'), to: '/admin/organizations', icon: 'building', requires: 'operator', status: ready, purpose: tx('Every business on the platform: status, verification, team and activity.') },
  // Catalogue
  { id: 'verticals', group: 'catalog', label: tx('Verticals'), to: '/admin/verticals', icon: 'layers', requires: 'operator', status: ready, purpose: tx('The commerce verticals, their capabilities and how many listings each holds.') },
  { id: 'categories', group: 'catalog', label: tx('Categories'), to: '/admin/categories', icon: 'filter', requires: 'operator', status: ready, purpose: tx('The category tree listings are published under.') },
  { id: 'listings', group: 'catalog', label: tx('Listings'), to: '/admin/listings', icon: 'shop', requires: 'operator', status: ready, purpose: tx('Review listings across businesses; publish or pause them.') },
  // Commerce
  { id: 'orders', group: 'commerce', label: tx('Orders'), to: '/admin/orders', icon: 'orders', requires: 'operator', status: ready, purpose: tx('Every order on the platform, by business and customer.') },
  { id: 'reservations', group: 'commerce', label: tx('Reservations'), to: '/admin/reservations', icon: 'calendar', requires: 'operator', status: ready, purpose: tx('Every booking on the platform, by business and customer.') },
  { id: 'payments', group: 'commerce', label: tx('Payments and finance'), to: '/admin/payments', icon: 'wallet', requires: 'operator', status: ready, purpose: tx('Payments, captures, refunds and failures.') },
  // Platform
  { id: 'providers', group: 'platform', label: tx('Providers'), to: '/admin/providers', icon: 'plug', requires: 'operator', status: ready, purpose: tx('Providers, their capabilities, health, executions and fallbacks.') },
  { id: 'ai', group: 'platform', label: tx('AI control centre'), to: '/admin/ai', icon: 'spark', requires: 'operator', status: ready, purpose: tx('AI providers, models, routing, usage, failures, latency and cost.') },
  { id: 'channels', group: 'platform', label: tx('Channels'), to: '/admin/channels', icon: 'globe', requires: 'operator', status: ready, purpose: tx('Web, mobile and WhatsApp: configuration status and message delivery health.') },
  { id: 'monetization', group: 'platform', label: tx('Monetization'), to: '/admin/monetization', icon: 'wallet', requires: 'operator', status: ready, purpose: tx('Plans, subscriptions, credits and the usage ledger.') },
  { id: 'search', group: 'platform', label: tx('Search and indexing'), to: '/admin/search', icon: 'search', requires: 'operator', status: ready, purpose: tx('Index health, indexing lag and search failures.') },
  // Operations
  { id: 'operations', group: 'operations', label: tx('Incidents and support'), to: '/admin/operations', icon: 'bell', requires: 'operator', status: ready, purpose: tx('Failed orders, reservations, payments and webhooks, and support cases.') },
  { id: 'audit', group: 'operations', label: tx('Audit'), to: '/admin/audit', icon: 'doc', requires: 'operator', status: ready, purpose: tx('Who did what, to which resource, and when.') },
  { id: 'security', group: 'operations', label: tx('Security'), to: '/admin/security', icon: 'shield', requires: 'operator', status: ready, purpose: tx('Sign-in activity, sessions, suspicious activity and rate limits.') },
  { id: 'health', group: 'operations', label: tx('System health'), to: '/admin/health', icon: 'pulse', requires: 'operator', status: ready, purpose: tx('Gateway and service health.') },
  // Settings
  { id: 'site', group: 'settings', label: tx('Site content'), to: '/admin/site', icon: 'globe', requires: 'admin', status: ready, purpose: tx('What visitors see on the home page: the live figures, the partner logos and the order of the sections.') },
  { id: 'settings', group: 'settings', label: tx('Platform settings'), to: '/admin/settings', icon: 'settings', requires: 'operator', status: unavailable(['G-C5']), purpose: tx('Platform settings and feature flags.') },
]

export interface Access {
  /** Seller: owner, admin or manager of the selected business. */
  canManage?: boolean
  /** Administration: platform roles of the signed-in user. */
  roles?: readonly string[]
}

/** Whether the signed-in user is meant to use the module (a convenience; the server decides). */
export function allows(requires: Requirement, access: Access): boolean {
  switch (requires) {
    case 'member': return true
    case 'manager': return !!access.canManage
    case 'operator': return !!access.roles?.some((r) => r === 'admin' || r === 'operator')
    case 'admin': return !!access.roles?.includes('admin')
  }
}

export interface ShellItem {
  id: string
  label: string
  to: string
  icon: IconName
  end?: boolean
  status: ModuleStatus['kind']
  /** Visible, but the user's role cannot operate it (seller members below manager). */
  restricted: boolean
}

export interface ShellGroup {
  id: string
  label: string
  items: ShellItem[]
}

/**
 * The rail's groups for a portal. Seller: every module is listed (read-only
 * members see which ones need a manager). Administration: modules the role
 * cannot use are not listed at all.
 */
export function shellGroups(portal: 'seller' | 'admin', access: Access): ShellGroup[] {
  const modules = portal === 'seller' ? sellerModules : adminModules
  const groups = portal === 'seller' ? sellerGroups : adminGroups
  return groups
    .map((g) => ({
      id: g.id,
      label: g.label,
      items: modules
        .filter((m) => m.group === g.id)
        .filter((m) => portal === 'seller' || allows(m.requires, access))
        .map((m) => ({ id: m.id, label: m.label, to: m.to, icon: m.icon, end: m.end, status: m.status.kind, restricted: !allows(m.requires, access) })),
    }))
    .filter((g) => g.items.length > 0)
}
