/** A tiny trend line along the foot of a KPI tile. Decorative: the figure above it says everything. */
export function Sparkline({ values, tall = false }: { values: number[]; tall?: boolean }) {
  const w = 120
  const h = tall ? 44 : 32
  if (values.length < 2 || Math.max(...values) <= 0) return <span className={`tzc-spark tzc-spark-empty${tall ? ' tall' : ''}`} aria-hidden="true" />
  const max = Math.max(...values)
  const pts = values.map((v, i) => [(i / (values.length - 1)) * w, h - 3 - (v / max) * (h - 8)] as const)
  const line = pts.map(([px, py], i) => `${i === 0 ? 'M' : 'L'}${px.toFixed(1)} ${py.toFixed(1)}`).join(' ')
  return (
    <svg className={`tzc-spark${tall ? ' tall' : ''}`} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" aria-hidden="true" focusable="false">
      <path className="tzc-spark-area" d={`${line} L${w} ${h} L0 ${h} Z`} />
      <path className="tzc-spark-line" d={line} />
    </svg>
  )
}
