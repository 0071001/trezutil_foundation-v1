import type { ReactNode } from 'react'
import type { ApiError } from '@/api/client'
import { locale, t } from '@/lib/i18n'
import { errorText } from '@/ui/kit'
import { CCIcon, type CCIconName } from '@/admin/icons'

/*
 * The building blocks of the Control Center (the Trezutil Control Center design): page head, metric cards, summary
 * strip, panels, status pills, table toolbar and pagination. They only lay out what the server returned; a figure the
 * server does not give is never drawn.
 */

export type Tone = 'good' | 'warn' | 'bad' | 'info' | 'neutral'

export function Status({ children, tone = 'neutral' }: { children: ReactNode; tone?: Tone }) {
  return <span className={`cc-status ${tone}`}><span />{children}</span>
}

/** The tone of a status word the backend uses (orders, payments, providers, accounts…). */
export function toneOf(status: string): Tone {
  const s = status.toLowerCase()
  if (/(fail|error|reject|suspend|unavailable|down|cancel|refus|locked|blocked|disabled|inactive)/.test(s)) return 'bad'
  if (/(pending|review|submitted|degraded|draft|awaiting|changes|processing|warn|paused|unknown)/.test(s)) return 'warn'
  if (/(active|ok|healthy|verified|paid|captured|confirmed|completed|fulfilled|published|up|connected|succeeded|operational)/.test(s)) return 'good'
  return 'neutral'
}

export function PageHead({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children?: ReactNode }) {
  return (
    <div className="cc-page-head">
      <div><div className="cc-eyebrow">{eyebrow}</div><h1>{title}</h1><p>{description}</p></div>
      {children && <div className="cc-head-actions">{children}</div>}
    </div>
  )
}

export function Metric({ label, value, note, icon, tone }: { label: string; value: ReactNode; note?: ReactNode; icon: CCIconName; tone?: 'amber' | 'coral' }) {
  return (
    <div className="cc-metric">
      <div className="cc-metric-top"><span>{label}</span><CCIcon name={icon} /></div>
      <div className="cc-metric-main"><strong>{value}</strong>{note !== undefined && note !== '' && <span className={tone ? `${tone}-text` : ''}>{note}</span>}</div>
    </div>
  )
}

export function Strip({ items }: { items: { label: string; value: ReactNode; note?: ReactNode; positive?: boolean }[] }) {
  return (
    <div className="cc-strip">
      {items.map((i) => <div key={i.label}><span>{i.label}</span><strong>{i.value}</strong>{i.note !== undefined && <small className={i.positive ? 'positive' : ''}>{i.note}</small>}</div>)}
    </div>
  )
}

export function Panel({ title, sub, action, children, wide, className = '' }: { title: string; sub?: string; action?: ReactNode; children: ReactNode; wide?: boolean; className?: string }) {
  return (
    <section className={`cc-panel ${wide ? 'wide' : ''} ${className}`}>
      <div className="cc-panel-head"><div><h2>{title}</h2>{sub && <p>{sub}</p>}</div>{action}</div>
      {children}
    </section>
  )
}

export function Notice({ title, children, tone = 'amber', action }: { title: string; children?: ReactNode; tone?: 'amber' | 'mint' | 'coral'; action?: ReactNode }) {
  return (
    <div className={`cc-notice ${tone}`} role="note">
      <div className="cc-notice-icon"><CCIcon name={tone === 'mint' ? 'check' : 'alert'} /></div>
      <div><strong>{title}</strong>{children && <span>{children}</span>}</div>
      {action}
    </div>
  )
}

export function Loading() {
  return <div className="cc-loading" role="status"><span aria-hidden="true" />{t('Chargement…')}</div>
}

export function Failed({ error, onRetry }: { error: ApiError; onRetry?: () => void }) {
  const text = error.status === 403 ? t('Ton rôle ne donne pas accès à ces données.') : errorText(error)
  return (
    <div className="cc-failed" role="alert">
      <CCIcon name="alert" /><span>{text}</span>
      {onRetry && error.status !== 403 && <button type="button" className="cc-secondary" onClick={onRetry}>{t('Réessayer')}</button>}
    </div>
  )
}

export function Empty({ icon = 'box', title, children }: { icon?: CCIconName; title: string; children?: ReactNode }) {
  return (
    <div className="cc-empty">
      <span><CCIcon name={icon} size={26} /></span>
      <h2>{title}</h2>
      {children && <p>{children}</p>}
    </div>
  )
}

/** A module the design shows but the backend does not offer yet: said plainly, with what it needs. */
export function Unavailable({ eyebrow, title, needs }: { eyebrow: string; title: string; needs: string }) {
  return (
    <>
      <PageHead eyebrow={eyebrow} title={title} description={t('Ce module n’est pas encore branché : aucune donnée n’est affichée tant qu’elle n’existe pas réellement.')} />
      <section className="cc-panel"><Empty icon="plug" title={t('Pas encore disponible')}>{needs}</Empty></section>
    </>
  )
}

export function Tabs<T extends string>({ value, options, onChange }: { value: T; options: { id: T; label: string; count?: number }[]; onChange: (v: T) => void }) {
  return (
    <div className="cc-tabs" role="tablist">
      {options.map((o) => (
        <button type="button" role="tab" aria-selected={value === o.id} key={o.id} className={value === o.id ? 'active' : ''} onClick={() => onChange(o.id)}>
          {o.label}{o.count !== undefined && <span>{o.count}</span>}
        </button>
      ))}
    </div>
  )
}

export function SearchBox({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  return (
    <label className="cc-searchbox"><CCIcon name="search" /><input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} aria-label={placeholder} /></label>
  )
}

export function Pager({ page, pageSize, total, onPage, noun }: { page: number; pageSize: number; total: number; onPage: (p: number) => void; noun: string }) {
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(total, page * pageSize)
  return (
    <div className="cc-pagination">
      <span>{t('{from}–{to} sur {total} {noun}', { from, to, total, noun })}</span>
      <div>
        <button type="button" disabled={page <= 1} onClick={() => onPage(page - 1)}>{t('Précédent')}</button>
        <button type="button" disabled={to >= total} onClick={() => onPage(page + 1)}>{t('Suivant')}</button>
      </div>
    </div>
  )
}

export function Person({ name, sub }: { name: string; sub?: string }) {
  const letters = name.replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase() || '?'
  return <div className="cc-person"><span>{letters}</span><div><strong>{name || '—'}</strong>{sub && <small>{sub}</small>}</div></div>
}

// --- formatting ---------------------------------------------------------------------------------------------

export const n = (v: number) => (Number.isFinite(v) ? v.toLocaleString(locale()) : '—')

export function when(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' })
}

export function day(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleDateString(locale(), { day: 'numeric', month: 'short', year: 'numeric' })
}

export function money(minor: number, currency: string): string {
  if (!currency) return n(minor / 100)
  try {
    return new Intl.NumberFormat(locale(), { style: 'currency', currency, maximumFractionDigits: minor % 100 ? 2 : 0 }).format(minor / 100)
  } catch {
    return `${currency} ${(minor / 100).toFixed(2)}`
  }
}

export const shortId = (id: string) => (id ? id.slice(0, 8) : '—')
