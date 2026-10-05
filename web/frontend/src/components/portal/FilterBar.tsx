import type { ReactNode } from 'react'
import { Icon } from '@/components/ui'
import { t } from '@/lib/i18n'

export interface FilterBarProps {
  /** Names the search landmark ("Filter orders"). */
  label: string
  search?: { value: string; onChange: (value: string) => void; label: string; placeholder?: string }
  /** Extra controls: selects, tabs, toggles. */
  children?: ReactNode
  /** Shows the reset button when something is filtered. */
  active?: boolean
  onReset?: () => void
}

/** The row above a table: search, filters and a reset that appears only when it has something to reset. */
export function FilterBar({ label, search, children, active, onReset }: FilterBarProps) {
  return (
    <form className="tz-filters" role="search" aria-label={label} onSubmit={(e) => e.preventDefault()}>
      {search && (
        <label className="tz-search">
          <Icon name="search" size={16} />
          <span className="sr-only">{search.label}</span>
          <input type="search" value={search.value} placeholder={search.placeholder} maxLength={100} onChange={(e) => search.onChange(e.target.value)} />
        </label>
      )}
      {children}
      {active && onReset && <button type="button" className="tz-small-btn" onClick={onReset}>{t('Reset filters')}</button>}
    </form>
  )
}
