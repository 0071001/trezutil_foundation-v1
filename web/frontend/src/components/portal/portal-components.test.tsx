import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ReactNode } from 'react'
import { MemoryRouter } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { Breadcrumbs } from '@/components/portal/Breadcrumbs'
import { ConfirmationDialog } from '@/components/portal/ConfirmationDialog'
import { DataTable, type Column } from '@/components/portal/DataTable'
import { DetailDrawer } from '@/components/portal/DetailDrawer'
import { FilterBar } from '@/components/portal/FilterBar'
import { Metric } from '@/components/portal/Metric'
import { ModuleHead, PlannedState, UnavailableState } from '@/components/portal/ModuleState'
import { Pagination } from '@/components/portal/Pagination'

const wrap = (ui: ReactNode) => render(<MemoryRouter>{ui}</MemoryRouter>)

interface Row { id: string; name: string; qty: number }
const rows: Row[] = [
  { id: 'a', name: 'Cassava', qty: 12 },
  { id: 'b', name: 'Banana', qty: 3 },
  { id: 'c', name: 'Mango', qty: 40 },
  { id: 'd', name: 'Avocado', qty: 7 },
  { id: 'e', name: 'Papaya', qty: 21 },
]
const columns: Column<Row>[] = [
  { id: 'name', header: 'Name', cell: (r) => r.name, sort: (r) => r.name, primary: true },
  { id: 'qty', header: 'Quantity', cell: (r) => String(r.qty), sort: (r) => r.qty, align: 'end' },
  { id: 'note', header: 'Note', cell: () => '—' },
]
const names = () => within(screen.getByRole('table')).getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0]!.textContent)

describe('DataTable', () => {
  it('is a real, named table with column headers', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} />)
    const table = screen.getByRole('table', { name: 'Stock' })
    expect(within(table).getAllByRole('columnheader')).toHaveLength(3)
    expect(names()).toEqual(['Cassava', 'Banana', 'Mango', 'Avocado', 'Papaya'])
  })

  it('sorts a column both ways and announces it with aria-sort', async () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} />)
    const header = (name: string) => screen.getByRole('columnheader', { name: new RegExp(name) })
    expect(header('Name')).toHaveAttribute('aria-sort', 'none')
    expect(header('Note')).not.toHaveAttribute('aria-sort') // an unsortable column says nothing

    await userEvent.click(within(header('Quantity')).getByRole('button'))
    expect(header('Quantity')).toHaveAttribute('aria-sort', 'ascending')
    expect(names()).toEqual(['Banana', 'Avocado', 'Cassava', 'Papaya', 'Mango']) // numeric, not "12" < "3"
    await userEvent.click(within(header('Quantity')).getByRole('button'))
    expect(header('Quantity')).toHaveAttribute('aria-sort', 'descending')
    expect(names()).toEqual(['Mango', 'Papaya', 'Cassava', 'Avocado', 'Banana'])
    expect(header('Name')).toHaveAttribute('aria-sort', 'none')
  })

  it('honours an initial sort', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} initialSort={{ column: 'name', direction: 'asc' }} />)
    expect(names()).toEqual(['Avocado', 'Banana', 'Cassava', 'Mango', 'Papaya'])
    expect(screen.getByRole('columnheader', { name: /Name/ })).toHaveAttribute('aria-sort', 'ascending')
  })

  it('pages rows that are already loaded', async () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} pageSize={2} />)
    expect(names()).toEqual(['Cassava', 'Banana'])
    expect(screen.getByRole('status')).toHaveTextContent('Showing 1–2 of 5 · Page 1 of 3')
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(names()).toEqual(['Mango', 'Avocado'])
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(names()).toEqual(['Papaya'])
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  it('leaves server paging to the caller: it shows the page it is given', async () => {
    const onPage = vi.fn()
    wrap(<DataTable caption="Stock" columns={columns} rows={rows.slice(0, 2)} rowKey={(r) => r.id} serverPaging={{ page: 2, pageSize: 2, total: 5, onPage }} />)
    expect(names()).toEqual(['Cassava', 'Banana'])
    expect(screen.getByRole('status')).toHaveTextContent('Showing 3–4 of 5 · Page 2 of 3')
    await userEvent.click(screen.getByRole('button', { name: 'Previous' }))
    expect(onPage).toHaveBeenCalledWith(1)
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(onPage).toHaveBeenCalledWith(3)
  })

  it('opens a record by click, and by Enter or Space on the focused row', async () => {
    const onOpen = vi.fn()
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} onOpen={onOpen} />)
    expect(screen.getByRole('table')).toHaveAccessibleName(/Press Enter on a row to open its details/)
    const first = within(screen.getByRole('table')).getAllByRole('row')[1]!
    expect(first).toHaveAttribute('tabindex', '0')

    await userEvent.click(first)
    expect(onOpen).toHaveBeenLastCalledWith(rows[0])
    first.focus()
    await userEvent.keyboard('{Enter}')
    expect(onOpen).toHaveBeenCalledTimes(2)
    await userEvent.keyboard(' ')
    expect(onOpen).toHaveBeenCalledTimes(3)
  })

  it('does not make rows focusable when nothing opens', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} />)
    expect(within(screen.getByRole('table')).getAllByRole('row')[1]).not.toHaveAttribute('tabindex')
  })

  it('shows placeholders while loading, and no stale rows', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} loading />)
    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true')
    expect(screen.queryByText('Cassava')).not.toBeInTheDocument()
    expect(screen.queryByText('Nothing to show.')).not.toBeInTheDocument()
  })

  it('says so when it is empty', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={[]} rowKey={(r) => r.id} />)
    expect(screen.getByText('Nothing to show.')).toBeInTheDocument()
    wrap(<DataTable caption="Orders" columns={columns} rows={[]} rowKey={(r) => r.id} empty="No order matches" />)
    expect(screen.getByText('No order matches')).toBeInTheDocument()
  })

  it('replaces the table with the error it is given', () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} error={<p role="alert">The list could not be loaded</p>} />)
    expect(screen.getByRole('alert')).toHaveTextContent('The list could not be loaded')
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('states a limit on the data instead of hiding it', () => {
    wrap(<DataTable caption="Orders" columns={columns} rows={rows} rowKey={(r) => r.id} notice="Showing the latest 100 orders" />)
    expect(screen.getByRole('note')).toHaveTextContent('Showing the latest 100 orders')
  })

  it('switches density', async () => {
    wrap(<DataTable caption="Stock" columns={columns} rows={rows} rowKey={(r) => r.id} densityToggle />)
    const toggle = screen.getByRole('button', { name: 'Compact rows' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    await userEvent.click(toggle)
    expect(screen.getByRole('button', { name: 'Comfortable rows' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('table')).toHaveClass('compact')
  })

  it('labels every cell for the stacked phone layout', () => {
    wrap(<DataTable caption="Stock" columns={[...columns.slice(0, 2), { ...columns[2]!, label: 'Remark' }]} rows={rows.slice(0, 1)} rowKey={(r) => r.id} />)
    const cells = within(screen.getAllByRole('row')[1]!).getAllByRole('cell')
    expect(cells.map((c) => c.getAttribute('data-label'))).toEqual(['Name', 'Quantity', 'Remark'])
  })
})

describe('Pagination', () => {
  it('renders nothing when everything fits on one page', () => {
    const { container } = render(<Pagination page={1} pageSize={25} total={25} onPage={() => {}} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('states the position in words and disables the ends', async () => {
    const onPage = vi.fn()
    const { rerender } = render(<Pagination page={1} pageSize={25} total={60} onPage={onPage} />)
    expect(screen.getByRole('status')).toHaveTextContent('Showing 1–25 of 60 · Page 1 of 3')
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(onPage).toHaveBeenCalledWith(2)

    rerender(<Pagination page={3} pageSize={25} total={60} onPage={onPage} />)
    expect(screen.getByRole('status')).toHaveTextContent('Showing 51–60 of 60 · Page 3 of 3')
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  it('clamps a page that is out of range', () => {
    render(<Pagination page={9} pageSize={10} total={25} onPage={() => {}} />)
    expect(screen.getByRole('status')).toHaveTextContent('Page 3 of 3')
  })
})

describe('Breadcrumbs', () => {
  it('links the ancestors and marks the current page', () => {
    wrap(<Breadcrumbs items={[{ label: 'Seller workspace', to: '/seller' }, { label: 'Orders', to: '/seller/orders' }, { label: 'Details' }]} />)
    const nav = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(within(nav).getByRole('link', { name: 'Seller workspace' })).toHaveAttribute('href', '/seller')
    expect(within(nav).getByRole('link', { name: 'Orders' })).toHaveAttribute('href', '/seller/orders')
    expect(within(nav).queryByRole('link', { name: 'Details' })).not.toBeInTheDocument()
    expect(within(nav).getByText('Details')).toHaveAttribute('aria-current', 'page')
  })

  it('does not link the crumb that is the current page, even when it has an address', () => {
    wrap(<Breadcrumbs items={[{ label: 'Seller workspace', to: '/seller' }, { label: 'Orders', to: '/seller/orders' }]} />)
    expect(screen.queryByRole('link', { name: 'Orders' })).not.toBeInTheDocument()
    expect(screen.getByText('Orders')).toHaveAttribute('aria-current', 'page')
  })

  it('renders nothing for no crumbs', () => {
    const { container } = wrap(<Breadcrumbs items={[]} />)
    expect(container).toBeEmptyDOMElement()
  })
})

describe('FilterBar', () => {
  it('is a labelled search landmark with a named field', async () => {
    const onChange = vi.fn()
    wrap(<FilterBar label="Filter orders" search={{ value: '', onChange, label: 'Search orders' }} />)
    expect(screen.getByRole('search', { name: 'Filter orders' })).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Search orders'), 'ab')
    expect(onChange).toHaveBeenLastCalledWith('b')
  })

  it('offers a reset only when something is filtered', async () => {
    const onReset = vi.fn()
    const { rerender } = wrap(<FilterBar label="Filter" active={false} onReset={onReset} />)
    expect(screen.queryByRole('button', { name: 'Reset filters' })).not.toBeInTheDocument()
    rerender(<MemoryRouter><FilterBar label="Filter" active onReset={onReset} /></MemoryRouter>)
    await userEvent.click(screen.getByRole('button', { name: 'Reset filters' }))
    expect(onReset).toHaveBeenCalledOnce()
  })

  it('does not submit the page', async () => {
    wrap(<FilterBar label="Filter" search={{ value: 'x', onChange: () => {}, label: 'Search' }} />)
    await userEvent.type(screen.getByLabelText('Search'), '{Enter}')
    expect(screen.getByRole('search')).toBeInTheDocument()
  })
})

describe('Metric', () => {
  it('names the source and the time of a figure', () => {
    wrap(<Metric label="Orders this week" value="12" note="4 paid" icon="orders" source="Incoming orders" asOf="4 Oct, 13:00" />)
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('Incoming orders · 4 Oct, 13:00')).toBeInTheDocument()
  })

  it('flags a figure computed from a truncated list as partial', () => {
    wrap(<Metric label="Orders" value="100+" note="" icon="orders" partial="Based on the latest 100 orders" source="Incoming orders" />)
    expect(screen.getByText('Based on the latest 100 orders')).toHaveClass('metric-partial')
  })

  it('shows no provenance line when none is given', () => {
    const { container } = wrap(<Metric label="Orders" value="12" note="" icon="orders" />)
    expect(container.querySelector('.metric-source')).toBeNull()
  })
})

describe('UnavailableState and PlannedState', () => {
  it('says what is missing, cites the gap, and shows no figure', () => {
    wrap(<UnavailableState gapIds={['G-B4']} />)
    const state = screen.getByRole('region', { name: 'Not available yet' })
    expect(state).toHaveAttribute('data-state', 'unavailable')
    expect(within(state).getByText(/Payments and settlements/)).toBeInTheDocument()
    expect(within(state).getByText('G-B4')).toBeInTheDocument()
    expect(within(state).getByText(/no list, ledger or payout view/i)).toBeInTheDocument()
    expect(within(state).queryByRole('table')).not.toBeInTheDocument()
    expect(within(state).queryByRole('button')).not.toBeInTheDocument() // no control that does nothing
  })

  it('offers a real alternative when there is one', () => {
    wrap(<UnavailableState gapIds={['G-C4']} link={{ to: '/seller/listings', label: 'Put a listing on sale from Listings' }} note="Audit events are written to the service logs." />)
    expect(screen.getByRole('link', { name: /Put a listing on sale from Listings/ })).toHaveAttribute('href', '/seller/listings')
    expect(screen.getByText('Audit events are written to the service logs.')).toBeInTheDocument()
  })

  it('lists the endpoints a planned screen will use and its known limits', () => {
    wrap(<PlannedState endpoints={['GET /orders/incoming', 'GET /orders/{id}']} limits={['G-B2']} />)
    const state = screen.getByRole('region', { name: 'Coming soon' })
    expect(state).toHaveAttribute('data-state', 'planned')
    expect(within(state).getByText('GET /orders/incoming')).toBeInTheDocument()
    expect(within(state).getByText('GET /orders/{id}')).toBeInTheDocument()
    expect(within(state).getByText('Known limits')).toBeInTheDocument()
    expect(within(state).getByText('G-B2')).toBeInTheDocument()
  })

  it('omits the limits block when there are none', () => {
    wrap(<PlannedState endpoints={['GET /verticals']} />)
    expect(screen.queryByText('Known limits')).not.toBeInTheDocument()
  })

  it('heads a module with one level-1 heading', () => {
    wrap(<ModuleHead eyebrow="Operate" title="Fulfilment" copy="Prepare and hand over." />)
    expect(screen.getByRole('heading', { level: 1, name: 'Fulfilment' })).toBeInTheDocument()
  })
})

function DrawerHarness() {
  const [open, setOpen] = useState(false)
  return (
    <div>
      <button onClick={() => setOpen(true)}>Open order</button>
      <DetailDrawer open={open} onClose={() => setOpen(false)} title="Order 1042" subtitle="Marie" actions={<button>Confirm</button>}>
        <a href="/orders/1042">Full page</a>
      </DetailDrawer>
    </div>
  )
}

describe('DetailDrawer', () => {
  it('is a named modal dialog, and absent until opened', async () => {
    render(<DrawerHarness />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Open order' }))
    const dialog = screen.getByRole('dialog', { name: 'Order 1042' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(within(dialog).getByText('Marie')).toBeInTheDocument()
  })

  it('moves focus in, keeps Tab inside, and returns focus on Escape', async () => {
    render(<DrawerHarness />)
    const opener = screen.getByRole('button', { name: 'Open order' })
    await userEvent.click(opener)
    const dialog = screen.getByRole('dialog')
    const close = within(dialog).getByRole('button', { name: 'Close' })
    await waitFor(() => expect(dialog).toContainElement(document.activeElement as HTMLElement))
    expect(close).toHaveFocus()

    await userEvent.tab()
    expect(within(dialog).getByRole('link', { name: 'Full page' })).toHaveFocus()
    await userEvent.tab()
    expect(within(dialog).getByRole('button', { name: 'Confirm' })).toHaveFocus()
    await userEvent.tab() // wraps to the first control instead of leaving the dialog
    expect(close).toHaveFocus()
    await userEvent.tab({ shift: true }) // and backwards
    expect(within(dialog).getByRole('button', { name: 'Confirm' })).toHaveFocus()

    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(opener).toHaveFocus()
  })

  it('closes from the close button and from the scrim, and releases the page scroll', async () => {
    render(<DrawerHarness />)
    await userEvent.click(screen.getByRole('button', { name: 'Open order' }))
    expect(document.body.style.overflow).toBe('hidden')
    await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(document.body.style.overflow).not.toBe('hidden')

    await userEvent.click(screen.getByRole('button', { name: 'Open order' }))
    const scrim = screen.getByRole('dialog').parentElement!
    await userEvent.pointer({ keys: '[MouseLeft]', target: scrim })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})

function DialogHarness({ onConfirm, tone }: { onConfirm: () => Promise<unknown> | void; tone?: 'danger' }) {
  const [open, setOpen] = useState(false)
  const [done, setDone] = useState(false)
  return (
    <div>
      <button onClick={() => { setDone(false); setOpen(true) }}>Refund</button>
      {done && <p>Closed after success</p>}
      <ConfirmationDialog open={open} tone={tone} title="Refund $40 to Marie?" confirmLabel="Refund $40" onConfirm={onConfirm} onClose={() => { setOpen(false); setDone(true) }}>
        The customer is notified.
      </ConfirmationDialog>
    </div>
  )
}

describe('ConfirmationDialog', () => {
  it('names the action and what it does, and runs it once', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined)
    render(<DialogHarness onConfirm={onConfirm} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    const dialog = screen.getByRole('dialog', { name: 'Refund $40 to Marie?' })
    expect(dialog).toHaveAccessibleDescription('The customer is notified.')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Refund $40' }))
    expect(onConfirm).toHaveBeenCalledOnce()
    expect(await screen.findByText('Closed after success')).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('cannot be double-submitted: both buttons are disabled while it works', async () => {
    let finish: () => void = () => {}
    const onConfirm = vi.fn(() => new Promise<void>((resolve) => { finish = resolve }))
    render(<DialogHarness onConfirm={onConfirm} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    await userEvent.click(screen.getByRole('button', { name: 'Refund $40' }))
    const working = screen.getByRole('button', { name: 'Working…' })
    expect(working).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-busy', 'true')
    await userEvent.click(working)
    await userEvent.keyboard('{Escape}') // cannot be dismissed mid-flight either
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(onConfirm).toHaveBeenCalledOnce()
    finish()
    expect(await screen.findByText('Closed after success')).toBeInTheDocument()
  })

  it('keeps the dialog open and shows the refusal in place when the server says no', async () => {
    const onConfirm = vi.fn().mockRejectedValue(new ApiError('This payment was already refunded', 409, 'PAYMENT_NOT_REFUNDABLE'))
    render(<DialogHarness onConfirm={onConfirm} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    await userEvent.click(screen.getByRole('button', { name: 'Refund $40' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('This payment was already refunded')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.queryByText('Closed after success')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refund $40' })).toBeEnabled() // can try again
  })

  it('treats a destructive action as an alert dialog with focus on Cancel', async () => {
    render(<DialogHarness tone="danger" onConfirm={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus())
  })

  it('puts focus on the first control for an ordinary confirmation, and Cancel closes without running it', async () => {
    const onConfirm = vi.fn()
    render(<DialogHarness onConfirm={onConfirm} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus())
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onConfirm).not.toHaveBeenCalled()
    expect(screen.getByText('Closed after success')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refund' })).toHaveFocus() // focus returns to what opened it
  })

  it('closes on Escape', async () => {
    render(<DialogHarness onConfirm={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: 'Refund' }))
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
