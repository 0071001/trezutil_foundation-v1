import { useEffect, useRef, useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router'
import { useSession } from '@/lib/session'
import { conversationStore, useConversations } from '@/conversation/store'
import { setMemoryOwner } from '@/memory/memory'
import { NotificationsDrawer } from '@/notifications/NotificationsDrawer'
import { CartDrawer } from '@/commerce/CartDrawer'
import { useCart } from '@/commerce/cart'
import { noticePrefs, useNotices } from '@/notifications/notifications'
import { profileStore } from '@/shell/profile'
import { Sidebar } from '@/shell/Sidebar'
import { useWorkspace, WorkspaceProvider } from '@/shell/workspace'
import { Icon } from '@/ui/Icon'
import { initials, OfflineBar, useOnline } from '@/ui/kit'
import { activeTab, TabBar } from '@/mobile/TabBar'
import { useMobile } from '@/lib/useMobile'
import { UnreadContext } from '@/mobile/parts'
import { t } from '@/lib/i18n'

/*
 * The frame: the sidebar (foldable on a large screen, a drawer on a phone), the top bar with the title of what is open,
 * the notifications, and the screen. The data on this device (conversations, memory, preferences) follows the account.
 */

const SIDE_KEY = 'trezutil_side_collapsed'

function titleOf(pathname: string, conversationTitle: string | undefined): string {
  if (pathname.startsWith('/c/')) return conversationTitle ?? t('Conversation')
  if (pathname.startsWith('/operations')) return t('Opérations')
  if (pathname.startsWith('/purchases')) return t('Achats et réservations')
  if (pathname.startsWith('/suppliers')) return t('Fournisseurs')
  if (pathname === '/supplier') return t('Devenir fournisseur')
  if (pathname.startsWith('/overview')) return t('Vue d’ensemble')
  if (pathname.startsWith('/settings')) return t('Paramètres')
  return t('Nouvelle conversation')
}

export function Shell() {
  return <WorkspaceProvider><Frame /></WorkspaceProvider>
}

function ConversationMenu({ id, title, operationId, more = false }: { id: string; title: string; operationId?: string; more?: boolean }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open])
  return (
    <div className="top-title-wrap" ref={box}>
      {more
        ? <button type="button" className="m-icon-btn" onClick={() => setOpen((o) => !o)} aria-haspopup="menu" aria-expanded={open} aria-label={t('Options de la conversation')}><Icon name="more" size={18} /></button>
        : <button type="button" className="top-title" onClick={() => setOpen((o) => !o)} aria-haspopup="menu" aria-expanded={open}><span>{title}</span><Icon name="chevron" size={14} /></button>}
      {open && (
        <div className="menu" role="menu">
          <button type="button" role="menuitem" onClick={() => { setOpen(false); const next = window.prompt(t('Nouveau titre'), title); if (next) conversationStore.rename(id, next) }}><Icon name="edit" size={15} />{t('Renommer')}</button>
          {operationId && <button type="button" role="menuitem" onClick={() => { setOpen(false); navigate(`/operations/${operationId}`) }}><Icon name="flow" size={15} />{t('Voir l’opération')}</button>}
          <button type="button" role="menuitem" onClick={() => { setOpen(false); conversationStore.archive(id); navigate('/') }}><Icon name="archive" size={15} />{t('Archiver')}</button>
          <button type="button" role="menuitem" className="danger" onClick={() => { setOpen(false); if (window.confirm(t('Supprimer cette conversation ? Ses messages sont effacés de Trezutil ; les commandes et réservations qu’elle a créées restent.'))) { conversationStore.remove(id); navigate('/') } }}><Icon name="trash" size={15} />{t('Supprimer')}</button>
        </div>
      )}
    </div>
  )
}

function Frame() {
  const { user } = useSession()
  const { conversations, pending } = useConversations()
  const { missions, orders } = useWorkspace()
  const { pathname } = useLocation()
  const [collapsed, setCollapsed] = useState(() => { try { return localStorage.getItem(SIDE_KEY) === '1' } catch { return false } })
  const [mobileOpen, setMobileOpen] = useState(false)
  const [drawer, setDrawer] = useState(false)
  const { notices, unread, prefs } = useNotices(conversations, missions.data ?? [], orders.data ?? [])

  // Conversations, memory and preferences on this device belong to the account (or to the guest).
  useEffect(() => {
    if (user === undefined) return
    const owner = user?.id ?? ''
    conversationStore.setOwner(owner)
    setMemoryOwner(owner)
    noticePrefs.setOwner(owner)
    profileStore.setOwner(owner)
    // The account's conversations held on the server (other devices, earlier visits) join the list.
    if (owner) void conversationStore.syncFromServer()
  }, [user])

  // A thread written in another tab shows up here too.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => { if (e.key?.startsWith('trezutil_conversations:')) conversationStore.reload() }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  useEffect(() => { setMobileOpen(false) }, [pathname])

  const convId = pathname.match(/^\/c\/([^/]+)/)?.[1]
  const conv = conversations.find((c) => c.id === convId)
  const title = titleOf(pathname, conv?.title)
  const wide = pathname.startsWith('/c/') && pathname.includes('/compare') || /^\/operations\/.+/.test(pathname)
  const name = user?.display_name || user?.phone || ''
  const cartItems = useCart()
  const cartCount = cartItems.reduce((n, i) => n + i.quantity, 0)
  const [cartOpen, setCartOpen] = useState(false)
  const navigate = useNavigate()
  const online = useOnline()
  const mobile = useMobile()
  const tab = activeTab(pathname)
  // On a phone the home, Discover, Activity and Profile draw their own header (Design Trezutil Mobile App).
  // The mobile screens open the cart and the notifications through the frame.
  useEffect(() => {
    const open = (e: Event) => { const which = (e as CustomEvent<string>).detail; if (which === 'cart') setCartOpen(true); if (which === 'notifications') setDrawer(true) }
    window.addEventListener('trezutil:open', open)
    return () => window.removeEventListener('trezutil:open', open)
  }, [])
  const ownHeader = pathname === '/' || pathname === '/discover' || pathname === '/activity' || pathname === '/profile'

  return (
    <div className={`app-shell${tab ? ` m-tab-${tab}` : ''}`}>
      <a className="skip-link" href="#main">{t('Aller au contenu')}</a>
      {/* On a phone the tab bar replaces the sidebar. */}
      {!mobile && <Sidebar
        collapsed={collapsed} onCollapse={() => setCollapsed((c) => { try { localStorage.setItem(SIDE_KEY, c ? '0' : '1') } catch { /* visit only */ } return !c })}
        mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)} conversations={conversations} pending={pending}
        activeOperations={(missions.data ?? []).filter((m) => m.status === 'active').length} onNotifications={() => setDrawer(true)}
      />}
      {mobileOpen && <div className="mobile-scrim" onClick={() => setMobileOpen(false)} aria-hidden="true" />}
      <main className="main-shell" id="main" tabIndex={-1}>
        {mobile && conv && (
          <header className="m-chat-head">
            <button type="button" className="m-icon-btn" onClick={() => navigate('/')} aria-label={t('Retour à l’accueil')}><Icon name="back" size={18} /></button>
            <div className="m-chat-title">
              <b>Trezutil</b>
              <small><i className={online ? 'on' : ''} />{online ? t('Disponible') : t('Hors ligne')}</small>
            </div>
            <div className="m-chat-actions">
              {cartCount > 0 && <button type="button" className="m-icon-btn m-badge" onClick={() => setCartOpen(true)} aria-label={t('Panier, {n} article(s)', { n: cartCount })}><Icon name="cart" size={18} /><i>{cartCount > 9 ? '9+' : cartCount}</i></button>}
              <ConversationMenu id={conv.id} title={title} operationId={conv.operationId} more />
            </div>
          </header>
        )}
        {!wide && !(mobile && (conv || ownHeader)) && (
          <header className={`topbar${ownHeader ? ' m-own-header' : ''}${conv ? ' m-chat-topbar' : ''}`}>
            <button type="button" className="icon-btn menu-trigger" onClick={() => setMobileOpen(true)} aria-label={t('Ouvrir le menu')}><Icon name="menu" /></button>
            {mobile && <button type="button" className="icon-btn m-back" onClick={() => (window.history.length > 1 ? navigate(-1) : navigate('/'))} aria-label={t('Retour')}><Icon name="back" /></button>}
            {conv ? <ConversationMenu id={conv.id} title={title} operationId={conv.operationId} /> : <div className="top-title"><span>{title}</span></div>}
            <div className="top-actions">
              {cartCount > 0 && (
                <button type="button" className="icon-btn notification-btn" onClick={() => setCartOpen(true)} aria-label={t('Panier, {n} article(s)', { n: cartCount })}>
                  <Icon name="cart" /><i>{cartCount > 9 ? '9+' : cartCount}</i>
                </button>
              )}
              <button type="button" className="icon-btn notification-btn" onClick={() => setDrawer(true)} aria-label={unread ? t('Notifications, {unread} non lues', { unread }) : t('Notifications')}>
                <Icon name="bell" />{unread > 0 && <i>{unread > 9 ? '9+' : unread}</i>}
              </button>
              {user ? <Link className="avatar small" to="/settings/profile" aria-label={t('Mon profil')}>{initials(name)}</Link> : user === null ? <Link className="secondary-btn small-btn" to={`/login?returnTo=${encodeURIComponent(pathname)}`}>{t('Se connecter')}</Link> : null}
            </div>
          </header>
        )}
        <OfflineBar />
        <div className="view-container"><UnreadContext.Provider value={{ unread }}><Outlet /></UnreadContext.Provider></div>
        {mobile && <TabBar />}
      </main>
      {drawer && <NotificationsDrawer notices={notices} read={prefs.read} onClose={() => setDrawer(false)} />}
      {cartOpen && <CartDrawer onClose={() => setCartOpen(false)} />}
    </div>
  )
}
