import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { BreakdownBars, Donut, statusTone } from '@/components/charts/Breakdown'
import { ChartCard } from '@/components/charts/ChartCard'
import { KpiCard } from '@/components/charts/KpiCard'
import { PeriodTabs } from '@/components/charts/PeriodTabs'
import { labelIndices, longDay, niceMax, niceScale, percentChange, shortDay, ticks } from '@/components/charts/scale'
import { TimeChart, seriesTable, summarize } from '@/components/charts/TimeChart'
import { setLang } from '@/lib/i18n'

const DATES = ['2026-10-02', '2026-10-03', '2026-10-04']

describe('chart scales', () => {
  it('rounds the top of an axis up to a number a person would pick', () => {
    expect(niceMax(0)).toBe(1)
    expect(niceMax(-4)).toBe(1)
    expect(niceMax(NaN)).toBe(1)
    expect(niceMax(7)).toBe(10)
    expect(niceMax(10)).toBe(10)
    expect(niceMax(11)).toBe(20)
    expect(niceMax(21)).toBe(25)
    expect(niceMax(26)).toBe(50)
    expect(niceMax(473)).toBe(500)
    expect(niceMax(0.3)).toBe(0.5)
  })

  it('draws ticks from zero to the top', () => {
    expect(ticks(100, 4)).toEqual([0, 25, 50, 75, 100])
  })

  it('builds an axis of equal round steps that always reaches the data', () => {
    for (const max of [0.7, 1, 3, 4, 7, 11, 26, 473, 4734, 99_999]) {
      const axis = niceScale(max, 4)
      expect(axis.max, `max ${max}`).toBeGreaterThanOrEqual(max)
      expect(axis.ticks).toHaveLength(5)
      expect(axis.ticks[0]).toBe(0)
      expect(axis.ticks[4]).toBeCloseTo(axis.max)
    }
    expect(niceScale(4, 4).ticks).toEqual([0, 1, 2, 3, 4])
    expect(niceScale(473, 4).ticks).toEqual([0, 200, 400, 600, 800])
    expect(niceScale(0, 4).max).toBeGreaterThan(0) // an empty chart still has an axis
  })

  it('never puts a fraction on the axis of a chart of counts', () => {
    for (const max of [1, 2, 3, 5, 9, 10, 13]) {
      for (const tick of niceScale(max, 4, true).ticks) expect(Number.isInteger(tick), `max ${max}: ${tick}`).toBe(true)
      expect(niceScale(max, 4, true).max).toBeGreaterThanOrEqual(max)
    }
  })

  it('compares with the period before only when there is one', () => {
    expect(percentChange(150, 100)).toBe(50)
    expect(percentChange(50, 100)).toBe(-50)
    expect(percentChange(5, 0)).toBeNull() // no infinite percentages
    expect(percentChange(0, 0)).toBeNull()
  })

  it('spreads a few labels over the days and always keeps the first and the last', () => {
    expect(labelIndices(0)).toEqual([])
    expect(labelIndices(3, 5)).toEqual([0, 1, 2])
    const picked = labelIndices(30, 5)
    expect(picked[0]).toBe(0)
    expect(picked[picked.length - 1]).toBe(29)
    expect(picked.length).toBeLessThanOrEqual(5)
    expect(picked).toEqual([...picked].sort((a, b) => a - b))
  })

  it('reads a calendar day the same in any time zone and in both languages', () => {
    expect(shortDay('2026-10-04')).toMatch(/Oct/)
    expect(shortDay('2026-10-04')).toMatch(/4/)
    expect(longDay('2026-10-04')).toMatch(/2026/)
    setLang('fr', false)
    expect(shortDay('2026-10-04')).toMatch(/oct/i)
    expect(shortDay('2026-01-01')).toMatch(/janv/i) // the 1st stays the 1st
  })
})

describe('TimeChart', () => {
  const series = [{ id: 'orders', label: 'Orders', values: [0, 1, 4] }]

  it('summarises the whole chart in one sentence for a screen reader', () => {
    render(<TimeChart label="Orders per day" dates={DATES} series={series} kind="bars" />)
    const group = screen.getByRole('group')
    expect(group).toHaveAccessibleName(/Orders per day\. From Oct 2 to Oct 4\. Orders: total 5, highest on Oct 4 \(4\)/)
  })

  it('says so when a series had no activity at all', () => {
    expect(summarize('Orders per day', DATES, [{ id: 'o', label: 'Orders', values: [0, 0, 0] }], String)).toMatch(/Orders: no activity/)
  })

  it('draws one bar per day, tall in proportion, and none invented for a zero', () => {
    const { container } = render(<TimeChart label="Orders per day" dates={DATES} series={series} kind="bars" />)
    const bars = [...container.querySelectorAll('.tzc-bar')]
    expect(bars).toHaveLength(3)
    const heights = bars.map((b) => Number(b.getAttribute('height')))
    expect(heights[0]).toBe(0) // a day with nothing is flat
    expect(heights[2]).toBeGreaterThan(heights[1]!)
    expect(heights[2]! / heights[1]!).toBeCloseTo(4, 0) // 4 vs 1
  })

  it('starts its axis at zero and labels it', () => {
    const { container } = render(<TimeChart label="Orders per day" dates={DATES} series={series} kind="area" />)
    const ticksText = [...container.querySelectorAll('.tzc-tick')].map((t) => t.textContent ?? '')
    expect(ticksText).toEqual(expect.arrayContaining(['0', '1', '2', '3', '4'])) // counts: whole numbers only, ending where the data does
    expect(ticksText.filter((t) => /^\d+\.\d/.test(t))).toEqual([])
  })

  it('draws an area and a line through every day, and a lone day as a dot', () => {
    const { container, rerender } = render(<TimeChart label="Orders per day" dates={DATES} series={series} kind="area" />)
    expect(container.querySelectorAll('.tzc-line')).toHaveLength(1)
    expect(container.querySelectorAll('.tzc-area')).toHaveLength(1)
    rerender(<TimeChart label="One day" dates={['2026-10-04']} series={[{ id: 'o', label: 'Orders', values: [3] }]} kind="area" />)
    expect(container.querySelectorAll('.tzc-line')).toHaveLength(0)
    expect(container.querySelectorAll('.tzc-dot')).toHaveLength(1)
  })

  it('walks the days with the arrow keys and announces each one', async () => {
    render(<TimeChart label="Orders per day" dates={DATES} series={series} kind="bars" format={(n) => `${n} orders`} />)
    const group = screen.getByRole('group')
    group.focus()
    expect(group).toHaveAccessibleDescription('Use the left and right arrow keys to read each day.')
    const live = screen.getByRole('status')
    expect(live).toHaveTextContent('')

    await userEvent.keyboard('{ArrowRight}')
    expect(live).toHaveTextContent(/Fri, Oct 2, 2026: Orders 0 orders/)
    await userEvent.keyboard('{ArrowRight}{ArrowRight}')
    expect(live).toHaveTextContent(/Sun, Oct 4, 2026: Orders 4 orders/)
    await userEvent.keyboard('{ArrowRight}') // stops at the last day
    expect(live).toHaveTextContent(/Oct 4, 2026/)
    await userEvent.keyboard('{Home}')
    expect(live).toHaveTextContent(/Oct 2, 2026/)
    await userEvent.keyboard('{End}')
    expect(live).toHaveTextContent(/Oct 4, 2026/)
    await userEvent.keyboard('{ArrowLeft}')
    expect(live).toHaveTextContent(/Oct 3, 2026/)
    await userEvent.keyboard('{Escape}')
    expect(live).toHaveTextContent('')
  })

  it('shows a tooltip with the day’s values for each series, and clears it on leaving', async () => {
    const two = [{ id: 'a', label: 'Paid', values: [1, 2, 3] }, { id: 'b', label: 'Cancelled', values: [0, 1, 0] }]
    const { container } = render(<TimeChart label="Orders" dates={DATES} series={two} kind="line" />)
    const group = screen.getByRole('group')
    group.focus()
    await userEvent.keyboard('{ArrowRight}{ArrowRight}')
    const tip = container.querySelector('.tzc-tip')!
    expect(tip).toHaveTextContent('Paid: 2')
    expect(tip).toHaveTextContent('Cancelled: 1')
    await userEvent.tab() // focus leaves the chart
    expect(container.querySelector('.tzc-tip')).toBeNull()
  })

  it('never draws a value above its axis or below zero', () => {
    const { container } = render(<TimeChart label="Odd" dates={DATES} series={[{ id: 'o', label: 'Orders', values: [-3, 2, 1_000_000] }]} kind="line" />)
    const d = container.querySelector('.tzc-line')!.getAttribute('d')!
    const ys = [...d.matchAll(/[ML][\d.]+ ([\d.-]+)/g)].map((m) => Number(m[1]))
    for (const y of ys) expect(y).toBeGreaterThanOrEqual(0)
  })

  it('offers the same numbers as table rows', () => {
    const table = seriesTable(DATES, [{ id: 'o', label: 'Orders', values: [0, 1, 4] }], (n) => `${n} orders`)
    expect(table.headers).toEqual(['Day', 'Orders'])
    expect(table.rows).toHaveLength(3)
    expect(table.rows[2]).toEqual([expect.stringMatching(/Oct 4, 2026/), '4 orders'])
  })
})

describe('ChartCard', () => {
  const table = { headers: ['Day', 'Orders'], rows: [['Fri, 2 Oct 2026', '0'], ['Sat, 3 Oct 2026', '1']] }

  it('is a named section with a heading', () => {
    render(<ChartCard title="Orders per day" subtitle="Last 30 days"><p>chart</p></ChartCard>)
    const section = screen.getByRole('region', { name: 'Orders per day' })
    expect(within(section).getByRole('heading', { level: 2, name: 'Orders per day' })).toBeInTheDocument()
    expect(within(section).getByText('Last 30 days')).toBeInTheDocument()
  })

  it('switches between the chart and a real table of the same numbers', async () => {
    render(<ChartCard title="Orders per day" table={table}><p>the chart</p></ChartCard>)
    expect(screen.getByText('the chart')).toBeInTheDocument()
    const toggle = screen.getByRole('button', { name: 'View as a table' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(toggle)
    const grid = screen.getByRole('table', { name: 'Orders per day' })
    expect(within(grid).getAllByRole('columnheader').map((h) => h.textContent)).toEqual(['Day', 'Orders'])
    expect(within(grid).getAllByRole('row')).toHaveLength(3)
    expect(screen.queryByText('the chart')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'View as a chart' }))
    expect(screen.getByText('the chart')).toBeInTheDocument()
  })

  it('shows a plain sentence instead of an empty chart, and no table switch for it', () => {
    render(<ChartCard title="Orders per day" table={table} empty="Nothing in this period."><p>the chart</p></ChartCard>)
    expect(screen.getByText('Nothing in this period.')).toBeInTheDocument()
    expect(screen.queryByText('the chart')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /table/ })).not.toBeInTheDocument()
  })

  it('holds its place while loading', () => {
    const { container } = render(<ChartCard title="Orders per day" loading><p>the chart</p></ChartCard>)
    expect(screen.getByRole('region')).toHaveAttribute('aria-busy', 'true')
    expect(container.querySelector('.skeleton')).not.toBeNull()
    expect(screen.queryByText('the chart')).not.toBeInTheDocument()
  })
})

describe('KpiCard', () => {
  const card = (props: Partial<Parameters<typeof KpiCard>[0]>) => render(<KpiCard label="Orders placed" value="42" icon="orders" days={30} {...props} />)

  it('shows the figure and its label', () => {
    card({})
    expect(screen.getByText('Orders placed')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('says how far up it is, in good news', () => {
    const { container } = card({ current: 150, previous: 100 })
    expect(screen.getByText(/\+50 % vs the previous 30 days/)).toBeInTheDocument()
    expect(container.querySelector('.tzc-delta')).toHaveClass('good')
  })

  it('treats a drop in a good measure as bad news, and a drop in a bad measure as good', () => {
    const down = card({ current: 50, previous: 100 })
    expect(down.container.querySelector('.tzc-delta')).toHaveClass('bad')
    expect(screen.getByText(/-50 % vs the previous 30 days/)).toBeInTheDocument()
    down.unmount()
    const cancelled = card({ label: 'Cancelled', current: 2, previous: 5, goodWhen: 'down' })
    expect(cancelled.container.querySelector('.tzc-delta')).toHaveClass('good')
  })

  it('never relies on colour alone: an arrow and a sign accompany it', () => {
    const { container } = card({ current: 150, previous: 100 })
    expect(container.querySelector('.tzc-delta [aria-hidden="true"]')?.textContent).toBe('▲')
    expect(container.querySelector('.tzc-delta')?.textContent).toContain('+50')
  })

  it('does not invent a percentage when the period before had nothing', () => {
    card({ current: 5, previous: 0 })
    expect(screen.getByText(/5 more than the previous 30 days \(none then\)/)).toBeInTheDocument()
    expect(screen.queryByText(/%/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Infinity|NaN/)).not.toBeInTheDocument()
  })

  it('says "same" for no change and notes when neither period had any activity', () => {
    const same = card({ current: 7, previous: 7 })
    expect(screen.getByText(/Same as the previous 30 days/)).toBeInTheDocument()
    same.unmount()
    card({ current: 0, previous: 0 })
    expect(screen.getByText('No activity in either period')).toBeInTheDocument()
  })

  it('words a one-day comparison as "the previous day"', () => {
    card({ current: 3, previous: 2, days: 1 })
    expect(screen.getByText(/vs the previous day/)).toBeInTheDocument()
  })

  it('compares nothing when it is not given a previous figure, and keeps a note', () => {
    const { container } = card({ note: 'Returning buyers: 2' })
    expect(container.querySelector('.tzc-delta')).toBeNull()
    expect(screen.getByText('Returning buyers: 2')).toBeInTheDocument()
  })

  it('draws a sparkline only when there is a trend to draw', () => {
    const flat = card({ spark: [0, 0, 0] })
    expect(flat.container.querySelector('svg.tzc-spark')).toBeNull()
    flat.unmount()
    const trend = card({ spark: [1, 3, 2] })
    expect(trend.container.querySelector('svg.tzc-spark')).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('BreakdownBars and Donut', () => {
  const items = [{ key: 'paid', label: 'Paid', value: 3, tone: 2 }, { key: 'cancelled', label: 'Cancelled', value: 1, tone: 3 }, { key: 'none', label: 'None', value: 0 }]

  it('lists parts of a whole largest first, each with its count and share as text', () => {
    render(<BreakdownBars label="Orders by status" items={items} />)
    const rows = within(screen.getByRole('list', { name: 'Orders by status' })).getAllByRole('listitem')
    expect(rows).toHaveLength(2) // a zero is not a bar
    expect(rows[0]).toHaveTextContent('Paid')
    expect(rows[0]).toHaveTextContent('3')
    expect(rows[0]).toHaveTextContent('75 %')
    expect(rows[1]).toHaveTextContent('25 %')
  })

  it('says so when there is nothing to break down', () => {
    render(<BreakdownBars label="Orders by status" items={[{ key: 'a', label: 'A', value: 0 }]} />)
    expect(screen.getByText('Nothing to show for this period.')).toBeInTheDocument()
  })

  it('shows a ring with the total in the middle and a legend that repeats every number', () => {
    const { container } = render(<Donut label="Orders by status" centre="orders" items={items} />)
    expect(container.querySelector('.tzc-donut-total')?.textContent).toBe('4')
    expect(container.querySelector('.tzc-donut-caption')?.textContent).toBe('orders')
    expect(container.querySelectorAll('.tzc-donut-seg')).toHaveLength(2)
    const legend = within(screen.getByRole('list', { name: 'Orders by status' })).getAllByRole('listitem')
    expect(legend.map((li) => li.textContent)).toEqual([expect.stringContaining('Paid'), expect.stringContaining('Cancelled')])
    expect(legend[0]).toHaveTextContent('3')
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true') // the legend is the accessible version
  })

  it('draws the segments around the ring in proportion', () => {
    const { container } = render(<Donut label="x" centre="x" items={[{ key: 'a', label: 'A', value: 1 }, { key: 'b', label: 'B', value: 3 }]} />)
    const lengths = [...container.querySelectorAll('.tzc-donut-seg')].map((c) => parseFloat(c.getAttribute('stroke-dasharray')!.split(' ')[0]!))
    expect(lengths[0]! / lengths[1]!).toBeCloseTo(3, 0) // the larger part (3) is drawn first and is three times the smaller
  })

  it('gives every status a stable meaning: done is teal, waiting amber, lost coral', () => {
    expect(['paid', 'completed', 'confirmed', 'published'].map(statusTone)).toEqual([2, 2, 2, 2])
    expect(['pending', 'pending_payment', 'draft'].map(statusTone)).toEqual([5, 5, 5])
    expect(['cancelled', 'refunded', 'expired'].map(statusTone)).toEqual([3, 3, 3])
  })
})

describe('PeriodTabs', () => {
  function Harness({ onChange }: { onChange: (n: number) => void }) {
    const [value, setValue] = useState(30)
    return <PeriodTabs value={value} onChange={(n) => { setValue(n); onChange(n) }} />
  }

  it('offers 7, 30 and 90 days as toggle buttons and says which is on', async () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    const group = screen.getByRole('group', { name: 'Period' })
    expect(within(group).getAllByRole('button').map((b) => b.textContent)).toEqual(['7 days', '30 days', '90 days'])
    expect(within(group).getByRole('button', { name: '30 days' })).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(within(group).getByRole('button', { name: '7 days' }))
    expect(onChange).toHaveBeenCalledWith(7)
    expect(within(group).getByRole('button', { name: '7 days' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(group).getByRole('button', { name: '30 days' })).toHaveAttribute('aria-pressed', 'false')
  })
})
