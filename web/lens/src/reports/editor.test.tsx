import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { createRoot } from 'solid-js'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { memoryCatalog } from '../stories/reportCatalog'
import type { Composition } from './contract/types'
import { createReportDraft, reportDraftToken, ReportRevisionConflictError, type ReportDraft, type ReportDraftHost } from './draft'
import { createReportLeaveGuard, type ReportLeaveGuard } from './guard'
import { configurePlacement, diffCompositions, movePlacement, movePlacementBefore, resizePlacement } from './layout'
import { ReportCompositionEditor, type ReportCompositionEditorProps, type ReportProposal } from './ReportCompositionEditor'

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

const base = (): Composition => ({
  id: 'board', revision: '3', title: 'Board', rows: [
    [{ id: 'a', reportId: 'report-01', span: 6 }, { id: 'b', reportId: 'report-02', span: 6 }],
    [{ id: 'c', reportId: 'report-03', filters: { constraints: [{ dimension: 'product', values: ['osago'] }] } }],
  ],
})
const order = (composition: Composition) => composition.rows.map((row) => row.map((placement) => `${placement.id}${placement.span ?? 12}`).join(' '))
const titles: Record<string, string> = { 'report-01': 'Revenue', 'report-02': 'Portfolio', 'report-03': 'Losses', 'report-04': 'Finance', 'report-05': 'Reserves', 'report-07': 'Claims' }

// False green: a move that only reordered the flat list would pass while it broke the twelve-column rows.
describe('composition layout', () => {
  it('moves cards within and across rows without overfilling a row', () => {
    expect(order(movePlacement(base(), 'b', 'left'))).toEqual(['b6 a6', 'c12'])
    expect(order(movePlacement(base(), 'a', 'left'))).toEqual(['a6', 'b6', 'c12'])
    expect(order(movePlacement(base(), 'b', 'right'))).toEqual(['a6', 'b6', 'c12'])
    expect(order(movePlacement(base(), 'c', 'up'))).toEqual(['c12', 'a6 b6'])
    expect(order(movePlacement(base(), 'a', 'down'))).toEqual(['b6', 'c12', 'a6'])
    expect(movePlacement(base(), 'c', 'down')).toEqual(base())
    expect(order(movePlacementBefore(base(), 'c', 'b'))).toEqual(['c12', 'a6 b6'])
    expect(order(movePlacementBefore(resizePlacement(base(), 'c', 3), 'c', 'b'))).toEqual(['c3', 'a6 b6'])
  })

  it('wraps cards that no longer fit after a resize and keeps other card filters when configuring', () => {
    expect(order(resizePlacement(base(), 'a', 9))).toEqual(['a9', 'b6', 'c12'])
    expect(order(resizePlacement(base(), 'a', 1))).toEqual(['a3 b6', 'c12'])
    const configured = configurePlacement(base(), 'c', { title: ' June ', span: 12, period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } })
    const card = configured.rows[1]![0]!
    expect(card.title).toBe('June')
    expect(card.filters).toEqual({ constraints: [{ dimension: 'product', values: ['osago'] }], period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } })
    const inherited = configurePlacement(configured, 'c', { title: '', span: 12 })
    expect(inherited.rows[1]![0]!.title).toBeUndefined()
    expect(inherited.rows[1]![0]!.filters).toEqual({ constraints: [{ dimension: 'product', values: ['osago'] }] })
  })

  it('lists additions, changes and removals between compositions', () => {
    const proposed = base()
    proposed.rows[0]![0]!.title = 'Renamed'
    proposed.rows[1] = [{ id: 'd', reportId: 'report-04' }]
    expect(diffCompositions(base(), proposed)).toEqual([
      { kind: 'change', placementId: 'a', reportId: 'report-01' },
      { kind: 'add', placementId: 'd', reportId: 'report-04' },
      { kind: 'remove', placementId: 'c', reportId: 'report-03' },
    ])
  })
})

function draftOf(host: ReportDraftHost<Composition>, initial = base()): { draft: ReportDraft<Composition>; dispose: () => void } {
  let dispose = () => {}
  const draft = createRoot((cleanupRoot) => {
    dispose = cleanupRoot
    return createReportDraft(initial, host)
  })
  return { draft, dispose }
}

// False green: a rebase that silently replaced the draft would lose the reader's edits on "keep mine".
describe('draft rebase and leave guard', () => {
  it('keeps or discards local edits explicitly and counts every local change', () => {
    const { draft, dispose } = draftOf({ save: (value) => Promise.resolve(value) })
    const latest = { ...base(), revision: '4', title: 'Elsewhere' }
    draft.update((value) => ({ ...value, title: 'Mine' }))
    const before = draft.sequence()
    draft.rebase(latest, 'keep')
    expect(draft.value().title).toBe('Mine')
    expect(draft.value().revision).toBe('4')
    expect(draft.saved()).toEqual(latest)
    expect(draft.sequence()).toBe(before + 1)
    draft.rebase(latest, 'discard')
    expect(draft.value()).toEqual(latest)
    expect(draft.dirty()).toBe(false)
    expect(() => draft.rebase({ ...latest, id: 'other' }, 'keep')).toThrow()
    dispose()
  })

  it('asks before leaving a dirty draft and never allows leaving after a failed save', async () => {
    let fail = true
    const { draft, dispose } = draftOf({ save: (value) => (fail ? Promise.reject(new Error('nope')) : Promise.resolve({ ...value, revision: '9' })) })
    const guard = createRoot(() => createReportLeaveGuard(draft))
    await expect(guard.request()).resolves.toBe(true)
    draft.update((value) => ({ ...value, title: 'Changed' }))
    const unload = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(unload)
    expect(unload.defaultPrevented).toBe(true)

    const stay = guard.request()
    expect(guard.pending()).toBe(true)
    await guard.choose('stay')
    await expect(stay).resolves.toBe(false)

    const failed = guard.request()
    let settled = false
    void failed.then(() => { settled = true })
    await guard.choose('save')
    expect(guard.failed()).toBe(true)
    expect(guard.pending()).toBe(true)
    expect(settled).toBe(false)
    fail = false
    await guard.choose('save')
    await expect(failed).resolves.toBe(true)
    expect(draft.dirty()).toBe(false)

    draft.update((value) => ({ ...value, title: 'Again' }))
    const discard = guard.request()
    await guard.choose('discard')
    await expect(discard).resolves.toBe(true)
    expect(draft.value().title).toBe('Changed')
    dispose()
  })
})

// False green: a guard that opened the dialog during a save would race it; one that let cancel/save throw would hang silently.
describe('leave guard during an in-flight save', () => {
  function deferredDraft() {
    let settle!: { resolve: () => void; reject: () => void }
    const { draft, dispose } = draftOf({ save: (value) => new Promise((resolve, reject) => {
      settle = { resolve: () => resolve({ ...value, revision: String(Number(value.revision) + 1) }), reject: () => reject(new Error('offline')) }
    }) })
    return { draft, dispose, settle: () => settle }
  }

  it('waits for the save and lets the reader leave when it succeeded', async () => {
    const { draft, dispose, settle } = deferredDraft()
    const guard = createRoot(() => createReportLeaveGuard(draft))
    draft.update((value) => ({ ...value, title: 'Saving' }))
    const saving = draft.save()
    const leaving = guard.request()
    expect(guard.pending()).toBe(false)
    settle().resolve()
    await saving
    await expect(leaving).resolves.toBe(true)
    expect(guard.pending()).toBe(false)
    dispose()
  })

  it('asks once the save failed, keeping the edits', async () => {
    const { draft, dispose, settle } = deferredDraft()
    const guard = createRoot(() => createReportLeaveGuard(draft))
    draft.update((value) => ({ ...value, title: 'Saving' }))
    const saving = draft.save().catch(() => undefined)
    const leaving = guard.request()
    settle().reject()
    await saving
    await waitFor(() => expect(guard.pending()).toBe(true))
    expect(draft.value().title).toBe('Saving')
    await guard.choose('stay')
    await expect(leaving).resolves.toBe(false)
    dispose()
  })

  it('lets a choice made during a save wait for it instead of failing', async () => {
    const { draft, dispose, settle } = deferredDraft()
    const guard = createRoot(() => createReportLeaveGuard(draft))
    draft.update((value) => ({ ...value, title: 'Mine' }))
    const leaving = guard.request()
    expect(guard.pending()).toBe(true)
    const saving = draft.save()
    const discard = guard.choose('discard')
    expect(guard.saving()).toBe(true)
    settle().resolve()
    await saving
    await discard
    await expect(leaving).resolves.toBe(true)
    expect(guard.failed()).toBe(false)
    expect(draft.value().title).toBe('Mine')
    dispose()
  })

  it('reports a failing cancel instead of losing it', async () => {
    const guard = createRoot(() => createReportLeaveGuard({
      dirty: () => true, saving: () => false, cancel: () => { throw new Error('locked') }, save: () => Promise.resolve(),
    }, { beforeUnload: false }))
    const leaving = guard.request()
    await expect(guard.choose('discard')).resolves.toBeUndefined()
    expect(guard.failure()).toBe('discard')
    expect(guard.pending()).toBe(true)
    await guard.choose('stay')
    await expect(leaving).resolves.toBe(false)
  })
})

interface Mounted {
  draft: ReportDraft<Composition>
  save: ReturnType<typeof vi.fn>
  guard: ReportLeaveGuard
}

function mountEditor(options: { save?: ReportDraftHost<Composition>['save']; initial?: Composition; props?: Partial<ReportCompositionEditorProps> } = {}): Mounted {
  const save = vi.fn(options.save ?? ((value: Composition) => Promise.resolve({ ...value, revision: String(Number(value.revision) + 1) })))
  let mounted!: Mounted
  render(() => {
    const draft = createReportDraft(options.initial ?? base(), { save })
    const guard = createReportLeaveGuard(draft)
    mounted = { draft, save, guard }
    return (
      <div class="lens-root" data-theme="light">
        <ReportCompositionEditor draft={draft} guard={guard} lifecycle="existing" reportTitle={(id) => titles[id]} {...options.props} />
      </div>
    )
  })
  return mounted
}

const status = () => screen.getByTestId('report-editor-status')
const card = (id: string) => document.querySelector<HTMLElement>(`[data-placement-editor="${id}"]`)!
const cardOrder = () => [...document.querySelectorAll<HTMLElement>('[data-placement-editor]')].map((element) => element.dataset.placementEditor)

async function configure(id: string, caption: string) {
  fireEvent.click(within(card(id)).getByRole('button', { name: 'Configure card' }))
  const dialog = await screen.findByRole('dialog', { name: /^Configure card/ })
  fireEvent.input(within(dialog).getByLabelText('Card caption'), { target: { value: caption } })
  fireEvent.click(within(dialog).getByRole('button', { name: 'Apply to draft' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
}

describe('ReportCompositionEditor', () => {
  // False green: mounting inside a test-provided .lens-root would supply the tokens a bare host page lacks.
  it('scopes the Lens tokens itself when a host mounts it without a Lens root', () => {
    render(() => <ReportCompositionEditor draft={createReportDraft(base(), { save: (value) => Promise.resolve(value) })} lifecycle="existing" reportTitle={(id) => titles[id]} theme="dark" />)
    const root = screen.getByTestId('report-editor').closest('.lens-root')
    expect(root).not.toBeNull()
    expect(root).toHaveClass('lens-root-inline')
    expect(root).toHaveAttribute('data-theme', 'dark')
  })

  // False green: a Cancel tested only after a reorder would miss captions, filters and added cards surviving it.
  it('cancels the whole session: layout, captions, local periods, added and removed cards', async () => {
    const { draft, save } = mountEditor({ props: { catalog: memoryCatalog() } })
    expect(status()).toHaveTextContent('No changes')
    await configure('a', 'Renamed')
    expect(within(card('a')).getByRole('heading')).toHaveTextContent('Renamed')
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-b'), { key: 'ArrowLeft' })
    fireEvent.click(within(card('c')).getByRole('button', { name: 'Remove Losses' }))
    fireEvent.click(screen.getByRole('button', { name: 'Add report' }))
    await waitFor(() => expect(document.querySelectorAll('[data-report-id]')).toHaveLength(20))
    fireEvent.click(document.querySelector<HTMLInputElement>('[data-report-id="report-05"] input')!)
    fireEvent.click(screen.getByTestId('report-picker-confirm'))
    await waitFor(() => expect(screen.getByTestId('report-editor-notice')).toHaveTextContent('Added 1 reports to the draft'))
    expect(cardOrder()).toHaveLength(3)
    expect(status()).toHaveTextContent('Unsaved changes')
    expect(save).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(draft.value()).toEqual(base())
    expect(cardOrder()).toEqual(['a', 'b', 'c'])
    expect(within(card('a')).getByRole('heading')).toHaveTextContent('Revenue')
  })

  // False green: a success shown before the host resolves, or edits reset on rejection.
  it('keeps every edit when saving fails and reports success only after the host acknowledges', async () => {
    let attempt = 0
    let acknowledge!: () => void
    const { draft, save } = mountEditor({ save: (value) => {
      attempt += 1
      if (attempt === 1) return Promise.reject(new Error('offline'))
      return new Promise((resolve) => { acknowledge = () => resolve({ ...value, revision: '4' }) })
    } })
    await configure('b', 'Mine')
    const saveButton = screen.getByRole('button', { name: 'Save changes' })
    fireEvent.click(saveButton)
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('They are kept here'))
    expect(draft.value().rows[0]![1]!.title).toBe('Mine')
    expect(status()).toHaveTextContent('Unsaved changes')
    fireEvent.click(saveButton)
    await waitFor(() => expect(status()).toHaveTextContent('Saving…'))
    expect(status()).not.toHaveTextContent('All changes saved')
    acknowledge()
    await waitFor(() => expect(status()).toHaveTextContent('All changes saved'))
    expect(save.mock.calls[1]![1]).toEqual({ expectedRevision: '3', intent: 'draft' })
    expect(screen.queryByRole('alert')).toBeNull()
  })

  // False green: a conflict treated as a plain failure would leave only "try again", which overwrites silently.
  it('lets the reader keep their changes or load the other version after a revision conflict', async () => {
    const latest = { ...base(), revision: '4', title: 'Saved elsewhere' }
    let conflicted = false
    const { draft, save } = mountEditor({ save: (value, options) => {
      if (!conflicted) {
        conflicted = true
        return Promise.reject(new ReportRevisionConflictError(latest))
      }
      return Promise.resolve({ ...value, revision: String(Number(options.expectedRevision) + 1) })
    } })
    await configure('a', 'Mine')
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    const conflict = await screen.findByTestId('report-editor-conflict')
    expect(draft.value().rows[0]![0]!.title).toBe('Mine')
    fireEvent.click(within(conflict).getByRole('button', { name: 'Keep my changes' }))
    expect(draft.value().rows[0]![0]!.title).toBe('Mine')
    expect(draft.saved().revision).toBe('4')
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(status()).toHaveTextContent('All changes saved'))
    expect(save.mock.calls[1]![1]).toEqual({ expectedRevision: '4', intent: 'draft' })
  })

  it('loads the other version only on request', async () => {
    const latest = { ...base(), revision: '4', title: 'Saved elsewhere' }
    const { draft } = mountEditor({ save: () => Promise.reject(new ReportRevisionConflictError(latest)) })
    await configure('a', 'Mine')
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    fireEvent.click(within(await screen.findByTestId('report-editor-conflict')).getByRole('button', { name: 'Load their version' }))
    expect(draft.value()).toEqual(latest)
    expect(screen.getByLabelText('Dashboard name')).toHaveValue('Saved elsewhere')
  })

  // False green: moving the card without keeping focus on its handle strands a keyboard user.
  it('reorders and resizes with the keyboard, keeping focus and announcing the result', async () => {
    const { draft } = mountEditor()
    const handle = screen.getByTestId('report-editor-handle-b')
    handle.focus()
    fireEvent.keyDown(handle, { key: 'ArrowLeft' })
    expect(cardOrder()).toEqual(['b', 'a', 'c'])
    await waitFor(() => expect(document.activeElement).toBe(screen.getByTestId('report-editor-handle-b')))
    expect(screen.getByTestId('report-editor-announcement')).toHaveTextContent('Moved Portfolio to row 1, position 1')
    // The next row is full, so the card gets a row of its own below it.
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-b'), { key: 'ArrowDown' })
    expect(order(draft.value())).toEqual(['a6', 'c12', 'b6'])
    const resize = screen.getByTestId('report-editor-resize-a')
    expect(resize).toHaveAttribute('aria-valuenow', '6')
    fireEvent.keyDown(resize, { key: 'ArrowLeft' })
    expect(order(draft.value())[0]).toBe('a4')
    fireEvent.keyDown(resize, { key: 'End' })
    expect(order(draft.value())[0]).toBe('a12')
    expect(screen.getByTestId('report-editor-announcement')).toHaveTextContent('Revenue is 12 of 12 columns wide')
  })

  it('starts a drag only from the handle and drops before the target card', () => {
    const { draft } = mountEditor()
    expect(card('a').getAttribute('draggable')).toBeNull()
    expect(screen.getByTestId('report-editor-handle-c')).toHaveAttribute('draggable', 'true')
    const data = new Map<string, string>()
    const dataTransfer = { types: ['application/x-lens-placement'], setData: (type: string, value: string) => data.set(type, value), getData: (type: string) => data.get(type) ?? '', effectAllowed: 'none' }
    fireEvent.dragStart(screen.getByTestId('report-editor-handle-c'), { dataTransfer })
    fireEvent.dragOver(card('a'), { dataTransfer })
    fireEvent.drop(card('a'), { dataTransfer })
    expect(order(draft.value())).toEqual(['c12', 'a6 b6'])
  })

  // False green: an apply button that stayed enabled after a manual edit would overwrite that edit.
  it('applies or rejects a proposal as a whole and refuses a stale one', () => {
    const settled = vi.fn()
    const regenerate = vi.fn()
    let proposal!: ReportProposal
    const make = (target: ReportDraft<Composition>): ReportProposal => {
      const next = target.value()
      next.rows = [[{ id: 'a', reportId: 'report-01', span: 6, title: 'Revenue this year' }, { id: 'b', reportId: 'report-02', span: 6 }], [{ id: 'n', reportId: 'report-07' }]]
      return { id: 'p1', base: reportDraftToken(target), summary: 'Focus on claims', composition: next }
    }
    let mounted!: Mounted
    render(() => {
      const value = createReportDraft(base(), { save: (composition) => Promise.resolve(composition) })
      const guard = createReportLeaveGuard(value)
      mounted = { draft: value, save: vi.fn(), guard }
      proposal = make(value)
      return <ReportCompositionEditor draft={value} guard={guard} lifecycle="existing" proposal={proposal} proposalHandlers={{ settled, regenerate }} reportTitle={(id) => titles[id]} />
    })
    const panel = screen.getByTestId('report-proposal')
    expect([...panel.querySelectorAll('li')].map((item) => item.textContent)).toEqual(['ChangeRevenue this year', 'AddClaims', 'RemoveLosses'])
    fireEvent.keyDown(screen.getByTestId('report-editor-handle-b'), { key: 'ArrowLeft' })
    expect(within(panel).getByRole('alert')).toHaveTextContent('can no longer be applied')
    expect(within(panel).queryByRole('button', { name: 'Apply to draft' })).toBeNull()
    fireEvent.click(within(panel).getByRole('button', { name: 'Prepare again' }))
    expect(regenerate).toHaveBeenCalledWith(proposal)
    // Cancel is a local change too: the proposal stays stale.
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(within(panel).queryByRole('button', { name: 'Apply to draft' })).toBeNull()
    cleanup()
    document.body.replaceChildren()

    render(() => {
      const value = createReportDraft(base(), { save: (composition) => Promise.resolve(composition) })
      mounted = { draft: value, save: vi.fn(), guard: createReportLeaveGuard(value) }
      return <ReportCompositionEditor draft={value} guard={mounted.guard} lifecycle="existing" proposal={make(value)} proposalHandlers={{ settled }} reportTitle={(id) => titles[id]} />
    })
    fireEvent.click(within(screen.getByTestId('report-proposal')).getByRole('button', { name: 'Reject' }))
    expect(settled).toHaveBeenLastCalledWith('p1', 'rejected')
    expect(mounted.draft.dirty()).toBe(false)
    expect(screen.queryByTestId('report-proposal')).toBeNull()
    cleanup()
    document.body.replaceChildren()

    render(() => {
      const value = createReportDraft(base(), { save: (composition) => Promise.resolve(composition) })
      mounted = { draft: value, save: vi.fn(), guard: createReportLeaveGuard(value) }
      return <ReportCompositionEditor draft={value} guard={mounted.guard} lifecycle="existing" proposal={make(value)} proposalHandlers={{ settled }} reportTitle={(id) => titles[id]} />
    })
    fireEvent.click(within(screen.getByTestId('report-proposal')).getByRole('button', { name: 'Apply to draft' }))
    expect(settled).toHaveBeenLastCalledWith('p1', 'applied')
    expect(cardOrder()).toEqual(['a', 'b', 'n'])
    expect(mounted.draft.value().revision).toBe('3')
    expect(mounted.draft.dirty()).toBe(true)
    expect(screen.getByTestId('report-editor-notice')).toHaveTextContent('Nothing is published until you save')
  })

  // False green: checking only that Apply is hidden would miss an invalid proposal written into the draft some other way.
  it('refuses an invalid proposal: duplicate placement ids or a row wider than twelve columns', () => {
    const settled = vi.fn()
    const openAsDraft = vi.fn()
    const broken = {
      duplicate: (composition: Composition) => ({ ...composition, rows: [[{ id: 'a', reportId: 'report-01', span: 6 }, { id: 'a', reportId: 'report-02', span: 6 }]] }),
      wide: (composition: Composition) => ({ ...composition, rows: [[{ id: 'a', reportId: 'report-01', span: 8 }, { id: 'b', reportId: 'report-02', span: 8 }]] }),
      empty: (composition: Composition) => ({ ...composition, rows: [[]] }),
    }
    for (const [name, mutate] of Object.entries(broken)) {
      let mounted!: Mounted
      render(() => {
        const value = createReportDraft(base(), { save: (composition) => Promise.resolve(composition) })
        mounted = { draft: value, save: vi.fn(), guard: createReportLeaveGuard(value) }
        const proposal: ReportProposal = { id: `p-${name}`, base: reportDraftToken(value), composition: mutate(value.value()) }
        return <ReportCompositionEditor draft={value} guard={mounted.guard} lifecycle="existing" proposal={proposal} proposalHandlers={{ settled, openAsDraft }} reportTitle={(id) => titles[id]} />
      })
      const panel = screen.getByTestId('report-proposal')
      expect(within(panel).getByRole('alert'), name).toHaveTextContent('its layout is not valid')
      expect(within(panel).queryByRole('button', { name: 'Apply to draft' }), name).toBeNull()
      expect(within(panel).queryByRole('button', { name: 'Prepare again' }), name).toBeNull()
      expect(panel.querySelectorAll('li'), name).toHaveLength(0)
      fireEvent.click(within(panel).getByRole('button', { name: 'Open as a separate draft' }))
      expect(openAsDraft).toHaveBeenLastCalledWith(expect.objectContaining({ id: `p-${name}` }))
      fireEvent.click(within(panel).getByRole('button', { name: 'Reject' }))
      expect(settled).toHaveBeenLastCalledWith(`p-${name}`, 'rejected')
      expect(mounted.draft.value(), name).toEqual(base())
      expect(mounted.draft.dirty(), name).toBe(false)
      cleanup()
      document.body.replaceChildren()
    }
  })

  // False green: an impact dialog that edited before showing the count would leave no chance to copy instead.
  it('shows the shared-edit impact before editing a report and returns focus on Escape', async () => {
    const onEditReport = vi.fn()
    const onCopyReport = vi.fn()
    mountEditor({ props: { onEditReport, onCopyReport, usage: () => Promise.resolve(4) } })
    const trigger = within(card('a')).getByRole('button', { name: 'Edit report' })
    fireEvent.click(trigger)
    const dialog = await screen.findByRole('alertdialog', { name: 'Edit report: Revenue' })
    await waitFor(() => expect(within(dialog).getByTestId('report-impact-usage')).toHaveTextContent('used in 4 places'))
    expect(onEditReport).not.toHaveBeenCalled()
    await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' })))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
    expect(document.activeElement).toBe(screen.getByTestId('report-editor-handle-a'))
    fireEvent.click(trigger)
    fireEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Make an independent copy' }))
    await waitFor(() => expect(onCopyReport).toHaveBeenCalledWith('a'))
    fireEvent.click(trigger)
    fireEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Edit the shared report' }))
    await waitFor(() => expect(onEditReport).toHaveBeenCalledWith('report-01', 'a'))
  })

  it('labels a new object Save draft and Publish and passes the intent', async () => {
    const { save } = mountEditor({ props: { lifecycle: 'new' }, initial: { id: 'board', revision: '1', title: 'New', rows: [] } })
    expect(screen.getByTestId('report-editor-empty')).toHaveTextContent('no reports yet')
    expect(status()).toHaveTextContent('Draft')
    expect(screen.queryByRole('button', { name: 'Save changes' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Publish' }))
    await waitFor(() => expect(save).toHaveBeenCalledOnce())
    expect(save.mock.calls[0]![1]).toEqual({ expectedRevision: '1', intent: 'publish' })
  })

  // False green: a guard dialog that resolved before the save finished would let navigation outrun a failure.
  it('asks to save, leave or stay through the in-app guard and keeps focus inside', async () => {
    const { guard, draft } = mountEditor({ save: () => Promise.reject(new Error('offline')) })
    await configure('a', 'Mine')
    const leaving = guard.request()
    const dialog = await screen.findByRole('alertdialog', { name: 'Save changes before leaving?' })
    await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Stay' })))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(within(dialog).getByRole('alert')).toHaveTextContent('you are still here'))
    const last = within(dialog).getByRole('button', { name: 'Save changes' })
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(within(dialog).getByRole('button', { name: 'Close' })).toBe(document.activeElement)
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await expect(leaving).resolves.toBe(false)
    expect(draft.value().rows[0]![0]!.title).toBe('Mine')
  })
})
