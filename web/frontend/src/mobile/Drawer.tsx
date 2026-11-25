import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { creditsApi } from '@/api/credits'
import { conversationStore, useConversations } from '@/conversation/store'
import { useSession } from '@/lib/session'
import { useAsync } from '@/lib/useAsync'
import { t } from '@/lib/i18n'
import { MIcon } from '@/mobile/icons'
import { useMobileFrame } from '@/mobile/ui'
import { dayLabel, Logo } from '@/ui/kit'
import { Icon } from '@/ui/Icon'
import { commercialMenu, isActiveEntry } from '@/shell/menu'
import { profileStore, useOrganizations } from '@/shell/profile'
import { ProfileMenu } from '@/shell/Sidebar'
import { useWorkspace } from '@/shell/workspace'

/*
 * The drawer of the design, with every menu of the computer's sidebar: new conversation, search, the history by day
 * (rename, archive, delete), the commercial space, settings and the account menu.
 */

const RECENT = 3

export function Drawer({ close }: { close: () => void }) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { user, signedIn } = useSession()
  const { conversations } = useConversations()
  const { openCart, cartCount, unread } = useMobileFrame()
  const [menuFor, setMenuFor] = useState('')
  const [allHistory, setAllHistory] = useState(false)
  const { search: params } = useLocation()
  const { missions } = useWorkspace()
  const { orgs } = useOrganizations(signedIn)
  const profile = profileStore.use()
  const credits = useAsync(() => creditsApi.mine(), [signedIn], signedIn)
  const go = (to: string) => { close(); navigate(to) }

  useEffect(() => {
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') close() }
    document.addEventListener('keydown', esc)
    return () => document.removeEventListener('keydown', esc)
  }, [close])

  const every = conversations.filter((c) => !c.archived).sort((a, b) => b.updatedAt - a.updatedAt).slice(0, 50)
  // The last few conversations, so the menus under them stay within reach.
  const list = allHistory ? every : every.slice(0, RECENT)
  const groups: { label: string; items: typeof list }[] = []
  for (const c of list) {
    const label = dayLabel(c.updatedAt)
    const last = groups[groups.length - 1]
    if (last && last.label === label) last.items.push(c)
    else groups.push({ label, items: [c] })
  }
  const org = profile.kind !== 'personal' ? orgs.find((o) => o.organization_id === profile.organizationId) : undefined
  const name = org?.display_name || user?.display_name || user?.phone || ''
  const commercial = commercialMenu({
    supplier: orgs.length > 0, alerts: unread, onAlerts: () => go('/notifications'),
    activeOperations: (missions.data ?? []).filter((m) => m.status === 'active').length,
  })
  const balance = credits.data && (credits.data.opened || credits.data.meteringActive) ? t('{n} crédits', { n: credits.data.balance }) : t('Profil personnel')

  return (
    <div className="mx-layer">
      <button type="button" className="mx-scrim" onClick={close} aria-label={t('Fermer')} />
      <aside className="mx-drawer" role="dialog" aria-modal="true" aria-label={t('Menu')}>
        <div className="mx-drawer-top"><Logo /><button type="button" className="mx-icon-btn" onClick={close} aria-label={t('Fermer')}><MIcon name="close" /></button></div>
        <button type="button" className="mx-new-chat" onClick={() => go('/')}><MIcon name="edit" size={19} />{t('Nouvelle conversation')}</button>
        <button type="button" className="mx-drawer-search" onClick={() => go('/search')}><MIcon name="search" size={19} /><span>{t('Rechercher')}</span></button>
        {cartCount > 0 && <button type="button" className="mx-drawer-search" onClick={() => { close(); openCart() }}><MIcon name="package" size={19} /><span>{t('Panier ({n})', { n: cartCount })}</span></button>}
        <div className="mx-history">
          <p className="mx-nav-label">{t('Conversations')}</p>
          {groups.length === 0 && <p className="mx-history-empty">{t('Tes conversations apparaîtront ici.')}</p>}
          {groups.map((g) => (
            <div key={g.label}>
              <p className="mx-history-label">{g.label}</p>
              {g.items.map((c) => (
                <div className={`mx-history-item${pathname === `/c/${c.id}` ? ' active' : ''}`} key={c.id}>
                  <button type="button" onClick={() => go(`/c/${c.id}`)}>{c.title}</button>
                  <button type="button" aria-label={t('Actions pour {title}', { title: c.title })} aria-haspopup="menu" aria-expanded={menuFor === c.id} onClick={() => setMenuFor((m) => (m === c.id ? '' : c.id))}><MIcon name="more" size={18} /></button>
                  {menuFor === c.id && (
                    <div className="mx-item-menu" role="menu">
                      <button type="button" role="menuitem" onClick={() => { setMenuFor(''); const next = window.prompt(t('Nouveau titre'), c.title); if (next) conversationStore.rename(c.id, next) }}><MIcon name="edit" size={16} />{t('Renommer')}</button>
                      <button type="button" role="menuitem" onClick={() => { setMenuFor(''); conversationStore.archive(c.id); if (pathname === `/c/${c.id}`) go('/') }}><MIcon name="archive" size={16} />{t('Archiver')}</button>
                      <button type="button" role="menuitem" className="danger" onClick={() => {
                        setMenuFor('')
                        if (window.confirm(t('Supprimer cette conversation ? Ses messages sont effacés de Trezutil ; les commandes et réservations qu’elle a créées restent.'))) { conversationStore.remove(c.id); if (pathname === `/c/${c.id}`) go('/') }
                      }}><MIcon name="trash" size={16} />{t('Supprimer')}</button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          ))}
          {every.length > RECENT && (
            <button type="button" className="mx-history-more" aria-expanded={allHistory} onClick={() => setAllHistory((v) => !v)}>
              {allHistory ? t('Afficher moins') : t('Toutes les conversations ({n})', { n: every.length })}
            </button>
          )}

          <nav className="mx-commercial" aria-label={t('Espace commercial')}>
            <p className="mx-nav-label">{t('Espace commercial')}</p>
            {commercial.map((item) => item.soon ? (
              <span key={item.label} className="mx-nav-item soon" aria-disabled="true"><Icon name={item.icon} size={18} /><span>{item.label}</span><em>{t('Bientôt')}</em></span>
            ) : (
              <button type="button" key={item.label} className={`mx-nav-item${item.to && isActiveEntry(item.to, pathname, params) ? ' active' : ''}`}
                aria-current={item.to && isActiveEntry(item.to, pathname, params) ? 'page' : undefined}
                onClick={() => (item.to ? go(item.to) : item.action?.())}>
                <Icon name={item.icon} size={18} /><span>{item.label}</span>
                {!!item.count && <i aria-label={t('{count} en cours', { count: item.count })}>{item.count}</i>}
              </button>
            ))}
          </nav>
        </div>
        <div className="mx-drawer-bottom">
          <button type="button" className={`mx-nav-item${pathname.startsWith('/settings') ? ' active' : ''}`} onClick={() => go('/settings')}><Icon name="settings" size={18} /><span>{t('Paramètres')}</span></button>
          {user ? (
            <ProfileMenu name={name} subtitle={org ? t('Entreprise · {role}', { role: org.role || t('membre') }) : balance} account={user.email || user.phone}
              orgs={orgs} activeOrg={profile.kind !== 'personal' ? profile.organizationId : ''} collapsed={false} onNavigate={close} />
          ) : user === null ? (
            <button type="button" className="mx-nav-item" onClick={() => go(`/login?returnTo=${encodeURIComponent(pathname)}`)}><Icon name="user" size={18} /><span>{t('Se connecter')}</span></button>
          ) : null}
        </div>
      </aside>
    </div>
  )
}
