import { useId, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button, Icon } from '@/components/ui'
import { t } from '@/lib/i18n'
import { overlayHost, useDialogFocus } from '@/lib/useDialogFocus'

export interface DetailDrawerProps {
  open: boolean
  onClose: () => void
  title: string
  subtitle?: string
  children: ReactNode
  /** Buttons for the footer (the allowed actions for this record). */
  actions?: ReactNode
}

/**
 * A record beside the list it came from. Focus moves in, Tab stays in,
 * Escape and the scrim close it, and focus returns to the row that opened
 * it. On phones it is a bottom sheet.
 */
export function DetailDrawer({ open, onClose, title, subtitle, children, actions }: DetailDrawerProps) {
  const panel = useRef<HTMLDivElement>(null)
  const titleId = useId()
  useDialogFocus(open, panel, { onEscape: onClose })
  if (!open) return null
  return createPortal(
    <div className="tz-overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div ref={panel} className="tz-drawer" role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <div className="tz-drawer-head">
          <div><h2 id={titleId}>{title}</h2>{subtitle && <p>{subtitle}</p>}</div>
          <Button className="icon-button" ariaLabel={t('Close')} onClick={onClose}><Icon name="close" /></Button>
        </div>
        <div className="tz-drawer-body">{children}</div>
        {actions && <div className="tz-drawer-foot">{actions}</div>}
      </div>
    </div>,
    overlayHost(),
  )
}
