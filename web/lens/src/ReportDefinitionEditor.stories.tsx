import { createSignal, For, onMount, Show, type JSX } from 'solid-js'
import type { LensThemeMode } from './runtime'
import type { ContextFilters, Definition, Parameters } from './reports/contract/types'
import { createReportDraft, parseReportDefinition, ReportRevisionConflictError, type ReportDraft } from './reports/draft'
import { createReportLeaveGuard } from './reports/guard'
import { ReportDefinitionEditor, type ReportEditContext } from './reports/ReportDefinitionEditor'

/** The shared report every story edits: synthetic revenue by channel, top five rows by default. */
function revenueReport(): Definition {
  return parseReportDefinition({
    id: 'report-01', revision: '7', title: 'Revenue by channel', view: 'table',
    source: { id: 'synthetic.revenue', version: 2 },
    filters: { period: { mode: 'relative', months: 12 } },
    parameters: { top: 5, measure: 'revenue' },
  })
}

const salesBoard: ReportEditContext = {
  origin: { scope: 'dashboard', id: 'sales-board', title: 'Sales board' },
  filters: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' }, constraints: [{ dimension: 'region', values: ['North', 'East'] }] },
}

const channels = [
  ['Direct', 412], ['Agents', 388], ['Partners', 251], ['Online', 197], ['Brokers', 143], ['Bank desks', 96], ['Phone', 61], ['Other', 22],
] as const

function Preview(props: { definition: Definition; filters: ContextFilters | undefined }): JSX.Element {
  const top = () => Number(props.definition.parameters?.top ?? 5)
  const scale = () => (props.filters?.constraints?.length ? 0.4 : 1)
  return (
    <table class="lens:w-full lens:max-w-xl lens:border-collapse lens:text-md" data-testid="definition-preview-table">
      <caption class="lens:pb-2 lens:text-left lens:text-muted">
        {props.definition.title} · {props.filters?.period?.mode === 'fixed' ? `${props.filters.period.start} – ${props.filters.period.end}` : 'Last 12 months'}
      </caption>
      <thead><tr><th class="lens:text-left">Channel</th><th class="lens:text-right">{props.definition.parameters?.measure === 'policies' ? 'Policies' : 'Revenue, mln'}</th></tr></thead>
      <tbody>
        <For each={channels.slice(0, top())}>
          {([name, value]) => <tr><td>{name}</td><td class="lens:text-right">{Math.round(value * scale())}</td></tr>}
        </For>
      </tbody>
    </table>
  )
}

function ParameterControls(props: { value: Parameters | undefined; onChange: (next: Parameters) => void }): JSX.Element {
  return (
    <div class="lens-report-form lens:flex lens:flex-wrap lens:gap-4" style={{ padding: '0', 'flex-direction': 'row' }}>
      <label class="lens-report-form-field">
        <span>Rows shown</span>
        <select onChange={(event) => props.onChange({ ...props.value, top: Number(event.currentTarget.value) })} value={String(Number(props.value?.top ?? 5))}>
          <For each={[3, 5, 8]}>{(value) => <option value={String(value)}>{value}</option>}</For>
        </select>
      </label>
      <label class="lens-report-form-field">
        <span>Measure</span>
        <select onChange={(event) => props.onChange({ ...props.value, measure: event.currentTarget.value })} value={props.value?.measure === 'policies' ? 'policies' : 'revenue'}>
          <option value="revenue">Revenue</option>
          <option value="policies">Policies</option>
        </select>
      </label>
    </div>
  )
}

interface StoryControl {
  rejectSave: boolean
  conflictSave: boolean
  rejectCopy: boolean
}

/** Story-only autoplay: waits for an element the scenario needs, then acts on it. */
function when(selector: string, act: (element: HTMLElement) => void, frames = 120): void {
  const element = globalThis.document.querySelector<HTMLElement>(selector)
  if (element) act(element)
  else if (frames > 0) requestAnimationFrame(() => when(selector, act, frames - 1))
}

function Editor(props: {
  theme: LensThemeMode
  context?: boolean
  setup?: (draft: ReportDraft<Definition>) => void
  after?: () => void
  control?: StoryControl
}): JSX.Element {
  const control = props.control ?? { rejectSave: false, conflictSave: false, rejectCopy: false }
  let stored = revenueReport()
  const draft = createReportDraft(stored, {
    save(value, options) {
      if (control.rejectSave) return Promise.reject(new Error('Save rejected by host'))
      if (control.conflictSave || options.expectedRevision !== stored.revision) {
        stored = { ...structuredClone(stored), revision: String(Number(stored.revision) + 1), title: 'Revenue by sales channel' }
        control.conflictSave = false
        return Promise.reject(new ReportRevisionConflictError(structuredClone(stored)))
      }
      stored = { ...structuredClone(value), revision: String(Number(stored.revision) + 1) }
      return Promise.resolve(structuredClone(stored))
    },
  })
  props.setup?.(draft)
  const guard = createReportLeaveGuard(draft)
  const [log, setLog] = createSignal('')
  onMount(() => props.after?.())

  return (
    <div class="lens-root" data-theme={props.theme}>
      <div class="lens-panel" style={{ padding: '16px' }}>
        <ReportDefinitionEditor
          context={props.context === false ? undefined : salesBoard}
          dimensionLabel={(dimension) => (dimension === 'region' ? 'Region' : dimension)}
          draft={draft}
          guard={guard}
          onCopy={() => {
            if (control.rejectCopy) return Promise.reject(new Error('Copy rejected'))
            setLog('Host saves an independent copy')
          }}
          onSaveSlice={(_definition, filters) => { setLog(`Host saves a new report filtered by ${filters.constraints?.map((item) => item.values.join('+')).join(', ')}`) }}
          placements={() => [
            { id: 'revenue', title: 'Sales board · Revenue', current: true },
            { id: 'exec-revenue', title: 'Executive summary · Revenue' },
            { id: 'region-revenue', title: 'Regional review · Revenue by region' },
          ]}
          renderParameters={(parameters) => <ParameterControls {...parameters} />}
          renderPreview={(definition, filters) => <Preview definition={definition} filters={filters} />}
          theme={props.theme}
        />
        <Show when={log()}><p data-testid="definition-host-log" role="status">{log()}</p></Show>
        <Show when={props.control}>
          <div class="lens:mt-4 lens:flex lens:flex-wrap lens:gap-3 lens:text-base">
            <label><input onChange={(event) => { control.rejectSave = event.currentTarget.checked }} type="checkbox" /> Reject next save</label>
            <label><input onChange={(event) => { control.conflictSave = event.currentTarget.checked }} type="checkbox" /> Conflict on next save</label>
            <label><input onChange={(event) => { control.rejectCopy = event.currentTarget.checked }} type="checkbox" /> Reject copy</label>
            <button class="lens-context-action" onClick={() => void guard.request().then((leave) => setLog(leave ? 'Navigation allowed' : 'Navigation cancelled'))} type="button">Navigate away</button>
          </div>
        </Show>
      </div>
    </div>
  )
}

const showMore = (draft: ReportDraft<Definition>) => draft.update((value) => ({ ...value, parameters: { ...value.parameters, top: 8 } }))
const click = (selector: string) => () => when(selector, (element) => element.click())

/** Entered from a dashboard: its filters drive the preview and are shown with their origin. */
export const ContextLight = () => <Editor theme="light" />
export const ContextDark = () => <Editor theme="dark" />
/** Opened on its own: only the report's defaults. */
export const StandaloneLight = () => <Editor context={false} theme="light" />
export const StandaloneDark = () => <Editor context={false} theme="dark" />
/** A parameter changed: the preview follows it, marked as not saved. */
export const PreviewUnsavedLight = () => <Editor setup={showMore} theme="light" />
export const PreviewUnsavedDark = () => <Editor setup={showMore} theme="dark" />
export const ConflictLight = () => <Editor after={click('[data-testid="report-definition-save"]')} control={{ rejectSave: false, conflictSave: true, rejectCopy: false }} setup={showMore} theme="light" />
export const ConflictDark = () => <Editor after={click('[data-testid="report-definition-save"]')} control={{ rejectSave: false, conflictSave: true, rejectCopy: false }} setup={showMore} theme="dark" />
/** Browser walkthrough: every host outcome is controllable from the page. */
export const WalkthroughLight = () => <Editor control={{ rejectSave: false, conflictSave: false, rejectCopy: false }} theme="light" />
