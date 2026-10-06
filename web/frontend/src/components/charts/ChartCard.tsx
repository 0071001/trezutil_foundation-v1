import { useId, useState, type ReactNode } from 'react'
import { t } from '@/lib/i18n'

export interface ChartTable {
  headers: string[]
  rows: string[][]
}

export interface ChartCardProps {
  /** An id for the card, so a control elsewhere can refer to it (aria-controls). */
  id?: string
  title: string
  /** What is measured and over which period: "Orders placed, last 30 days". */
  subtitle?: string
  /** Controls on the right of the title (a currency switch). */
  actions?: ReactNode
  /** The chart's numbers as a table, for people who read tables better than shapes. */
  table?: ChartTable
  /** Shown instead of the chart when there is nothing to draw. */
  empty?: string
  loading?: boolean
  /** A card spanning two columns of the grid. */
  wide?: boolean
  children: ReactNode
}

/** A titled panel for one chart, with a "View as a table" switch that shows the same numbers as text. */
export function ChartCard({ id: given, title, subtitle, actions, table, empty, loading, wide, children }: ChartCardProps) {
  const [asTable, setAsTable] = useState(false)
  const generated = useId()
  const id = given ?? generated
  return (
    <section id={given} className={`tzc-card ${wide ? 'wide' : ''}`} aria-labelledby={`${id}-title`} aria-busy={loading || undefined}>
      <div className="tzc-card-head">
        <div>
          <h2 id={`${id}-title`}>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        <div className="tzc-card-actions">
          {actions}
          {table && !loading && !empty && (
            <button type="button" className="tz-small-btn" aria-pressed={asTable} onClick={() => setAsTable((v) => !v)}>{asTable ? t('View as a chart') : t('View as a table')}</button>
          )}
        </div>
      </div>
      <div className="tzc-card-body">
        {loading ? <div className="skeleton tzc-skeleton" aria-hidden="true" />
          : empty ? <p className="tzc-empty">{empty}</p>
          : asTable && table ? (
            <div className="tzc-table-wrap">
              <table className="tzc-table">
                <caption className="sr-only">{title}</caption>
                <thead><tr>{table.headers.map((h, i) => <th key={h} scope="col" className={i > 0 ? 'end' : undefined}>{h}</th>)}</tr></thead>
                <tbody>{table.rows.map((row, r) => <tr key={r}>{row.map((cell, i) => <td key={i} className={i > 0 ? 'end' : undefined}>{cell}</td>)}</tr>)}</tbody>
              </table>
            </div>
          ) : children}
      </div>
    </section>
  )
}
