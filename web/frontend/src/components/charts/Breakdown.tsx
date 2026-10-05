import { t } from '@/lib/i18n'
import { integer } from '@/components/charts/scale'

export interface BreakdownItem {
  key: string
  label: string
  value: number
  /** 1 to 6: a colour of the chart palette. */
  tone?: number
}

/** Statuses read the same everywhere: done is teal, waiting is amber, lost is coral, the rest blue. */
export function statusTone(status: string): number {
  switch (status) {
    case 'paid': case 'confirmed': case 'completed': case 'fulfilled': case 'published': case 'available': return 2
    case 'pending': case 'pending_payment': case 'pending_review': case 'processing': case 'ready_for_fulfillment': case 'draft': case 'paused': return 5
    case 'cancelled': case 'refunded': case 'expired': case 'disputed': case 'suspended': case 'closed': return 3
    default: return 1
  }
}

const percent = (value: number, total: number) => (total > 0 ? Math.round((value / total) * 100) : 0)

/**
 * Parts of a whole as horizontal bars with their count and share. Every bar
 * carries its label and number as text, so nothing depends on colour.
 */
export function BreakdownBars({ items, label }: { items: BreakdownItem[]; label: string }) {
  const shown = items.filter((i) => i.value > 0).sort((a, b) => b.value - a.value)
  const total = shown.reduce((sum, i) => sum + i.value, 0)
  if (total === 0) return <p className="tzc-empty">{t('Nothing to show for this period.')}</p>
  const top = shown[0]!.value
  return (
    <ul className="tzc-bars" aria-label={label}>
      {shown.map((item) => (
        <li key={item.key}>
          <div className="tzc-bars-row"><span>{item.label}</span><strong>{integer(item.value)} <small>{percent(item.value, total)} %</small></strong></div>
          <div className="tzc-bars-track" aria-hidden="true"><span data-series={(item.tone ?? 1) - 1} style={{ width: `${Math.max(2, (item.value / top) * 100)}%` }} /></div>
        </li>
      ))}
    </ul>
  )
}

/** The same parts as a ring with the total in the middle, and a legend that repeats every number. */
export function Donut({ items, label, centre }: { items: BreakdownItem[]; label: string; centre: string }) {
  const shown = items.filter((i) => i.value > 0).sort((a, b) => b.value - a.value)
  const total = shown.reduce((sum, i) => sum + i.value, 0)
  if (total === 0) return <p className="tzc-empty">{t('Nothing to show for this period.')}</p>
  const r = 52
  const c = 2 * Math.PI * r
  let offset = 0
  return (
    <div className="tzc-donut">
      <svg viewBox="0 0 140 140" aria-hidden="true" focusable="false" className="tzc-donut-svg">
        <circle className="tzc-donut-track" cx="70" cy="70" r={r} />
        {shown.map((item) => {
          const length = (item.value / total) * c
          const seg = <circle key={item.key} className="tzc-donut-seg" data-series={(item.tone ?? 1) - 1} cx="70" cy="70" r={r} strokeDasharray={`${Math.max(0, length - (shown.length > 1 ? 2 : 0))} ${c}`} strokeDashoffset={-offset} transform="rotate(-90 70 70)" />
          offset += length
          return seg
        })}
        <text x="70" y="68" textAnchor="middle" className="tzc-donut-total">{integer(total)}</text>
        <text x="70" y="86" textAnchor="middle" className="tzc-donut-caption">{centre}</text>
      </svg>
      <ul className="tzc-legend" aria-label={label}>
        {shown.map((item) => (
          <li key={item.key} data-series={(item.tone ?? 1) - 1}><i aria-hidden="true" /><span>{item.label}</span><strong>{integer(item.value)} <small>{percent(item.value, total)} %</small></strong></li>
        ))}
      </ul>
    </div>
  )
}
