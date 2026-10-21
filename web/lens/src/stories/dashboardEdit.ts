import fixture from '../../fixtures/report-card.json'
import { parseDocument, type DashboardDocument } from '../contract'
import type { Composition } from '../reports/contract/types'
import { parseReportComposition } from '../reports/draft'

// The composition behind fixtures/report-card.json's embedded board: the
// placement ids are the document's report groups.
export const editReportTitles: Record<string, string> = {
  'report-revenue': 'Revenue',
  'report-legacy': 'Legacy chart',
  'report-archived': 'Archived forecast',
  'report-hidden': 'Hidden report',
}

export function editComposition(): Composition {
  return parseReportComposition({
    id: 'card-dashboard', revision: '1', title: 'Card dashboard', rows: [
      [{ id: 'revenue', reportId: 'report-revenue' }],
      [{ id: 'legacy', reportId: 'report-legacy', span: 4 }, { id: 'deleted', reportId: 'report-archived', span: 4 }, { id: 'forbidden', reportId: 'report-hidden', span: 4 }],
    ],
  })
}

export function emptyComposition(): Composition {
  return { ...editComposition(), rows: [] }
}

export const editDocument = (): DashboardDocument => parseDocument(structuredClone(fixture.embedded))

export function emptyDocument(): DashboardDocument {
  const document = editDocument()
  return { ...document, panels: [], frames: {}, layout: { ...document.layout, rows: [] } }
}

/**
 * The same board with every panel deferred behind the panel endpoint. The
 * fetcher answers at once and records each batch, so a test can prove that a
 * layout change requests nothing.
 */
export function deferredEditDocument(): { document: DashboardDocument; fetcher: typeof fetch; requests: string[][] } {
  const document = parseDocument(structuredClone(fixture.progressive))
  const frames = fixture.embedded.frames as Record<string, unknown>
  const requests: string[][] = []
  const fetcher: typeof fetch = (_input, init) => {
    const body = JSON.parse(init?.body as string) as { panels: Array<{ panelId: string }> }
    requests.push(body.panels.map(({ panelId }) => panelId))
    const events: unknown[] = body.panels.map(({ panelId }) => {
      const panel = document.panels.find((candidate) => candidate.id === panelId)!
      return { panelId, result: { frames: { [panel.frame]: frames[panel.frame] }, calculation: { durationMs: 12, cacheHit: false, calculatedAt: '2026-10-12T06:00:00Z' } } }
    })
    events.push({ complete: true })
    return Promise.resolve(new Response(`${events.map((event) => JSON.stringify(event)).join('\n')}\n`, { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } }))
  }
  return { document, fetcher, requests }
}
