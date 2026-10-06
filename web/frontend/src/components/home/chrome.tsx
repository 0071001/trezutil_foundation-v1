import type { ReactNode } from 'react'
import { ChannelQr, reach } from '@/components/channels'
import { LegalLinks } from '@/components/legal'
import { OfflineBanner } from '@/components/states'
import { ActionLink, Brand, Icon } from '@/components/ui'
import { MobileNav, SiteHeader } from '@/components/header'
import { whatsappHref } from '@/lib/config'
import { COMPANY_LABEL, COMPANY_LINKS } from '@/lib/company'
import { usePrefs } from '@/lib/prefs'
import { t, useLang } from '@/lib/i18n'

/*
 * What surrounds the public pages (the home page, "About", the legal pages): the
 * header and the phone's bottom bar (the same ones as every other page, so the menu is
 * the same everywhere), and the footer. The frame carries the home page's own look
 * (styles/home.css, scoped to `.app.home`).
 */

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
      <div className="home-footer-bottom"><span>© {new Date().getFullYear()} {t('Trezutil. Commerce in motion.')}</span><LegalLinks /><span>{t('Kinshasa · Africa · Everywhere')}</span></div>
    </footer>
  )
}

/** A public page in the home page's frame: top bar, the page, footer, phone bar. */
export function HomeFrame({ children }: { children: ReactNode }) {
  const { theme } = usePrefs()
  return (
    <div className={`app home ${theme === 'dark' ? 'dark' : ''}`}>
      <SiteHeader sticky />
      <OfflineBanner />
      <main id="main" tabIndex={-1}>{children}</main>
      <Footer />
      <MobileNav />
    </div>
  )
}
