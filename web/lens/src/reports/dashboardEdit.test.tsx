import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import scopedFixture from '../../fixtures/scoped-filters.json'
import { parseDocument, type DashboardDocument, type Filter } from '../contract'
import { LensDashboard, type LensDashboardProps } from '../LensDashboard'
import { deferredEditDocument, editComposition, editDocument, editReportTitles, emptyComposition, emptyDocument } from '../stories/dashboardEdit'
import { memoryCatalog } from '../stories/reportCatalog'
import type { Composition } from './contract/types'
import type { LensDashboardEditing } from './dashboardEditTypes'
import { createReportDraft, ReportRevisionConflictError, type ReportDraftHost } from './draft'
import type { ReportPlacementHost } from './placementHost'

beforeAll(() => {
  window.scrollTo = vi.fn() as typeof window.scrollTo
  Element.prototype.scrollIntoView = vi.fn()
  Object.defineProperty(HTMLElement.prototype, 'offsetParent', {
    configurable: true,
    get(this: HTMLElement) { return this.closest('[hidden]') ? null : this.parentElement },
  })
})

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const compareFilter: Filter = {
  id: 'compare', kind: 'compare', label: 'Compare with',
  compare: { modeParam: 'compare', startParam: 'compare_start', endParam: 'compare_end', compareTo: 'period', value: { mode: 'year_ago' } },
}

const nextFrame = () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
const never: typeof fetch = () => new Promise<Response>(() => undefined)

interface Mounted {
  edit: LensDashboardEditing
  save: ReturnType<typeof vi.fn>
  onSaved: ReturnType<typeof vi.fn>
  onCancel: ReturnType<typeof vi.fn>
  editing: () => boolean
}

function mount(options: {
  document?: DashboardDocument
  composition?: Composition
  fetcher?: typeof fetch
  save?: ReportDraftHost<Composition>['save']
  edit?: Partial<LensDashboardEditing>
  props?: Partial<LensDashboardProps>
} = {}): Mounted {
  const save = vi.fn(options.save ?? ((value: Composition) => Promise.resolve({ ...value, revision: String(Number(value.revision) + 1) })))
  const onSaved = vi.fn()
  const onCancel = vi.fn()
  let mounted!: Mounted
  render(() => {
    const draft = createReportDraft(options.composition ?? editComposition(), { save })
    const [editing, setEditing] = createSignal(false)
    const edit: LensDashboardEditing = {
      draft, editing, setEditing, lifecycle: 'existing', reportTitle: (id) => editReportTitles[id], onSaved, onCancel, ...options.edit,
    }
    mounted = { edit, save, onSaved, onCancel, editing }
    return <LensDashboard edit={edit} fetcher={options.fetcher ?? never} initialDocument={options.document ?? editDocument()} {...options.props} />
  })
  return mounted
}

const order = () => [...document.querySelectorAll<HTMLElement>('[data-edit-placement]')].map((element) => element.dataset.editPlacement)
const card = (id: string) => document.querySelector<HTMLElement>(`[data-edit-placement="${id}"]`)!
const span = (id: string) => card(id).style.getPropertyValue('--lens-panel-span')
const rows = (composition: Composition) => composition.rows.map((row) => row.map((placement) => `${placement.id}${placement.span ?? 12}`).join(' '))

describe('LensDashboard edit mode', () => {
  // False green: checking only that the bar appears would pass while focus stayed on a removed button.
  it('enters and leaves edit mode in place, moving focus to the first handle and back to Edit', async () => {
    const { editing } = mount()
    expect(screen.queryByTestId('dashboard-edit-bar')).toBeNull()
    expect(screen.queryByTestId('report-editor-handle-revenue')).toBeNull()
    const toggle = await screen.findByTestId('dashboard-edit')
    toggle.focus()
    fireEvent.click(toggle)
    expect(editing()).toBe(true)
    expect(screen.getByTestId('dashboard-edit-bar')).toBeInTheDocument()
    await nextFrame()
    expect(screen.getByTestId('report-editor-handle-revenue')).toHaveFocus()
    expect(screen.getByTestId('dashboard-edit-announcement')).toHaveTextContent('Editing the dashboard')
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-revenue'), { key: 'Escape' })
    expect(editing()).toBe(false)
    await nextFrame()
    expect(screen.getByTestId('dashboard-edit')).toHaveFocus()
    expect(screen.getByTestId('dashboard-edit-announcement')).toHaveTextContent('Finished editing')
  })

  // False green: a reorder that rebuilt every card would also produce the right order, but refetch and remount the figures.
  it('reorders and resizes the live cards from the keyboard without remounting or refetching them', async () => {
    const { document: deferred, fetcher, requests } = deferredEditDocument()
    const { edit } = mount({ document: deferred, fetcher })
    await waitFor(() => expect(requests.length).toBeGreaterThan(0))
    await waitFor(() => expect(card('revenue').querySelector('[aria-busy="true"]')).toBeNull())
    const settled = requests.length
    const revenue = card('revenue')
    const chartNode = revenue.querySelector('[data-panel-id]')
    expect(chartNode).not.toBeNull()
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-revenue'), { key: 'ArrowDown' })
    expect(rows(edit.draft.value())).toEqual(['legacy4 deleted4 forbidden4', 'revenue12'])
    expect(order()).toEqual(['legacy', 'deleted', 'forbidden', 'revenue'])
    expect(card('revenue')).toBe(revenue)
    expect(card('revenue').style.getPropertyValue('--lens-edit-row')).toBe('2')
    fireEvent.keyDown(screen.getByTestId('report-editor-resize-legacy'), { key: 'ArrowRight' })
    expect(span('legacy')).toBe('6')
    expect(screen.getByTestId('report-editor-resize-legacy')).toHaveAttribute('aria-valuenow', '6')
    expect(screen.getByTestId('report-editor-status')).toHaveTextContent('Unsaved changes')
    expect(card('revenue').querySelector('[data-panel-id]')).toBe(chartNode)
    await nextFrame()
    expect(requests.length).toBe(settled)
  })

  // False green: asserting the draft alone would miss a drop that never reached the rendered order.
  it('drags a card by its handle and drops it before another', async () => {
    const { edit } = mount()
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    const data = new Map<string, string>()
    const dataTransfer = { types: ['application/x-lens-placement'], setData: (type: string, value: string) => data.set(type, value), getData: (type: string) => data.get(type) ?? '', effectAllowed: 'none' }
    expect(card('revenue').getAttribute('draggable')).toBeNull()
    fireEvent.dragStart(screen.getByTestId('report-editor-handle-forbidden'), { dataTransfer })
    fireEvent.dragOver(card('legacy'), { dataTransfer })
    expect(card('legacy')).toHaveAttribute('data-drop-target')
    fireEvent.drop(card('legacy'), { dataTransfer })
    expect(rows(edit.draft.value())).toEqual(['revenue12', 'forbidden4 legacy4 deleted4'])
    expect(order()).toEqual(['revenue', 'forbidden', 'legacy', 'deleted'])
  })

  // False green: a Cancel that only left edit mode would keep showing the moved layout.
  it('restores the original layout on Cancel and on Escape', async () => {
    const { edit, onCancel, save, editing } = mount()
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-legacy'), { key: 'ArrowUp' })
    fireEvent.keyDown(screen.getByTestId('report-editor-resize-revenue'), { key: 'Home' })
    expect(order()).not.toEqual(['revenue', 'legacy', 'deleted', 'forbidden'])
    fireEvent.click(screen.getByTestId('dashboard-edit-cancel'))
    expect(editing()).toBe(false)
    expect(edit.draft.value()).toEqual(editComposition())
    expect(order()).toEqual(['revenue', 'legacy', 'deleted', 'forbidden'])
    expect(span('revenue')).toBe('12')
    expect(onCancel).toHaveBeenCalledTimes(1)

    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-deleted'), { key: 'ArrowLeft' })
    fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(editing()).toBe(false)
    expect(edit.draft.value()).toEqual(editComposition())
    expect(save).not.toHaveBeenCalled()
  })

  // False green: a host save that resolved before the acknowledgement would report success for an unsaved layout.
  it('saves through the host draft against the opened revision and ends edit mode after the acknowledgement', async () => {
    const { edit, save, onSaved, editing } = mount()
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    expect(screen.getByTestId('report-editor-save')).toBeDisabled()
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-revenue'), { key: 'ArrowDown' })
    fireEvent.click(screen.getByTestId('report-editor-save'))
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1))
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ id: 'card-dashboard' }), { expectedRevision: '1', intent: 'draft' })
    expect(onSaved.mock.calls[0]![0]).toEqual(expect.objectContaining({ revision: '2' }))
    expect(editing()).toBe(false)
    expect(edit.draft.dirty()).toBe(false)
    // The saved layout stays on screen until the host refreshes the document.
    expect(order()).toEqual(['legacy', 'deleted', 'forbidden', 'revenue'])
  })

  // False green: one Save button for both lifecycles would never pass the publish intent.
  it('offers Save draft and Publish for a draft and passes the intent', async () => {
    const { save, onSaved } = mount({ edit: { lifecycle: 'new' } })
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.click(screen.getByTestId('report-editor-publish'))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(expect.anything(), 'publish'))
    expect(save.mock.calls[0]![1]).toEqual({ expectedRevision: '1', intent: 'publish' })
    expect(screen.queryByTestId('report-editor-save')).toBeNull()
  })

  // False green: a conflict that silently ended edit mode would drop the reader's layout.
  it('keeps the edits and offers the conflict choices when someone else saved first', async () => {
    const latest = { ...editComposition(), revision: '5' }
    const { edit, editing } = mount({ save: () => Promise.reject(new ReportRevisionConflictError(latest)) })
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-revenue'), { key: 'ArrowDown' })
    fireEvent.click(screen.getByTestId('report-editor-save'))
    const conflict = await screen.findByTestId('report-editor-conflict')
    expect(editing()).toBe(true)
    fireEvent.click(within(conflict).getByRole('button', { name: 'Keep my changes' }))
    expect(edit.draft.saved().revision).toBe('5')
    expect(rows(edit.draft.value())).toEqual(['legacy4 deleted4 forbidden4', 'revenue12'])
  })

  // False green: an added card rendered as an empty box would pass a count of cards.
  it('adds reports from the picker and fetches the new card’s figures only', async () => {
    const catalog = memoryCatalog()
    const preview = vi.spyOn(catalog, 'preview') as unknown as { mock: { calls: Array<[{ id: string }]> } }
    const { edit } = mount({ edit: { catalog } })
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.click(screen.getByTestId('dashboard-edit-add'))
    await waitFor(() => expect(document.querySelectorAll('[data-report-id]')).toHaveLength(20))
    fireEvent.click(document.querySelector<HTMLInputElement>('[data-report-id="report-05"] input')!)
    fireEvent.click(screen.getByTestId('report-picker-confirm'))
    await waitFor(() => expect(order()).toHaveLength(5))
    const added = edit.draft.value().rows.at(-1)![0]!
    expect(added.reportId).toBe('report-05')
    expect(preview.mock.calls).toHaveLength(1)
    expect(preview.mock.calls[0]![0].id).toBe('report-05')
    const body = await screen.findByTestId(`dashboard-edit-preview-${added.id}`)
    await waitFor(() => expect(body.querySelector('.lens-root [data-panel-id]')).not.toBeNull())
    expect(card(added.id)).toHaveAttribute('data-added')
    expect(within(card(added.id)).getByRole('heading', { level: 3, name: 'Reserves report 05' })).toBeInTheDocument()
  })

  // False green: a host document resolver that was never consulted would still render the catalog preview.
  it('prefers the host’s placement document for an added card', async () => {
    const placementDocument = vi.fn(() => Promise.resolve(editDocument()))
    const { edit } = mount({ edit: { catalog: memoryCatalog(), placementDocument } })
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    edit.draft.update((value) => ({ ...value, rows: [...value.rows, [{ id: 'extra', reportId: 'report-07' }]] }))
    await screen.findByTestId('dashboard-edit-preview-extra')
    expect(placementDocument).toHaveBeenCalledWith(expect.objectContaining({ id: 'extra' }), expect.objectContaining({ signal: expect.any(AbortSignal) as AbortSignal }))
  })

  // False green: a menu still calling the host's remove would delete the card outside the draft.
  it('removes a card from the draft through the card menu while editing', async () => {
    const hostRemove = vi.fn()
    const placementHost: ReportPlacementHost = { actions: { remove: hostRemove } }
    const { edit } = mount({ props: { reportPlacementHost: placementHost } })
    fireEvent.click(await screen.findByTestId('dashboard-edit'))
    fireEvent.click(within(card('deleted')).getByRole('button', { name: /^Card actions/ }))
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Remove from dashboard' }))
    await waitFor(() => expect(order()).toEqual(['revenue', 'legacy', 'forbidden']))
    expect(hostRemove).not.toHaveBeenCalled()
    expect(edit.draft.dirty()).toBe(true)
  })
})

describe('LensDashboard empty state', () => {
  // False green: the legacy placeholder also says the board is empty, but offers no way to fill it.
  it('offers Add report on an empty editable board, entering edit mode with the picker open', async () => {
    const { editing } = mount({ document: emptyDocument(), composition: emptyComposition(), edit: { catalog: memoryCatalog() } })
    await screen.findByTestId('dashboard-empty-add')
    const empty = screen.getByTestId('dashboard-empty')
    expect(within(empty).getByRole('heading', { name: 'This dashboard has no reports yet' })).toBeInTheDocument()
    fireEvent.click(within(empty).getByRole('button', { name: 'Add report' }))
    expect(editing()).toBe(true)
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(screen.getByTestId('dashboard-empty')).toBeInTheDocument()
  })

  // False green: rendering the button for every host would let a reader without edit rights start an edit.
  it('explains an empty board without an Add action when the host cannot edit', () => {
    render(() => <LensDashboard fetcher={never} initialDocument={emptyDocument()} reportPlacementHost={{}} />)
    const empty = screen.getByTestId('dashboard-empty')
    expect(within(empty).queryByRole('button')).toBeNull()
    expect(empty).toHaveTextContent('Reports placed on this dashboard will appear here.')
  })
})

describe('LensDashboard chrome', () => {
  // False green: hiding the h1 by dropping the title would also drop the landmark's accessible name.
  it('hides the document title and description for a host that renders its own header', () => {
    const doc = editDocument()
    doc.header = { title: 'Card dashboard', subtitle: 'A description from another report' }
    render(() => <LensDashboard chrome={{ title: false, description: false }} fetcher={never} initialDocument={doc} />)
    const header = document.querySelector<HTMLElement>('.lens-dashboard-header')!
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull()
    expect(within(header).queryByText('A description from another report')).toBeNull()
    expect(screen.getByRole('main', { name: 'Card dashboard' })).toBeInTheDocument()
    cleanup()
    render(() => <LensDashboard fetcher={never} initialDocument={doc} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Card dashboard' })).toBeInTheDocument()
    expect(within(document.querySelector<HTMLElement>('.lens-dashboard-header')!).getByText('A description from another report')).toBeInTheDocument()
  })

  // False green: moving the controls into a closed panel that is never shown would also leave the scope row short.
  it('folds the secondary filter verbs into a More menu and keeps the period on the scope row', async () => {
    const doc = parseDocument({ ...structuredClone(scopedFixture[1]!.document), filters: [compareFilter] })
    render(() => <LensDashboard chrome={{ toolbar: 'compact' }} fetcher={never} initialDocument={doc} />)
    const scope = document.querySelector<HTMLElement>('[data-testid="context-controls-dashboard-dashboard"]')!
    expect(scope.querySelector('.lens-filter-trigger')).not.toBeNull()
    expect(scope.querySelector('[data-testid="context-inherit"], [data-testid="context-reset"]')).toBeNull()
    expect(document.querySelector('.lens-dashboard-scope > .lens-compare-filter, .lens-dashboard-scope > * > .lens-compare-filter')).toBeNull()
    const more = await screen.findByTestId('toolbar-overflow')
    await waitFor(() => expect(more).toBeVisible())
    const panel = screen.getByTestId('toolbar-overflow-panel')
    expect(panel).not.toBeVisible()
    expect(within(panel).getByTestId('context-inherit')).toBeInTheDocument()
    expect(within(panel).getByTestId('context-reset')).toBeInTheDocument()
    expect(panel.querySelector('.lens-compare-trigger')).not.toBeNull()
    fireEvent.click(more)
    expect(more).toHaveAttribute('aria-expanded', 'true')
    expect(panel).toBeVisible()
    fireEvent.keyDown(panel, { key: 'Escape' })
    expect(more).toHaveAttribute('aria-expanded', 'false')
    expect(more).toHaveFocus()
  })
})
