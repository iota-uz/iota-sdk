import { describe, expect, it } from 'vitest'
import type { DashboardDocument, Frame, Level, Panel } from '../contract'
import type { PrintSection } from '../runtime/print'
import { buildChapterFootnotes } from './footnotes'
import type { PrintFigure } from './outline'

const translate = (_key: string, fallback: string, vars?: Record<string, string | number>) => (
  fallback.replace(/\{(\w+)\}/g, (_match, name: string) => String(vars?.[name] ?? ''))
)

function documentWith(): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'snapshot',
    meta: { dashboardId: 'board', title: 'Board', generatedAt: '2026-07-28T00:00:00Z', locale: 'en' },
    layout: { rows: [] },
    panels: [],
    frames: {},
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function figureWith(panel: Partial<Panel>, frame?: Frame): PrintFigure {
  const full: Panel = {
    id: panel.id ?? 'panel',
    kind: 'stat',
    title: 'Panel',
    semantics: 'evidence',
    frame: 'panel:root',
    encoding: { label: 'label', value: 'value' },
    format: {},
    actions: [],
    ...panel,
  }
  const level: Level = { path: [], label: full.title, children: [], perspectives: [] }
  const section: PrintSection = {
    id: `${full.id}:root`,
    document: documentWith(),
    panel: full,
    level,
    ...(frame ? { frame } : {}),
    path: [],
    breadcrumb: [full.title],
    depth: 0,
    root: true,
  }
  return { section, number: '1.1', chart: false, metric: true, width: 'half', kpi: false, breakdown: [] }
}

describe('buildChapterFootnotes', () => {
  it('numbers a panel’s hover-only explanation where it applies', () => {
    const notes = buildChapterFootnotes(
      [figureWith({ id: 'gwp', info: 'Written premium before any cession.' })],
      translate,
    )

    expect(notes.get('gwp:root')).toEqual({
      markers: [1],
      notes: [{ number: 1, text: 'Written premium before any cession.' }],
    })
  })

  it('explains a proxy once for the chapter and marks every figure that carries it', () => {
    const notes = buildChapterFootnotes(
      [
        figureWith({ id: 'ibnr', confidence: 'proxy' }),
        figureWith({ id: 'coverage', confidence: 'proxy' }),
      ],
      translate,
    )

    // The second figure references the first note rather than repeating it.
    expect(notes.get('ibnr:root')?.notes).toHaveLength(1)
    expect(notes.get('coverage:root')).toEqual({ markers: [1], notes: [] })
  })

  it('says nothing about a verified reading', () => {
    const notes = buildChapterFootnotes([figureWith({ id: 'gwp', confidence: 'verified' })], translate)

    expect(notes.size).toBe(0)
  })

  it('explains an unavailable reading rather than leaving a bare dash', () => {
    const notes = buildChapterFootnotes(
      [figureWith({ id: 'reserves', availability: 'config_required' })],
      translate,
    )

    expect(notes.get('reserves:root')?.notes[0]?.text).toContain('not configured')
  })

  it('collects the qualities a formula declares on its own stages', () => {
    const notes = buildChapterFootnotes(
      [figureWith({
        id: 'backlog',
        kind: 'metric_flow',
        metricFlow: {
          stages: [
            { key: 'opening', label: 'Opening', role: 'input', confidence: 'proxy' },
            { key: 'incoming', label: 'Incoming', role: 'add', confidence: 'verified' },
            { key: 'closing', label: 'Closing', role: 'result', availability: 'unavailable' },
          ],
        },
      })],
      translate,
    )

    expect(notes.get('backlog:root')?.notes).toHaveLength(2)
  })

  it('carries a row badge’s hint into the notes, deduplicated', () => {
    const frame: Frame = {
      columns: [
        { name: 'product', type: 'string' },
        { name: 'hint', type: 'string' },
      ],
      rows: [
        ['Motor', 'Discontinued product'],
        ['Cargo', 'Discontinued product'],
        ['Travel', ''],
      ],
    }
    const notes = buildChapterFootnotes(
      [figureWith({
        id: 'products',
        kind: 'table',
        columns: [{ field: 'product', label: 'Product', cell: { kind: 'plain' }, badgeField: 'hint' }],
      }, frame)],
      translate,
    )

    expect(notes.get('products:root')?.notes).toEqual([{ number: 1, text: 'Discontinued product' }])
  })
})
