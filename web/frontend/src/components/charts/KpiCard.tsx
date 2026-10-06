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
  /** The first and largest tile of a board. */
  hero?: boolean
  /** Makes the tile a toggle that chooses what the trend panel below the board shows. */
  selected?: boolean
  onSelect?: () => void
  /** The id of the panel this tile controls. */
  controls?: string
}

/** The change since the period before, as a small coloured chip and the words that say what it is compared with. */
function Delta({ current, previous, goodWhen = 'up', days }: { current: number; previous: number; goodWhen: 'up' | 'down'; days: number }) {
  if (current === 0 && previous === 0) return <span className="tzc-delta flat"><span className="tzc-delta-ref">{t('No activity in either period')}</span></span>
  const change = percentChange(current, previous)
  const diff = current - previous
  const direction = diff === 0 ? 'flat' : diff > 0 ? 'up' : 'down'
  const mood = direction === 'flat' ? 'flat' : direction === goodWhen ? 'good' : 'bad'
  const arrow = direction === 'up' ? '▲' : direction === 'down' ? '▼' : '='
  // Each phrase is whole, so every language can say "the previous 30 days" its own way. With nothing
  // before to compare to, the difference is said in words rather than as an infinite percentage.
  if (direction === 'flat') {
    return <span className="tzc-delta flat"><span className="tzc-delta-ref">{tn(days, 'Same as the previous day', 'Same as the previous {n} days')}</span></span>
  }
  const signed = `${diff > 0 ? '+' : ''}${change === null ? integer(diff) : `${change} %`}`
  const phrase = change === null
    ? tn(days, 'more than the previous day (none then)', 'more than the previous {n} days (none then)')
    : tn(days, 'vs the previous day', 'vs the previous {n} days')
  return (
    <span className={`tzc-delta ${mood}`}>
      <span className="tzc-delta-chip"><span aria-hidden="true">{arrow}</span> {signed}</span>{' '}
      <span className="tzc-delta-ref">{phrase}</span>
    </span>
  )
}

/** One headline figure with its trend and its comparison to the period before. */
export function KpiCard({ label, value, icon, current, previous, goodWhen = 'up', days = 30, note, spark, hero, selected, onSelect, controls }: KpiCardProps) {
  const body = (
    <>
      <span className="tzc-kpi-top"><span className="tzc-kpi-icon" aria-hidden="true"><Icon name={icon} size={hero ? 20 : 17} /></span><small>{label}</small></span>
      <strong className="tzc-kpi-value">{value.replace(/ /g, ' ')}</strong>
      {current !== undefined && previous !== undefined && <Delta current={current} previous={previous} goodWhen={goodWhen} days={days} />}
      {note && <span className="tzc-kpi-note">{note}</span>}
      {spark && <Sparkline values={spark} tall={hero} />}
    </>
  )
  const cls = `tzc-kpi${hero ? ' lead' : ''}`
  return onSelect
    ? <button type="button" className={cls} aria-pressed={selected} aria-controls={controls} onClick={onSelect}>{body}</button>
    : <div className={cls}>{body}</div>
}
