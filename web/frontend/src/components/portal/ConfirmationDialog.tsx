import { useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { toApiError } from '@/api/client'
import { t } from '@/lib/i18n'
import { overlayHost, useDialogFocus } from '@/lib/useDialogFocus'

export interface ConfirmationDialogProps {
  open: boolean
  title: string
  /** Names the target and the effect: "Refund $40 to Marie? The customer is notified." */
  children: ReactNode
  confirmLabel: string
  cancelLabel?: string
  /** `danger` is for actions that cannot be undone; focus then starts on Cancel. */
  tone?: 'default' | 'danger'
  /** Runs the action. A thrown error stays in the dialog (inline), the dialog stays open. */
  onConfirm: () => Promise<unknown> | void
  /** Called after success, on Cancel, on Escape and on the scrim. */
  onClose: () => void
}

/**
 * Every state-changing action in the consoles goes through this: it says what
 * will happen to what, cannot be double-submitted, and shows a refusal from
 * the server in place instead of closing as if it had worked.
 */
export function ConfirmationDialog({ open, title, children, confirmLabel, cancelLabel, tone = 'default', onConfirm, onClose }: ConfirmationDialogProps) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const panel = useRef<HTMLDivElement>(null)
  const cancel = useRef<HTMLButtonElement>(null)
  const titleId = useId()
  const bodyId = useId()

  const close = () => { if (!busy) { setError(null); onClose() } }
  useDialogFocus(open, panel, { onEscape: close, initial: tone === 'danger' ? cancel : undefined })
  if (!open) return null

  const confirm = async () => {
    setBusy(true)
    setError(null)
    try {
      await onConfirm()
      setBusy(false)
      onClose()
    } catch (err) {
      setBusy(false)
      setError(toApiError(err).message)
    }
  }

  return createPortal(
    <div className="tz-dialog-overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) close() }}>
      <div ref={panel} className="tz-dialog" role={tone === 'danger' ? 'alertdialog' : 'dialog'} aria-modal="true" aria-labelledby={titleId} aria-describedby={bodyId} aria-busy={busy || undefined}>
        <h2 id={titleId}>{title}</h2>
        <div id={bodyId}>{typeof children === 'string' ? <p>{children}</p> : children}</div>
        {error && <p className="tz-dialog-error" role="alert">{error}</p>}
        <div className="tz-dialog-actions">
          <button ref={cancel} type="button" className="tz-btn ghost" disabled={busy} onClick={close}>{cancelLabel ?? t('Cancel')}</button>
          <button type="button" className={`tz-btn ${tone === 'danger' ? 'danger' : ''}`} disabled={busy} onClick={() => void confirm()}>{busy ? t('Working…') : confirmLabel}</button>
        </div>
      </div>
    </div>,
    overlayHost(),
  )
}
