import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, NavLink, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router'
import { isStaff } from '@/api/admin'
import { supplierApi } from '@/api/supplier'
import { useSession } from '@/lib/session'
import { setTheme, useTheme } from '@/lib/theme'
import { t, tx } from '@/lib/i18n'
import { initials } from '@/ui/kit'
import { CCIcon, type CCIconName } from '@/admin/icons'
import { Loading } from '@/admin/ui'
import { Alerts, CommandCenter, HealthPage } from '@/admin/CommandCenter'
import { Accounts, Catalogue, Categories, Orders, Organizations, Reservations, Verification } from '@/admin/CommercePages'
import { AIActivityPage, Audit, ExchangeRatePage, Integrations, Payments, Staff } from '@/admin/PlatformPages'
import { Billing } from '@/admin/BillingPage'
import '@/admin/admin.css'

/*
 * The Trezutil Control Center (/admin): the back office of the Trezutil team, after the Control Center design. Every
 * page reads the real administration API; the server checks the caller's role on every call (admin, operator,
 * moderator), so this screen only decides what to show, never who may act. A module the backend does not offer yet is
 * listed as "Bientôt" and does nothing.
 */

interface NavItem { label: string; icon: CCIconName; to?: string; badge?: 'verification' }
const NAV: { group: string; items: NavItem[] }[] = [
  { group: tx('Vue d’ensemble'), items: [
    { label: tx('Command Center'), icon: 'grid', to: '' },
    { label: tx('Alertes et incidents'), icon: 'alert', to: 'alerts' },
    { label: tx('Santé des services'), icon: 'server', to: 'health' },
  ] },
  { group: tx('Commerce'), items: [
    { label: tx('Fournisseurs'), icon: 'building', to: 'organizations' },
    { label: tx('Vérification fournisseurs'), icon: 'shield', to: 'verification', badge: 'verification' },
    { label: tx('Comptes clients'), icon: 'users', to: 'accounts' },
    { label: tx('Catalogue'), icon: 'layers', to: 'catalogue' },
    { label: tx('Catégories'), icon: 'tag', to: 'categories' },
  ] },
  { group: tx('Opérations'), items: [
    { label: tx('Commandes'), icon: 'cart', to: 'orders' },
    { label: tx('Réservations'), icon: 'calendar', to: 'reservations' },
    { label: tx('Négociations'), icon: 'message' },
  ] },
  { group: tx('IA'), items: [
    { label: tx('Activité de l’assistant'), icon: 'spark', to: 'ai' },
  ] },
  { group: tx('Crédits et revenus'), items: [
    { label: tx('Crédits et facturation'), icon: 'coins', to: 'billing' },
    { label: tx('Paiements'), icon: 'card', to: 'payments' },
  ] },
  { group: tx('Intégrations'), items: [
    { label: tx('Canaux et fournisseurs'), icon: 'plug', to: 'integrations' },
  ] },
  { group: tx('Plateforme'), items: [
    { label: tx('Taux de change'), icon: 'globe', to: 'exchange-rate' },
    { label: tx('Analytics'), icon: 'chart' },
  ] },
  { group: tx('Sécurité'), items: [
    { label: tx('Administrateurs et rôles'), icon: 'lock', to: 'staff' },
    { label: tx('Journal d’audit'), icon: 'book', to: 'audit' },
  ] },
]
const ITEMS = NAV.flatMap((g) => g.items)

const ROLE_LABEL: Record<string, string> = { admin: tx('Administrateur'), operator: tx('Opérateur'), moderator: tx('Modérateur') }

function Gate({ children }: { children: ReactNode }) {
  const { user } = useSession()
  if (user === undefined) return <div className="cc-gate"><Loading /></div>
  if (!user || !isStaff(user)) {
    return (
      <div className="cc-gate">
        <div className="cc-gate-card">
          <div className="cc-brand-dots" aria-hidden="true"><i /><i /><i /></div>
          <h1>{t('Trezutil Control Center')}</h1>
          <p>{user ? t('Cet espace est réservé à l’équipe Trezutil (administrateurs, opérateurs, modérateurs). Ton compte n’a pas ce rôle.') : t('Connecte-toi avec un compte de l’équipe Trezutil pour ouvrir le back-office.')}</p>
          {user ? <Link className="cc-primary" to="/">{t('Retour à Trezutil')}</Link> : <Link className="cc-primary" to="/login?returnTo=%2Fadmin">{t('Se connecter')}</Link>}
        </div>
      </div>
    )
  }
  return <>{children}</>
}

function Shell() {
  const { user, signOut } = useSession()
  const theme = useTheme()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [closed, setClosed] = useState<string[]>([])
  const [find, setFind] = useState('')
  const [pending, setPending] = useState<number | null>(null)

  const sub = pathname.replace(/^\/admin\/?/, '').split('/')[0] ?? ''
  const current = ITEMS.find((i) => i.to === sub) ?? ITEMS[0]!
  useEffect(() => { document.title = `${t(current.label)} — ${t('Trezutil Control Center')}` }, [current.label])
  useEffect(() => {
    let live = true
    supplierApi.queue('submitted').then((q) => { if (live) setPending(q.length) }).catch(() => undefined)
    return () => { live = false }
  }, [pathname])

  const matches = useMemo(() => {
    const q = find.trim().toLowerCase()
    return q ? ITEMS.filter((i) => i.to !== undefined && t(i.label).toLowerCase().includes(q)).slice(0, 6) : []
  }, [find])
  const go = (to: string) => { setFind(''); setOpen(false); navigate(`/admin${to ? `/${to}` : ''}`) }
  const roles = (user?.roles ?? []).filter((r) => ROLE_LABEL[r]).map((r) => t(ROLE_LABEL[r]!)).join(' · ')
  const env = typeof window !== 'undefined' && window.location.hostname.endsWith('trezutil.com') ? t('Production') : t('Local')

  return (
    <div className="cc-shell">
      <aside className={`cc-sidebar ${open ? 'open' : ''}`} aria-label={t('Navigation du back-office')}>
        <div className="cc-brand">
          <img src="/brand/trezutil-logo-white.png" alt="Trezutil" width={1000} height={147} />
          <small>{t('CONTROL CENTER')}</small>
          <button type="button" className="cc-mobile-close" aria-label={t('Fermer le menu')} onClick={() => setOpen(false)}><CCIcon name="close" /></button>
        </div>
        <nav>
          {NAV.map((g) => (
            <div className="cc-nav-group" key={g.group}>
              <button type="button" className={`cc-group-title ${closed.includes(g.group) ? 'closed' : ''}`} aria-expanded={!closed.includes(g.group)}
                onClick={() => setClosed((c) => (c.includes(g.group) ? c.filter((x) => x !== g.group) : [...c, g.group]))}>
                <span>{t(g.group)}</span><CCIcon name="chevron" size={13} />
              </button>
              {!closed.includes(g.group) && (
                <div className="cc-nav-items">
                  {g.items.map((i) => i.to === undefined ? (
                    <span key={i.label} className="cc-soon" title={t('Bientôt : ce module n’a pas encore de service côté serveur.')}><CCIcon name={i.icon} /><span>{t(i.label)}</span><b className="neutral">{t('Bientôt')}</b></span>
                  ) : (
                    <NavLink key={i.label} end={i.to === ''} to={`/admin${i.to ? `/${i.to}` : ''}`} className={({ isActive }) => (isActive ? 'active' : '')} onClick={() => setOpen(false)}>
                      <CCIcon name={i.icon} /><span>{t(i.label)}</span>
                      {i.badge === 'verification' && pending ? <b>{pending}</b> : null}
                    </NavLink>
                  ))}
                </div>
              )}
            </div>
          ))}
        </nav>
        <div className="cc-sidebar-foot">
          <div className="cc-environment"><span /><div><small>{t('ENVIRONNEMENT')}</small><strong>{env}</strong></div></div>
          <div className="cc-profile">
            <span>{initials(user?.display_name || user?.email || '?')}</span>
            <div><strong>{user?.display_name || user?.email || user?.phone}</strong><small>{roles}</small></div>
            <button type="button" aria-label={t('Se déconnecter')} title={t('Se déconnecter')} onClick={() => { signOut(); navigate('/') }}><CCIcon name="arrow" /></button>
          </div>
        </div>
      </aside>
      {open && <div className="cc-scrim" onClick={() => setOpen(false)} />}
      <div className="cc-main-shell">
        <header className="cc-topbar">
          <button type="button" className="cc-mobile-menu" aria-label={t('Ouvrir le menu')} onClick={() => setOpen(true)}><CCIcon name="menu" /></button>
          <div className="cc-breadcrumb"><span className="cc-crumb-icon"><CCIcon name={current.icon} /></span><strong>{t(current.label)}</strong></div>
          <div className="cc-global-search">
            <CCIcon name="search" />
            <input value={find} onChange={(e) => setFind(e.target.value)} placeholder={t('Aller à une page du back-office…')} aria-label={t('Aller à une page du back-office…')}
              onKeyDown={(e) => { if (e.key === 'Enter' && matches[0]) go(matches[0].to!); if (e.key === 'Escape') setFind('') }} />
            {matches.length > 0 && (
              <div className="cc-find" role="listbox">
                {matches.map((m) => <button type="button" role="option" aria-selected={false} key={m.label} onMouseDown={(e) => { e.preventDefault(); go(m.to!) }}><CCIcon name={m.icon} />{t(m.label)}</button>)}
              </div>
            )}
          </div>
          <div className="cc-top-actions">
            <button type="button" aria-label={t('Changer de thème')} title={t('Changer de thème')} onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}><CCIcon name="toggle" /></button>
            <Link to="/" title={t('Retour à Trezutil')} aria-label={t('Retour à Trezutil')}><CCIcon name="message" /></Link>
            <span className="cc-top-avatar" aria-hidden="true">{initials(user?.display_name || '?')}</span>
          </div>
        </header>
        <main className="cc-main">
          <Routes>
            <Route index element={<CommandCenter />} />
            <Route path="alerts" element={<Alerts />} />
            <Route path="health" element={<HealthPage />} />
            <Route path="organizations" element={<Organizations />} />
            <Route path="verification" element={<Verification onChange={() => supplierApi.queue('submitted').then((q) => setPending(q.length)).catch(() => undefined)} />} />
            <Route path="accounts" element={<Accounts />} />
            <Route path="catalogue" element={<Catalogue />} />
            <Route path="categories" element={<Categories />} />
            <Route path="orders" element={<Orders />} />
            <Route path="reservations" element={<Reservations />} />
            <Route path="ai" element={<AIActivityPage />} />
            <Route path="billing" element={<Billing />} />
            <Route path="payments" element={<Payments />} />
            <Route path="integrations" element={<Integrations />} />
            <Route path="exchange-rate" element={<ExchangeRatePage />} />
            <Route path="staff" element={<Staff />} />
            <Route path="audit" element={<Audit />} />
            <Route path="*" element={<Navigate to="/admin" replace />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

export function AdminApp() {
  return <Gate><Shell /></Gate>
}
