import { createSignal, For, Show } from 'solid-js'
import { LensDashboard } from './LensDashboard'
import { parseDocument } from './contract'
import { decodeFilterBoard } from './reports/filterDraft'
import type { ContextBoard, ContextFilters } from './contract/types'
import type { LensThemeMode } from './runtime'
import fixtures from '../fixtures/scoped-filters.json'

// Responses are precompiled by Go; the story never reimplements context resolution.
function selectionKey(board: ContextBoard) {
  const filters = (value: ContextFilters) => ({
    period: value.period,
    constraints: (value.constraints ?? []).map((constraint) => ({ ...constraint, values: [...constraint.values].sort() })).sort((a, b) => a.dimension.localeCompare(b.dimension)),
  })
  const scopes = (values?: Record<string, ContextFilters>) => Object.entries(values ?? {})
    .filter(([, value]) => value.period || value.constraints?.length)
    .sort(([a], [b]) => a.localeCompare(b)).map(([id, value]) => [id, filters(value)])
  return JSON.stringify({ dashboard: filters(board.dashboard), cards: scopes(board.cards), sections: scopes(board.sections) })
}

function Example(props: { theme: LensThemeMode; scenario: number }) {
  const [scenario, setScenario] = createSignal(props.scenario)
  const [reject, setReject] = createSignal(false)
  const [notice, setNotice] = createSignal('')
  const fetcher: typeof fetch = (input) => {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, window.location.href)
    const value = url.searchParams.get('lens-filters')
    const board = value ? decodeFilterBoard(value) : fixtures[scenario()]!.document.contextEditor.value
    const fixture = fixtures.find((candidate) => selectionKey(candidate.document.contextEditor.value) === selectionKey(board))
    if (!fixture) return Promise.resolve(new Response('This synthetic host only serves the listed precompiled scenarios', { status: 422 }))
    return Promise.resolve(Response.json(fixture.document))
  }
  return <div class="lens-root" data-theme={props.theme}>
    <div class="lens-panel lens-scoped-controls">
      <label>Precompiled host scenario <select value={scenario()} onChange={(event) => setScenario(Number(event.currentTarget.value))}>
        <For each={fixtures}>{(fixture, index) => <option value={index()}>{fixture.name}</option>}</For>
      </select></label>
      <label><input type="checkbox" checked={reject()} onChange={(event) => setReject(event.currentTarget.checked)} /> Reject default save</label>
      <Show when={notice()}><span role="status">{notice()}</span></Show>
    </div>
    <LensDashboard src="/scoped-document" initialDocument={parseDocument(fixtures[scenario()]!.document)} theme={props.theme} fetcher={fetcher}
      filterDefaultsHost={{ canSave: (origin) => origin.scope === 'card', saveDefaults(origin) {
        if (reject()) return Promise.reject(new Error('Host rejected save'))
        setNotice(`Defaults acknowledged for ${origin.id}`)
        return Promise.resolve()
      } }} />
  </div>
}

export const Light = () => <Example theme="light" scenario={2} />
export const Dark = () => <Example theme="dark" scenario={2} />
export const ConflictLight = () => <Example theme="light" scenario={3} />
export const ConflictDark = () => <Example theme="dark" scenario={3} />
export const ReportPeriodsLight = () => <Example theme="light" scenario={0} />
export const ReportPeriodsDark = () => <Example theme="dark" scenario={0} />
