import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { mapAnalytics, type RankedEntry } from '@/api/analytics'
import { ActivityBoard } from '@/components/dashboard/ActivityBoard'
import { analyticsCsv, analyticsFilename } from '@/components/dashboard/csv'
import { DashSkeleton, DashToolbar } from '@/components/dashboard/DashToolbar'
import { RankedList } from '@/components/dashboard/shared'
import { analyticsBody, quietAnalytics } from '@/test/fixtures'

/*
 * The pieces both dashboards are built from: the board of tiles and the chart
 * under it, the toolbar, the loading shape, the ranking, and the CSV export.
 */

const platform = mapAnalytics({ ...analyticsBody(true), scope: 'platform' })
const business = mapAnalytics({ ...analyticsBody(false), scope: 'organization' })

describe('the board', () => {
  const metrics = [
    { id: 'a', tile: { label: 'Orders', value: '5', icon: 'orders' as const }, panel: <p>chart of orders</p> },
    { id: 'b', tile: { label: 'Buyers', value: '3', icon: 'users' as const } },
    { id: 'c', tile: { label: 'Bookings', value: '2', icon: 'calendar' as const }, panel: <p>chart of bookings</p> },
  ]

  it('opens the chart of the first figure that has one, and says which tile is selected', () => {
    render(<ActivityBoard label="Key figures" metrics={metrics} />)
    expect(screen.getByText('chart of orders')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Orders/ })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: /Bookings/ })).toHaveAttribute('aria-pressed', 'false')
  })

  it('swaps the chart when another tile is picked, and ties each tile to the panel it controls', async () => {
    const { container } = render(<ActivityBoard label="Key figures" metrics={metrics} />)
    await userEvent.click(screen.getByRole('button', { name: /Bookings/ }))
    expect(screen.getByText('chart of bookings')).toBeInTheDocument()
    expect(screen.queryByText('chart of orders')).toBeNull()
    const panel = container.querySelector('.tz-trend') as HTMLElement
    expect(screen.getByRole('button', { name: /Bookings/ })).toHaveAttribute('aria-controls', panel.id)
  })

  it('keeps a figure with no chart as a plain figure', () => {
    render(<ActivityBoard label="Key figures" metrics={metrics} />)
    expect(screen.getAllByRole('button')).toHaveLength(2)
    expect(within(screen.getByRole('group', { name: 'Key figures' })).getByText('Buyers')).toBeInTheDocument()
  })

  it('shows no panel when no figure has a chart', () => {
    const { container } = render(<ActivityBoard label="Key figures" metrics={[metrics[1]!]} />)
    expect(container.querySelector('.tz-trend')).toBeNull()
  })
})

describe('the toolbar', () => {
  it('offers refresh and export only when it can do them, and says when the figures were counted', async () => {
    const onRefresh = vi.fn()
    const first = render(<DashToolbar asOf="Counted today" period={30} onPeriod={() => {}} />)
    expect(screen.getByText('Counted today')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Export' })).toBeNull()
    first.unmount()

    render(<DashToolbar period={30} onPeriod={() => {}} onRefresh={onRefresh} exportData={platform} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(onRefresh).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: 'Export' })).toHaveAttribute('title', 'Download the day-by-day figures as a CSV file')
  })

  it('has a loading shape that announces itself and hides the placeholders from assistive technology', () => {
    const { container } = render(<DashSkeleton label="Loading the activity…" />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading the activity…')
    expect(container.querySelectorAll('.tz-skel-tile')).toHaveLength(8)
    expect(container.querySelector('.tz-board')).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('the ranking', () => {
  const entries: RankedEntry[] = [
    { id: 'a', name: 'Gombe Stays', orders: 40, units: 0, revenue: { USD: 350000, CDF: 5000000 } },
    { id: 'b', name: 'Kasa Market', orders: 10, units: 0, revenue: { USD: 123400 } },
    { id: 'c', name: 'Quiet Shop', orders: 1, units: 0, revenue: {} },
  ]
  const show = () => render(<RankedList label="Top" entries={entries} empty="Nobody yet." meta={(e) => `${e.orders} orders`} weight={(e) => e.orders} />)

  it('puts the leader first with a full bar and the others in proportion, never below a visible sliver', () => {
    const { container } = show()
    const widths = [...container.querySelectorAll('.share i')].map((i) => (i as HTMLElement).style.width)
    expect(widths).toEqual(['100%', '25%', '3%'])
    expect(container.querySelector('.share')).toHaveAttribute('aria-hidden', 'true') // decoration: the numbers are in the text
  })

  it('names each entry, what it is ranked on, and its money, each currency on its own', () => {
    show()
    const rows = within(screen.getByRole('list', { name: 'Top' })).getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('Gombe Stays')
    expect(rows[0]).toHaveTextContent('40 orders')
    expect(rows[0]).toHaveTextContent(/\$3,500 \+ .*50,000/)
    expect(rows[2]).toHaveTextContent('—') // nothing earned: a dash, not a zero in some currency
  })

  it('says so when there is nothing to rank', () => {
    render(<RankedList label="Top" entries={[]} empty="Nobody yet." meta={() => ''} weight={() => 0} />)
    expect(screen.getByText('Nobody yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).toBeNull()
  })
})

describe('the CSV export', () => {
  const lines = (text: string) => text.replace(/^﻿/, '').trim().split('\r\n')

  it('has a header, one row per day and the server’s own totals, with each currency in a column of its own', () => {
    const rows = lines(analyticsCsv(platform))
    expect(rows[0]).toBe('Day,Orders,Bookings,New listings,New accounts,Conversations,Revenue (USD),Revenue (CDF)')
    expect(rows).toHaveLength(1 + 3 + 1)
    expect(rows[1]).toBe('2026-10-02,0,0,0,0,3,0.00,0.00')
    expect(rows[3]).toBe('2026-10-04,4,2,0,2,4,4734.00,50000.00')
    expect(rows[4]).toBe('Total,5,3,1,2,9,4734.00,50000.00')
  })

  it('leaves out the platform-only columns for a business', () => {
    const rows = lines(analyticsCsv(business))
    expect(rows[0]).toBe('Day,Orders,Bookings,New listings,Revenue (USD),Revenue (CDF)')
    expect(rows[3]).toBe('2026-10-04,4,2,0,4734.00,50000.00')
  })

  it('writes a quiet period as real zeros, in USD when no currency earned anything', () => {
    const rows = lines(analyticsCsv(mapAnalytics({ ...quietAnalytics(true), scope: 'platform' })))
    expect(rows[0]).toBe('Day,Orders,Bookings,New listings,New accounts,Conversations,Revenue (USD)')
    expect(rows.slice(1).every((r) => /,0\.00$/.test(r))).toBe(true)
  })

  it('starts with a byte-order mark so a spreadsheet reads accents, and ends each line with CRLF', () => {
    const text = analyticsCsv(platform)
    expect(text.startsWith('﻿')).toBe(true)
    expect(text.endsWith('\r\n')).toBe(true)
  })

  it('is named after what it holds: the platform, or the business, and the days it covers', () => {
    expect(analyticsFilename(platform)).toBe('trezutil-platform-2026-10-02-to-2026-10-04.csv')
    expect(analyticsFilename(business, 'Café Mokili & Fils')).toBe('trezutil-cafe-mokili-fils-2026-10-02-to-2026-10-04.csv')
    expect(analyticsFilename(business, '')).toBe('trezutil-business-2026-10-02-to-2026-10-04.csv')
  })
})
