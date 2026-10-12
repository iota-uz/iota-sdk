import { createMemo, createSignal, Show } from 'solid-js'
import { LensDashboard } from './LensDashboard'
import { parseDocument } from './contract'
import { createReportDraft, parseReportComposition } from './reports/draft'
import fixture from '../fixtures/reusable-reports.json'
import type { LensThemeMode } from './runtime'

function Example(props: { theme: LensThemeMode }) {
  const [rejectSave, setRejectSave] = createSignal(false)
  let revision = 1
  const draft = createReportDraft(parseReportComposition(fixture.composition), {
    save(value) {
      return rejectSave() ? Promise.reject(new Error('Save rejected by host')) : Promise.resolve({ ...value, revision: String(++revision) })
    },
  })
  const document = createMemo(() => {
    const doc = parseDocument(structuredClone(fixture.document))
    const first = draft.value().rows[0]![0]!
    for (const row of doc.layout.rows) {
      for (const item of row.panels) {
        for (const group of item.groups ?? []) {
          if (group.id === first.id) group.label = first.title ?? fixture.definition.title
        }
      }
    }
    return doc
  })
  return <div class="lens-root" data-theme={props.theme}>
    <div class="lens-panel">
      <label>First placement caption <input aria-label="First placement caption" value={draft.value().rows[0]![0]!.title ?? ''} onInput={(event) => draft.update((value) => {
        value.rows[0]![0]!.title = event.currentTarget.value
        return value
      })} /></label>
      <label><input type="checkbox" checked={rejectSave()} onChange={(event) => setRejectSave(event.currentTarget.checked)} /> Reject next save</label>
      <button type="button" onClick={() => void draft.save().catch(() => undefined)}>Save draft</button>
      <button type="button" onClick={() => draft.cancel()}>Cancel</button>
      <p role="status">{draft.dirty() ? 'Unsaved changes' : 'Saved'} · revision {draft.saved().revision}</p>
      <Show when={Boolean(draft.error())}><p role="alert">Save failed. Your changes are retained.</p></Show>
    </div>
    <LensDashboard initialDocument={document()} theme={props.theme} />
  </div>
}

export const Light = () => <Example theme="light" />
export const Dark = () => <Example theme="dark" />
