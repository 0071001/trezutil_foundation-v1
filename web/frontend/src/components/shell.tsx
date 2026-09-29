import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useLocation } from 'react-router'
import { ActionLink, Brand, Button, Icon } from '@/components/ui'
import { OfflineBanner } from '@/components/states'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'

const primaryNav: [string, string][] = [
  ['Discover', '/discover'],
  ['Products', '/search?type=products'],
  ['Services', '/search?type=services'],
  ['Real Estate', '/search?type=property'],
  ['Reservations', '/reservations'],
]

export function initials(name: string | undefined, fallback = 'T'): string {
  const parts = (name ?? '').trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return fallback
  return (parts[0][0] + (parts[1]?.[0] ?? '')).toUpperCase()
}

/** The accessible mobile menu (the Figma menu button opens it). */
function MobileMenu({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { signedIn, signOut } = useSession()
  const panel = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    panel.current?.querySelector<HTMLElement>('a, button')?.focus()
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => { document.removeEventListener('keydown', onKey); document.body.style.overflow = ''; previous?.focus() }
  }, [open, onClose])
  if (!open) return null
  return (
    <div className="drawer-backdrop" onClick={onClose}>
      <div ref={panel} className="drawer" role="dialog" aria-modal="true" aria-label="Menu" onClick={(e) => e.stopPropagation()}>
        <div className="drawer-head"><Brand /><Button className="icon-button" ariaLabel="Close menu" onClick={onClose}><Icon name="close" /></Button></div>
        <nav aria-label="Menu">
          {primaryNav.map(([label, href]) => <ActionLink key={label} href={href}>{label}</ActionLink>)}
          <ActionLink href="/conversation"><Icon name="spark" size={17} />Ask Trezutil</ActionLink>
          <ActionLink href="/orders">Orders</ActionLink>
          <ActionLink href="/wishlist">Saved</ActionLink>
          <ActionLink href="/cart">Bag</ActionLink>
          <ActionLink href="/seller">Seller workspace</ActionLink>
          {signedIn ? <><ActionLink href="/account">Account</ActionLink><Button className="drawer-signout" onClick={() => { signOut(); onClose() }}>Sign out</Button></>
            : <ActionLink href="/login">Sign in</ActionLink>}
        </nav>
      </div>
    </div>
  )
}

function Navbar() {
  const { theme, toggleTheme, bag } = usePrefs()
  const { user } = useSession()
  const [menu, setMenu] = useState(false)
  const location = useLocation()
  useEffect(() => setMenu(false), [location.pathname])
  const bagCount = bag.reduce((n, b) => n + b.quantity, 0)
  return (
    <header className="navbar product-navbar">
      <div className="nav-inner">
        <Brand />
        <nav className="desktop-nav" aria-label="Primary">
          {primaryNav.map(([label, href]) => <ActionLink key={label} href={href}>{label}</ActionLink>)}
        </nav>
        <ActionLink className="nav-search" href="/search"><Icon name="search" size={17} /><span>What are you looking for?</span></ActionLink>
        <div className="nav-actions">
          <span className="icon-button location-button" title="Trezutil currently serves Kinshasa"><Icon name="location" size={18} /><span>Kinshasa</span></span>
          <ActionLink className="icon-button" href="/notifications" ariaLabel="Notifications"><Icon name="bell" size={18} /></ActionLink>
          <ActionLink className="icon-button nav-bag" href="/cart" ariaLabel={`Bag, ${bagCount} item${bagCount === 1 ? '' : 's'}`}><Icon name="orders" size={18} />{bagCount > 0 && <span className="nav-count" aria-hidden="true">{bagCount}</span>}</ActionLink>
          {user
            ? <ActionLink className="icon-button nav-avatar" href="/account" ariaLabel="Your account"><span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span></ActionLink>
            : <ActionLink className="icon-button" href="/login" ariaLabel="Sign in"><Icon name="user" size={18} /></ActionLink>}
          <Button className="icon-button" ariaLabel={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
          <ActionLink className="whatsapp-mini" href="/conversation"><Icon name="spark" size={17} /><span>Ask Trezutil</span></ActionLink>
        </div>
        <Button className="mobile-menu icon-button" ariaLabel="Open menu" onClick={() => setMenu(true)}><Icon name="menu" /></Button>
      </div>
      <MobileMenu open={menu} onClose={() => setMenu(false)} />
    </header>
  )
}

function MobileNav() {
  return (
    <nav className="mobile-bottom" aria-label="Mobile navigation">
      <ActionLink href="/" nav end><Icon name="home" size={20} /><span>Home</span></ActionLink>
      <ActionLink href="/discover" nav><Icon name="search" size={20} /><span>Discover</span></ActionLink>
      <ActionLink href="/conversation" nav><Icon name="spark" size={20} /><span>Ask</span></ActionLink>
      <ActionLink href="/orders" nav><Icon name="orders" size={20} /><span>Orders</span></ActionLink>
      <ActionLink href="/account" nav><Icon name="user" size={20} /><span>Account</span></ActionLink>
    </nav>
  )
}

export function ConversationDock({ label = 'Ask Trezutil anything about this page', prompt }: { label?: string; prompt?: string }) {
  return (
    <ActionLink className="conversation-dock" href={prompt ? `/conversation?q=${encodeURIComponent(prompt)}` : '/conversation'}>
      <span className="dock-logo"><Icon name="spark" /></span>
      <span><strong>{label}</strong><small>Compare, refine or find something nearby</small></span>
      <Icon name="arrow" />
    </ActionLink>
  )
}

export function AppShell({ children, compact = false, hideDock = false, dockPrompt }: { children: ReactNode; compact?: boolean; hideDock?: boolean; dockPrompt?: string }) {
  const { theme } = usePrefs()
  return (
    <div className={`app product-app ${theme === 'dark' ? 'dark' : ''}`}>
      <Navbar />
      <OfflineBanner />
      <main id="main" tabIndex={-1} className={compact ? 'app-content compact-content' : 'app-content'}>{children}</main>
      {!hideDock && <ConversationDock prompt={dockPrompt} />}
      <MobileNav />
    </div>
  )
}
