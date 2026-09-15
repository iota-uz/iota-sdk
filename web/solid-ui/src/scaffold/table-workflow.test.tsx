import { fireEvent, render, screen } from '@solidjs/testing-library'
import { describe, expect, it, vi } from 'vitest'
import { ContentHTMX, DateTime, DefaultPanelSkeleton, FillerRowsWrapper, SearchClearButton, Table, type ScaffoldTableLoadRequest, type ScaffoldTableResult } from './TableWorkflow'

const columns = [{ key: 'name', label: 'Name', sortable: true }, { key: 'owner', label: 'Owner', priority: 3 }]

describe('scaffold table workflow', () => {
  it('renders responsive rows and drives sort, clear and infinite callbacks', async () => {
    const sort = vi.fn()
    const more = vi.fn()
    const clear = vi.fn()
    render(() => <><SearchClearButton onClear={clear} /><Table columns={columns} query={{ search: '', page: 1, limit: 25 }} result={{ rows: [{ key: 'p1', cells: { name: 'CASCO', owner: 'Alice' } }], hasMore: true }} onSort={sort} onLoadMore={more} /><FillerRowsWrapper stickyHeader noWrap><span>Grid</span></FillerRowsWrapper></>)
    await fireEvent.click(screen.getByRole('button', { name: 'Clear search' }))
    expect(clear).toHaveBeenCalledOnce()
    await fireEvent.click(screen.getByRole('columnheader', { name: 'Name' }))
    expect(sort).toHaveBeenCalledWith('name', 'asc')
    expect(screen.getByRole('cell', { name: 'Alice' })).toHaveClass('max-lg:hidden')
    await fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(more).toHaveBeenCalledOnce()
    expect(screen.getByText('Grid').parentElement).toHaveClass('table-filler-container', 'table-nowrap')
  })

  it('aborts stale adapter requests and appends only the current result', async () => {
    const pending: { request: ScaffoldTableLoadRequest; resolve: (result: ScaffoldTableResult) => void }[] = []
    const adapter = { load: (request: ScaffoldTableLoadRequest) => new Promise<ScaffoldTableResult>((resolve) => pending.push({ request, resolve })) }
    render(() => <ContentHTMX adapter={adapter} autoLoad={false} columns={columns} searchClearable />)
    const search = screen.getByPlaceholderText('Search')
    await fireEvent.input(search, { target: { value: 'first' } })
    await fireEvent.input(search, { target: { value: 'second' } })
    expect(pending[0]?.request.signal.aborted).toBe(true)
    pending[1]?.resolve({ rows: [{ key: 'new', cells: { name: 'Second result', owner: 'Bob' } }] })
    expect(await screen.findByText('Second result')).toBeVisible()
    pending[0]?.resolve({ rows: [{ key: 'old', cells: { name: 'Stale result', owner: 'Eve' } }] })
    await Promise.resolve()
    expect(screen.queryByText('Stale result')).not.toBeInTheDocument()
  })

  it('uses semantic time and canonical deferred skeleton surfaces', () => {
    render(() => <><DateTime value="2026-09-14T10:00:00Z" locale="en" /><DefaultPanelSkeleton /></>)
    expect(document.querySelector('time')).toHaveAttribute('datetime', '2026-09-14T10:00:00.000Z')
    expect(document.querySelector('[aria-busy="true"]')).toHaveClass('flex', 'flex-wrap', 'gap-3')
  })
})
