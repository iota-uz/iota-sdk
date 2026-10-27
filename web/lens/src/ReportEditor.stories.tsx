import { createSignal, onMount, Show, type JSX } from 'solid-js'
import type { LensThemeMode } from './runtime'
import type { Composition, Placement } from './reports/contract/types'
import { createReportDraft, parseReportComposition, reportDraftToken, ReportRevisionConflictError, type ReportDraft } from './reports/draft'
import { createReportLeaveGuard } from './reports/guard'
import { ReportCompositionEditor, type ReportProposal } from './reports/ReportCompositionEditor'
import { catalogEntries, memoryCatalog } from './stories/reportCatalog'

const entries = new Map(catalogEntries().map((entry) => [entry.id, entry]))
const reportTitle = (id: string) => entries.get(id)?.title

/** The saved dashboard every story starts from: two rows, mixed widths, one card-local period. */
function editorComposition(): Composition {
  return parseReportComposition({
    id: 'sales-board', revision: '3', title: 'Sales board', rows: [
      [{ id: 'revenue', reportId: 'report-01', span: 8 }, { id: 'portfolio', reportId: 'report-02', span: 4 }],
      [{ id: 'losses', reportId: 'report-03', span: 6, title: 'Large losses, June', filters: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } } },
        { id: 'finance', reportId: 'report-04', span: 6 }],
    ],
  })
}

function Body(props: { placement: Placement }): JSX.Element {
  const entry = () => entries.get(props.placement.reportId)
  return <p class="lens:m-0 lens:text-base lens:text-muted">{entry()?.description ?? props.placement.reportId}</p>
}

/** Story-only autoplay: waits for an element the scenario needs, then acts on it. */
function when(selector: string, act: (element: HTMLElement) => void, frames = 120): void {
  const element = globalThis.document.querySelector<HTMLElement>(selector)
  if (element) act(element)
  else if (frames > 0) requestAnimationFrame(() => when(selector, act, frames - 1))
}

interface StoryControl {
  rejectSave: boolean
  conflictSave: boolean
}

function Editor(props: {
  theme: LensThemeMode
  lifecycle?: 'new' | 'existing'
  initial?: Composition
  setup?: (draft: ReportDraft<Composition>) => void
  after?: () => void
  control?: StoryControl
  proposal?: (draft: ReportDraft<Composition>) => ReportProposal
  guard?: boolean
}): JSX.Element {
  const control = props.control ?? { rejectSave: false, conflictSave: false }
  let stored = structuredClone(props.initial ?? editorComposition())
  const draft = createReportDraft(stored, {
    save(value, options) {
      if (control.rejectSave) return Promise.reject(new Error('Save rejected by host'))
      if (control.conflictSave || options.expectedRevision !== stored.revision) {
        const latest = { ...structuredClone(stored), revision: String(Number(stored.revision) + 1), title: 'Sales board (edited elsewhere)' }
        stored = latest
        control.conflictSave = false
        return Promise.reject(new ReportRevisionConflictError(structuredClone(latest)))
      }
      stored = { ...structuredClone(value), revision: String(Number(stored.revision) + 1) }
      return Promise.resolve(structuredClone(stored))
    },
  })
  props.setup?.(draft)
  const guard = createReportLeaveGuard(draft)
  const [log, setLog] = createSignal('')
  const [proposal, setProposal] = createSignal(props.proposal?.(draft))
  onMount(() => props.after?.())
  if (props.guard) onMount(() => void guard.request().then((leave) => setLog(leave ? 'Navigation allowed' : 'Navigation cancelled')))

  return (
    <div class="lens-root" data-theme={props.theme}>
      <div class="lens-panel" style={{ padding: '16px' }}>
        <ReportCompositionEditor
          catalog={memoryCatalog()}
          draft={draft}
          guard={guard}
          lifecycle={props.lifecycle ?? 'existing'}
          onCopyReport={(id) => { setLog(`Host copies the report of ${id}`) }}
          onEditReport={(id) => { setLog(`Host opens ${id} for editing`) }}
          onSaveCopy={() => { setLog('Host saves an independent copy') }}
          proposal={proposal()}
          proposalHandlers={{
            settled: (id, outcome) => setLog(`Proposal ${id} ${outcome}`),
            regenerate: () => setProposal(addFinanceProposal(draft)),
            openAsDraft: () => setLog('Host opens the proposal as a separate draft'),
          }}
          renderPlacement={(placement) => <Body placement={placement} />}
          reportTitle={reportTitle}
          theme={props.theme}
          usage={(id) => (id === 'report-01' ? 4 : 1)}
        />
        <Show when={log()}><p data-testid="editor-host-log" role="status">{log()}</p></Show>
        <Show when={props.control}>
          <div class="lens:mt-4 lens:flex lens:flex-wrap lens:gap-3 lens:text-base">
            <label><input onChange={(event) => { control.rejectSave = event.currentTarget.checked }} type="checkbox" /> Reject next save</label>
            <label><input onChange={(event) => { control.conflictSave = event.currentTarget.checked }} type="checkbox" /> Conflict on next save</label>
            <button class="lens-context-action" onClick={() => void guard.request().then((leave) => setLog(leave ? 'Navigation allowed' : 'Navigation cancelled'))} type="button">Navigate away</button>
            <button class="lens-context-action" onClick={() => setProposal(addFinanceProposal(draft))} type="button">Receive proposal</button>
          </div>
        </Show>
      </div>
    </div>
  )
}

/** A synthetic external proposal: drop one card, retitle another and add a report, prepared against the current draft. */
function addFinanceProposal(draft: ReportDraft<Composition>): ReportProposal {
  const composition = draft.value()
  const rows = composition.rows.map((row) => row.filter((placement) => placement.id !== 'finance')
    .map((placement) => (placement.id === 'revenue' ? { ...placement, title: 'Revenue this year' } : placement)))
  rows.push([{ id: 'proposal-claims', reportId: 'report-07' }])
  return { id: `proposal-${draft.sequence()}`, base: reportDraftToken(draft), summary: 'Focus the board on revenue and claims.', composition: { ...composition, rows: rows.filter((row) => row.length) } }
}

const rename = (draft: ReportDraft<Composition>) => draft.update((value) => {
  value.rows[0]![0]!.title = 'Revenue, this quarter'
  return value
})
const click = (selector: string) => () => when(selector, (element) => element.click())

export const ExistingLight = () => <Editor theme="light" />
export const ExistingDark = () => <Editor theme="dark" />
export const NewEmptyLight = () => <Editor theme="light" lifecycle="new" initial={{ id: 'new-board', revision: '1', title: 'New dashboard', rows: [] }} />
export const NewEmptyDark = () => <Editor theme="dark" lifecycle="new" initial={{ id: 'new-board', revision: '1', title: 'New dashboard', rows: [] }} />
export const DirtyLight = () => <Editor theme="light" setup={rename} />
export const DirtyDark = () => <Editor theme="dark" setup={rename} />
export const SaveFailedLight = () => <Editor theme="light" setup={rename} control={{ rejectSave: true, conflictSave: false }} after={click('.lens-report-editor-actions .lens-report-primary')} />
export const SaveFailedDark = () => <Editor theme="dark" setup={rename} control={{ rejectSave: true, conflictSave: false }} after={click('.lens-report-editor-actions .lens-report-primary')} />
export const ConflictLight = () => <Editor theme="light" setup={rename} control={{ rejectSave: false, conflictSave: true }} after={click('.lens-report-editor-actions .lens-report-primary')} />
export const ConflictDark = () => <Editor theme="dark" setup={rename} control={{ rejectSave: false, conflictSave: true }} after={click('.lens-report-editor-actions .lens-report-primary')} />
export const LeaveGuardLight = () => <Editor theme="light" setup={rename} guard />
export const LeaveGuardDark = () => <Editor theme="dark" setup={rename} guard />
export const ConfigureCardLight = () => <Editor theme="light" after={click('[data-placement-editor="losses"] .lens-report-editor-card-actions button')} />
export const ConfigureCardDark = () => <Editor theme="dark" after={click('[data-placement-editor="losses"] .lens-report-editor-card-actions button')} />
export const SharedEditImpactLight = () => <Editor theme="light" after={click('[data-placement-editor="revenue"] .lens-report-editor-card-actions button:nth-child(2)')} />
export const SharedEditImpactDark = () => <Editor theme="dark" after={click('[data-placement-editor="revenue"] .lens-report-editor-card-actions button:nth-child(2)')} />
export const ProposalLight = () => <Editor theme="light" proposal={addFinanceProposal} />
export const ProposalDark = () => <Editor theme="dark" proposal={addFinanceProposal} />
export const ProposalStaleLight = () => <Editor theme="light" proposal={(draft) => ({ ...addFinanceProposal(draft), base: 'outdated' })} />
export const ProposalStaleDark = () => <Editor theme="dark" proposal={(draft) => ({ ...addFinanceProposal(draft), base: 'outdated' })} />
/** Browser walkthrough: every host outcome is controllable from the page. */
export const WalkthroughLight = () => <Editor theme="light" control={{ rejectSave: false, conflictSave: false }} />
export const WalkthroughDark = () => <Editor theme="dark" control={{ rejectSave: false, conflictSave: false }} />
