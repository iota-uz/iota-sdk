import { createSignal, For, onMount, Show, type JSX } from 'solid-js'
import type { LensThemeMode } from './runtime'
import { createReportDraft } from './reports/draft'
import { appendReportPicks, type ReportCatalogHost, type ReportDestinationConfirmation } from './reports/picker'
import { ReportDestinationPicker } from './reports/ReportDestinationPicker'
import { ReportPicker } from './reports/ReportPicker'
import { catalogEntries, catalogTarget, memoryCatalog, memoryDestinations } from './stories/reportCatalog'

/** Story-only autoplay: waits for an element the scenario needs, then acts on it. */
function when(selector: string, act: (element: HTMLElement) => void, frames = 120): void {
  const element = globalThis.document.querySelector<HTMLElement>(selector)
  if (element) act(element)
  else if (frames > 0) requestAnimationFrame(() => when(selector, act, frames - 1))
}

const target = { id: 'sales-board', title: 'Sales board' }

function Picker(props: {
  theme: LensThemeMode
  host?: ReportCatalogHost
  initialSearch?: string
  confirm?: () => Promise<void>
  after?: () => void
}): JSX.Element {
  const host = props.host ?? memoryCatalog()
  onMount(() => props.after?.())
  return (
    <div class="lens-root" data-theme={props.theme}>
      <ReportPicker
        host={host}
        initialSearch={props.initialSearch}
        onClose={() => undefined}
        onConfirm={props.confirm ?? (() => Promise.resolve())}
        target={target}
        theme={props.theme}
      />
    </div>
  )
}

const select = (...ids: string[]) => () => {
  for (const id of ids) when(`[data-report-id="${id}"] input`, (input) => input.click())
}
const selectWithAnother = () => {
  select('report-03', 'report-02')()
  when('[data-report-id="report-01"] button[aria-pressed]', (button) => button.click())
}
const preview = () => when('[data-report-id="report-01"] button[aria-label^="Preview"]', (button) => button.click())
const addFailing = () => {
  select('report-02')()
  when('[data-report-id="report-02"][data-selected]', () => when('[data-testid="report-picker-confirm"]:not([disabled])', (button) => button.click()))
}

export const Light = () => <Picker theme="light" />
export const Dark = () => <Picker theme="dark" />
export const SelectedLight = () => <Picker theme="light" after={selectWithAnother} />
export const SelectedDark = () => <Picker theme="dark" after={selectWithAnother} />
export const PreviewLight = () => <Picker theme="light" after={preview} />
export const PreviewDark = () => <Picker theme="dark" after={preview} />
export const LoadingLight = () => <Picker theme="light" host={memoryCatalog({ pending: true })} />
export const LoadingDark = () => <Picker theme="dark" host={memoryCatalog({ pending: true })} />
export const EmptyLight = () => <Picker theme="light" host={memoryCatalog({ entries: [] })} />
export const EmptyDark = () => <Picker theme="dark" host={memoryCatalog({ entries: [] })} />
export const NoMatchesLight = () => <Picker theme="light" initialSearch="Nonexistent" />
export const NoMatchesDark = () => <Picker theme="dark" initialSearch="Nonexistent" />
export const LoadFailedLight = () => <Picker theme="light" host={memoryCatalog({ failures: 1 })} />
export const LoadFailedDark = () => <Picker theme="dark" host={memoryCatalog({ failures: 1 })} />
export const AddFailedLight = () => <Picker theme="light" after={addFailing} confirm={() => Promise.reject(new Error('Draft rejected'))} />
export const AddFailedDark = () => <Picker theme="dark" after={addFailing} confirm={() => Promise.reject(new Error('Draft rejected'))} />

function Destination(props: { theme: LensThemeMode }): JSX.Element {
  const host = memoryDestinations()
  return (
    <div class="lens-root" data-theme={props.theme}>
      <ReportDestinationPicker
        host={host}
        onClose={() => undefined}
        onConfirm={() => Promise.resolve()}
        report={{ id: 'report-01', title: 'Revenue' }}
        theme={props.theme}
      />
    </div>
  )
}

export const DestinationLight = () => <Destination theme="light" />
export const DestinationDark = () => <Destination theme="dark" />

/**
 * A synthetic editor page: the picker adds to a draft only on confirmation,
 * and the "Add to dashboard" dialog hands its choice to the host. The page is
 * taller than the viewport so a restored scroll position is observable.
 */
function InDraft(props: { theme: LensThemeMode }): JSX.Element {
  const titles = new Map(catalogEntries().map((entry) => [entry.id, entry.title]))
  const draft = createReportDraft(catalogTarget(), { save: (value) => Promise.resolve({ ...value, revision: String(Number(value.revision) + 1) }) })
  const host = memoryCatalog()
  const destinations = memoryDestinations({ failures: 1 })
  const [picking, setPicking] = createSignal(false)
  const [adding, setAdding] = createSignal(false)
  const [rejectNext, setRejectNext] = createSignal(false)
  const [log, setLog] = createSignal('')
  const placed = (reportId: string) => draft.value().rows.flat().filter((placement) => placement.reportId === reportId).length
  let trigger: HTMLButtonElement | undefined
  let addTrigger: HTMLButtonElement | undefined

  const confirm = (confirmation: Parameters<typeof appendReportPicks>[1]) => {
    if (rejectNext()) {
      setRejectNext(false)
      return Promise.reject(new Error('Draft rejected'))
    }
    const { added } = appendReportPicks(draft.value(), confirmation)
    draft.update((value) => appendReportPicks(value, confirmation).composition)
    setLog(`Added ${added.length} to the draft`)
    return Promise.resolve()
  }
  const destination = (confirmation: ReportDestinationConfirmation) => {
    setLog(confirmation.choice.kind === 'new' ? 'Host creates a new dashboard' : `Host adds to ${confirmation.choice.destination.title}`)
    return Promise.resolve()
  }

  return (
    <div class="lens-root" data-theme={props.theme}>
      <div class="lens-panel" style={{ padding: '16px', 'min-height': '1600px' }}>
        <p data-testid="draft-status" role="status">{draft.dirty() ? 'Unsaved changes' : 'Saved'} · {draft.value().rows.flat().length} cards</p>
        <ol data-testid="draft-cards">
          <For each={draft.value().rows.flat()}>{(placement) => <li>{titles.get(placement.reportId) ?? placement.reportId}</li>}</For>
        </ol>
        <label><input checked={rejectNext()} onChange={(event) => setRejectNext(event.currentTarget.checked)} type="checkbox" /> Reject next add</label>
        <div style={{ 'margin-top': '1200px' }}>
          <button class="lens-context-action" onClick={() => setPicking(true)} ref={(element) => { trigger = element }} type="button">Add reports</button>
          <button class="lens-context-action" onClick={() => setAdding(true)} ref={(element) => { addTrigger = element }} type="button">Add Revenue to a dashboard</button>
          <button class="lens-context-action" onClick={() => draft.cancel()} type="button">Cancel draft</button>
        </div>
        <Show when={log()}><p data-testid="draft-log">{log()}</p></Show>
      </div>
      <Show when={picking()}>
        <ReportPicker host={host} onClose={() => setPicking(false)} onConfirm={confirm} placed={placed} restoreFocus={trigger} target={target} theme={props.theme} />
      </Show>
      <Show when={adding()}>
        <ReportDestinationPicker host={destinations} onClose={() => setAdding(false)} onConfirm={destination} report={{ id: 'report-01', title: 'Revenue' }} restoreFocus={addTrigger} theme={props.theme} />
      </Show>
    </div>
  )
}

export const InDraftLight = () => <InDraft theme="light" />
export const InDraftDark = () => <InDraft theme="dark" />
