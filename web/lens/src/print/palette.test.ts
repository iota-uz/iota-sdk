import { describe, expect, it } from 'vitest'
import type { DashboardDocument, Frame, Level, Panel, Theme } from '../contract'
import type { PrintReport, PrintSection } from '../runtime/print'
import { buildLabelPalette, printTheme } from './palette'

const theme: Theme = {
  palette: { 'series-1': '#111', 'series-2': '#222', 'series-3': '#333' },
  series: {},
}

function documentWith(overrides: Partial<Theme> = {}): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'snapshot',
    meta: { dashboardId: 'board', title: 'Board', generatedAt: '2026-07-28T00:00:00Z', locale: 'ru' },
    layout: { rows: [] },
    panels: [],
    frames: {},
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { ...theme, ...overrides },
  }
}

function sectionWith(id: string, labels: Array<string>, document = documentWith()): PrintSection {
  const panel: Panel = {
    id,
    kind: 'donut',
    title: id,
    semantics: 'partition',
    frame: `${id}:root`,
    encoding: { label: 'label', value: 'value' },
    format: {},
    actions: [],
  }
  const frame: Frame = {
    columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
    rows: labels.map((label, index) => [label, index + 1]),
  }
  const level: Level = { path: [], label: panel.title, children: [], perspectives: [] }
  return {
    id: `${id}:root`,
    document,
    panel,
    level,
    frame,
    path: [],
    breadcrumb: [panel.title],
    depth: 0,
    root: true,
  }
}

function reportWith(sections: Array<PrintSection>): PrintReport {
  return {
    document: sections[0]!.document,
    sections,
    warnings: [],
    truncated: false,
  } as PrintReport
}

describe('buildLabelPalette', () => {
  it('gives a name the same colour in every figure that mentions it', () => {
    const labels = buildLabelPalette(reportWith([
      sectionWith('bridge', ['Премия', 'Аквизиция', 'Опекс']),
      sectionWith('expenses', ['Аквизиция', 'Опекс', 'Перестрахование']),
    ]))

    expect(labels.get('Аквизиция')).toBe('#222')
    expect(labels.get('Опекс')).toBe('#333')
    // The second figure's new name continues the assignment rather than
    // restarting it and colliding with «Премия».
    expect(labels.get('Перестрахование')).toBe('#111')
  })

  it('keeps a colour the producer pinned to a label', () => {
    const document = documentWith({ series: { Аквизиция: 'series-3' } })
    const labels = buildLabelPalette(reportWith([sectionWith('bridge', ['Премия', 'Аквизиция'], document)]))

    expect(labels.get('Аквизиция')).toBe('#333')
    expect(labels.get('Премия')).toBe('#111')
  })
})

describe('printTheme', () => {
  it('drops the row-position entries the shared palette replaces', () => {
    const source: Theme = { ...theme, series: { 'panel:0': 'series-2', Премия: 'series-1' } }
    const result = printTheme(source, new Map([['Аквизиция', '#222']]))

    expect(result.series['panel:0']).toBeUndefined()
    expect(result.series['Премия']).toBe('series-1')
    expect(result.series['Аквизиция']).toBe('#222')
  })

  it('leaves a themeless document alone', () => {
    expect(printTheme(theme, new Map())).toBe(theme)
  })
})
