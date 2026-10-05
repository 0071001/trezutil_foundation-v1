import { Icon, type IconName } from '@/components/ui'

export interface MetricProps {
  label: string
  value: string
  note: string
  icon: IconName
  /** Where the figure comes from ("Incoming orders"). Every number names its source. */
  source?: string
  /** When the figure was read, already formatted. */
  asOf?: string
  /** Set when the figure is computed from a truncated list: "Based on the latest 100 orders". It is then a lower bound, never a total. */
  partial?: string
}

/** One figure with its provenance. Used sparingly: a page gets one strip of these at most. */
export function Metric({ label, value, note, icon, source, asOf, partial }: MetricProps) {
  return (
    <div className="metric">
      <span><Icon name={icon} /></span>
      <small>{label}</small>
      <strong>{value}</strong>
      <p>{note}</p>
      {partial && <small className="metric-source metric-partial">{partial}</small>}
      {(source || asOf) && <small className="metric-source">{[source, asOf].filter(Boolean).join(' · ')}</small>}
    </div>
  )
}
