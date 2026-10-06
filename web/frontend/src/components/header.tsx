import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router'
import { NavGroups, PrimaryNav } from '@/components/nav'
import { ActionLink, Brand, Button, Icon } from '@/components/ui'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { LANGUAGES, setLang, t, tn, useLang } from '@/lib/i18n'

/*
 * The header and the phone's menu, one for every page: the product pages and the home page (its frame only sets
 * whether the bar sticks or is fixed). The primary navigation inside it is one component (components/nav.tsx), so the
 * menu looks and behaves the same everywhere: same words, same place, same breakpoints, same drawer on a phone.
 */

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

/** The accessible phone menu (the menu button of the header opens it). */
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
          <NavGroups />
          <div className="drawer-sep" role="separator" />
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

/**
 * The top bar of every page: the brand, the primary navigation, and a few actions (For businesses, the bag, theme,
 * language, the account, and the request button). `sticky` is for a page that scrolls under it in the flow (the home
 * page); the others keep it fixed and leave room for it.
 */
export function SiteHeader({ sticky = false }: { sticky?: boolean }) {
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
    <header className={`navbar product-navbar site-header ${sticky ? 'is-sticky' : ''}`}>
      <div className="nav-inner">
        <Brand />
        <PrimaryNav />
        <div className="nav-actions">
          <span className="icon-button location-button" title={t('Trezutil currently serves Kinshasa')}><Icon name="location" size={18} /><span>{t('Kinshasa')}</span></span>
          <ActionLink className="nav-business" href="/seller">{t('For businesses')}</ActionLink>
          <ActionLink className="icon-button nav-bag" href="/cart" ariaLabel={tn(bagCount, 'Bag, {n} item', 'Bag, {n} items')}><Icon name="orders" size={18} />{bagCount > 0 && <span className="nav-count" aria-hidden="true">{bagCount}</span>}</ActionLink>
          <Button className="icon-button" ariaLabel={theme === 'dark' ? t('Switch to light mode') : t('Switch to dark mode')} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
          <LanguageSwitch />
          {user
            ? <ActionLink className="icon-button nav-avatar" href="/account" ariaLabel={t('Your account')}><span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span></ActionLink>
            : <ActionLink className="nav-signin" href="/login">{t('Sign in')}</ActionLink>}
          <ActionLink className="whatsapp-mini" href="/conversation" ariaLabel={t('Make a request')}><Icon name="chat" size={17} /><span>{t('Make a request')}</span></ActionLink>
        </div>
        <Button className="mobile-menu icon-button" ariaLabel={t('Open menu')} onClick={() => setMenu(true)}><Icon name="menu" /></Button>
      </div>
    </header>
    <MobileMenu open={menu} onClose={() => setMenu(false)} />
    </>
  )
}

/** The phone's bottom bar. */
export function MobileNav() {
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
