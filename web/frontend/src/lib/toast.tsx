import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { t } from '@/lib/i18n'

type Tone = 'info' | 'success' | 'error'

interface Toast {
  id: number
  tone: Tone
  message: string
}

const ToastContext = createContext<((message: string, tone?: Tone) => void) | null>(null)

/** Non-blocking notifications announced to screen readers (replaces alert()). */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const seq = useRef(0)

  const notify = useCallback((message: string, tone: Tone = 'info') => {
    const id = ++seq.current
    setToasts((list) => [...list.slice(-2), { id, tone, message }])
    window.setTimeout(() => setToasts((list) => list.filter((t) => t.id !== id)), tone === 'error' ? 7000 : 4500)
  }, [])

  const value = useMemo(() => notify, [notify])
  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-region" role="status" aria-live="polite">
        {toasts.map((toast) => (
          <div key={toast.id} className={`toast toast-${toast.tone}`}>
            <span className="toast-dot" aria-hidden="true" />
            <span>{toast.message}</span>
            <button type="button" className="toast-close" aria-label={t('Dismiss')} onClick={() => setToasts((list) => list.filter((x) => x.id !== toast.id))}>×</button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast outside ToastProvider')
  return ctx
}
