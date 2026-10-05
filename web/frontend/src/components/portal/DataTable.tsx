import { useMemo, useState, type ReactNode } from 'react'
import { Pagination } from '@/components/portal/Pagination'
import { t } from '@/lib/i18n'

export interface Column<T> {
  id: string
  header: string
  cell: (row: T) => ReactNode
  /** The value rows are ordered by; omit to make the column unsortable. */
  sort?: (row: T) => string | number
  align?: 'end'
  /** The identity column: first line strong, a second line muted. On phones it heads the stacked row. */
  primary?: boolean
  /** Label shown beside the value when rows stack on a phone (defaults to the header). */
  label?: string
}

export interface DataTableProps<T> {
  /** Names the table for assistive technology. */
  caption: string
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  /** Opens the record (a DetailDrawer); rows become keyboard-operable. */
  onOpen?: (row: T) => void
  initialSort?: { column: string; direction: 'asc' | 'desc' }
  /** Client paging of the rows already loaded. */
  pageSize?: number
  /** Server paging: `rows` is the current page. */
  serverPaging?: { page: number; pageSize: number; total: number; onPage: (page: number) => void }
  loading?: boolean
  /** Shown instead of the table (an ErrorState). */
  error?: ReactNode
  /** Shown when there are no rows. */
  empty?: ReactNode
  /** A limit the reader must know ("Showing the latest 100 orders"). */
  notice?: string
  densityToggle?: boolean
}

function compare(a: string | number, b: string | number): number {
  if (typeof a === 'number' && typeof b === 'number') return a - b
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}

/**
 * A real table: sortable headers (`aria-sort`), a primary identity column,
 * client or server paging, keyboard-operable rows that open a detail drawer,
 * a density switch, and a stacked layout under 760 px. A limit on the data is
 * stated (`notice`), never hidden.
 */
export function DataTable<T>({ caption, columns, rows, rowKey, onOpen, initialSort, pageSize, serverPaging, loading, error, empty, notice, densityToggle }: DataTableProps<T>) {
  const [sort, setSort] = useState(initialSort ?? null)
  const [page, setPage] = useState(1)
  const [compact, setCompact] = useState(false)

  const sorted = useMemo(() => {
    const col = columns.find((c) => c.id === sort?.column)
    if (!col?.sort || !sort) return rows
    const key = col.sort
    const dir = sort.direction === 'asc' ? 1 : -1
    return [...rows].sort((a, b) => dir * compare(key(a), key(b)))
  }, [rows, columns, sort])

  if (error) return <>{error}</>

  const paged = !serverPaging && pageSize ? sorted.slice((page - 1) * pageSize, page * pageSize) : sorted
  const visible = serverPaging ? sorted : paged
  const toggle = (id: string) => setSort((s) => (s?.column === id ? { column: id, direction: s.direction === 'asc' ? 'desc' : 'asc' } : { column: id, direction: 'asc' }))
  const ariaSort = (c: Column<T>): 'ascending' | 'descending' | 'none' | undefined => (c.sort ? (sort?.column === c.id ? (sort.direction === 'asc' ? 'ascending' : 'descending') : 'none') : undefined)
  const captionText = onOpen ? `${caption}. ${t('Press Enter on a row to open its details.')}` : caption

  return (
    <div>
      {(densityToggle || notice) && (
        <div className="tz-table-tools">
          {notice ? <p className="tz-notice" role="note">{notice}</p> : <span />}
          {densityToggle && <button type="button" className="tz-density" aria-pressed={compact} onClick={() => setCompact((c) => !c)}>{compact ? t('Comfortable rows') : t('Compact rows')}</button>}
        </div>
      )}
      <div className="tz-table-wrap">
        <table className={`tz-table ${compact ? 'compact' : ''}`} aria-busy={loading || undefined}>
          <caption className="sr-only">{captionText}</caption>
          <thead>
            <tr>
              {columns.map((c) => (
                <th key={c.id} scope="col" className={c.align === 'end' ? 'end' : undefined} aria-sort={ariaSort(c)}>
                  {c.sort ? <button type="button" onClick={() => toggle(c.id)}>{c.header}<span aria-hidden="true">{sort?.column === c.id ? (sort.direction === 'asc' ? '↑' : '↓') : ''}</span></button> : c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading && Array.from({ length: 5 }, (_, i) => (
              <tr key={`s${i}`} aria-hidden="true">{columns.map((c) => <td key={c.id} data-label={c.label ?? c.header}><span className="skeleton" style={{ display: 'block', width: c.primary ? '70%' : '45%' }} /></td>)}</tr>
            ))}
            {!loading && visible.length === 0 && (
              <tr><td className="tz-empty-row" colSpan={columns.length}>{empty ?? t('Nothing to show.')}</td></tr>
            )}
            {!loading && visible.map((row) => (
              <tr
                key={rowKey(row)}
                data-open={onOpen ? 'true' : undefined}
                tabIndex={onOpen ? 0 : undefined}
                onClick={onOpen ? () => onOpen(row) : undefined}
                onKeyDown={onOpen ? (e) => { if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); onOpen(row) } } : undefined}
              >
                {columns.map((c) => (
                  <td key={c.id} className={[c.align === 'end' ? 'end' : '', c.primary ? 'tz-cell-primary' : ''].filter(Boolean).join(' ') || undefined} data-label={c.label ?? c.header}>{c.cell(row)}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {serverPaging
        ? <Pagination page={serverPaging.page} pageSize={serverPaging.pageSize} total={serverPaging.total} onPage={serverPaging.onPage} />
        : pageSize && <Pagination page={page} pageSize={pageSize} total={sorted.length} onPage={setPage} />}
    </div>
  )
}
