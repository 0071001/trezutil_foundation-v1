import { Link } from 'react-router'
import { t } from '@/lib/i18n'

export interface Crumb {
  label: string
  /** Omit for the current page (and for a group that is not a page). */
  to?: string
}

/** Where the user is, as links. The last crumb is the current page. */
export function Breadcrumbs({ items }: { items: Crumb[] }) {
  if (items.length === 0) return null
  return (
    <nav className="tz-crumbs" aria-label={t('Breadcrumb')}>
      <ol>
        {items.map((c, i) => {
          const last = i === items.length - 1
          return (
            <li key={`${c.label}-${i}`}>
              {c.to && !last ? <Link to={c.to}>{c.label}</Link> : <span aria-current={last ? 'page' : undefined}>{c.label}</span>}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
