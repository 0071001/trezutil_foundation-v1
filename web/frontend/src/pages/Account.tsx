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
import { t } from '@/lib/i18n'

function greeting(): string {
  const h = new Date().getHours()
  return h < 12 ? t('Good morning') : h < 18 ? t('Good afternoon') : t('Good evening')
}

export function AccountPage() {
  const { user, signOut } = useSession()
  const { theme, toggleTheme } = usePrefs()
  useEffect(() => { document.title = t('Your account — Trezutil') }, [])
  if (user === undefined) return <AppShell><Loading label={t('Loading your account…')} /></AppShell>
  if (!user) return <Navigate to="/login?returnTo=%2Faccount" replace />
  const name = user.display_name || 'there'
  const links: [IconName, string, string, string][] = [
    ['orders', t('Orders'), t('Purchases, payments and delivery'), '/orders'],
    ['calendar', t('Reservations'), t('Stays, services and tables'), '/reservations'],
    ['heart', t('Saved'), t('Products and places you saved'), '/wishlist'],
    ['spark', t('Ask Trezutil'), t('Continue your conversation'), '/conversation'],
    ['bell', t('Activity'), t('Updates on your orders and bookings'), '/notifications'],
    ['shop', t('Seller workspace'), t('Listings, orders and bookings for your business'), '/seller'],
  ]
  return (
    <AppShell>
      <section className="page-section account-page">
        <PageIntro eyebrow={t('Personal account')} title={`${greeting()}, ${name.split(' ')[0]}.`} copy={t('Everything connected to your Trezutil experience.')} />
        <div className="account-profile">
          <span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span>
          <div><Heading level={3}>{user.display_name || t('Trezutil member')}</Heading><p>{user.phone}{user.email ? ` · ${user.email}` : ''}</p></div>
          <span className="status-badge completed">{user.verified_at ? t('Phone verified') : t('Active')}</span>
        </div>
        <div className="account-grid">
          {links.map(([icon, title, copy, href]) => <ActionLink className="account-link" href={href} key={title}><span><Icon name={icon} /></span><div><strong>{title}</strong><small>{copy}</small></div><Icon name="chevron" /></ActionLink>)}
        </div>
        <div className="account-footer-actions">
          <Button onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} />{theme === 'dark' ? t('Light mode') : t('Dark mode')}</Button>
          <Button onClick={signOut}>{t('Sign out')}</Button>
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
  useEffect(() => { document.title = t('Activity — Trezutil') }, [])
  const items = useMemo<ActivityItem[]>(() => [
    ...(orders.data ?? []).map((o) => ({ id: `o-${o.id}`, icon: 'orders' as IconName, title: `${o.title || t('Order')} · ${statusLabel(o.status)}`, copy: t('Order #{slice}', { slice: o.id.slice(0, 8) }), at: o.updated_at || o.created_at, href: `/orders/${o.id}`, kind: 'Orders' as const })),
    ...(reservations.data ?? []).map((r) => ({ id: `r-${r.id}`, icon: 'calendar' as IconName, title: t('Reservation {toLowerCase}', { toLowerCase: statusLabel(r.status).toLowerCase() }), copy: `${formatDate(r.start_at)} → ${formatDate(r.end_at)}`, at: r.updated_at || r.created_at, href: `/reservations/${r.id}`, kind: 'Reservations' as const })),
  ].sort((a, b) => b.at - a.at).slice(0, 40), [orders.data, reservations.data])
  const shown = items.filter((i) => filter === 'All' || i.kind === filter)
  const recent = Date.now() / 1000 - 86400
  return (
    <AppShell>
      <section className="page-section narrow-page">
        <PageIntro eyebrow={t('Stay up to date')} title={t('Activity')} copy={t('Orders and bookings — only what matters.')} />
        <Tabs label={t('Activity type')} items={['All', 'Orders', 'Reservations'] as const} value={filter} onChange={setFilter} />
        {(orders.loading || reservations.loading) && <Loading label={t('Loading activity…')} />}
        {orders.error && <ErrorState error={orders.error} onRetry={() => { orders.reload(); reservations.reload() }} />}
        {!orders.loading && !orders.error && shown.length === 0 && <EmptyState icon="bell" title={t('Nothing new')} copy={t('Updates on your orders and reservations will appear here.')} action={t('Start discovering')} href="/discover" />}
        {shown.length > 0 && (
          <div className="notification-list">
            {shown.map((n, i) => (
              <ActionLink className={n.at > recent ? 'notification unread' : 'notification'} href={n.href} key={n.id}>
                <span className={`activity-icon ${i % 2 ? 'mint' : ''}`}><Icon name={n.icon} /></span>
                <div><strong>{n.title}</strong><p>{n.copy}</p></div>
                <small>{timeAgo(n.at)}</small>{n.at > recent && <i aria-label={t('New')} />}
              </ActionLink>
            ))}
          </div>
        )}
      </section>
    </AppShell>
  )
}
