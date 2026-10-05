/** A tiny trend line for a KPI card. Decorative: the figure beside it says everything. */
export function Sparkline({ values }: { values: number[] }) {
  const w = 96
  const h = 28
  if (values.length < 2 || Math.max(...values) <= 0) return <span className="tzc-spark tzc-spark-empty" aria-hidden="true" />
  const max = Math.max(...values)
  const pts = values.map((v, i) => [(i / (values.length - 1)) * w, h - 2 - (v / max) * (h - 6)] as const)
  const line = pts.map(([px, py], i) => `${i === 0 ? 'M' : 'L'}${px.toFixed(1)} ${py.toFixed(1)}`).join(' ')
  return (
    <svg className="tzc-spark" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" aria-hidden="true" focusable="false">
      <path className="tzc-spark-area" d={`${line} L${w} ${h} L0 ${h} Z`} />
      <path className="tzc-spark-line" d={line} />
    </svg>
  )
}
