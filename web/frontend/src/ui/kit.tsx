import { useEffect, useState, type ReactNode } from 'react'
import type { ApiError } from '@/api/client'
import { Icon, type IconName } from '@/ui/Icon'

/* Small shared pieces: the logo, badges, and the states every screen needs (loading, error, empty, offline). */

export function Logo({ compact = false }: { compact?: boolean }) {
  return <div className="logo"><span className="logo-mark" aria-hidden="true"><span /></span>{!compact && <span>Trezutil</span>}</div>
}

export type Tone = 'green' | 'amber' | 'neutral' | 'blue' | 'red'

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: Tone }) {
  return <span className={`badge ${tone}`}>{children}</span>
}

export function Loading({ label = 'Chargement…' }: { label?: string }) {
  return <div className="state-loading" role="status"><span aria-hidden="true" />{label}</div>
}

export function errorText(error: ApiError): string {
  if (error.isNetwork) return 'Trezutil est injoignable. Vérifie ta connexion puis réessaie.'
  if (error.status === 401) return 'Ta session a expiré. Reconnecte-toi pour continuer.'
  if (error.status === 403) return 'Ce contenu n’est pas accessible avec ce compte.'
  if (error.status === 404) return 'Introuvable : il a peut-être été supprimé.'
  return error.message || 'Une erreur est survenue.'
}

export function ErrorNote({ error, onRetry }: { error: ApiError; onRetry?: () => void }) {
  return (
    <div className="state-error" role="alert">
      <Icon name="x" size={16} />
      <span>{errorText(error)}</span>
      {onRetry && <button type="button" className="secondary-btn small-btn" onClick={onRetry}>Réessayer</button>}
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
  return <div className="offline-bar" role="status"><Icon name="offline" size={15} />Hors ligne : tes messages partiront quand la connexion reviendra, si tu les renvoies.</div>
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

export const clock = (at: number) => new Date(at).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })

export function dayLabel(at: number): string {
  const today = new Date()
  const start = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime()
  if (at >= start) return 'Aujourd’hui'
  if (at >= start - 86_400_000) return 'Hier'
  if (at >= start - 6 * 86_400_000) return '7 derniers jours'
  if (at >= start - 29 * 86_400_000) return '30 derniers jours'
  return new Date(at).toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })
}
