import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { Link, useLocation } from 'react-router'
import { ActionLink, Icon } from '@/components/ui'
import { HOW_IT_WORKS, NAV_MENUS, pathOf, type NavMenu } from '@/lib/nav'
import { useSession } from '@/lib/session'
import { t } from '@/lib/i18n'

/*
 * The primary navigation (src/lib/nav.ts): "How it works" and three menus that open under their word. A menu is a
 * disclosure, not an application menu: a button that shows or hides a short list of ordinary links, which is what
 * it is. It opens on click, Enter or Space, on ArrowDown, and (for a mouse) when the pointer rests on it; it closes
 * on Escape (focus returns to its button), on a click elsewhere, when focus leaves it, and when a link is followed.
 * Only one is open at a time. The panel is a short list with one line under each entry, not a wall of categories.
 */

/** Which menu the current page belongs to, so its word can show where you are. */
function menuIsCurrent(menu: NavMenu, pathname: string): boolean {
  return menu.items.some((item) => {
    // "My missions" opens the Organiser page, which is Organise's: that page marks Organise only
    if (menu.id === 'activities' && item.id === 'missions') return false
    const p = pathOf(item.to())
    return pathname === p || pathname.startsWith(`${p}/`)
  })
}

function MenuLinks({ menu, onFollow }: { menu: NavMenu; onFollow?: () => void }) {
  return (
    <ul>
      {menu.items.map((item) => (
        <li key={item.id}>
          <Link className="site-nav-link" to={item.to()} onClick={onFollow}>
            <span className="site-nav-icon" aria-hidden="true"><Icon name={item.icon} size={17} /></span>
            <span className="site-nav-text"><strong>{t(item.label)}</strong><small>{t(item.hint)}</small></span>
          </Link>
        </li>
      ))}
    </ul>
  )
}

function GuestNote() {
  const { signedIn } = useSession()
  if (signedIn) return null
  return (
    <p className="site-nav-note">
      {t('Sign in to find what you started here.')} <Link to="/login">{t('Sign in')}</Link>
    </p>
  )
}

/** The header's navigation: the same on every page. Its `desktop-nav` class is what the header's rules hide on a phone. */
export function PrimaryNav() {
  const location = useLocation()
  const [open, setOpen] = useState<string | null>(null)
  // A menu a mouse merely rests on closes when the mouse leaves; one that was clicked, or opened from the keyboard, stays
  // until it is closed (a click on the word while it is open, Escape, a click elsewhere, or the focus leaving).
  const [pinned, setPinned] = useState(false)
  const root = useRef<HTMLElement>(null)
  const leaveTimer = useRef<number | undefined>(undefined)
  const baseId = useId()

  const close = (focusTrigger = false) => {
    if (open && focusTrigger) root.current?.querySelector<HTMLElement>(`[data-menu="${open}"]`)?.focus()
    setOpen(null)
    setPinned(false)
  }

  // A followed link, or any other change of page, closes the menu.
  useEffect(() => { setOpen(null); setPinned(false) }, [location.pathname, location.search, location.hash])

  // A click or tap elsewhere closes it.
  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) { setOpen(null); setPinned(false) } }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  useEffect(() => () => window.clearTimeout(leaveTimer.current), [])

  const focusItem = (menu: string, which: 'first' | 'last') => {
    // after the panel is in the page
    requestAnimationFrame(() => {
      const links = root.current?.querySelectorAll<HTMLElement>(`[data-panel="${menu}"] a`)
      if (links?.length) links[which === 'first' ? 0 : links.length - 1]!.focus()
    })
  }

  const onTriggerKey = (e: KeyboardEvent, menu: string) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setOpen(menu); setPinned(true); focusItem(menu, 'first') }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setOpen(menu); setPinned(true); focusItem(menu, 'last') }
    else if (e.key === 'Escape' && open === menu) { e.preventDefault(); close(true) }
  }

  const onPanelKey = (e: KeyboardEvent, menu: string) => {
    const links = Array.from(root.current?.querySelectorAll<HTMLElement>(`[data-panel="${menu}"] a`) ?? [])
    const at = links.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'Escape') { e.preventDefault(); close(true) }
    else if (e.key === 'ArrowDown') { e.preventDefault(); links[(at + 1) % links.length]?.focus() }
    else if (e.key === 'ArrowUp') { e.preventDefault(); links[(at - 1 + links.length) % links.length]?.focus() }
    else if (e.key === 'Home') { e.preventDefault(); links[0]?.focus() }
    else if (e.key === 'End') { e.preventDefault(); links[links.length - 1]?.focus() }
  }

  return (
    <nav ref={root} className="site-nav desktop-nav" aria-label={t('Primary')}>
      <ActionLink href={HOW_IT_WORKS.to}><Icon name={HOW_IT_WORKS.icon} size={18} /><span>{t(HOW_IT_WORKS.label)}</span></ActionLink>
      {NAV_MENUS.map((menu) => {
        const isOpen = open === menu.id
        const panelId = `${baseId}-${menu.id}`
        return (
          <div
            key={menu.id}
            className="site-nav-item"
            onPointerEnter={(e) => {
              if (e.pointerType !== 'mouse') return
              window.clearTimeout(leaveTimer.current)
              if (!pinned) setOpen(menu.id) // a menu that was clicked open is not taken over by a pointer passing by
            }}
            onPointerLeave={(e) => {
              if (e.pointerType !== 'mouse' || pinned) return
              window.clearTimeout(leaveTimer.current)
              leaveTimer.current = window.setTimeout(() => setOpen((c) => (c === menu.id ? null : c)), 140)
            }}
            onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) { setOpen((c) => (c === menu.id ? null : c)); setPinned(false) } }}
          >
            <button
              type="button"
              className={`action-link site-nav-trigger ${menuIsCurrent(menu, location.pathname) ? 'active' : ''}`}
              data-menu={menu.id}
              aria-expanded={isOpen}
              aria-controls={panelId}
              onClick={() => {
                // a click on a menu a mouse had only rested on keeps it open (and keeps it); on a pinned one it closes it
                if (isOpen && pinned) { setOpen(null); setPinned(false) } else { setOpen(menu.id); setPinned(true) }
              }}
              onKeyDown={(e) => onTriggerKey(e, menu.id)}
            >
              <Icon name={menu.icon} size={18} />
              <span>{t(menu.label)}</span>
              <span className="site-nav-caret" aria-hidden="true" />
            </button>
            {isOpen && (
              <div id={panelId} className="site-nav-panel" data-panel={menu.id} role="group" aria-label={t(menu.label)} onKeyDown={(e) => onPanelKey(e, menu.id)}>
                <MenuLinks menu={menu} />
                {menu.id === 'activities' && <GuestNote />}
              </div>
            )}
          </div>
        )
      })}
    </nav>
  )
}

/**
 * The same structure as plain groups, for the phone's menus (the drawer and the home frame's menu): the same words
 * in the same order, each group under its own heading, with the one line under each entry.
 */
export function NavGroups() {
  return (
    <>
      <ActionLink href={HOW_IT_WORKS.to}><Icon name={HOW_IT_WORKS.icon} size={17} />{t(HOW_IT_WORKS.label)}</ActionLink>
      {NAV_MENUS.map((menu) => (
        <section className="nav-group" key={menu.id} aria-label={t(menu.label)}>
          <h3><Icon name={menu.icon} size={15} />{t(menu.label)}</h3>
          {menu.items.map((item) => (
            <ActionLink key={item.id} href={item.to()}>
              <Icon name={item.icon} size={17} />
              <span className="nav-group-text"><strong>{t(item.label)}</strong><small>{t(item.hint)}</small></span>
            </ActionLink>
          ))}
          {menu.id === 'activities' && <GuestNote />}
        </section>
      ))}
    </>
  )
}
