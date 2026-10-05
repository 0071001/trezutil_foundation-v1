import { useState, type ReactNode } from 'react'
import { ChannelQr, reach } from '@/components/channels'
import { OfflineBanner } from '@/components/states'
import { ActionLink, Brand, Button, Icon, type IconName } from '@/components/ui'
import { LanguageSwitch, PrimaryNavLinks, initials, primaryNav } from '@/components/shell'
import { whatsappHref } from '@/lib/config'
import { COMPANY_LABEL, COMPANY_LINKS } from '@/lib/company'
import { usePrefs } from '@/lib/prefs'
import { useSession } from '@/lib/session'
import { t, useLang } from '@/lib/i18n'

/*
 * What surrounds the public pages (the home page and "About"): the top bar, the
 * footer and the phone's bottom bar. They carry the home page's own look
 * (styles/home.css, scoped to `.app.home`) rather than the product shell's.
 */

export function HomeNav() {
  const { theme, toggleTheme } = usePrefs()
  const { user } = useSession()
  const [menu, setMenu] = useState(false)
  // the same entries as every other page (src/components/shell.tsx): the request first, then the needs, then the person's own pages
  const menuItems: [string, string, IconName][] = [
    [t('Make a request'), '/conversation', 'chat'],
    ...primaryNav.map(([label, href, icon]): [string, string, IconName] => [t(label), href, icon]),
    [t('Orders'), '/orders', 'orders'],
    [t('Reservations'), '/reservations', 'calendar'],
    [user ? t('Account') : t('Sign in'), user ? '/account' : '/login', 'user'],
    [t('For businesses'), '/seller', 'shop'],
  ]
  return (
    <header className="home-nav">
      <div className="home-nav-inner">
        <Brand href="/" />
        <nav className="home-nav-links" aria-label={t('Primary')}>
          <PrimaryNavLinks icons={false} />
        </nav>
        <ActionLink className="home-nav-business" href="/seller">{t('For businesses')}</ActionLink>
        <div className="home-nav-actions">
          <LanguageSwitch />
          <Button className="icon-button" ariaLabel={theme === 'dark' ? t('Switch to light mode') : t('Switch to dark mode')} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
          {user
            ? <ActionLink className="home-nav-account" href="/account" ariaLabel={t('Your account')}><span aria-hidden="true">{initials(user.display_name || user.phone.slice(-2))}</span></ActionLink>
            : <ActionLink className="home-nav-signin" href="/login">{t('Sign in')}</ActionLink>}
        </div>
        <Button className="home-nav-menu icon-button" ariaLabel={menu ? t('Close menu') : t('Open menu')} onClick={() => setMenu(!menu)}><Icon name={menu ? 'close' : 'menu'} /></Button>
      </div>
      {menu && (
        <nav className="home-menu" aria-label={t('Menu')}>
          {menuItems.map(([label, href, icon]) => <ActionLink key={label} href={href}><Icon name={icon} size={17} />{label}</ActionLink>)}
          <LanguageSwitch className="drawer-lang" />
        </nav>
      )}
    </header>
  )
}

/**
 * Where Trezutil can be reached, as the footer says it: "Available on WhatsApp, PlayStore and AppStore",
 * and a QR code for the WhatsApp chat. Each name is a link when its channel is real (the WhatsApp number
 * or the store page is configured, src/lib/config.ts) and plain text when it is only a placeholder; the
 * QR code is the real one or a frame without data. Placeholders show only when they are switched on
 * (staging): a production build never names an app or a number that does not exist.
 */
function Reach() {
  const lang = useLang()
  const { channels, whatsapp, qr } = reach()
  if (channels.length === 0) return null
  // "WhatsApp, PlayStore et AppStore": the list follows the language's own rules
  const parts = new Intl.ListFormat(lang === 'fr' ? 'fr' : 'en-GB', { style: 'long', type: 'conjunction' }).formatToParts(channels.map((c) => c.name))
  return (
    <>
      <p className="footer-available">
        {t('Available on')}{' '}
        {parts.map((part, i) => {
          const channel = part.type === 'element' ? channels.find((c) => c.name === part.value) : undefined
          if (!channel) return part.value
          return channel.href
            ? <a key={i} href={channel.href} target="_blank" rel="noopener noreferrer">{part.value}</a>
            : <span key={i} className="channel-soon" data-placeholder>{part.value}</span>
        })}.
      </p>
      {qr && (
        <figure className="footer-qr">
          <ChannelQr whatsapp={whatsapp} />
          <figcaption>{whatsapp ? t('Scan to chat with Trezutil on WhatsApp') : t('QR code coming soon')}</figcaption>
        </figure>
      )}
    </>
  )
}

export function Footer() {
  const wa = whatsappHref()
  const columns: [string, [string, string][]][] = [
    [t('Discover'), [[t('Discover'), '/discover'], [t('Search'), '/search'], [t('Reservations'), '/reservations']]],
    [t(COMPANY_LABEL), COMPANY_LINKS.map(([label, href]): [string, string] => [t(label), href])],
    [t('For businesses'), [[t('Sell on Trezutil'), '/seller/onboarding'], [t('Manage listings'), '/seller/listings'], [t('Orders'), '/seller/orders'], [t('Seller workspace'), '/seller']]],
    [t('Your account'), [[t('Sign in'), '/login'], [t('Orders'), '/orders'], [t('Saved'), '/wishlist'], [t('Make a request'), '/conversation']]],
  ]
  return (
    <footer className="home-footer">
      <div className="home-footer-main">
        <div className="footer-brand"><Brand /><p>{t('Commerce through conversation.')}<br />{t('Built around what you need.')}</p><Reach />{wa && <ActionLink className="whatsapp-footer" href={wa}><Icon name="whatsapp" size={18} />{t('Find us on WhatsApp')}</ActionLink>}</div>
        {columns.map(([title, links]) => <nav className="footer-column" key={title} aria-label={title}><strong>{title}</strong>{links.map(([label, href]) => <ActionLink href={href} key={label}>{label}</ActionLink>)}</nav>)}
      </div>
      <div className="home-footer-bottom"><span>© {new Date().getFullYear()} {t('Trezutil. Commerce in motion.')}</span><span>{t('Kinshasa · Africa · Everywhere')}</span></div>
    </footer>
  )
}

export function MobileBottomNav() {
  return (
    <nav className="mobile-bottom" aria-label={t('Mobile navigation')}>
      <ActionLink href="/" nav end><Icon name="home" size={20} /><span>{t('Home')}</span></ActionLink>
      <ActionLink href="/discover"><Icon name="search" size={20} /><span>{t('Discover')}</span></ActionLink>
      <ActionLink href="/conversation"><Icon name="chat" size={20} /><span>{t('Ask')}</span></ActionLink>
      <ActionLink href="/orders"><Icon name="orders" size={20} /><span>{t('Orders')}</span></ActionLink>
      <ActionLink href="/account"><Icon name="user" size={20} /><span>{t('Account')}</span></ActionLink>
    </nav>
  )
}

/** A public page in the home page's frame: top bar, the page, footer, phone bar. */
export function HomeFrame({ children }: { children: ReactNode }) {
  const { theme } = usePrefs()
  return (
    <div className={`app home ${theme === 'dark' ? 'dark' : ''}`}>
      <HomeNav />
      <OfflineBanner />
      <main id="main" tabIndex={-1}>{children}</main>
      <Footer />
      <MobileBottomNav />
    </div>
  )
}
