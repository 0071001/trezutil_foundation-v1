import { http, query } from '@/api/client'
import { arr, num, obj, str } from '@/api/json'

/*
 * The figures behind the dashboards, computed by the platform from its own
 * orders, bookings, listings, accounts and conversations (see
 * docs/web/DASHBOARDS_AND_SITE_CONTENT.md). Days are Africa/Kinshasa days; a
 * day with no activity arrives as a real zero. Amounts are minor units, per
 * currency, and are never added across currencies.
 */

export interface DayCount { date: string; count: number }
export interface DayRevenue { date: string; amounts: Record<string, number> }

export interface PeriodTotals {
  orders: number
  reservations: number
  conversations: number
  newUsers: number
  newListings: number
  customers: number
  returningCustomers: number
  revenue: Record<string, number>
}

export interface RankedEntry {
  id: string
  name: string
  orders: number
  units: number
  revenue: Record<string, number>
}

export interface Analytics {
  /** `platform` for the administration, `organization` for a seller's business. */
  scope: 'platform' | 'organization'
  generatedAt: string
  timezone: string
  days: number
  /** First and last day of the window, inclusive. */
  from: string
  to: string
  revenueCurrencies: string[]
  current: PeriodTotals
  previous: PeriodTotals
  orders: DayCount[]
  revenue: DayRevenue[]
  reservations: DayCount[]
  newListings: DayCount[]
  newUsers: DayCount[]
  conversations: DayCount[]
  ordersByStatus: Record<string, number>
  reservationsByStatus: Record<string, number>
  listingsByStatus: Record<string, number>
  publishedByVertical: Record<string, number>
  usersByRole: Record<string, number>
  topOrganizations: RankedEntry[]
  topListings: RankedEntry[]
}

/** The windows the dashboards offer. */
export const PERIODS = [7, 30, 90] as const
export type Period = (typeof PERIODS)[number]

function counts(value: unknown): Record<string, number> {
  const out: Record<string, number> = {}
  for (const [k, v] of Object.entries(obj(value))) out[k] = num(v)
  return out
}

const days = (value: unknown): DayCount[] => arr(value).map((v) => ({ date: str(obj(v).date), count: num(obj(v).count) }))

function totals(value: unknown): PeriodTotals {
  const raw = obj(value)
  return {
    orders: num(raw.orders), reservations: num(raw.reservations), conversations: num(raw.conversations),
    newUsers: num(raw.new_users), newListings: num(raw.new_listings),
    customers: num(raw.customers), returningCustomers: num(raw.returning_customers), revenue: counts(raw.revenue),
  }
}

function ranked(value: unknown): RankedEntry[] {
  return arr(value).map((v) => {
    const raw = obj(v)
    return { id: str(raw.id), name: str(raw.name), orders: num(raw.orders), units: num(raw.units), revenue: counts(raw.revenue) }
  })
}

export function mapAnalytics(value: unknown): Analytics {
  const raw = obj(value)
  return {
    scope: raw.new_users === undefined ? 'organization' : 'platform',
    generatedAt: str(raw.generated_at), timezone: str(raw.timezone), days: num(raw.days), from: str(raw.from), to: str(raw.to),
    revenueCurrencies: arr(raw.revenue_currencies).map(str).filter(Boolean),
    current: totals(raw.current), previous: totals(raw.previous),
    orders: days(raw.orders),
    revenue: arr(raw.revenue).map((v) => ({ date: str(obj(v).date), amounts: counts(obj(v).amounts) })),
    reservations: days(raw.reservations), newListings: days(raw.new_listings), newUsers: days(raw.new_users), conversations: days(raw.conversations),
    ordersByStatus: counts(raw.orders_by_status), reservationsByStatus: counts(raw.reservations_by_status),
    listingsByStatus: counts(raw.listings_by_status), publishedByVertical: counts(raw.published_listings_by_vertical), usersByRole: counts(raw.users_by_role),
    topOrganizations: ranked(raw.top_organizations), topListings: ranked(raw.top_listings),
  }
}

export const analyticsApi = {
  /** The whole platform (administrators and operators). */
  async platform(period: number): Promise<Analytics> {
    return mapAnalytics(await http.get<unknown>(`/admin/analytics${query({ days: period })}`))
  },
  /** One business (its owner, admins and managers; platform staff). */
  async organization(organizationId: string, period: number): Promise<Analytics> {
    return mapAnalytics(await http.get<unknown>(`/sellers/analytics${query({ organization_id: organizationId, days: period })}`))
  },
}
