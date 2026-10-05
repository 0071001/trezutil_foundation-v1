import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router'
import { Icon, type IconName } from '@/components/ui'
import { portalInfo, type PortalKind } from '@/portal/portals'
import { t } from '@/lib/i18n'

const icons: Record<PortalKind, IconName> = { customer: 'home', seller: 'shop', admin: 'shield' }

/**
 * Switches between the portals the person holds (customer, seller, admin).
 * It lists only those; opening one the server would refuse still ends in a
 * Forbidden screen, because the lists are advisory.
 */
export function PortalSwitcher({ current, portals }: { current: PortalKind; portals: PortalKind[] }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const menuId = useId()

  useEffect(() => {
    if (!open) return
    const items = () => [...(root.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
    items()[Math.max(0, portals.indexOf(current))]?.focus()
    const onDown = (e: MouseEvent) => { if (root.current && !root.current.contains(e.target as Node)) setOpen(false) }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setOpen(false); button.current?.focus(); return }
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp' && e.key !== 'Home' && e.key !== 'End') return
      e.preventDefault()
      const list = items()
      const at = list.indexOf(document.activeElement as HTMLElement)
      const next = e.key === 'Home' ? 0 : e.key === 'End' ? list.length - 1 : (at + (e.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length
      list[next]?.focus()
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => { document.removeEventListener('mousedown', onDown); document.removeEventListener('keydown', onKey) }
  }, [open, current, portals])

  const here = portalInfo[current]
  return (
    <div className="tz-switcher" ref={root}>
      <button ref={button} type="button" className="tz-switcher-btn" aria-haspopup="menu" aria-expanded={open} aria-controls={open ? menuId : undefined} onClick={() => setOpen((o) => !o)}>
        <Icon name={icons[current]} size={16} />
        <span>{t(here.label)}</span>
        <Icon name="chevron" size={14} />
      </button>
      {open && (
        <ul className="tz-menu" role="menu" id={menuId} aria-label={t('Switch portal')}>
          {portals.map((kind) => {
            const info = portalInfo[kind]
            return (
              <li role="none" key={kind}>
                <Link className="tz-menu-item" role="menuitem" to={info.to} aria-current={kind === current ? 'true' : undefined} onClick={() => setOpen(false)}>
                  <Icon name={icons[kind]} size={18} />
                  <span><strong>{t(info.label)}</strong><small>{t(info.description)}</small></span>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
