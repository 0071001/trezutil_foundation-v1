import { useEffect, type ReactNode } from 'react'
import { ModuleHead } from '@/components/portal/ModuleState'
import { EmptyState } from '@/components/states'
import { locale, t } from '@/lib/i18n'

/*
 * Small pieces shared by the console modules: a row of figures, a titled panel, a fact in a drawer, a text field
 * and a reason field for dialogs. Every figure shown with them comes from a server answer; none is computed from a
 * partial list in the browser.
 */

export const isoDate = (iso: string) => (iso ? new Date(iso).toLocaleDateString(locale(), { dateStyle: 'medium' }) : '—')
export const isoDateTime = (iso: string) => (iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' }) : '—')
export const shortId = (id: string) => (id ? `#${id.slice(0, 8)}` : '—')
export const count = (n: number) => new Intl.NumberFormat(locale()).format(n)

export interface Stat {
  label: string
  value: string
  note?: string
  tone?: 'good' | 'attention' | 'neutral'
}

/** A row of figures, each with what it counts. */
export function StatTiles({ items, label }: { items: Stat[]; label: string }) {
  return (
    <ul className="tzx-stats" aria-label={label}>
      {items.map((s) => (
        <li key={s.label} className={s.tone ? `tone-${s.tone}` : undefined}>
          <small>{s.label}</small>
          <strong>{s.value}</strong>
          {s.note && <span>{s.note}</span>}
        </li>
      ))}
    </ul>
  )
}

/** A titled block of a module page. */
export function Panel({ title, copy, actions, children }: { title: string; copy?: string; actions?: ReactNode; children: ReactNode }) {
  return (
    <section className="tzx-panel">
      <header>
        <div><h2>{title}</h2>{copy && <p>{copy}</p>}</div>
        {actions && <div className="tzx-panel-actions">{actions}</div>}
      </header>
      {children}
    </section>
  )
}

/** One fact in a drawer or a panel. */
export const Fact = ({ label, children }: { label: string; children: ReactNode }) => <div><dt>{label}</dt><dd>{children}</dd></div>

/** A labelled input for console forms and dialogs. */
export function TextField({ label, value, onChange, hint, type = 'text', multiline, maxLength, required, autoComplete, inputMode }: {
  label: string
  value: string
  onChange: (v: string) => void
  hint?: string
  type?: string
  multiline?: boolean
  maxLength?: number
  required?: boolean
  autoComplete?: string
  inputMode?: 'text' | 'tel' | 'email' | 'numeric'
}) {
  return (
    <label className="tz-dialog-field tzx-field">
      <span>{label}{required ? ' *' : ''}</span>
      {multiline
        ? <textarea value={value} maxLength={maxLength} rows={3} onChange={(e) => onChange(e.target.value)} />
        : <input type={type} value={value} maxLength={maxLength} required={required} autoComplete={autoComplete} inputMode={inputMode} onChange={(e) => onChange(e.target.value)} />}
      {hint && <small>{hint}</small>}
    </label>
  )
}

/** The reason that goes to the audit trail with a sensitive change. */
export function ReasonField({ value, onChange, optional }: { value: string; onChange: (v: string) => void; optional?: boolean }) {
  return (
    <label className="tz-dialog-field">
      <span>{optional ? t('Note (optional, kept in the audit trail)') : t('Reason (kept in the audit trail)')}</span>
      <textarea value={value} maxLength={300} rows={3} onChange={(e) => onChange(e.target.value)} />
    </label>
  )
}

/** The page frame of a module: its heading, and the reason when the role cannot use it. */
export function ModulePage({ eyebrow, title, copy, allowed = true, deniedCopy, children }: {
  eyebrow: string
  title: string
  copy: string
  allowed?: boolean
  deniedCopy?: string
  children: ReactNode
}) {
  useEffect(() => { document.title = `${title} — Trezutil` }, [title])
  return (
    <div className="tz-module">
      <ModuleHead eyebrow={eyebrow} title={title} copy={copy} />
      {allowed ? children : <EmptyState icon="user" title={t('You don’t have access')} copy={deniedCopy ?? t('This page is for the owner, admins and managers of this business.')} />}
    </div>
  )
}

/** Money per currency, never added across currencies: "$12.00 · FC 5,000". */
export function perCurrency(amounts: Record<string, number>, format: (minor: number, currency: string) => string): string {
  const parts = Object.entries(amounts).filter(([, v]) => v !== 0).map(([c, v]) => format(v, c))
  return parts.length ? parts.join(' · ') : '—'
}
