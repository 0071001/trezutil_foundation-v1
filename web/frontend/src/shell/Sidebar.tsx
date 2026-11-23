import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router'
import { useSession } from '@/lib/session'
import { setTheme, useTheme, type Theme } from '@/lib/theme'
import { conversationStore, type Conversation } from '@/conversation/store'
import { profileStore, useOrganizations } from '@/shell/profile'
import { Icon, type IconName } from '@/ui/Icon'
import { dayLabel, initials, Logo } from '@/ui/kit'

/*
 * The sidebar of the design: new conversation, search, the conversations by day (rename, archive, delete), the
 * commercial space, then settings and the account. Entries whose backend service does not exist yet are marked
 * "Bientôt" and do nothing, so they are never mistaken for working ones.
 */

interface Props {
  collapsed: boolean
  onCollapse: () => void
  mobileOpen: boolean
  onClose: () => void
  conversations: Conversation[]
  pending: Record<string, string>
  activeOperations: number
  onNotifications: () => void
}

interface CommercialItem { icon: IconName; label: string; to?: string; action?: () => void; soon?: boolean; count?: number }

/**
 * The account menu, like ChatGPT's: who is signed in, the profile in use (personal or a business), quick links, and
 * "change account" / "sign out". Signing out clears this device's session (and Google's automatic re-selection).
 */
function ProfileMenu({ name, subtitle, account, orgs, activeOrg, collapsed, onNavigate }: {
  name: string; subtitle: string; account: string; orgs: { organization_id: string; display_name: string; role?: string }[]
  activeOrg: string; collapsed: boolean; onNavigate: () => void
}) {
  const navigate = useNavigate()
  const { signOut } = useSession()
  const theme = useTheme()
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false) }
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', esc)
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', esc) }
  }, [open])
  const go = (to: string) => { setOpen(false); onNavigate(); navigate(to) }
  const leave = (to: string) => { setOpen(false); onNavigate(); signOut(); navigate(to) }
  const choose = (organizationId: string) => {
    profileStore.set(organizationId ? { kind: 'company', organizationId } : { kind: 'personal', organizationId: '' })
    setOpen(false)
  }
  return (
    <div className="profile-box" ref={box}>
      <button type="button" className={`profile ${open ? 'open' : ''}`} aria-haspopup="menu" aria-expanded={open} aria-label={`Compte : ${name}`} onClick={() => setOpen((o) => !o)}>
        <span className="avatar">{initials(name)}</span>
        <span className="profile-copy"><b>{name}</b><small>{subtitle}</small></span>
        <Icon name="more" size={16} />
      </button>
      {open && (
        <div className={`menu up profile-menu ${collapsed ? 'beside' : ''}`} role="menu" aria-label="Compte">
          <div className="profile-menu-head" role="presentation">
            <span className="avatar">{initials(name)}</span>
            <span><b>{name}</b>{account && <small>{account}</small>}</span>
          </div>
          {orgs.length > 0 && (
            <>
              <div className="menu-sep" role="separator" />
              <button type="button" role="menuitemradio" aria-checked={!activeOrg} onClick={() => choose('')}><Icon name="user" size={15} />Profil personnel{!activeOrg && <Icon name="check" size={15} />}</button>
              {orgs.map((o) => (
                <button type="button" role="menuitemradio" key={o.organization_id} aria-checked={activeOrg === o.organization_id} onClick={() => choose(o.organization_id)}>
                  <Icon name="building" size={15} />{o.display_name}{activeOrg === o.organization_id && <Icon name="check" size={15} />}
                </button>
              ))}
            </>
          )}
          <div className="menu-sep" role="separator" />
          <button type="button" role="menuitem" onClick={() => go('/settings/profile')}><Icon name="user" size={15} />Mon profil</button>
          <button type="button" role="menuitem" onClick={() => go('/settings/credits')}><Icon name="wallet" size={15} />Crédits et usage</button>
          <button type="button" role="menuitem" onClick={() => go('/settings')}><Icon name="settings" size={15} />Paramètres</button>
          <button type="button" role="menuitem" onClick={() => go('/supplier')}><Icon name="building" size={15} />Devenir fournisseur</button>
          <button type="button" role="menuitem" onClick={() => go('/legal')}><Icon name="shield" size={15} />Informations légales</button>
          <div className="menu-sep" role="separator" />
          <div className="theme-switch" role="radiogroup" aria-label="Thème">
            {(['system', 'light', 'dark'] as Theme[]).map((t) => (
              <button type="button" key={t} role="radio" aria-checked={theme === t} className={theme === t ? 'on' : ''} onClick={() => setTheme(t)}>
                {t === 'system' ? 'Auto' : t === 'light' ? 'Clair' : 'Sombre'}
              </button>
            ))}
          </div>
          <div className="menu-sep" role="separator" />
          <button type="button" role="menuitem" onClick={() => leave('/login')}><Icon name="refresh" size={15} />Changer de compte</button>
          <button type="button" role="menuitem" className="danger" onClick={() => leave('/')}><Icon name="logout" size={15} />Se déconnecter</button>
        </div>
      )}
    </div>
  )
}

function norm(s: string) { return s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '') }

function HistoryItem({ conv, active, busy, collapsed, onNavigate }: { conv: Conversation; active: boolean; busy: boolean; collapsed: boolean; onNavigate: () => void }) {
  const navigate = useNavigate()
  const [menu, setMenu] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [title, setTitle] = useState(conv.title)
  const box = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menu) return
    const close = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setMenu(false) }
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') setMenu(false) }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', esc)
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', esc) }
  }, [menu])

  if (renaming && !collapsed) {
    const done = () => { conversationStore.rename(conv.id, title); setRenaming(false) }
    return (
      <input className="side-search rename" aria-label="Nouveau titre" value={title} autoFocus onChange={(e) => setTitle(e.target.value)}
        onBlur={done} onKeyDown={(e) => { if (e.key === 'Enter') done(); if (e.key === 'Escape') { setTitle(conv.title); setRenaming(false) } }} />
    )
  }
  return (
    <div className={`history-row ${active ? 'active' : ''}`} ref={box}>
      <Link to={`/c/${conv.id}`} className={`history-item ${active ? 'active' : ''}`} onClick={onNavigate} title={conv.title} aria-current={active ? 'page' : undefined}>
        {collapsed ? <Icon name="message" /> : <span>{conv.title}</span>}
        {!collapsed && busy && <i className="busy-dot" aria-label="Réponse en cours" />}
        {!collapsed && !busy && conv.operationId && <i className="op-dot" aria-label="Liée à une opération" />}
      </Link>
      {!collapsed && (
        <button type="button" className="history-more" aria-label={`Actions pour ${conv.title}`} aria-haspopup="menu" aria-expanded={menu} onClick={() => setMenu((m) => !m)}>
          <Icon name="more" size={16} />
        </button>
      )}
      {menu && (
        <div className="menu" role="menu">
          <button type="button" role="menuitem" onClick={() => { setMenu(false); setTitle(conv.title); setRenaming(true) }}><Icon name="edit" size={15} />Renommer</button>
          <button type="button" role="menuitem" onClick={() => { setMenu(false); conversationStore.archive(conv.id); if (active) navigate('/') }}><Icon name="archive" size={15} />Archiver</button>
          <button type="button" role="menuitem" className="danger" onClick={() => {
            setMenu(false)
            if (window.confirm('Supprimer cette conversation ? Ses messages sont effacés de Trezutil ; les commandes et réservations qu’elle a créées restent.')) { conversationStore.remove(conv.id); if (active) navigate('/') }
          }}><Icon name="trash" size={15} />Supprimer</button>
        </div>
      )}
    </div>
  )
}

export function Sidebar({ collapsed, onCollapse, mobileOpen, onClose, conversations, pending, activeOperations, onNotifications }: Props) {
  const { user, signedIn } = useSession()
  const { pathname, search: params } = useLocation()
  const isActive = (to: string) => {
    const [path, q] = to.split('?')
    return pathname.startsWith(path!) && (q ? params.includes(q) : !params.includes('tab='))
  }
  const [searching, setSearching] = useState(false)
  const [query, setQuery] = useState('')
  const profile = profileStore.use()
  const { orgs } = useOrganizations(signedIn)
  const currentId = pathname.match(/^\/c\/([^/]+)/)?.[1] ?? ''
  const search = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setSearching(true); requestAnimationFrame(() => search.current?.focus()) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const visible = useMemo(() => {
    const list = conversations.filter((c) => !c.archived)
    const q = norm(query.trim())
    return q ? list.filter((c) => norm(c.title).includes(q) || c.turns.some((t) => norm(t.text).includes(q))) : list
  }, [conversations, query])

  const commercial: CommercialItem[] = [
    { icon: 'target', label: 'Vue d’ensemble', to: '/overview' },
    { icon: 'flow', label: 'Opérations', to: '/operations', count: activeOperations },
    { icon: 'handshake', label: 'Négociations', soon: true },
    { icon: 'orders', label: 'Achats et commandes', to: '/purchases' },
    { icon: 'calendar', label: 'Réservations et organisation', to: '/purchases?tab=bookings' },
    { icon: 'building', label: 'Fournisseurs', to: '/suppliers' },
    { icon: 'refresh', label: 'Automatisations', soon: true },
    { icon: 'bell', label: 'Alertes et opportunités', action: () => { onNotifications(); onClose() } },
  ]

  const org = profile.kind !== 'personal' ? orgs.find((o) => o.organization_id === profile.organizationId) : undefined
  const name = org?.display_name || user?.display_name || user?.phone || ''

  let lastDay = ''
  return (
    <aside className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileOpen ? 'mobile-open' : ''}`} aria-label="Navigation">
      <div className="side-head">
        <Link to="/" onClick={onClose} aria-label="Trezutil, nouvelle conversation"><Logo compact={collapsed} /></Link>
        <button type="button" className="icon-btn desktop-only" onClick={onCollapse} aria-label={collapsed ? 'Développer le menu' : 'Réduire le menu'} aria-expanded={!collapsed}><Icon name="panel" /></button>
        <button type="button" className="icon-btn mobile-close" onClick={onClose} aria-label="Fermer le menu"><Icon name="x" /></button>
      </div>
      <div className="side-actions">
        <Link className="side-action primary" to="/" onClick={onClose} title={collapsed ? 'Nouvelle conversation' : undefined}><Icon name="edit" /><span>Nouvelle conversation</span></Link>
        <button type="button" className="side-action" onClick={() => { if (collapsed) onCollapse(); setSearching((s) => !s); requestAnimationFrame(() => search.current?.focus()) }} aria-expanded={searching} title={collapsed ? 'Rechercher' : undefined}>
          <Icon name="search" /><span>Rechercher</span><kbd>Ctrl K</kbd>
        </button>
        {searching && !collapsed && (
          <input ref={search} className="side-search" type="search" placeholder="Rechercher une conversation…" aria-label="Rechercher une conversation" value={query}
            onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === 'Escape') { setQuery(''); setSearching(false) } }} />
        )}
      </div>

      <div className="history">
        {!collapsed && <p className="side-label">Conversations</p>}
        {!collapsed && visible.length === 0 && <p className="history-empty">{query ? 'Aucune conversation ne correspond.' : 'Tes conversations apparaîtront ici.'}</p>}
        {visible.slice(0, 50).map((c) => {
          const day = dayLabel(c.updatedAt)
          const head = day !== lastDay ? day : ''
          lastDay = day
          return (
            <div key={c.id}>
              {!collapsed && head && <p className="history-group">{head}</p>}
              <HistoryItem conv={c} active={c.id === currentId} busy={!!pending[c.id]} collapsed={collapsed} onNavigate={onClose} />
            </div>
          )
        })}

        <nav className="commercial-nav" aria-label="Espace commercial">
          {!collapsed && <p className="side-label">Espace commercial</p>}
          {commercial.map((item) => item.soon ? (
            <span key={item.label} className="side-action commercial-link soon" aria-disabled="true" title={`${item.label} — bientôt disponible`}>
              <Icon name={item.icon} /><span>{item.label}</span><em>Bientôt</em>
            </span>
          ) : item.to ? (
            <Link key={item.label} to={item.to} onClick={onClose} title={item.label} aria-current={isActive(item.to) ? 'page' : undefined}
              className={`side-action commercial-link ${isActive(item.to) ? 'active' : ''}`}>
              <Icon name={item.icon} /><span>{item.label}</span>{!!item.count && <i aria-label={`${item.count} en cours`}>{item.count}</i>}
            </Link>
          ) : (
            <button type="button" key={item.label} className="side-action commercial-link" onClick={item.action} title={item.label}><Icon name={item.icon} /><span>{item.label}</span></button>
          ))}
        </nav>
      </div>

      <div className="side-bottom">
        <NavLink to="/settings" onClick={onClose} className={({ isActive }) => `side-action ${isActive ? 'active' : ''}`} title="Paramètres"><Icon name="settings" /><span>Paramètres</span></NavLink>
        {user === undefined ? null : user ? (
          <ProfileMenu name={name} subtitle={org ? `Entreprise · ${org.role || 'membre'}` : 'Profil personnel'} account={user.email || user.phone}
            orgs={orgs} activeOrg={profile.kind !== 'personal' ? profile.organizationId : ''} collapsed={collapsed} onNavigate={onClose} />
        ) : (
          <Link to={`/login?returnTo=${encodeURIComponent(pathname)}`} onClick={onClose} className="side-action sign-in"><Icon name="user" /><span>Se connecter</span><Icon name="arrow" size={15} /></Link>
        )}
      </div>
    </aside>
  )
}
