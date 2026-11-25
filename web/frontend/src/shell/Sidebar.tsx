import { commercialMenu, isActiveEntry } from '@/shell/menu'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router'
import { useSession } from '@/lib/session'
import { isStaff } from '@/api/admin'
import { setTheme, useTheme, type Theme } from '@/lib/theme'
import { conversationStore, type Conversation } from '@/conversation/store'
import { profileStore, useOrganizations } from '@/shell/profile'
import { Icon } from '@/ui/Icon'
import { dayLabel, initials, Logo } from '@/ui/kit'
import { LANGUAGES, switchLanguage, t, useLang } from '@/lib/i18n'

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

/**
 * The account menu, like ChatGPT's: who is signed in, the profile in use (personal or a business), quick links, and
 * "change account" / "sign out". Signing out clears this device's session (and Google's automatic re-selection).
 */
export function ProfileMenu({ name, subtitle, account, orgs, activeOrg, collapsed, onNavigate }: {
  name: string; subtitle: string; account: string; orgs: { organization_id: string; display_name: string; role?: string }[]
  activeOrg: string; collapsed: boolean; onNavigate: () => void
}) {
  const navigate = useNavigate()
  const { signOut, user } = useSession()
  const theme = useTheme()
  const lang = useLang()
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
      <button type="button" className={`profile ${open ? 'open' : ''}`} aria-haspopup="menu" aria-expanded={open} aria-label={t('Compte : {name}', { name })} onClick={() => setOpen((o) => !o)}>
        <span className="avatar">{initials(name)}</span>
        <span className="profile-copy"><b>{name}</b><small>{subtitle}</small></span>
        <Icon name="more" size={16} />
      </button>
      {open && (
        <div className={`menu up profile-menu ${collapsed ? 'beside' : ''}`} role="menu" aria-label={t('Compte')}>
          <div className="profile-menu-head" role="presentation">
            <span className="avatar">{initials(name)}</span>
            <span><b>{name}</b>{account && <small>{account}</small>}</span>
          </div>
          {orgs.length > 0 && (
            <>
              <div className="menu-sep" role="separator" />
              <button type="button" role="menuitemradio" aria-checked={!activeOrg} onClick={() => choose('')}><Icon name="user" size={15} />{t('Profil personnel')}{!activeOrg && <Icon name="check" size={15} />}</button>
              {orgs.map((o) => (
                <button type="button" role="menuitemradio" key={o.organization_id} aria-checked={activeOrg === o.organization_id} onClick={() => choose(o.organization_id)}>
                  <Icon name="building" size={15} />{o.display_name}{activeOrg === o.organization_id && <Icon name="check" size={15} />}
                </button>
              ))}
            </>
          )}
          <div className="menu-sep" role="separator" />
          <button type="button" role="menuitem" onClick={() => go('/settings/profile')}><Icon name="user" size={15} />{t('Mon profil')}</button>
          <button type="button" role="menuitem" onClick={() => go('/settings/credits')}><Icon name="wallet" size={15} />{t('Crédits et usage')}</button>
          <button type="button" role="menuitem" onClick={() => go('/settings')}><Icon name="settings" size={15} />{t('Paramètres')}</button>
          <button type="button" role="menuitem" onClick={() => go('/supplier')}><Icon name="building" size={15} />{t('Devenir fournisseur')}</button>
          <button type="button" role="menuitem" onClick={() => go('/legal')}><Icon name="shield" size={15} />{t('Informations légales')}</button>
          {isStaff(user) && <button type="button" role="menuitem" onClick={() => go('/admin')}><Icon name="settings" size={15} />{t('Control Center')}</button>}
          <div className="menu-sep" role="separator" />
          <div className="theme-switch" role="radiogroup" aria-label={t('Thème')}>
            {(['system', 'light', 'dark'] as Theme[]).map((mode) => (
              <button type="button" key={mode} role="radio" aria-checked={theme === mode} className={theme === mode ? 'on' : ''} onClick={() => setTheme(mode)}>
                {mode === 'system' ? t('Auto') : mode === 'light' ? t('Clair') : t('Sombre')}
              </button>
            ))}
          </div>
          <div className="theme-switch" role="radiogroup" aria-label={t('Langue')}>
            {LANGUAGES.map((l) => (
              <button type="button" key={l.code} role="radio" lang={l.code} aria-checked={lang === l.code} className={lang === l.code ? 'on' : ''} onClick={() => switchLanguage(l.code)}>
                {l.label}
              </button>
            ))}
          </div>
          <div className="menu-sep" role="separator" />
          <button type="button" role="menuitem" onClick={() => leave('/login')}><Icon name="refresh" size={15} />{t('Changer de compte')}</button>
          <button type="button" role="menuitem" className="danger" onClick={() => leave('/')}><Icon name="logout" size={15} />{t('Se déconnecter')}</button>
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
      <input className="side-search rename" aria-label={t('Nouveau titre')} value={title} autoFocus onChange={(e) => setTitle(e.target.value)}
        onBlur={done} onKeyDown={(e) => { if (e.key === 'Enter') done(); if (e.key === 'Escape') { setTitle(conv.title); setRenaming(false) } }} />
    )
  }
  return (
    <div className={`history-row ${active ? 'active' : ''}`} ref={box}>
      <Link to={`/c/${conv.id}`} className={`history-item ${active ? 'active' : ''}`} onClick={onNavigate} title={conv.title} aria-current={active ? 'page' : undefined}>
        {collapsed ? <Icon name="message" /> : <span>{conv.title}</span>}
        {!collapsed && busy && <i className="busy-dot" aria-label={t('Réponse en cours')} />}
        {!collapsed && !busy && conv.operationId && <i className="op-dot" aria-label={t('Liée à une opération')} />}
      </Link>
      {!collapsed && (
        <button type="button" className="history-more" aria-label={t('Actions pour {title}', { title: conv.title })} aria-haspopup="menu" aria-expanded={menu} onClick={() => setMenu((m) => !m)}>
          <Icon name="more" size={16} />
        </button>
      )}
      {menu && (
        <div className="menu" role="menu">
          <button type="button" role="menuitem" onClick={() => { setMenu(false); setTitle(conv.title); setRenaming(true) }}><Icon name="edit" size={15} />{t('Renommer')}</button>
          <button type="button" role="menuitem" onClick={() => { setMenu(false); conversationStore.archive(conv.id); if (active) navigate('/') }}><Icon name="archive" size={15} />{t('Archiver')}</button>
          <button type="button" role="menuitem" className="danger" onClick={() => {
            setMenu(false)
            if (window.confirm(t('Supprimer cette conversation ? Ses messages sont effacés de Trezutil ; les commandes et réservations qu’elle a créées restent.'))) { conversationStore.remove(conv.id); if (active) navigate('/') }
          }}><Icon name="trash" size={15} />{t('Supprimer')}</button>
        </div>
      )}
    </div>
  )
}

export function Sidebar({ collapsed, onCollapse, mobileOpen, onClose, conversations, pending, activeOperations, onNotifications }: Props) {
  const { user, signedIn } = useSession()
  const { pathname, search: params } = useLocation()
  const isActive = (to: string) => isActiveEntry(to, pathname, params)
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
    return q ? list.filter((c) => norm(c.title).includes(q) || c.turns.some((x) => norm(x.text).includes(q))) : list
  }, [conversations, query])

  const commercial = commercialMenu({ supplier: orgs.length > 0, activeOperations, onAlerts: () => { onNotifications(); onClose() } })

  const org = profile.kind !== 'personal' ? orgs.find((o) => o.organization_id === profile.organizationId) : undefined
  const name = org?.display_name || user?.display_name || user?.phone || ''

  let lastDay = ''
  return (
    <aside className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileOpen ? 'mobile-open' : ''}`} aria-label={t('Navigation')}>
      <div className="side-head">
        <Link to="/" onClick={onClose} aria-label={t('Trezutil, nouvelle conversation')}><Logo compact={collapsed} /></Link>
        <button type="button" className="icon-btn desktop-only" onClick={onCollapse} aria-label={collapsed ? t('Développer le menu') : t('Réduire le menu')} aria-expanded={!collapsed}><Icon name="panel" /></button>
        <button type="button" className="icon-btn mobile-close" onClick={onClose} aria-label={t('Fermer le menu')}><Icon name="x" /></button>
      </div>
      <div className="side-actions">
        <Link className="side-action primary" to="/" onClick={onClose} title={collapsed ? t('Nouvelle conversation') : undefined}><Icon name="edit" /><span>{t('Nouvelle conversation')}</span></Link>
        <button type="button" className="side-action" onClick={() => { if (collapsed) onCollapse(); setSearching((s) => !s); requestAnimationFrame(() => search.current?.focus()) }} aria-expanded={searching} title={collapsed ? t('Rechercher') : undefined}>
          <Icon name="search" /><span>{t('Rechercher')}</span><kbd>{t('Ctrl K')}</kbd>
        </button>
        {searching && !collapsed && (
          <input ref={search} className="side-search" type="search" placeholder={t('Rechercher une conversation…')} aria-label={t('Rechercher une conversation')} value={query}
            onChange={(e) => setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === 'Escape') { setQuery(''); setSearching(false) } }} />
        )}
      </div>

      <div className="history">
        {!collapsed && <p className="side-label">{t('Conversations')}</p>}
        {!collapsed && visible.length === 0 && <p className="history-empty">{query ? t('Aucune conversation ne correspond.') : t('Tes conversations apparaîtront ici.')}</p>}
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

        <nav className="commercial-nav" aria-label={t('Espace commercial')}>
          {!collapsed && <p className="side-label">{t('Espace commercial')}</p>}
          {commercial.map((item) => item.soon ? (
            <span key={item.label} className="side-action commercial-link soon" aria-disabled="true" title={t('{label} — bientôt disponible', { label: item.label })}>
              <Icon name={item.icon} /><span>{item.label}</span><em>{t('Bientôt')}</em>
            </span>
          ) : item.to ? (
            <Link key={item.label} to={item.to} onClick={onClose} title={item.label} aria-current={isActive(item.to) ? 'page' : undefined}
              className={`side-action commercial-link ${isActive(item.to) ? 'active' : ''}`}>
              <Icon name={item.icon} /><span>{item.label}</span>{!!item.count && <i aria-label={t('{count} en cours', { count: item.count })}>{item.count}</i>}
            </Link>
          ) : (
            <button type="button" key={item.label} className="side-action commercial-link" onClick={item.action} title={item.label}><Icon name={item.icon} /><span>{item.label}</span></button>
          ))}
        </nav>
      </div>

      <div className="side-bottom">
        <NavLink to="/settings" onClick={onClose} className={({ isActive }) => `side-action ${isActive ? 'active' : ''}`} title={t('Paramètres')}><Icon name="settings" /><span>{t('Paramètres')}</span></NavLink>
        {user === undefined ? null : user ? (
          <ProfileMenu name={name} subtitle={org ? t('Entreprise · {role}', { role: org.role || t('membre') }) : t('Profil personnel')} account={user.email || user.phone}
            orgs={orgs} activeOrg={profile.kind !== 'personal' ? profile.organizationId : ''} collapsed={collapsed} onNavigate={onClose} />
        ) : (
          <Link to={`/login?returnTo=${encodeURIComponent(pathname)}`} onClick={onClose} className="side-action sign-in"><Icon name="user" /><span>{t('Se connecter')}</span><Icon name="arrow" size={15} /></Link>
        )}
      </div>
    </aside>
  )
}
