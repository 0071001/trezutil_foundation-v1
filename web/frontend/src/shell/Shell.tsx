import { preloadTrezy } from '@/ui/Trezy'
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
import { initials, OfflineBar } from '@/ui/kit'
import { Drawer } from '@/mobile/Drawer'
import { AppHeader, MobileFrameContext } from '@/mobile/ui'
import { haptic, mobilePrefs } from '@/mobile/prefs'
import { useMobile } from '@/lib/useMobile'
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
  if (pathname.startsWith('/supplier/requests')) return t('Demandes de prix')
  if (pathname.startsWith('/supplier/offers')) return t('Mes offres')
  if (pathname.startsWith('/negotiations/')) return t('Négociation')
  if (pathname.startsWith('/negotiations')) return t('Négociations')
  if (pathname.startsWith('/overview')) return t('Vue d’ensemble')
  if (pathname.startsWith('/settings')) return t('Paramètres')
  return t('Nouvelle conversation')
}

export function Shell() {
  return <WorkspaceProvider><Frame /></WorkspaceProvider>
}

function ConversationMenu({ id, title, operationId }: { id: string; title: string; operationId?: string }) {
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
      <button type="button" className="top-title" onClick={() => setOpen((o) => !o)} aria-haspopup="menu" aria-expanded={open}><span>{title}</span><Icon name="chevron" size={14} /></button>
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

  useEffect(() => { preloadTrezy('je-m-en-occupe', 'commande-reussie') }, [])

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
  const mobile = useMobile()
  const platform = mobilePrefs.use().platform
  const [menu, setMenu] = useState(false)
  useEffect(() => { setMenu(false) }, [pathname])
  // A short vibration on taps, where the person kept it on.
  useEffect(() => {
    if (!mobile) return
    const tap = (e: Event) => { if ((e.target as Element | null)?.closest?.('button, a')) haptic() }
    document.addEventListener('click', tap)
    return () => document.removeEventListener('click', tap)
  }, [mobile])
  // On a phone these screens draw the header of the mobile design themselves; the others get a back header.
  const ownHeader = pathname === '/' || pathname.startsWith('/c/') || pathname === '/search' || pathname === '/profile' || pathname === '/notifications'
    || pathname === '/operations' || pathname === '/settings' || pathname === '/settings/credits' || pathname.startsWith('/confirmed/')

  // List pages whose heading would repeat the header's title.
  const titled = ['/overview', '/purchases', '/bookings', '/negotiations', '/suppliers', '/supplier/requests', '/supplier/offers'].includes(pathname)

  if (mobile) {
    return (
      <MobileFrameContext.Provider value={{ openDrawer: () => setMenu(true), openCart: () => setCartOpen(true), cartCount, unread }}>
        <div className={`mx-app ${platform}`}>
          <a className="skip-link" href="#main">{t('Aller au contenu')}</a>
          <main className={`mx-main${titled ? ' mx-titled' : ''}`} id="main" tabIndex={-1}>
            {!ownHeader && <AppHeader title={title} back />}
            <OfflineBar />
            <div className="view-container"><Outlet /></div>
          </main>
          {menu && <Drawer close={() => setMenu(false)} />}
          {cartOpen && <CartDrawer onClose={() => setCartOpen(false)} />}
        </div>
      </MobileFrameContext.Provider>
    )
  }

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main">{t('Aller au contenu')}</a>
      <Sidebar
        collapsed={collapsed} onCollapse={() => setCollapsed((c) => { try { localStorage.setItem(SIDE_KEY, c ? '0' : '1') } catch { /* visit only */ } return !c })}
        mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)} conversations={conversations} pending={pending}
        activeOperations={(missions.data ?? []).filter((m) => m.status === 'active').length} onNotifications={() => setDrawer(true)}
      />
      {mobileOpen && <div className="mobile-scrim" onClick={() => setMobileOpen(false)} aria-hidden="true" />}
      <main className="main-shell" id="main" tabIndex={-1}>
        {!wide && (
          <header className="topbar">
            <button type="button" className="icon-btn menu-trigger" onClick={() => setMobileOpen(true)} aria-label={t('Ouvrir le menu')}><Icon name="menu" /></button>
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
        <div className="view-container"><Outlet /></div>
      </main>
      {drawer && <NotificationsDrawer notices={notices} read={prefs.read} onClose={() => setDrawer(false)} />}
      {cartOpen && <CartDrawer onClose={() => setCartOpen(false)} />}
    </div>
  )
}
