import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useLocation } from 'react-router'
import { useChatWidget } from '@/components/ChatWidget'
import { ActionLink, Brand, Button, Icon, type IconName } from '@/components/ui'
import { OfflineBanner } from '@/components/states'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { LANGUAGES, setLang, t, tn, tx, useLang } from '@/lib/i18n'
import { MENU_NEEDS, otherSearches } from '@/lib/needs'

/**
 * The primary menu is the needs Trezutil helps with (src/lib/needs.ts), in the
 * order of the home page, then Discover to browse. It used to be the list of
 * catalogue categories (Products, Services, Real Estate) plus "Reservations",
 * which is a personal page and now lives with Orders and Saved.
 * See docs/web/PRIMARY_MENU.md.
 */
export const primaryNav: [string, string, IconName][] = [
  ...MENU_NEEDS.map((need): [string, string, IconName] => [need.title, need.domains[0]![1], need.icon]),
  [tx('Discover'), '/discover', 'search'],
]

/** The primary menu entries, each with its icon (the homepage adds its own extras). */
export function PrimaryNavLinks({ extra = [], icons = true }: { extra?: [string, string, IconName][]; icons?: boolean }) {
  return <>{[...primaryNav, ...extra].map(([label, href, icon]) => <ActionLink key={href + label} href={href}>{icons && <Icon name={icon} size={17} />}{t(label)}</ActionLink>)}</>
}

export function initials(name: string | undefined, fallback = 'T'): string {
  const parts = (name ?? '').trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return fallback
  return (parts[0][0] + (parts[1]?.[0] ?? '')).toUpperCase()
}

/** French / English switch; the choice is remembered on this device. */
export function LanguageSwitch({ className = '' }: { className?: string }) {
  const lang = useLang()
  return (
    <div className={`lang-switch ${className}`} role="group" aria-label={t('Language')}>
      {LANGUAGES.map((l) => (
        <button key={l.code} type="button" lang={l.code} title={l.label} aria-label={l.label} aria-pressed={lang === l.code} className={lang === l.code ? 'active' : ''} onClick={() => setLang(l.code)}>{l.short}</button>
      ))}
    </div>
  )
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
      <div ref={panel} className="drawer" role="dialog" aria-modal="true" aria-label={t('Menu')} onClick={(e) => e.stopPropagation()}>
        <div className="drawer-head"><Brand /><Button className="icon-button" ariaLabel={t('Close menu')} onClick={onClose}><Icon name="close" /></Button></div>
        <nav aria-label={t('Menu')}>
          <ActionLink href="/conversation"><Icon name="chat" size={17} />{t('Make a request')}</ActionLink>
          {primaryNav.map(([label, href, icon]) => {
            const more = MENU_NEEDS.find((need) => need.domains[0]![1] === href)
            return (
              <div className="drawer-need" key={href}>
                <ActionLink href={href}><Icon name={icon} size={17} />{t(label)}</ActionLink>
                {more && otherSearches(more).map(([name, to]) => <ActionLink className="drawer-sub" href={to} key={to}>{t(name)}</ActionLink>)}
              </div>
            )
          })}
          <div className="drawer-sep" role="separator" />
          <ActionLink href="/orders">{t('Orders')}</ActionLink>
          <ActionLink href="/reservations">{t('Reservations')}</ActionLink>
          <ActionLink href="/wishlist">{t('Saved')}</ActionLink>
          <ActionLink href="/cart">{t('Bag')}</ActionLink>
          <div className="drawer-sep" role="separator" />
          <ActionLink href="/seller">{t('For businesses')}</ActionLink>
          <LanguageSwitch className="drawer-lang" />
          {signedIn ? <><ActionLink href="/account">{t('Account')}</ActionLink><Button className="drawer-signout" onClick={() => { onClose(); signOut() }}>{t('Sign out')}</Button></>
            : <ActionLink href="/login">{t('Sign in')}</ActionLink>}
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
  // The drawer sits beside the header, not inside it: the header blurs what is behind it (backdrop-filter),
  // which turns it into the frame of every fixed child and would squeeze the drawer into the height of the bar.
  return (
    <>
    <header className="navbar product-navbar">
      <div className="nav-inner">
        <Brand />
        <nav className="desktop-nav" aria-label={t('Primary')}>
          <PrimaryNavLinks />
        </nav>
        <div className="nav-actions">
          <span className="icon-button location-button" title={t('Trezutil currently serves Kinshasa')}><Icon name="location" size={18} /><span>{t('Kinshasa')}</span></span>
          <ActionLink className="icon-button" href="/notifications" ariaLabel={t('Notifications')}><Icon name="bell" size={18} /></ActionLink>
          <ActionLink className="icon-button nav-bag" href="/cart" ariaLabel={tn(bagCount, 'Bag, {n} item', 'Bag, {n} items')}><Icon name="orders" size={18} />{bagCount > 0 && <span className="nav-count" aria-hidden="true">{bagCount}</span>}</ActionLink>
          {user
            ? <ActionLink className="icon-button nav-avatar" href="/account" ariaLabel={t('Your account')}><span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span></ActionLink>
            : <ActionLink className="icon-button" href="/login" ariaLabel={t('Sign in')}><Icon name="user" size={18} /></ActionLink>}
          <Button className="icon-button" ariaLabel={theme === 'dark' ? t('Switch to light mode') : t('Switch to dark mode')} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
          <LanguageSwitch />
          <ActionLink className="whatsapp-mini" href="/conversation" ariaLabel={t('Make a request')}><Icon name="chat" size={17} /><span>{t('Make a request')}</span></ActionLink>
        </div>
        <Button className="mobile-menu icon-button" ariaLabel={t('Open menu')} onClick={() => setMenu(true)}><Icon name="menu" /></Button>
      </div>
    </header>
    <MobileMenu open={menu} onClose={() => setMenu(false)} />
    </>
  )
}

function MobileNav() {
  return (
    <nav className="mobile-bottom" aria-label={t('Mobile navigation')}>
      <ActionLink href="/" nav end><Icon name="home" size={20} /><span>{t('Home')}</span></ActionLink>
      <ActionLink href="/discover" nav><Icon name="search" size={20} /><span>{t('Discover')}</span></ActionLink>
      <ActionLink href="/conversation" nav><Icon name="chat" size={20} /><span>{t('Ask')}</span></ActionLink>
      <ActionLink href="/orders" nav><Icon name="orders" size={20} /><span>{t('Orders')}</span></ActionLink>
      <ActionLink href="/account" nav><Icon name="user" size={20} /><span>{t('Account')}</span></ActionLink>
    </nav>
  )
}

/** The "Ask Trezutil" bar opens the floating chat with the page's question; without it, the full conversation page. */
export function ConversationDock({ label = t('Ask Trezutil anything about this page'), prompt }: { label?: string; prompt?: string }) {
  const widget = useChatWidget()
  const href = prompt ? `/conversation?q=${encodeURIComponent(prompt)}` : '/conversation'
  return (
    <Link className="action-link conversation-dock" to={href} onClick={(e) => { if (widget.available) { e.preventDefault(); widget.show(prompt) } }}>
      <span className="dock-logo"><Icon name="chat" /></span>
      <span><strong>{label}</strong><small>{t('Compare, refine or find something nearby')}</small></span>
      <Icon name="arrow" />
    </Link>
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
