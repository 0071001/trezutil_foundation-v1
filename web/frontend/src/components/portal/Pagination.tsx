import { t } from '@/lib/i18n'

export interface PaginationProps {
  /** 1-based. */
  page: number
  pageSize: number
  total: number
  onPage: (page: number) => void
}

/**
 * Previous / next with a plain-language position. Works for server paging
 * (limit/offset APIs) and for client paging of rows already loaded.
 */
export function Pagination({ page, pageSize, total, onPage }: PaginationProps) {
  const pages = Math.max(1, Math.ceil(total / Math.max(1, pageSize)))
  const current = Math.min(Math.max(1, page), pages)
  const from = total === 0 ? 0 : (current - 1) * pageSize + 1
  const to = Math.min(total, current * pageSize)
  if (total <= pageSize) return null
  return (
    <div className="tz-pager">
      <span role="status" aria-live="polite">{t('Showing {from}–{to} of {total}', { from, to, total })} · {t('Page {page} of {pages}', { page: current, pages })}</span>
      <div className="tz-pager-buttons">
        <button type="button" className="tz-small-btn" disabled={current <= 1} onClick={() => onPage(current - 1)}>{t('Previous')}</button>
        <button type="button" className="tz-small-btn" disabled={current >= pages} onClick={() => onPage(current + 1)}>{t('Next')}</button>
      </div>
    </div>
  )
}
