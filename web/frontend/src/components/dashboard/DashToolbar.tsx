import type { ReactNode } from 'react'
import type { Analytics } from '@/api/analytics'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { exportAnalytics } from '@/components/dashboard/csv'
import { Icon } from '@/components/ui'
import { t } from '@/lib/i18n'

/** The bar above a dashboard: when the figures were counted, and the controls that change or take them. */
export function DashToolbar({ asOf, period, onPeriod, onRefresh, exportData, exportName, children }: {
  asOf?: string
  period: number
  onPeriod: (days: number) => void
  onRefresh?: () => void
  /** The figures on screen; with them, the toolbar offers "Export" (a CSV of the same numbers). */
  exportData?: Analytics | null
  /** The business's name, used in the file name. */
  exportName?: string
  children?: ReactNode
}) {
  return (
    <div className="tz-toolbar">
      {asOf ? <p className="tz-asof">{asOf}</p> : <span />}
      <div className="tz-dash-controls">
        <PeriodTabs value={period} onChange={onPeriod} />
        {onRefresh && <button type="button" className="tz-tool-btn" onClick={onRefresh}><Icon name="refresh" size={15} />{t('Refresh')}</button>}
        {exportData && <button type="button" className="tz-tool-btn" title={t('Download the day-by-day figures as a CSV file')} onClick={() => exportAnalytics(exportData, exportName)}><Icon name="download" size={15} />{t('Export')}</button>}
        {children}
      </div>
    </div>
  )
}

/** The shape of a board and its chart while the figures load: the page does not jump when they arrive. */
export function DashSkeleton({ label }: { label: string }) {
  return (
    <div className="tz-skel" role="status">
      <span className="sr-only">{label}</span>
      <div className="tz-board" aria-hidden="true">
        {Array.from({ length: 8 }, (_, i) => <div key={i} className={`skeleton tz-skel-tile${i === 0 ? ' lead' : ''}`} />)}
      </div>
      <div className="skeleton tz-skel-panel" aria-hidden="true" />
    </div>
  )
}
