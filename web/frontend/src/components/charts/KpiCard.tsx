import { Icon, type IconName } from '@/components/ui'
import { t, tn } from '@/lib/i18n'
import { integer, percentChange } from '@/components/charts/scale'
import { Sparkline } from '@/components/charts/Sparkline'

export interface KpiCardProps {
  label: string
  /** Already formatted: "1 240", "$4,734". */
  value: string
  icon: IconName
  /** This period's figure and the previous period's, for the comparison. */
  current?: number
  previous?: number
  /** Which direction is good news: more orders are, more cancellations are not. */
  goodWhen?: 'up' | 'down'
  /** The length of each of the two periods compared, in days. */
  days?: number
  /** One line under the figure ("Returning buyers: 2"). */
  note?: string
  /** A per-day trend. */
  spark?: number[]
}

function Delta({ current, previous, goodWhen = 'up', days }: { current: number; previous: number; goodWhen: 'up' | 'down'; days: number }) {
  if (current === 0 && previous === 0) return <span className="tzc-delta flat">{t('No activity in either period')}</span>
  const change = percentChange(current, previous)
  const diff = current - previous
  const direction = diff === 0 ? 'flat' : diff > 0 ? 'up' : 'down'
  const mood = direction === 'flat' ? 'flat' : direction === goodWhen ? 'good' : 'bad'
  const arrow = direction === 'up' ? '▲' : direction === 'down' ? '▼' : '='
  const signed = `${diff > 0 ? '+' : ''}${change}`
  // Each sentence is whole, so every language can phrase "the previous 30 days" its own way.
  // With nothing before to compare to, the difference is said in words rather than as an infinite percentage.
  const text = direction === 'flat'
    ? tn(days, 'Same as the previous day', 'Same as the previous {n} days')
    : change === null
      ? tn(days, '{count} more than the previous day (none then)', '{count} more than the previous {n} days (none then)', { count: integer(diff) })
      : tn(days, '{percent} % vs the previous day', '{percent} % vs the previous {n} days', { percent: signed })
  return <span className={`tzc-delta ${mood}`}><span aria-hidden="true">{arrow}</span> {text}</span>
}

/** One headline figure with its trend and its comparison to the period before. */
export function KpiCard({ label, value, icon, current, previous, goodWhen = 'up', days = 30, note, spark }: KpiCardProps) {
  return (
    <div className="tzc-kpi">
      <div className="tzc-kpi-top"><span className="tzc-kpi-icon" aria-hidden="true"><Icon name={icon} size={18} /></span><small>{label}</small></div>
      <strong className="tzc-kpi-value">{value}</strong>
      {current !== undefined && previous !== undefined && <Delta current={current} previous={previous} goodWhen={goodWhen} days={days} />}
      {note && <p className="tzc-kpi-note">{note}</p>}
      {spark && <Sparkline values={spark} />}
    </div>
  )
}
