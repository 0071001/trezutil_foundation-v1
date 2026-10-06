import { useId, useState, type ReactNode } from 'react'
import { KpiCard, type KpiCardProps } from '@/components/charts/KpiCard'

/*
 * The key figures of a period as a board of tiles, and under it the chart of
 * the figure that is selected. One chart at a time keeps the page short; every
 * figure is still one click away, and each chart keeps its "View as a table".
 * A tile with no chart (buyers, live listings) is a plain figure.
 */

export interface BoardMetric {
  id: string
  tile: KpiCardProps
  /** The chart this tile opens below the board; a tile without one cannot be selected. */
  panel?: ReactNode
}

export function ActivityBoard({ label, metrics }: { label: string; metrics: BoardMetric[] }) {
  const base = useId()
  const [chosen, setChosen] = useState<string | null>(null)
  const current = metrics.find((m) => m.panel && m.id === chosen) ?? metrics.find((m) => m.panel)
  const panelId = `${base}-trend`
  return (
    <>
      <div className="tz-board" role="group" aria-label={label}>
        {metrics.map((m) => (
          <KpiCard key={m.id} {...m.tile} selected={m.panel ? m.id === current?.id : undefined} onSelect={m.panel ? () => setChosen(m.id) : undefined} controls={m.panel ? panelId : undefined} />
        ))}
      </div>
      {current && <div className="tz-trend" id={panelId} key={current.id}>{current.panel}</div>}
    </>
  )
}
