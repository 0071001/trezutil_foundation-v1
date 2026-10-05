import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router'
import { OfflineBanner } from '@/components/states'
import { LanguageSwitch } from '@/components/shell'
import { Brand, Button, Icon } from '@/components/ui'
import { Breadcrumbs, type Crumb } from '@/components/portal/Breadcrumbs'
import { appEnvLabel } from '@/lib/config'
import { t } from '@/lib/i18n'
import { usePrefs } from '@/lib/prefs'
import { useDialogFocus } from '@/lib/useDialogFocus'
import { useMediaQuery } from '@/lib/useMediaQuery'
import { portalInfo } from '@/portal/portals'
import type { ShellGroup, ShellItem } from '@/portal/modules'

export interface PortalShellProps {
  kind: 'seller' | 'admin'
  groups: ShellGroup[]
  /** The block at the top of the rail: the business, or the administration tag. */
  identity: ReactNode
  /** Controls under the identity (the business picker). */
  railTop?: ReactNode
  user: { name: string; detail: string }
  onSignOut: () => void
  /** The portal switcher (rendered in the header). */
  switcher: ReactNode
  /** Seller on phones: ids of the items shown in the bottom tab bar (four; "More" opens the rail). */
  tabIds?: string[]
  children: ReactNode
}

/** The accessible name of a rail item: the label plus what a person would otherwise have to guess. */
export function itemName(item: ShellItem): string {
  const parts = [t(item.label)]
  if (item.status === 'planned') parts.push(t('coming soon'))
  if (item.status === 'unavailable') parts.push(t('not available yet'))
  if (item.restricted) parts.push(t('managers only'))
  return parts.join(', ')
}

function matches(item: ShellItem, pathname: string): boolean {
  return item.end ? pathname === item.to : pathname === item.to || pathname.startsWith(`${item.to}/`)
}

/** Portal › module, plus a generic "Details" crumb on a record below a module. */
export function crumbsFor(kind: 'seller' | 'admin', groups: ShellGroup[], pathname: string): Crumb[] {
  const root: Crumb = { label: t(portalInfo[kind].label), to: portalInfo[kind].to }
  const all = groups.flatMap((g) => g.items)
  const item = [...all].sort((a, b) => b.to.length - a.to.length).find((i) => matches(i, pathname))
  if (!item) return [root]
  if (item.to === root.to) return [{ label: t(item.label) }]
  const crumbs: Crumb[] = [root, { label: t(item.label), to: item.to }]
  if (pathname !== item.to) crumbs.push({ label: t('Details') })
  return crumbs
}

function RailLink({ item }: { item: ShellItem }) {
  const mark = item.status === 'planned' ? { icon: 'clock' as const, title: t('Coming soon') } : item.status === 'unavailable' ? { icon: 'lock' as const, title: t('Not available yet') } : null
  return (
    <NavLink to={item.to} end={item.end} className={({ isActive }) => `tz-link ${isActive ? 'active' : ''}`} data-status={item.status} data-restricted={item.restricted || undefined} aria-label={itemName(item)}>
      <Icon name={item.icon} size={17} />
      <span className="tz-link-text">{t(item.label)}</span>
      {mark && <span className="tz-mark" title={mark.title} aria-hidden="true"><Icon name={mark.icon} size={13} /></span>}
    </NavLink>
  )
}

/**
 * The frame of the seller and administration consoles: a rail grouped by job,
 * a header (breadcrumbs, portal switcher, language, theme, person), and the
 * page. Below 900 px the rail is a slide-over opened from the header; on
 * phones the seller also gets a bottom tab bar. The page's `<main id="main">`
 * is the target of the site-wide skip link and of route-change focus.
 */
export function PortalShell({ kind, groups, identity, railTop, user, onSignOut, switcher, tabIds = [], children }: PortalShellProps) {
  const { theme, toggleTheme } = usePrefs()
  const location = useLocation()
  const narrow = useMediaQuery('(max-width: 899px)')
  const phone = useMediaQuery('(max-width: 639px)')
  const [open, setOpen] = useState(false)
  const rail = useRef<HTMLElement>(null)
  const opener = useRef<HTMLButtonElement>(null)
  const env = kind === 'admin' ? appEnvLabel() : ''

  // A new page closes the slide-over; widening the window closes it too.
  useEffect(() => setOpen(false), [location.pathname])
  useEffect(() => { if (!narrow) setOpen(false) }, [narrow])
  const modal = narrow && open
  useDialogFocus(modal, rail, { onEscape: () => { setOpen(false); opener.current?.focus() } })

  const crumbs = useMemo(() => crumbsFor(kind, groups, location.pathname), [kind, groups, location.pathname])
  const tabs = useMemo(() => {
    const flat = groups.flatMap((g) => g.items)
    return tabIds.map((id) => flat.find((i) => i.id === id)).filter((i): i is ShellItem => !!i)
  }, [groups, tabIds])
  const label = kind === 'seller' ? t('Seller workspace') : t('Administration menu')

  return (
    <div className={`app portal portal-${kind} ${theme === 'dark' ? 'dark' : ''}`}>
      {modal && <div className="tz-scrim" onClick={() => setOpen(false)} aria-hidden="true" />}
      <aside ref={rail} id="portal-rail" className={`tz-rail ${open ? 'open' : ''}`} aria-label={label} role={modal ? 'dialog' : undefined} aria-modal={modal || undefined}>
        <Brand href={portalInfo[kind].to} />
        {identity}
        {railTop}
        <nav className="tz-nav" aria-label={label}>
          {groups.map((g) => (
            <div className="tz-group" key={g.id} role="group" aria-label={t(g.label)}>
              <p className="tz-group-label" aria-hidden="true">{t(g.label)}</p>
              {g.items.map((item) => <RailLink item={item} key={item.id} />)}
            </div>
          ))}
        </nav>
        {phone && <div className="tz-rail-foot"><p className="tz-label">{t('Language')}</p><LanguageSwitch /></div>}
      </aside>

      <div className="tz-main">
        <header className="tz-header">
          <button ref={opener} type="button" className="button icon-button tz-menu-btn" aria-label={t('Open menu')} aria-expanded={modal} aria-controls="portal-rail" onClick={() => setOpen(true)}><Icon name="menu" /></button>
          <Breadcrumbs items={crumbs} />
          <div className="tz-header-spacer" />
          <div className="tz-header-actions">
            {env && <span className="tz-env" title={t('Environment')}>{env}</span>}
            {switcher}
            {!phone && <LanguageSwitch />}
            <Button className="icon-button" ariaLabel={theme === 'dark' ? t('Switch to light mode') : t('Switch to dark mode')} onClick={toggleTheme}><Icon name={theme === 'dark' ? 'sun' : 'moon'} size={18} /></Button>
            <div className="tz-who"><strong>{user.name}</strong><small>{user.detail}</small></div>
            <Button className="tz-small-btn tz-signout" onClick={onSignOut}><span className="tz-signout-text">{t('Sign out')}</span><span className="tz-signout-icon" aria-hidden="true"><Icon name="logout" size={17} /></span></Button>
          </div>
        </header>
        <OfflineBanner />
        <main id="main" tabIndex={-1} className="tz-content">{children}</main>
      </div>

      {kind === 'seller' && phone && tabs.length > 0 && (
        <nav className="tz-tabbar" aria-label={t('Quick navigation')}>
          {tabs.map((item) => (
            <NavLink key={item.id} to={item.to} end={item.end} className={({ isActive }) => (isActive ? 'active' : '')}><Icon name={item.icon} size={20} /><span>{t(item.label)}</span></NavLink>
          ))}
          <button type="button" onClick={() => setOpen(true)} aria-haspopup="dialog"><Icon name="menu" size={20} /><span>{t('More')}</span></button>
        </nav>
      )}
    </div>
  )
}
