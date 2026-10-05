/* Server answers shared by the dashboard and site-content tests. Amounts are minor units, as the API sends them. */

const day = (date: string, count: number) => ({ date, count })
const DATES = ['2026-10-02', '2026-10-03', '2026-10-04']

/** What GET /admin/analytics (platform) or GET /sellers/analytics (organization) returns for a 3-day window. */
export function analyticsBody(platform: boolean, extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    generated_at: '2026-10-04T12:00:00Z', timezone: 'Africa/Kinshasa', days: 3, from: '2026-10-02', to: '2026-10-04',
    revenue_currencies: ['USD', 'CDF'],
    current: { orders: 5, reservations: 3, conversations: 9, new_users: 2, new_listings: 1, customers: 3, returning_customers: 1, revenue: { USD: 473400, CDF: 5000000 } },
    previous: { orders: 2, reservations: 1, conversations: 4, new_users: 1, new_listings: 0, customers: 1, returning_customers: 0, revenue: { USD: 40000 } },
    orders: [day(DATES[0]!, 0), day(DATES[1]!, 1), day(DATES[2]!, 4)],
    revenue: [{ date: DATES[0], amounts: {} }, { date: DATES[1], amounts: {} }, { date: DATES[2], amounts: { USD: 473400, CDF: 5000000 } }],
    reservations: [day(DATES[0]!, 0), day(DATES[1]!, 1), day(DATES[2]!, 2)],
    new_listings: [day(DATES[0]!, 0), day(DATES[1]!, 1), day(DATES[2]!, 0)],
    orders_by_status: { paid: 3, completed: 1, cancelled: 1 },
    reservations_by_status: { confirmed: 1, pending: 1, cancelled: 1 },
    listings_by_status: { published: 4, draft: 1 },
    published_listings_by_vertical: { RETAIL: 3, HOTEL: 1 },
    ...(platform
      ? {
          new_users: [day(DATES[0]!, 0), day(DATES[1]!, 0), day(DATES[2]!, 2)],
          conversations: [day(DATES[0]!, 3), day(DATES[1]!, 2), day(DATES[2]!, 4)],
          users_by_role: { customer: 4, admin: 1 },
          top_organizations: [
            { id: 'o1', name: 'Gombe Stays', orders: 4, units: 0, revenue: { USD: 350000, CDF: 5000000 } },
            { id: 'o2', name: 'Kasa Market', orders: 1, units: 0, revenue: { USD: 123400 } },
          ],
        }
      : { top_listings: [{ id: 'l1', name: 'Café Mokili', orders: 2, units: 3, revenue: { USD: 150000 } }, { id: 'l2', name: 'Sofa', orders: 1, units: 1, revenue: { USD: 200000 } }] }),
    ...extra,
  }
}

/** A platform with nothing happening: real zeros everywhere. */
export function quietAnalytics(platform: boolean): Record<string, unknown> {
  const zero = DATES.map((d) => day(d, 0))
  return analyticsBody(platform, {
    revenue_currencies: [],
    current: { orders: 0, reservations: 0, conversations: 0, new_users: 0, new_listings: 0, customers: 0, returning_customers: 0, revenue: {} },
    previous: { orders: 0, reservations: 0, conversations: 0, new_users: 0, new_listings: 0, customers: 0, returning_customers: 0, revenue: {} },
    orders: zero, reservations: zero, new_listings: zero, revenue: DATES.map((d) => ({ date: d, amounts: {} })),
    ...(platform ? { new_users: zero, conversations: zero, top_organizations: [] } : { top_listings: [] }),
    orders_by_status: {}, reservations_by_status: {}, listings_by_status: {}, published_listings_by_vertical: {},
  })
}

const trust = (metric: string, enabled: boolean, min_value = 1, extra: Record<string, unknown> = {}) => ({ metric, enabled, min_value, ...extra })

/**
 * What GET /admin/site/content returns before anything was saved: the defaults (the four figures of the strip on:
 * what Trezutil enables, in the server's order; users, partners and the older metrics off), and the live figures.
 * As on the real server, the partners figure is counted from the partner list.
 */
export function siteAdminBody(extra: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    content: {
      trust: [
        trust('conversations', true), trust('connections', true), trust('transactions', true), trust('merchants', true),
        trust('registered_users', false), trust('partners', false),
        trust('live_listings', false), trust('verified_sellers', false), trust('deals_live', false), trust('cities_covered', false),
        trust('bookings_confirmed', false), trust('orders_completed', false), trust('verticals_live', false),
      ],
      partners: [
        { id: 'rawbank', name: 'Rawbank', kind: 'bank', logo_url: '/partners/rawbank.png', visible: true },
        { id: 'tmb', name: 'TMB', kind: 'bank', logo_url: '/partners/tmb.png', link_url: 'https://tmb.cd', visible: true },
        { id: 'stripe', name: 'Stripe', kind: 'tech', logo_url: '/partners/stripe.svg', visible: false },
      ],
      sections: ['categories', 'nearby', 'how', 'whatsapp', 'trust', 'seller', 'partners'].map((key) => ({ key, visible: true })),
    },
    version: 0, stored: false, updated_at: '',
    stats: { conversations: 340, connections: 41, registered_users: 38, merchants: 20, transactions: 12, partners: 2, live_listings: 197, verified_sellers: 2, deals_live: 7, cities_covered: 1, bookings_confirmed: 0, orders_completed: 12, verticals_live: 3 },
    limits: { max_trust_items: 16, max_partners: 40, max_partner_name: 60, max_trust_label: 40, max_asset_bytes: 524288 },
    ...extra,
  }
}
