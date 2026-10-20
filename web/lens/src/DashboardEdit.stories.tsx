import { createSignal, onMount, Show, type JSX } from 'solid-js'
import scopedFixtures from '../fixtures/scoped-filters.json'
import { parseDocument, type DashboardDocument, type Filter } from './contract'
import { LensDashboard, type LensDashboardProps } from './LensDashboard'
import type { LensThemeMode } from './runtime'
import type { Composition } from './reports/contract/types'
import { createReportDraft, ReportRevisionConflictError, type ReportDraft } from './reports/draft'
import { createReportLeaveGuard } from './reports/guard'
import { movePlacement, resizePlacement } from './reports/layout'
import type { ReportPlacementAction, ReportPlacementHost } from './reports/placementHost'
import { editComposition, editDocument, editReportTitles, emptyComposition, emptyDocument } from './stories/dashboardEdit'
import { memoryCatalog } from './stories/reportCatalog'

// The report-card fixture's composed board, edited in place: the cards keep
// their live figures while the layout comes from the host's draft.

interface StoryControl {
  conflictSave: boolean
}

function placementHost(record: (message: string) => void): ReportPlacementHost {
  const action = (name: ReportPlacementAction) => (id: string) => record(`${name} ${id}`)
  return {
    capabilities: (id) => id === 'legacy' ? { drill: false, save: false, export: true } : { drill: true, save: true, export: true },
    actions: { open: action('open'), edit: action('edit'), configure: action('configure'), copy: action('copy'), remove: action('remove') },
  }
}

function Board(props: {
  theme: LensThemeMode
  editing?: boolean
  lifecycle?: 'new' | 'existing'
  composition?: Composition
  document?: DashboardDocument
  setup?: (draft: ReportDraft<Composition>) => void
  after?: (draft: ReportDraft<Composition>) => void
  control?: StoryControl
  chrome?: LensDashboardProps['chrome']
}): JSX.Element {
  const control = props.control ?? { conflictSave: false }
  let stored = structuredClone(props.composition ?? editComposition())
  const draft = createReportDraft(stored, {
    save(value, options) {
      if (control.conflictSave || options.expectedRevision !== stored.revision) {
        stored = { ...structuredClone(stored), revision: String(Number(stored.revision) + 1) }
        control.conflictSave = false
        return Promise.reject(new ReportRevisionConflictError(structuredClone(stored)))
      }
      stored = { ...structuredClone(value), revision: String(Number(stored.revision) + 1) }
      return Promise.resolve(structuredClone(stored))
    },
  })
  props.setup?.(draft)
  const guard = createReportLeaveGuard(draft, { beforeUnload: false })
  const [editing, setEditing] = createSignal(props.editing ?? false)
  const [log, setLog] = createSignal('')
  onMount(() => props.after?.(draft))
  return (
    <div class="lens-root" data-theme={props.theme}>
      <LensDashboard
        chrome={props.chrome}
        edit={{
          draft,
          guard,
          editing,
          setEditing,
          lifecycle: props.lifecycle ?? 'existing',
          catalog: memoryCatalog(),
          reportTitle: (id) => editReportTitles[id],
          usage: (id) => (id === 'report-revenue' ? 3 : 1),
          onEditReport: (id) => { setLog(`Host opens ${id} for editing`) },
          onCopyReport: (id) => { setLog(`Host copies the report of ${id}`) },
          onSaveCopy: () => { setLog('Host saves an independent copy') },
          onSaved: (_composition, intent) => setLog(`Saved (${intent})`),
          onCancel: () => setLog('Editing cancelled'),
        }}
        initialDocument={props.document ?? editDocument()}
        reportPlacementHost={placementHost(setLog)}
        theme={props.theme}
      />
      <Show when={log()}><p data-testid="dashboard-edit-host-log" role="status">{log()}</p></Show>
    </div>
  )
}

const rearrange = (draft: ReportDraft<Composition>) => draft.update((value) => resizePlacement(movePlacement(value, 'forbidden', 'up'), 'revenue', 8))
const saveNow = (draft: ReportDraft<Composition>) => { void draft.save('draft').catch(() => undefined) }

const compareFilter: Filter = {
  id: 'compare', kind: 'compare', label: 'Compare with',
  compare: { modeParam: 'compare', startParam: 'compare_start', endParam: 'compare_end', compareTo: 'period', value: { mode: 'year_ago' } },
}
const scopedBoard = () => parseDocument({ ...structuredClone(scopedFixtures[1]!.document), filters: [compareFilter] })

/** Read-only view of an edit-capable board: the Edit action sits with the dashboard actions. */
export const ViewLight = () => <Board theme="light" />
export const ViewDark = () => <Board theme="dark" />
/** Edit mode over the live cards: handles, width sliders and the edit bar. */
export const EditLight = () => <Board editing theme="light" />
export const EditDark = () => <Board editing theme="dark" />
/** A draft dashboard offers Save draft and Publish. */
export const EditDraftLight = () => <Board editing lifecycle="new" theme="light" />
export const EditDraftDark = () => <Board editing lifecycle="new" theme="dark" />
/** After a move and a resize: the cards re-render from the draft, the bar reports unsaved changes. */
export const DirtyLight = () => <Board editing setup={rearrange} theme="light" />
export const DirtyDark = () => <Board editing setup={rearrange} theme="dark" />
/** Someone else saved first: the reader's layout stays, with the conflict choices above it. */
export const ConflictLight = () => <Board after={saveNow} control={{ conflictSave: true }} editing setup={rearrange} theme="light" />
export const ConflictDark = () => <Board after={saveNow} control={{ conflictSave: true }} editing setup={rearrange} theme="dark" />
/** An empty editable board explains itself and offers Add report. */
export const EmptyLight = () => <Board composition={emptyComposition()} document={emptyDocument()} theme="light" />
export const EmptyDark = () => <Board composition={emptyComposition()} document={emptyDocument()} theme="dark" />
export const EmptyEditingLight = () => <Board composition={emptyComposition()} document={emptyDocument()} editing theme="light" />
export const EmptyEditingDark = () => <Board composition={emptyComposition()} document={emptyDocument()} editing theme="dark" />
/** Story-only autoplay: waits for an element the scenario needs, then clicks it. */
function clickWhenReady(selector: string, frames = 120): void {
  const element = globalThis.document.querySelector<HTMLElement>(selector)
  if (element && !element.hidden) element.click()
  else if (frames > 0) requestAnimationFrame(() => clickWhenReady(selector, frames - 1))
}

function HostChrome(props: { theme: LensThemeMode; open?: boolean }): JSX.Element {
  onMount(() => { if (props.open) clickWhenReady('[data-testid="toolbar-overflow"]') })
  return (
    <div class="lens-root" data-theme={props.theme}>
      <LensDashboard chrome={{ title: false, description: false, toolbar: 'compact' }} initialDocument={scopedBoard()} theme={props.theme} />
    </div>
  )
}

/** A host page with its own header: no Lens title, and the secondary filter verbs folded into More. */
export const HostChromeLight = () => <HostChrome theme="light" />
export const HostChromeDark = () => <HostChrome theme="dark" />
export const HostChromeMoreOpenLight = () => <HostChrome open theme="light" />
export const HostChromeMoreOpenDark = () => <HostChrome open theme="dark" />
