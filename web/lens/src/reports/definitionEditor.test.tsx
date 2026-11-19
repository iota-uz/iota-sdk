import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import type { ContextFilters, Definition } from './contract/types'
import { createReportDraft, ReportRevisionConflictError, type ReportDraft, type ReportDraftHost } from './draft'
import { createReportLeaveGuard, type ReportLeaveGuard } from './guard'
import { ReportDefinitionEditor, type ReportDefinitionEditorProps, type ReportEditContext } from './ReportDefinitionEditor'

beforeAll(() => {
  window.scrollTo = vi.fn() as typeof window.scrollTo
})

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const report = (): Definition => ({
  id: 'report-01', revision: '7', title: 'Revenue', view: 'table', source: { id: 'synthetic.revenue', version: 2 },
  filters: { period: { mode: 'relative', months: 12 } },
  parameters: { top: 5 },
})

const board: ReportEditContext = {
  origin: { scope: 'dashboard', id: 'sales', title: 'Sales board' },
  filters: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' }, constraints: [{ dimension: 'region', values: ['North'] }] },
}

interface Mounted {
  draft: ReportDraft<Definition>
  guard: ReportLeaveGuard
  save: ReturnType<typeof vi.fn>
  previews: Array<{ top: unknown; filters: ContextFilters | undefined }>
}

function mount(options: { save?: ReportDraftHost<Definition>['save']; props?: Partial<ReportDefinitionEditorProps> } = {}): Mounted {
  const save = vi.fn(options.save ?? ((value: Definition) => Promise.resolve({ ...value, revision: String(Number(value.revision) + 1) })))
  const previews: Mounted['previews'] = []
  let mounted!: Mounted
  render(() => {
    const draft = createReportDraft(report(), { save })
    const guard = createReportLeaveGuard(draft)
    mounted = { draft, guard, save, previews }
    return (
      <ReportDefinitionEditor
        context={board}
        dimensionLabel={(dimension) => (dimension === 'region' ? 'Region' : dimension)}
        draft={draft}
        guard={guard}
        renderParameters={(parameters) => (
          <label>
            Rows
            <input onChange={(event) => parameters.onChange({ ...parameters.value, top: Number(event.currentTarget.value) })} type="number" value={Number(parameters.value?.top ?? 0)} />
          </label>
        )}
        renderPreview={(definition, filters) => {
          previews.push({ top: definition.parameters?.top, filters })
          return <p data-testid="preview-body">{`${String(definition.parameters?.top)} rows, ${filters?.period?.mode ?? 'none'}`}</p>
        }}
        {...options.props}
      />
    )
  })
  return mounted
}

const status = () => screen.getByTestId('report-definition-status')
const saveButton = () => screen.getByRole('button', { name: 'Save changes' })
const rows = () => screen.getByLabelText('Rows')

describe('ReportDefinitionEditor', () => {
  // False green: reusing the card dialog title would let a reader edit a shared report thinking it was one card.
  it('is titled as a shared report edit and shows its reach before any change', async () => {
    mount({ props: { placements: () => [{ id: 'a', title: 'Sales board · Revenue', current: true }, { id: 'b', title: 'Executive · Revenue' }], onCopy: vi.fn() } })
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Edit report: Revenue')
    const impact = screen.getByTestId('report-definition-impact')
    await waitFor(() => expect(impact).toHaveTextContent('used in 2 places'))
    expect(within(impact).getByTestId('report-usage-list')).toHaveTextContent('Sales board · RevenueThis card')
    expect(within(impact).getByRole('button', { name: 'Make an independent copy' })).toBeInTheDocument()
  })

  // False green: a preview that saved, or dashboard filters written into the definition, would pass a render-only check.
  it('shows the origin of the dashboard filters and previews with them without saving them', () => {
    const { draft, save, previews } = mount()
    expect(screen.getByTestId('report-definition-origin')).toHaveTextContent('Opened from Sales board: 01.06.2026 – 30.06.2026; Region: North')
    expect(screen.getByTestId('report-definition-defaults')).toHaveTextContent('Report defaults: Last 12 months')
    expect(previews.at(-1)!.filters).toEqual(board.filters)
    fireEvent.click(screen.getByLabelText('Report defaults'))
    expect(screen.getByTestId('preview-body')).toHaveTextContent('relative')
    expect(draft.dirty()).toBe(false)
    expect(saveButton()).toBeDisabled()
    expect(save).not.toHaveBeenCalled()
  })

  // False green: a parameter change saved on input would make preview and save the same action.
  it('previews a parameter change at once and saves it only on Save changes', async () => {
    const { draft, save } = mount()
    fireEvent.change(rows(), { target: { value: '8' } })
    expect(screen.getByTestId('preview-body')).toHaveTextContent('8 rows')
    expect(screen.getByTestId('report-definition-preview')).toHaveTextContent('Not saved')
    expect(status()).toHaveTextContent('Unsaved changes')
    expect(save).not.toHaveBeenCalled()
    fireEvent.click(saveButton())
    await waitFor(() => expect(status()).toHaveTextContent('All changes saved'))
    expect(save).toHaveBeenCalledOnce()
    const submitted = save.mock.calls[0]![0] as Definition
    expect(submitted.parameters).toEqual({ top: 8 })
    expect(submitted.filters).toEqual(report().filters)
    expect(draft.saved().revision).toBe('8')
  })

  // False green: an always-successful host save hides lost edits on failure.
  it('keeps the edits after a failed save and restores the whole session on Cancel', async () => {
    const { draft } = mount({ save: () => Promise.reject(new Error('offline')) })
    fireEvent.change(rows(), { target: { value: '3' } })
    const title = screen.getByLabelText('Report name')
    fireEvent.change(title, { target: { value: 'Revenue, short' } })
    fireEvent.click(saveButton())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('They are kept here'))
    expect(status()).toHaveTextContent('Unsaved changes')
    expect(draft.value().parameters).toEqual({ top: 3 })
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(draft.value()).toEqual(report())
    expect(title).toHaveValue('Revenue')
    expect(screen.getByTestId('preview-body')).toHaveTextContent('5 rows')
  })

  // False green: a conflict shown as a plain failure would leave only a retry that overwrites the other save.
  it('never overwrites a newer version: review theirs, keep mine, or save an independent copy', async () => {
    const latest = { ...report(), revision: '8', title: 'Revenue by channel' }
    const onCopy = vi.fn()
    let first = true
    const { draft, save } = mount({
      props: { onCopy },
      save: (value, options) => {
        if (first) {
          first = false
          return Promise.reject(new ReportRevisionConflictError(latest))
        }
        return Promise.resolve({ ...value, revision: String(Number(options.expectedRevision) + 1) })
      },
    })
    fireEvent.change(rows(), { target: { value: '8' } })
    fireEvent.click(saveButton())
    const conflict = await screen.findByTestId('report-definition-conflict')
    expect(draft.value().parameters).toEqual({ top: 8 })
    fireEvent.click(within(conflict).getByRole('button', { name: 'Save as a separate copy' }))
    await waitFor(() => expect(onCopy).toHaveBeenCalledWith(expect.objectContaining({ parameters: { top: 8 } })))
    fireEvent.click(within(conflict).getByRole('button', { name: 'Keep my changes' }))
    expect(draft.saved().title).toBe('Revenue by channel')
    fireEvent.click(saveButton())
    await waitFor(() => expect(status()).toHaveTextContent('All changes saved'))
    expect(save.mock.calls[1]![1]).toEqual({ expectedRevision: '8', intent: 'draft' })
  })

  it('saves the current dashboard view as a new report through the host and reports a failed copy', async () => {
    const onSaveSlice = vi.fn(() => Promise.resolve())
    const { draft } = mount({ props: { onSaveSlice, onCopy: () => Promise.reject(new Error('denied')) } })
    fireEvent.click(screen.getByTestId('report-definition-slice'))
    await waitFor(() => expect(onSaveSlice).toHaveBeenCalledWith(report(), board.filters))
    expect(await screen.findByRole('status')).toHaveTextContent('The shared report is unchanged')
    expect(draft.dirty()).toBe(false)
    fireEvent.click(screen.getByTestId('report-definition-copy'))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('could not be completed'))
  })

  // False green: allowing navigation after a failed save would lose the edits it claimed to protect.
  it('asks before leaving a dirty session and stays after a failed save', async () => {
    const { guard } = mount({ save: () => Promise.reject(new Error('offline')) })
    fireEvent.change(rows(), { target: { value: '3' } })
    const leaving = guard.request()
    const dialog = await screen.findByRole('alertdialog', { name: 'Save changes before leaving?' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(within(dialog).getByRole('alert')).toHaveTextContent('could not be saved'))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Stay' }))
    await expect(leaving).resolves.toBe(false)
  })
})
