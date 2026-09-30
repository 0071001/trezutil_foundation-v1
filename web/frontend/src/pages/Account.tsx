import { useEffect, useMemo, useState } from 'react'
import { Navigate } from 'react-router'
import { ordersApi, reservationsApi } from '@/api/commerce'
import { AppShell, initials } from '@/components/shell'
import { EmptyState, ErrorState, Loading } from '@/components/states'
import { ActionLink, Button, Heading, Icon, PageIntro, Tabs, statusLabel, type IconName } from '@/components/ui'
import { formatDate, timeAgo } from '@/lib/format'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'

function greeting(): string {
  const h = new Date().getHours()
  return h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
}

export function AccountPage() {
  const { user, signOut } = useSession()
  const { theme, toggleTheme } = usePrefs()
  useEffect(() => { document.title = 'Your account — Trezutil' }, [])
  if (user === undefined) return <AppShell><Loading label="Loading your account…" /></AppShell>
  if (!user) return <Navigate to="/login?returnTo=%2Faccount" replace />
  const name = user.display_name || 'there'
  const links: [IconName, string, string, string][] = [
    ['orders', 'Orders', 'Purchases, payments and delivery', '/orders'],
    ['calendar', 'Reservations', 'Stays, services and tables', '/reservations'],
    ['heart', 'Saved', 'Products and places you saved', '/wishlist'],
    ['spark', 'Ask Trezutil', 'Continue your conversation', '/conversation'],
    ['bell', 'Activity', 'Updates on your orders and bookings', '/notifications'],
    ['shop', 'Seller workspace', 'Listings, orders and bookings for your business', '/seller'],
  ]
  return (
    <AppShell>
      <section className="page-section account-page">
        <PageIntro eyebrow="Personal account" title={`${greeting()}, ${name.split(' ')[0]}.`} copy="Everything connected to your Trezutil experience." />
        <div className="account-profile">
          <span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span>
          <div><Heading level={3}>{user.display_name || 'Trezutil member'}</Heading><p>{user.phone}{user.email ? ` · ${user.email}` : ''}</p></div>
          <span className="status-badge completed">{user.verified_at ? 'Phone verified' : 'Active'}</span>
        </div>
        <div className="account-grid">
          {links.map(([icon, title, copy, href]) => <ActionLink className="account-link" href={href} key={title}><span><Icon name={icon} /></span><div><strong>{title}</strong><small>{copy}</small></div><Icon name="chevron" /></ActionLink>)}
        </div>
        <div className="account-footer-actions">
          <Button onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} />{theme === 'dark' ? 'Light mode' : 'Dark mode'}</Button>
          <Button onClick={signOut}>Sign out</Button>
        </div>
      </section>
    </AppShell>
  )
}

interface ActivityItem { id: string; icon: IconName; title: string; copy: string; at: number; href: string; kind: 'Orders' | 'Reservations' }

/**
 * Figma NotificationsPage. There is no notification service yet, so this is
 * the account's real activity: order and reservation status changes.
 */
export function ActivityPage() {
  const orders = useAsync(() => ordersApi.list(), [])
  const reservations = useAsync(() => reservationsApi.list(), [])
  const [filter, setFilter] = useState<'All' | 'Orders' | 'Reservations'>('All')
  useEffect(() => { document.title = 'Activity — Trezutil' }, [])
  const items = useMemo<ActivityItem[]>(() => [
    ...(orders.data ?? []).map((o) => ({ id: `o-${o.id}`, icon: 'orders' as IconName, title: `${o.title || 'Order'} · ${statusLabel(o.status)}`, copy: `Order #${o.id.slice(0, 8)}`, at: o.updated_at || o.created_at, href: `/orders/${o.id}`, kind: 'Orders' as const })),
    ...(reservations.data ?? []).map((r) => ({ id: `r-${r.id}`, icon: 'calendar' as IconName, title: `Reservation ${statusLabel(r.status).toLowerCase()}`, copy: `${formatDate(r.start_at)} → ${formatDate(r.end_at)}`, at: r.updated_at || r.created_at, href: `/reservations/${r.id}`, kind: 'Reservations' as const })),
  ].sort((a, b) => b.at - a.at).slice(0, 40), [orders.data, reservations.data])
  const shown = items.filter((i) => filter === 'All' || i.kind === filter)
  const recent = Date.now() / 1000 - 86400
  return (
    <AppShell>
      <section className="page-section narrow-page">
        <PageIntro eyebrow="Stay up to date" title="Activity" copy="Orders and bookings — only what matters." />
        <Tabs label="Activity type" items={['All', 'Orders', 'Reservations'] as const} value={filter} onChange={setFilter} />
        {(orders.loading || reservations.loading) && <Loading label="Loading activity…" />}
        {orders.error && <ErrorState error={orders.error} onRetry={() => { orders.reload(); reservations.reload() }} />}
        {!orders.loading && !orders.error && shown.length === 0 && <EmptyState icon="bell" title="Nothing new" copy="Updates on your orders and reservations will appear here." action="Start discovering" href="/discover" />}
        {shown.length > 0 && (
          <div className="notification-list">
            {shown.map((n, i) => (
              <ActionLink className={n.at > recent ? 'notification unread' : 'notification'} href={n.href} key={n.id}>
                <span className={`activity-icon ${i % 2 ? 'mint' : ''}`}><Icon name={n.icon} /></span>
                <div><strong>{n.title}</strong><p>{n.copy}</p></div>
                <small>{timeAgo(n.at)}</small>{n.at > recent && <i aria-label="New" />}
              </ActionLink>
            ))}
          </div>
        )}
      </section>
    </AppShell>
  )
}
