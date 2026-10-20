import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { createRoot, createSignal, Show } from 'solid-js'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { catalogEntries, catalogTarget, memoryCatalog, memoryDestinations } from '../stories/reportCatalog'
import { appendReportPicks, createReportPickerSelection, parseCatalogPage, type ReportCatalogHost, type ReportPickerConfirmation } from './picker'
import { ReportDestinationPicker } from './ReportDestinationPicker'
import { ReportPicker } from './ReportPicker'

beforeAll(() => {
  window.scrollTo = vi.fn() as typeof window.scrollTo
  // jsdom lays nothing out; the focus trap skips elements without an offsetParent.
  Object.defineProperty(HTMLElement.prototype, 'offsetParent', {
    configurable: true,
    get(this: HTMLElement) { return this.closest('[hidden]') ? null : this.parentElement },
  })
})

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const entries = catalogEntries()
const entry = (id: string) => entries.find((candidate) => candidate.id === id)!
const target = { id: 'sales-board', title: 'Sales board' }
const row = (id: string) => document.querySelector<HTMLElement>(`[data-report-id="${id}"]`)!
const checkbox = (id: string) => row(id).querySelector<HTMLInputElement>('input[type="checkbox"]')!
const dialog = () => screen.getByRole('dialog', { name: 'Add reports to Sales board' })

function inRoot<T>(build: () => T): T {
  return createRoot(() => build())
}

// False green: checking only the confirmation payload would pass while a retry minted a new id and doubled the cards.
describe('report picker selection', () => {
  it('keeps click order, refuses a plain re-pick of a placed report and keeps one id per choice until settled', () => {
    const selection = inRoot(() => createReportPickerSelection((value) => value.placed ?? 0))
    selection.toggle(entry('report-03'))
    selection.toggle(entry('report-02'))
    selection.toggle(entry('report-01'))
    expect(selection.picks().map((pick) => pick.entry.id)).toEqual(['report-03', 'report-02'])
    selection.addAnother(entry('report-01'))
    selection.addAnother(entry('report-04'))
    expect(selection.picks().map((pick) => [pick.entry.id, pick.duplicate])).toEqual([['report-03', false], ['report-02', false], ['report-01', true]])
    const first = selection.confirmation('board')
    expect(selection.confirmation('board').id).toBe(first.id)
    selection.toggle(entry('report-02'))
    const changed = selection.confirmation('board')
    expect(changed.id).not.toBe(first.id)
    selection.settle()
    expect(selection.confirmation('board').id).not.toBe(changed.id)
  })

  it('appends picks to the end of the draft once, inheriting filters, and keeps a deliberate second card', () => {
    const composition = catalogTarget()
    const confirmation: ReportPickerConfirmation = { id: 'pick-1', target: composition.id, picks: [
      { entry: entry('report-02'), duplicate: false },
      { entry: entry('report-01'), duplicate: false },
      { entry: entry('report-01'), duplicate: true },
    ] }
    const first = appendReportPicks(composition, confirmation)
    expect(first.added).toEqual([{ id: 'pick-1-1', reportId: 'report-02' }, { id: 'pick-1-3', reportId: 'report-01' }])
    expect(first.composition.rows.flat().map((placement) => placement.id)).toEqual(['placed-revenue', 'pick-1-1', 'pick-1-3'])
    expect(composition.rows).toHaveLength(1)
    const retried = appendReportPicks(first.composition, confirmation)
    expect(retried.added).toEqual([])
    expect(retried.composition).toEqual(first.composition)
    expect(appendReportPicks(composition, confirmation, { span: 6 }).added[0]!.span).toBe(6)
  })

  it('rejects a catalog page that repeats a report', () => {
    expect(() => parseCatalogPage({ entries: [entries[0], entries[0]] })).toThrow()
    expect(() => parseCatalogPage({ entries: [{ ...entries[0], sql: 'select 1' }] })).toThrow()
  })
})

function mountPicker(options: {
  host?: ReportCatalogHost
  confirm?: (confirmation: ReportPickerConfirmation) => Promise<void> | void
  placed?: (reportId: string) => number
} = {}) {
  const onConfirm = vi.fn(options.confirm ?? (() => undefined))
  const onClose = vi.fn()
  const host = options.host ?? memoryCatalog()
  const [open, setOpen] = createSignal(false)
  let trigger!: HTMLButtonElement
  render(() => (
    <div class="lens-root" data-theme="light">
      <button onClick={() => setOpen(true)} ref={trigger} type="button">Add reports</button>
      <Show when={open()}>
        <ReportPicker
          host={host}
          onClose={() => { onClose(); setOpen(false) }}
          onConfirm={onConfirm}
          placed={options.placed}
          restoreFocus={trigger}
          target={target}
        />
      </Show>
    </div>
  ))
  trigger.focus()
  fireEvent.click(trigger)
  return { onConfirm, onClose, trigger, host, open }
}

const listed = () => [...document.querySelectorAll<HTMLElement>('[data-report-id]')].map((element) => element.dataset.reportId)

describe('ReportPicker', () => {
  // False green: a picker that wrote to the draft on each click would also show the confirmed result.
  it('changes nothing until Add N, then hands over the ordered choice and returns focus', async () => {
    const { onConfirm, onClose, trigger } = mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('searchbox', { name: 'Search reports' })))
    expect(within(dialog()).getByRole('heading', { name: 'Add reports to Sales board' })).toBeTruthy()
    const confirmButton = screen.getByTestId('report-picker-confirm')
    expect(confirmButton).toBeDisabled()
    fireEvent.click(checkbox('report-03'))
    fireEvent.click(checkbox('report-02'))
    expect(onConfirm).not.toHaveBeenCalled()
    expect(confirmButton).toHaveTextContent('Add 2')
    expect(dialog()).toHaveTextContent('Large losses: Shows the last 12 months only; export is unavailable.')
    fireEvent.click(confirmButton)
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    const confirmation = onConfirm.mock.calls[0]![0]
    expect(confirmation.target).toBe('sales-board')
    expect(confirmation.picks.map((pick) => pick.entry.id)).toEqual(['report-03', 'report-02'])
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })

  // False green: a disabled checkbox alone would hide that a second explicit card is impossible.
  it('marks an already placed report and offers a deliberate second card instead of a duplicate click', async () => {
    const { onConfirm } = mountPicker({ placed: (id) => (id === 'report-04' ? 1 : 0) })
    await waitFor(() => expect(listed()).toHaveLength(20))
    // The draft's own count wins over the catalog's, which describes the saved composition.
    expect(row('report-01')).not.toHaveAttribute('data-placed')
    expect(checkbox('report-01')).toBeEnabled()
    expect(row('report-04')).toHaveTextContent('Already on this dashboard')
    expect(checkbox('report-04')).toBeDisabled()
    fireEvent.click(checkbox('report-04'))
    expect(screen.getByTestId('report-picker-confirm')).toBeDisabled()
    const another = within(row('report-04')).getByRole('button', { name: 'Add another card' })
    fireEvent.click(another)
    expect(another).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByTestId('report-picker-confirm'))
    await waitFor(() => expect(onConfirm).toHaveBeenCalledOnce())
    expect(onConfirm.mock.calls[0]![0].picks.map((pick) => [pick.entry.id, pick.duplicate])).toEqual([['report-04', true]])
  })

  // False green: a single "no results" message would read the same for an empty library, a filter and a failure.
  it('distinguishes an empty catalog, no matches and a failed load, and retries the same search', async () => {
    mountPicker({ host: memoryCatalog({ entries: [] }) })
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('There are no reports to add yet.'))
    cleanup()
    document.body.replaceChildren()

    const host = memoryCatalog()
    const search = vi.spyOn(host, 'search')
    mountPicker({ host })
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.input(screen.getByRole('searchbox', { name: 'Search reports' }), { target: { value: 'zzz' } })
    await waitFor(() => expect(document.querySelector('[data-state="no-matches"]')).toHaveTextContent('No reports match this search.'))
    expect(search.mock.calls.at(-1)![0]).toMatchObject({ search: 'zzz', scope: 'all', target: 'sales-board' })
    cleanup()
    document.body.replaceChildren()

    const failing = memoryCatalog({ failures: 1 })
    mountPicker({ host: failing })
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('The reports could not be loaded.'))
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(listed()).toHaveLength(20))
    expect(failing.searches).toBe(2)
  })

  it('narrows by owner with keyboard tabs and pages with Show more', async () => {
    mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.click(screen.getByRole('button', { name: 'Show more reports' }))
    await waitFor(() => expect(listed()).toHaveLength(40))
    const all = screen.getByRole('tab', { name: 'All' })
    all.focus()
    fireEvent.keyDown(all, { key: 'ArrowRight' })
    const mine = screen.getByRole('tab', { name: 'Mine' })
    expect(mine).toHaveAttribute('aria-selected', 'true')
    expect(document.activeElement).toBe(mine)
    await waitFor(() => expect(listed()).toEqual(entries.filter((value) => value.ownership === 'personal').slice(0, 20).map((value) => value.id)))
    const shared = screen.getByRole('tab', { name: 'Shared with me' })
    fireEvent.click(shared)
    // Built-in reports were shared by nobody: they appear only under All.
    await waitFor(() => expect(listed()).toEqual(entries.filter((value) => value.ownership === 'shared').slice(0, 20).map((value) => value.id)))
    expect(document.querySelector('.lens-report-picker-list')).not.toHaveTextContent('Built-in')
    fireEvent.click(screen.getByRole('tab', { name: 'All' }))
    await waitFor(() => expect(row('report-01')).toHaveTextContent('Built-in'))
  })

  // False green: asserting only that onConfirm ran twice would miss a lost selection or a fresh id that duplicates cards.
  it('keeps search and selection when adding fails and retries with the same confirmation', async () => {
    let attempts = 0
    const { onConfirm, onClose } = mountPicker({ confirm: () => (attempts++ === 0 ? Promise.reject(new Error('rejected')) : Promise.resolve()) })
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.input(screen.getByRole('searchbox', { name: 'Search reports' }), { target: { value: 'report' } })
    await waitFor(() => expect(listed()).not.toContain('report-01'))
    fireEvent.click(checkbox('report-05'))
    fireEvent.click(screen.getByTestId('report-picker-confirm'))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Your selection is kept'))
    expect(onClose).not.toHaveBeenCalled()
    expect(screen.getByRole('searchbox', { name: 'Search reports' })).toHaveValue('report')
    expect(checkbox('report-05')).toBeChecked()
    fireEvent.click(screen.getByTestId('report-picker-confirm'))
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(onConfirm.mock.calls[1]![0].id).toBe(onConfirm.mock.calls[0]![0].id)
  })

  // False green: a confirm button that stays enabled would let a double click add the choice twice.
  it('ignores a second confirmation while the first is in flight', async () => {
    let resolve!: () => void
    const { onConfirm } = mountPicker({ confirm: () => new Promise<void>((done) => { resolve = done }) })
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.click(checkbox('report-02'))
    const button = screen.getByTestId('report-picker-confirm')
    fireEvent.click(button)
    fireEvent.click(button)
    expect(onConfirm).toHaveBeenCalledOnce()
    expect(button).toHaveAttribute('aria-busy', 'true')
    resolve()
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  // False green: closing on Escape without checking the callbacks would also pass if Escape confirmed the choice.
  it('discards the picker choice on Cancel and Escape and returns focus each time', async () => {
    const { onConfirm, onClose, trigger } = mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.click(checkbox('report-02'))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(document.activeElement).toBe(trigger)
    fireEvent.click(trigger)
    await waitFor(() => expect(listed()).toHaveLength(20))
    expect(checkbox('report-02')).not.toBeChecked()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(onClose).toHaveBeenCalledTimes(2)
    expect(onConfirm).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(trigger)
  })

  it('traps Tab inside the dialog', async () => {
    mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    const close = within(dialog()).getByRole('button', { name: 'Close' })
    const add = screen.getByTestId('report-picker-confirm')
    fireEvent.click(checkbox('report-02'))
    add.focus()
    fireEvent.keyDown(add, { key: 'Tab' })
    expect(document.activeElement).toBe(close)
    fireEvent.keyDown(close, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(add)
  })

  // False green: a preview that replaced the list would lose the search and selection it returns to.
  it('previews a report and returns to the same search, selection and row', async () => {
    mountPicker({ host: memoryCatalog({ previewFailures: 1 }) })
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.input(screen.getByRole('searchbox', { name: 'Search reports' }), { target: { value: 'Large' } })
    await waitFor(() => expect(listed()).toEqual(['report-03']))
    fireEvent.click(checkbox('report-03'))
    const previewButton = screen.getByRole('button', { name: 'Preview Large losses' })
    fireEvent.click(previewButton)
    const preview = await screen.findByRole('region', { name: 'Preview Large losses' })
    await waitFor(() => expect(within(preview).getByRole('alert')).toHaveTextContent('The preview could not be loaded.'))
    fireEvent.click(within(preview).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(preview.querySelector('table')).not.toBeNull())
    expect(within(preview).getByRole('button', { name: 'Select report' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(within(preview).getByRole('button', { name: 'Back to reports' }))
    await waitFor(() => expect(document.activeElement).toBe(previewButton))
    expect(screen.getByRole('searchbox', { name: 'Search reports' })).toHaveValue('Large')
    expect(checkbox('report-03')).toBeChecked()
  })

  // False green: an Escape that closed the dialog would also leave the preview, but discard the selection.
  it('steps back from the preview on Escape and closes only on an Escape from the list', async () => {
    const { onClose, onConfirm, trigger } = mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    fireEvent.click(checkbox('report-02'))
    const previewButton = screen.getByRole('button', { name: 'Preview Large losses' })
    fireEvent.click(previewButton)
    const preview = await screen.findByRole('region', { name: 'Preview Large losses' })
    await waitFor(() => expect(document.activeElement).toBe(within(preview).getByRole('button', { name: 'Back to reports' })))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Preview Large losses' })).toBeNull())
    expect(dialog()).toBeTruthy()
    expect(onClose).not.toHaveBeenCalled()
    expect(checkbox('report-02')).toBeChecked()
    await waitFor(() => expect(document.activeElement).toBe(previewButton))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(onClose).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(trigger)
  })

  it('offers no preview without a host preview', async () => {
    const host = memoryCatalog()
    mountPicker({ host: { search: host.search.bind(host) } })
    await waitFor(() => expect(listed()).toHaveLength(20))
    expect(screen.queryByRole('button', { name: /^Preview/ })).toBeNull()
  })

  it('shows a compound report as one choice', async () => {
    mountPicker()
    await waitFor(() => expect(listed()).toHaveLength(20))
    expect(row('report-02')).toHaveTextContent('One block · 3 panels')
    expect(row('report-02').querySelectorAll('input[type="checkbox"]')).toHaveLength(1)
  })
})

describe('ReportDestinationPicker', () => {
  // False green: checking the destination id alone would miss a lost slice or a retry that changed identity.
  it('hands the chosen destination and the shown slice to the host, keeping the choice on failure', async () => {
    let attempts = 0
    const onConfirm = vi.fn(() => (attempts++ === 0 ? Promise.reject(new Error('rejected')) : Promise.resolve()))
    const onClose = vi.fn()
    const context = { period: { selection: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } } } as never
    const host = memoryDestinations({ failures: 1 })
    render(() => (
      <ReportDestinationPicker context={context} host={host} onClose={onClose} onConfirm={onConfirm} report={{ id: 'report-01', title: 'Revenue' }} />
    ))
    expect(screen.getByRole('dialog', { name: 'Add “Revenue” to a dashboard' })).toBeTruthy()
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('The dashboards could not be loaded.'))
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    const claims = await screen.findByRole('radio', { name: 'Claims board' })
    expect(screen.getByText('Already has this report')).toBeTruthy()
    fireEvent.click(claims)
    fireEvent.click(screen.getByTestId('report-destination-confirm'))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Your choice is kept'))
    expect(claims).toBeChecked()
    fireEvent.click(screen.getByTestId('report-destination-confirm'))
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    const calls = onConfirm.mock.calls as unknown as Array<[{ id: string; choice: unknown; context: unknown }]>
    expect(calls[1]![0].id).toBe(calls[0]![0].id)
    expect(calls[1]![0].choice).toEqual({ kind: 'existing', destination: { id: 'claims-board', title: 'Claims board' } })
    expect(calls[1]![0].context).toEqual(context)
  })

  it('offers a new dashboard', async () => {
    const onConfirm = vi.fn()
    render(() => <ReportDestinationPicker host={memoryDestinations()} onClose={() => undefined} onConfirm={onConfirm} report={{ id: 'report-01', title: 'Revenue' }} />)
    fireEvent.click(screen.getByRole('radio', { name: 'New dashboard' }))
    fireEvent.click(screen.getByTestId('report-destination-confirm'))
    await waitFor(() => expect(onConfirm).toHaveBeenCalledOnce())
    expect((onConfirm.mock.calls[0] as unknown as [{ choice: unknown }])[0].choice).toEqual({ kind: 'new' })
  })
})

