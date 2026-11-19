import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/investigation.json'
import { parseDocument, type DashboardDocument, type Node } from '../contract'
import {
  investigationProposal, investigationSaveability, nodeValue, positionProvenance, suggestedInvestigationTitle,
  type InvestigationHost, type InvestigationSlice,
} from './investigation'
import { SaveInvestigation } from './SaveInvestigation'

const document = parseDocument(fixture.document)
const panel = document.panels[0]!
const regions = 'margin::margin/by-region/composition/regions'
const regionsPath = ['margin::margin', 'margin::margin/by-region', 'margin::margin/by-region/composition', regions]
const northPath = [...regionsPath, `${regions}/north`]
type Caps = { save: boolean; drill: boolean; export: boolean; dimensions?: Array<string> }
const capabilities: Caps = fixture.source.capabilities
const label = (node: Node) => ( node.label || (node.target ? document.drill.edges[node.target]?.label : '') || nodeValue(node))

function saveable(doc: DashboardDocument = document, path = northPath, caps: Caps | null = capabilities) {
  return investigationSaveability({ document: doc, panel, navigation: { panelId: panel.id, path }, capabilities: caps ?? undefined, labelFor: label })
}

function slice(): InvestigationSlice {
  const result = saveable()
  if (!result.saveable) throw new Error(result.reason)
  return result.slice
}

afterEach(cleanup)

describe('investigation slice', () => {
  it('carries the placement context and one constraint per drill step, each with its origin', () => {
    const value = slice()
    expect(value.placementId).toBe('margin')
    expect(value.source).toMatchObject({ id: 'margin', version: 1, view: 'summary' })
    expect(value.period).toMatchObject({ start: '2026-06-01', end: '2026-06-30', origin: { scope: 'dashboard' } })
    expect(value.constraints.map(({ dimension, values, origin }) => [dimension, values, origin.scope])).toEqual([
      ['segment', ['retail'], 'dashboard'],
      ['region', ['north'], 'drill'],
    ])
    expect(suggestedInvestigationTitle(value)).toBe('Branch margin — North')
  })

  it('proposes a definition of source, view and filters — never a snapshot — and leaves the parent untouched', () => {
    const before = JSON.stringify(document)
    const proposal = investigationProposal(slice(), { title: '  North margin ', dashboardId: document.meta.dashboardId })
    expect(proposal).toMatchObject({
      title: 'North margin',
      source: { id: 'margin', version: 1 },
      view: 'summary',
      filters: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' }, constraints: [{ dimension: 'segment', values: ['retail'] }, { dimension: 'region', values: ['north'] }] },
      origin: { placementId: 'margin', path: northPath },
    })
    expect(JSON.stringify(proposal)).not.toContain(document.snapshotId)
    expect(JSON.stringify(document)).toBe(before)
  })

  it('explains each unsupported case instead of offering a broader save', () => {
    expect(saveable(document, northPath, null)).toMatchObject({ saveable: false, reason: 'source-cannot-save' })
    expect(saveable(document, northPath, { ...capabilities, save: false })).toMatchObject({ saveable: false, reason: 'source-cannot-save' })
    expect(saveable(document, northPath, { ...capabilities, dimensions: ['segment'] })).toMatchObject({ saveable: false, reason: 'dimension-unsupported', detail: 'region' })

    const edited = (edit: (doc: DashboardDocument) => void) => {
      const doc = structuredClone(document)
      edit(doc)
      return doc
    }
    const context = (doc: DashboardDocument) => doc.layout.rows[0]!.panels[0]!.groups![0]!.filterContext!
    expect(saveable(edited((doc) => { delete doc.drill.edges[regions]!.dimension }))).toMatchObject({ saveable: false, reason: 'level-undeclared', detail: 'Regions' })
    expect(saveable(edited((doc) => { context(doc).intersection.push({ dimension: 'region', values: ['south'] }) }))).toMatchObject({ saveable: false, reason: 'drill-conflict', detail: 'region' })
    expect(saveable(edited((doc) => { context(doc).conflicts.push('segment') }))).toMatchObject({ saveable: false, reason: 'context-blocked' })
    expect(saveable(edited((doc) => { delete doc.layout.rows[0]!.panels[0]!.groups }))).toMatchObject({ saveable: false, reason: 'not-a-report' })
  })

  it('states the provenance of a position that cannot be saved', () => {
    const provenance = positionProvenance(document, panel, { panelId: panel.id, path: northPath }, label)
    expect(provenance.context?.period.origin.scope).toBe('dashboard')
    expect(provenance.constraints.map(({ labels }) => labels)).toEqual([['Retail'], ['North']])
  })
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (cause: unknown) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}

describe('SaveInvestigation', () => {
  it('disables every competing action while the host saves and reuses the id when the reader retries', async () => {
    const attempts: Array<ReturnType<typeof deferred<{ reportId: string; title: string }>>> = []
    const save = vi.fn(() => {
      const attempt = deferred<{ reportId: string; title: string }>()
      attempts.push(attempt)
      return attempt.promise
    })
    const host: InvestigationHost = { save }
    const onClose = vi.fn()
    render(() => <SaveInvestigation dashboardId="finance-overview" host={host} onClose={onClose} slice={slice()} />)

    expect(screen.queryByTestId('investigation-publish-add')).toBeNull()
    fireEvent.click(screen.getByTestId('investigation-publish'))
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1))
    for (const id of ['investigation-save-draft', 'investigation-publish']) {
      expect(screen.getByTestId(id).hasAttribute('disabled')).toBe(true)
    }
    expect(screen.getByText('Cancel').hasAttribute('disabled')).toBe(true)
    expect(screen.getByTestId('investigation-publish').getAttribute('aria-busy')).toBe('true')
    // A second press while the first is in flight sends nothing.
    fireEvent.click(screen.getByTestId('investigation-publish'))
    expect(save).toHaveBeenCalledTimes(1)

    attempts[0]!.reject(new Error('offline'))
    await screen.findByRole('alert')
    expect(screen.queryByTestId('investigation-result')).toBeNull()
    fireEvent.click(screen.getByTestId('investigation-publish'))
    await waitFor(() => expect(save).toHaveBeenCalledTimes(2))
    const calls = save.mock.calls as unknown as Array<[{ id: string; intent: string }]>
    expect(calls[1]![0].id).toBe(calls[0]![0].id)
    attempts[1]!.resolve({ reportId: 'r-1', title: 'Branch margin — North' })
    expect((await screen.findByTestId('investigation-result')).textContent).toContain('Published')
    expect(screen.queryByTestId('investigation-publish')).toBeNull()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('treats a changed name or intent as a new save', async () => {
    const save = vi.fn().mockRejectedValue(new Error('offline'))
    render(() => <SaveInvestigation dashboardId="finance-overview" host={{ save }} onClose={() => undefined} slice={slice()} />)
    fireEvent.click(screen.getByTestId('investigation-save-draft'))
    await screen.findByRole('alert')
    fireEvent.input(screen.getByTestId('investigation-name'), { target: { value: 'Another name' } })
    fireEvent.click(screen.getByTestId('investigation-save-draft'))
    await waitFor(() => expect(save).toHaveBeenCalledTimes(2))
    const calls = save.mock.calls as unknown as Array<[{ id: string; intent: string; proposal: { title: string } }]>
    expect(calls[1]![0].id).not.toBe(calls[0]![0].id)
    expect(calls[1]![0]).toMatchObject({ intent: 'draft', proposal: { title: 'Another name' } })
  })

  it('needs a name', () => {
    render(() => <SaveInvestigation dashboardId="finance-overview" host={{ save: vi.fn() }} onClose={() => undefined} slice={slice()} />)
    fireEvent.input(screen.getByTestId('investigation-name'), { target: { value: '   ' } })
    expect(screen.getByTestId('investigation-save-draft').hasAttribute('disabled')).toBe(true)
  })
})
