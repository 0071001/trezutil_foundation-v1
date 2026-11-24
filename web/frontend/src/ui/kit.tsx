import { useEffect, useState, type ReactNode } from 'react'
import type { ApiError } from '@/api/client'
import { Icon, type IconName } from '@/ui/Icon'
import { LANGUAGES, getLang, locale, switchLanguage, t } from '@/lib/i18n'

/* Small shared pieces: the logo, badges, and the states every screen needs (loading, error, empty, offline). */

/* The official Trezutil logo (source files in /logos): the full horizontal logo, or only its three-dot mark when compact. */
export function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <span className="logo">
      {compact ? (
        <>
          <img className="logo-mark on-light" src="/brand/trezutil-mark.svg" alt="" width={642} height={184} />
          <img className="logo-mark on-dark" src="/brand/trezutil-mark-white.svg" alt="" width={642} height={184} />
        </>
      ) : (
        <>
          <img className="logo-full on-light" src="/brand/trezutil-logo.png" alt="Trezutil" width={1000} height={147} />
          <img className="logo-full on-dark" src="/brand/trezutil-logo-white.png" alt="Trezutil" width={1000} height={147} />
        </>
      )}
    </span>
  )
}

/** FR / EN, for screens a visitor reaches before the settings: switches the whole interface (the page reloads). */
export function LanguageToggle() {
  const other = LANGUAGES.find((l) => l.code !== getLang())!
  return (
    <button type="button" className="lang-toggle" lang={other.code} aria-label={other.label} title={other.label} onClick={() => switchLanguage(other.code)}>
      <Icon name="globe" size={15} />{other.short}
    </button>
  )
}

export type Tone = 'green' | 'amber' | 'neutral' | 'blue' | 'red'

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: Tone }) {
  return <span className={`badge ${tone}`}>{children}</span>
}

export function Loading({ label = 'Chargement…' }: { label?: string }) {
  return <div className="state-loading" role="status"><span aria-hidden="true" />{label}</div>
}

export function errorText(error: ApiError): string {
  if (error.isNetwork) return t('Trezutil est injoignable. Vérifie ta connexion puis réessaie.')
  if (error.status === 401) return t('Ta session a expiré. Reconnecte-toi pour continuer.')
  if (error.status === 403) return t('Ce contenu n’est pas accessible avec ce compte.')
  if (error.status === 404) return t('Introuvable : il a peut-être été supprimé.')
  if (error.code === 'OTP_DELIVERY_FAILED') return t('Le code n’a pas pu être envoyé sur WhatsApp pour le moment. Réessaie dans quelques minutes.')
  // The server never explains a 5xx (its text is a generic English notice): say it plainly, in French.
  if (error.status >= 500) return t('Trezutil rencontre un problème de son côté. Réessaie dans un instant.')
  return error.message || t('Une erreur est survenue.')
}

export function ErrorNote({ error, onRetry }: { error: ApiError; onRetry?: () => void }) {
  return (
    <div className="state-error" role="alert">
      <Icon name="x" size={16} />
      <span>{errorText(error)}</span>
      {onRetry && <button type="button" className="secondary-btn small-btn" onClick={onRetry}>{t('Réessayer')}</button>}
    </div>
  )
}

export function Empty({ icon, title, children, action }: { icon: IconName; title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="empty-setting">
      <span><Icon name={icon} size={26} /></span>
      <h2>{title}</h2>
      {children && <p>{children}</p>}
      {action}
    </div>
  )
}

/** A capability the backend does not offer yet: said plainly, never simulated. */
export function NotConnected({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="pending-note" role="note">
      <Icon name="link" size={16} />
      <p><b>{title}</b><br />{children}</p>
    </div>
  )
}

export function useOnline(): boolean {
  const [online, setOnline] = useState(() => (typeof navigator === 'undefined' ? true : navigator.onLine !== false))
  useEffect(() => {
    const on = () => setOnline(true)
    const off = () => setOnline(false)
    window.addEventListener('online', on)
    window.addEventListener('offline', off)
    return () => { window.removeEventListener('online', on); window.removeEventListener('offline', off) }
  }, [])
  return online
}

export function OfflineBar() {
  const online = useOnline()
  if (online) return null
  return <div className="offline-bar" role="status"><Icon name="offline" size={15} />{t('Hors ligne : tes messages partiront quand la connexion reviendra, si tu les renvoies.')}</div>
}

export function initials(name: string): string {
  const parts = name.replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return '?'
  return ((parts[0]![0] ?? '') + (parts.length > 1 ? parts[parts.length - 1]![0] ?? '' : parts[0]![1] ?? '')).toUpperCase()
}

export function newId(): string {
  const c = globalThis.crypto
  return c?.randomUUID ? c.randomUUID() : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`
}

export const clock = (at: number) => new Date(at).toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' })

export function dayLabel(at: number): string {
  const today = new Date()
  const start = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime()
  if (at >= start) return t('Aujourd’hui')
  if (at >= start - 86_400_000) return t('Hier')
  if (at >= start - 6 * 86_400_000) return t('7 derniers jours')
  if (at >= start - 29 * 86_400_000) return t('30 derniers jours')
  return new Date(at).toLocaleDateString(locale(), { month: 'long', year: 'numeric' })
}
