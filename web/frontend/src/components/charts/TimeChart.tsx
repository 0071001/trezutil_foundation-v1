import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { t } from '@/lib/i18n'
import { compact, labelIndices, longDay, niceScale, shortDay } from '@/components/charts/scale'

export interface ChartSeries {
  id: string
  label: string
  values: number[]
  /** How one value reads in the tooltip and the table; defaults to the chart's `format`. */
  format?: (value: number) => string
}

export interface TimeChartProps {
  /** Names the chart: "Orders per day". */
  label: string
  /** One calendar day ("2026-10-04") per point. */
  dates: string[]
  series: ChartSeries[]
  kind: 'area' | 'line' | 'bars'
  format?: (value: number) => string
  /** Shorter form for the axis; defaults to compact numbers. */
  axisFormat?: (value: number) => string
}

/** Used until the chart has been measured (and where nothing can be measured, as in tests). */
const DEFAULT_BOX = { w: 640, h: 240 }

/** The width the chart is displayed at: drawing at that size keeps the text at its real size. */
function useBox() {
  const ref = useRef<HTMLDivElement>(null)
  const [box, setBox] = useState(DEFAULT_BOX)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(([entry]) => {
      const w = Math.round(entry?.contentRect.width ?? 0)
      if (w > 0) setBox({ w, h: Math.max(180, Math.min(320, Math.round(w * 0.36))) })
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])
  return { ref, W: box.w, H: box.h }
}

const plain = (n: number) => String(n)

/** The sentence a screen reader hears for the whole chart. */
export function summarize(label: string, dates: string[], series: ChartSeries[], format: (n: number) => string): string {
  if (dates.length === 0) return label
  const parts = series.map((s) => {
    const total = s.values.reduce((a, b) => a + b, 0)
    const peak = Math.max(...s.values)
    const at = s.values.indexOf(peak)
    const fmt = s.format ?? format
    return peak > 0
      ? t('{series}: total {total}, highest on {day} ({value})', { series: s.label, total: fmt(total), day: shortDay(dates[at]!), value: fmt(peak) })
      : t('{series}: no activity', { series: s.label })
  })
  return `${label}. ${t('From {from} to {to}.', { from: shortDay(dates[0]!), to: shortDay(dates[dates.length - 1]!) })} ${parts.join(' ')}`
}

/**
 * Values over days, as an area, a line or bars. Hover or touch shows the day's
 * values; with the keyboard, arrows walk the days and the day is announced.
 * The chart is drawn from the numbers it is given and nothing else: the y axis
 * starts at zero and no point is smoothed, interpolated or invented.
 */
export function TimeChart({ label, dates, series, kind, format = plain, axisFormat }: TimeChartProps) {
  const [active, setActive] = useState<number | null>(null)
  const id = useId()
  const { ref, W, H } = useBox()
  const M = { top: 14, right: 12, bottom: 28, left: W < 420 ? 40 : 48 }
  const INNER_W = W - M.left - M.right
  const INNER_H = H - M.top - M.bottom
  const n = dates.length
  const { max, ticks: gridTicks } = useMemo(() => {
    const values = series.flatMap((s) => s.values)
    return niceScale(Math.max(0, ...values), 4, values.every(Number.isInteger))
  }, [series])
  const band = n > 0 ? INNER_W / n : INNER_W
  const x = (i: number) => (kind === 'bars' ? M.left + band * (i + 0.5) : M.left + (n <= 1 ? INNER_W / 2 : (i / (n - 1)) * INNER_W))
  const y = (v: number) => M.top + INNER_H - (Math.min(v, max) / max) * INNER_H
  const axis = axisFormat ?? ((v: number) => compact(v))
  const labelAt = labelIndices(n, W < 420 ? 3 : n > 14 ? 5 : 4)

  const move = (e: PointerEvent<SVGRectElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    if (rect.width <= 0 || n === 0) return
    const rel = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    setActive(kind === 'bars' ? Math.min(n - 1, Math.floor(rel * n)) : Math.round(rel * Math.max(0, n - 1)))
  }
  const key = (e: KeyboardEvent<HTMLDivElement>) => {
    if (n === 0) return
    const step = (to: number) => { e.preventDefault(); setActive(Math.min(n - 1, Math.max(0, to))) }
    if (e.key === 'ArrowRight') step((active ?? -1) + 1)
    else if (e.key === 'ArrowLeft') step((active ?? n) - 1)
    else if (e.key === 'Home') step(0)
    else if (e.key === 'End') step(n - 1)
    else if (e.key === 'Escape') setActive(null)
  }

  const path = (values: number[]) => values.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)} ${y(v).toFixed(1)}`).join(' ')
  const baseline = M.top + INNER_H
  const tipLeft = active === null ? 0 : (x(active) / W) * 100
  const flip = active !== null && x(active) > W * 0.6

  return (
    <div className="tzc" ref={ref} onKeyDown={key} onBlur={() => setActive(null)} onPointerLeave={() => setActive(null)} tabIndex={0} role="group" aria-label={summarize(label, dates, series, format)} aria-describedby={`${id}-hint`}>
      <span id={`${id}-hint`} className="sr-only">{t('Use the left and right arrow keys to read each day.')}</span>
      <svg viewBox={`0 0 ${W} ${H}`} width={W} height={H} aria-hidden="true" focusable="false" className="tzc-svg" data-kind={kind} data-has-active={active !== null || undefined}>
        {gridTicks.map((v) => (
          <g key={v}>
            <line className={`tzc-grid${v === 0 ? ' base' : ''}`} x1={M.left} x2={W - M.right} y1={y(v)} y2={y(v)} />
            <text className="tzc-tick" x={M.left - 8} y={y(v) + 4} textAnchor="end">{axis(v)}</text>
          </g>
        ))}
        {labelAt.map((i) => <text key={i} className="tzc-tick" x={x(i)} y={H - 8} textAnchor={i === 0 && kind !== 'bars' ? 'start' : i === n - 1 && kind !== 'bars' ? 'end' : 'middle'}>{shortDay(dates[i]!)}</text>)}

        {kind === 'bars' && series.slice(0, 1).map((s) => s.values.map((v, i) => {
          const h = baseline - y(v)
          return <rect key={dates[i]} className="tzc-bar" data-series={0} data-active={active === i || undefined} x={x(i) - Math.min(band * 0.7, 28) / 2} width={Math.min(band * 0.7, 28)} y={baseline - Math.max(h, v > 0 ? 2 : 0)} height={Math.max(h, v > 0 ? 2 : 0)} rx={3} />
        }))}
        {kind !== 'bars' && series.map((s, si) => (
          <g key={s.id} data-series={si}>
            {kind === 'area' && n > 1 && <path className="tzc-area" d={`${path(s.values)} L${x(n - 1).toFixed(1)} ${baseline} L${x(0).toFixed(1)} ${baseline} Z`} />}
            {n > 1 ? <path className="tzc-line" d={path(s.values)} /> : s.values.map((v, i) => <circle key={i} className="tzc-dot" cx={x(i)} cy={y(v)} r={4} />)}
          </g>
        ))}

        {active !== null && (
          <g>
            {kind !== 'bars' && <line className="tzc-guide" x1={x(active)} x2={x(active)} y1={M.top} y2={baseline} />}
            {kind !== 'bars' && series.map((s, si) => <circle key={s.id} className="tzc-dot" data-series={si} cx={x(active)} cy={y(s.values[active] ?? 0)} r={4.5} />)}
          </g>
        )}
        <rect x={M.left} y={M.top} width={INNER_W} height={INNER_H} fill="transparent" onPointerMove={move} onPointerDown={move} />
      </svg>

      {active !== null && dates[active] && (
        <div className="tzc-tip" style={{ left: `${tipLeft}%`, transform: flip ? 'translateX(calc(-100% - 10px))' : 'translateX(10px)' }}>
          <strong>{longDay(dates[active]!)}</strong>
          {series.map((s, si) => <span key={s.id} data-series={si}><i aria-hidden="true" />{series.length > 1 || kind !== 'bars' ? `${s.label}: ` : ''}{(s.format ?? format)(s.values[active] ?? 0)}</span>)}
        </div>
      )}
      <div className="sr-only" role="status" aria-live="polite">
        {active !== null && dates[active] ? `${longDay(dates[active]!)}: ${series.map((s) => `${s.label} ${(s.format ?? format)(s.values[active] ?? 0)}`).join(', ')}` : ''}
      </div>
    </div>
  )
}

/** The chart's numbers as table rows: a date, then one column per series. */
export function seriesTable(dates: string[], series: ChartSeries[], format: (n: number) => string = plain): { headers: string[]; rows: string[][] } {
  return {
    headers: [t('Day'), ...series.map((s) => s.label)],
    rows: dates.map((d, i) => [longDay(d), ...series.map((s) => (s.format ?? format)(s.values[i] ?? 0))]),
  }
}
