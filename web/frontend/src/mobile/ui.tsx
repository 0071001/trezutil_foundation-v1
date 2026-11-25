import { createContext, useContext, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { t } from '@/lib/i18n'
import { MIcon, type MIconName } from '@/mobile/icons'

/* Pieces of the mobile design: the logo, the app header, list rows and the switch. */

/** What the frame provides to the mobile screens: the drawer, the cart and the unread notifications. */
export interface MobileFrame { openDrawer: () => void; openCart: () => void; cartCount: number; unread: number }
export const MobileFrameContext = createContext<MobileFrame>({ openDrawer: () => {}, openCart: () => {}, cartCount: 0, unread: 0 })
export const useMobileFrame = () => useContext(MobileFrameContext)

export function MLogo({ small = false }: { small?: boolean }) {
  return <span className={small ? 'mx-logo small' : 'mx-logo'} aria-hidden="true"><span>t</span></span>
}

/**
 * The header of the design: menu (or back) on the left, the title in the middle, an action on the right. The home and
 * the conversation show "Trezutil · Commerce OS" and the new conversation.
 */
export function AppHeader({ title = 'Trezutil', back, onNew }: { title?: string; back?: boolean | (() => void); onNew?: () => void }) {
  const navigate = useNavigate()
  const { openDrawer, openCart, cartCount } = useMobileFrame()
  const goBack = typeof back === 'function' ? back : () => (window.history.length > 1 ? navigate(-1) : navigate('/'))
  return (
    <header className="mx-header">
      <button type="button" className="mx-icon-btn" onClick={back ? goBack : openDrawer} aria-label={back ? t('Retour') : t('Ouvrir le menu')}>
        <MIcon name={back ? 'back' : 'menu'} />
      </button>
      <div className="mx-title"><span>{title}</span>{!back && <span className="mx-pill">Commerce OS</span>}</div>
      <div className="mx-header-end">
        {!back && cartCount > 0 && (
          <button type="button" className="mx-icon-btn mx-badge" onClick={openCart} aria-label={t('Panier, {n} article(s)', { n: cartCount })}>
            <MIcon name="package" size={20} /><i>{cartCount > 9 ? '9+' : cartCount}</i>
          </button>
        )}
        {onNew && <button type="button" className="mx-icon-btn" onClick={onNew} aria-label={t('Nouvelle conversation')}><MIcon name="edit" /></button>}
      </div>
    </header>
  )
}

export function ListRow({ icon, title, detail, onClick, accent, danger }: { icon: MIconName; title: string; detail?: string; onClick?: () => void; accent?: boolean; danger?: boolean }) {
  const body = (
    <>
      <span className="mx-row-icon"><MIcon name={icon} size={19} /></span>
      <span className="mx-row-text"><strong>{title}</strong>{detail && <small>{detail}</small>}</span>
      {onClick && <MIcon name="chevron" size={17} />}
    </>
  )
  const cls = `mx-list-row${accent ? ' accent' : ''}${danger ? ' danger' : ''}`
  return onClick ? <button type="button" className={cls} onClick={onClick}>{body}</button> : <div className={cls}>{body}</div>
}

export function Group({ children }: { children: ReactNode }) {
  return <div className="mx-group">{children}</div>
}

export function SectionLabel({ children }: { children: ReactNode }) {
  return <p className="mx-section-label">{children}</p>
}

export function Switch({ checked, onChange, label, disabled }: { checked: boolean; onChange: (v: boolean) => void; label: string; disabled?: boolean }) {
  return <button type="button" role="switch" aria-checked={checked} aria-label={label} disabled={disabled} className={`mx-toggle${checked ? ' on' : ''}`} onClick={() => onChange(!checked)}><i /></button>
}

export function ToggleRow({ title, detail, checked, onChange, disabled }: { title: string; detail: string; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return <div className="mx-setting-row"><span className="mx-row-text"><strong>{title}</strong><small>{detail}</small></span><Switch checked={checked} onChange={onChange} label={title} disabled={disabled} /></div>
}

/** A screen under a back header, on the design's grey background. */
export function Screen({ title, children, back = true, className = '' }: { title: string; children: ReactNode; back?: boolean | (() => void); className?: string }) {
  return (
    <div className="mx-screen">
      <AppHeader title={title} back={back} />
      <div className={`mx-subscreen ${className}`}>{children}</div>
    </div>
  )
}
