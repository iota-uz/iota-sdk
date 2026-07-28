import { describe, expect, it } from 'vitest'
import type { DashboardDocument, Frame, Level, Panel } from '../contract'
import type { PrintReport, PrintSection } from '../runtime/print'
import { buildOutline } from './outline'

function panelWith(id: string, overrides: Partial<Panel> = {}): Panel {
  return {
    id,
    kind: 'donut',
    title: id,
    semantics: 'partition',
    frame: `${id}:root`,
    encoding: { label: 'label', value: 'value' },
    format: {},
    actions: [],
    ...overrides,
  }
}

function frameWith(rows: Array<Array<unknown>>): Frame {
  return {
    columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
    rows,
  }
}

function documentWith(panels: Array<Panel>, layout: DashboardDocument['layout']): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'snapshot',
    meta: { dashboardId: 'board', title: 'Board', generatedAt: '2026-07-28T00:00:00Z', locale: 'en' },
    layout,
    panels,
    frames: {},
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function sectionWith(
  document: DashboardDocument,
  panel: Panel,
  frame: Frame | undefined,
  overrides: Partial<PrintSection> = {},
): PrintSection {
  const level: Level = { path: [], label: panel.title, children: [], perspectives: [] }
  return {
    id: `${panel.id}:${overrides.breadcrumb?.join('/') ?? 'root'}`,
    document,
    panel,
    level,
    ...(frame ? { frame } : {}),
    path: [],
    breadcrumb: [panel.title],
    depth: 0,
    root: true,
    ...overrides,
  }
}

function reportWith(document: DashboardDocument, sections: Array<PrintSection>): PrintReport {
  return { document, sections, warnings: [], truncated: false }
}

describe('buildOutline', () => {
  it('opens a chapter per layout heading and numbers figures inside it', () => {
    const premium = panelWith('premium')
    const claims = panelWith('claims')
    const document = documentWith([premium, claims], {
      rows: [
        { heading: 'Premium', panels: [{ panelId: 'premium', span: 6 }] },
        { heading: 'Claims', panels: [{ panelId: 'claims', span: 6 }] },
      ],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, premium, frameWith([['Earned', 7], ['Unearned', 3]])),
        sectionWith(document, claims, frameWith([['Paid', 4], ['Reserved', 6]])),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters.map(({ number, title }) => `${number} ${title}`)).toEqual(['1 Premium', '2 Claims'])
    expect(outline.chapters.map(({ figures }) => figures.map(({ number }) => number))).toEqual([['1.1'], ['2.1']])
    expect(outline.figureCount).toBe(2)
  })

  it('continues the current chapter through rows that carry no heading', () => {
    const first = panelWith('first')
    const second = panelWith('second')
    const document = documentWith([first, second], {
      rows: [
        { heading: 'Premium', panels: [{ panelId: 'first', span: 6 }] },
        { panels: [{ panelId: 'second', span: 6 }] },
      ],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, first, frameWith([['a', 1], ['b', 2]])),
        sectionWith(document, second, frameWith([['c', 3], ['d', 4]])),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters).toHaveLength(1)
    expect(outline.chapters[0]?.figures.map(({ number }) => number)).toEqual(['1.1', '1.2'])
  })

  it('names the opening chapter when the first row has no heading of its own', () => {
    const panel = panelWith('hero')
    const document = documentWith([panel], { rows: [{ panels: [{ panelId: 'hero', span: 12 }] }] })
    const outline = buildOutline(
      reportWith(document, [sectionWith(document, panel, frameWith([['a', 1], ['b', 2]]))]),
      'Summary',
      'Other',
    )

    expect(outline.chapters[0]?.title).toBe('Summary')
  })

  it('routes drill readings into the detail appendix of their own chapter', () => {
    const panel = panelWith('premium')
    const document = documentWith([panel], {
      rows: [{ heading: 'Premium', panels: [{ panelId: 'premium', span: 12 }] }],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, panel, frameWith([['Earned', 7], ['Unearned', 3]])),
        sectionWith(document, panel, frameWith([['Motor', 5], ['Cargo', 2]]), {
          root: false,
          breadcrumb: ['Premium', 'Earned', 'By product'],
        }),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters[0]?.figures).toHaveLength(1)
    expect(outline.appendix).toHaveLength(1)
    expect(outline.chapters[0]?.details[0]).toMatchObject({
      number: 'A1.1',
      // Only where the path ends: the shared head repeats on every detail.
      trail: 'Earned › By product',
    })
  })

  it('prints a drill reading that only restates its parent once', () => {
    const panel = panelWith('premium')
    const document = documentWith([panel], {
      rows: [{ heading: 'Premium', panels: [{ panelId: 'premium', span: 12 }] }],
    })
    const rows = frameWith([['Earned', 7], ['Unearned', 3]])
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, panel, rows),
        sectionWith(document, panel, frameWith([['Earned', 7], ['Unearned', 3]]), {
          root: false,
          breadcrumb: ['Premium', 'Recognition'],
        }),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters[0]?.figures).toHaveLength(1)
    expect(outline.chapters[0]?.details).toHaveLength(0)
    expect(outline.appendix).toHaveLength(0)
  })

  it('discloses an empty reading by name instead of printing it', () => {
    const panel = panelWith('ibnr')
    const document = documentWith([panel], {
      rows: [{ heading: 'Reserves', panels: [{ panelId: 'ibnr', span: 12 }] }],
    })
    const outline = buildOutline(
      reportWith(document, [sectionWith(document, panel, undefined)]),
      'Summary',
      'Other',
    )

    expect(outline.chapters).toHaveLength(0)
    expect(outline.missing).toEqual(['ibnr'])
  })

  it('lifts the opening chapter single-value readings onto the cover', () => {
    const hero = panelWith('hero', { kind: 'stat', semantics: 'evidence' })
    const share = panelWith('share')
    const document = documentWith([hero, share], {
      rows: [
        { heading: 'Summary', panels: [{ panelId: 'hero', span: 3 }] },
        { heading: 'Premium', panels: [{ panelId: 'share', span: 12 }] },
      ],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, hero, frameWith([['Earned premium', 178]])),
        sectionWith(document, share, frameWith([['Earned', 7], ['Unearned', 3]])),
      ]),
      'Summary',
      'Other',
    )

    // The headline reading leaves the body entirely: chapter 1 of the printed
    // report is the first chapter that still has something to show.
    expect(outline.kpis).toHaveLength(1)
    expect(outline.kpis[0]).toMatchObject({ kpi: true, chart: false, width: 'half' })
    expect(outline.chapters.map(({ number, title }) => `${number} ${title}`)).toEqual(['1 Premium'])
    expect(outline.chapters[0]?.figures[0]).toMatchObject({ kpi: false, chart: true, number: '1.1' })
  })

  it('carries a group caption onto its chapter', () => {
    const panel = panelWith('ratios')
    const document = documentWith([panel], {
      rows: [{
        heading: 'Key ratios',
        panels: [{
          panelId: 'ratios',
          span: 12,
          group: { id: 'ratios', kind: 'metrics', span: 12, caption: 'Diagnostics, not a verdict.' },
        }],
      }],
    })
    const outline = buildOutline(
      reportWith(document, [sectionWith(document, panel, frameWith([['a', 1], ['b', 2]]))]),
      'Summary',
      'Other',
    )

    expect(outline.chapters[0]?.caption).toBe('Diagnostics, not a verdict.')
  })

  it('collects readings the layout never places into a closing chapter', () => {
    const placed = panelWith('placed')
    const loose = panelWith('loose')
    const document = documentWith([placed, loose], {
      rows: [{ heading: 'Premium', panels: [{ panelId: 'placed', span: 12 }] }],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, placed, frameWith([['a', 1], ['b', 2]])),
        sectionWith(document, loose, frameWith([['c', 3], ['d', 4]])),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters.map(({ title }) => title)).toEqual(['Premium', 'Other'])
  })

  it('marks a single-value reading as a metric and carries its strip', () => {
    const mix = panelWith('mix')
    const ratio = panelWith('loss-ratio', { kind: 'stat', semantics: 'evidence' })
    const document = documentWith([mix, ratio], {
      rows: [
        { heading: 'Premium', panels: [{ panelId: 'mix', span: 12 }] },
        {
          heading: 'Key ratios',
          panels: [{
            panelId: 'loss-ratio',
            span: 6,
            groups: [
              { id: 'ratios', kind: 'metrics', span: 12, label: 'Ratios' },
              { id: 'period', kind: 'metrics', span: 6, label: 'On premium for the period', caption: 'Diagnostics.' },
            ],
          }],
        },
      ],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, mix, frameWith([['Earned', 7], ['Unearned', 3]])),
        sectionWith(document, ratio, frameWith([['', 0.189]])),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.chapters[1]?.figures[0]).toMatchObject({
      metric: true,
      chart: false,
      width: 'half',
      // The innermost strip is the one printed above the numbers.
      group: { id: 'period', label: 'On premium for the period', caption: 'Diagnostics.' },
    })
  })

  it('prints a drawer breakdown with the reading it explains', () => {
    const mix = panelWith('mix')
    const ratio = panelWith('loss-ratio', { kind: 'stat', semantics: 'evidence' })
    const numerator = panelWith('ratio-numerator', { kind: 'stat', semantics: 'evidence' })
    const document = documentWith([mix, ratio], {
      rows: [
        { heading: 'Premium', panels: [{ panelId: 'mix', span: 12 }] },
        { heading: 'Ratios', panels: [{ panelId: 'loss-ratio', span: 6 }] },
      ],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, mix, frameWith([['Earned', 7], ['Unearned', 3]])),
        sectionWith(document, ratio, frameWith([['Loss ratio', 0.028]])),
        sectionWith(document, numerator, frameWith([['Claims paid', 5.63]]), {
          root: false,
          owner: 'loss-ratio',
          breadcrumb: ['Loss ratio', 'Numerator'],
        }),
      ]),
      'Summary',
      'Other',
    )

    // The breakdown belongs to the tile, not to a loose chapter at the end.
    expect(outline.chapters.map(({ title }) => title)).toEqual(['Premium', 'Ratios'])
    expect(outline.chapters[1]?.figures[0]?.breakdown.map(({ panel }) => panel.id)).toEqual(['ratio-numerator'])
    expect(outline.chapters[1]?.details).toHaveLength(0)
  })

  it('keeps a shared denominator under every ratio that divides by it', () => {
    const loss = panelWith('loss-ratio', { kind: 'stat', semantics: 'evidence' })
    const expense = panelWith('expense-ratio', { kind: 'stat', semantics: 'evidence' })
    const document = documentWith([loss, expense], {
      rows: [{ heading: 'Ratios', panels: [{ panelId: 'loss-ratio', span: 6 }, { panelId: 'expense-ratio', span: 6 }] }],
    })
    const denominator = frameWith([['Earned premium', 199.89]])
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, loss, frameWith([['Loss ratio', 2.8]])),
        sectionWith(document, expense, frameWith([['Expense ratio', 30.4]])),
        sectionWith(document, panelWith('loss-denominator', { kind: 'stat', semantics: 'evidence' }), denominator, {
          root: false,
          owner: 'loss-ratio',
          breadcrumb: ['Loss ratio', 'Denominator'],
        }),
        sectionWith(document, panelWith('expense-denominator', { kind: 'stat', semantics: 'evidence' }), denominator, {
          root: false,
          owner: 'expense-ratio',
          breadcrumb: ['Expense ratio', 'Denominator'],
        }),
      ]),
      'Summary',
      'Other',
    )

    // Identical rows, but each formula still states what it divides by.
    expect(outline.kpis.map(({ section }) => section.panel.id)).toEqual(['loss-ratio', 'expense-ratio'])
    expect(outline.kpis[0]?.breakdown.map(({ panel }) => panel.id)).toEqual(['loss-denominator'])
    expect(outline.kpis[1]?.breakdown.map(({ panel }) => panel.id)).toEqual(['expense-denominator'])
  })

  it('counts the readings that are an estimate rather than a settled figure', () => {
    const proxy = panelWith('ibnr', { confidence: 'proxy' })
    const verified = panelWith('gwp', { confidence: 'verified' })
    const document = documentWith([proxy, verified], {
      rows: [{ heading: 'Reserves', panels: [{ panelId: 'ibnr', span: 6 }, { panelId: 'gwp', span: 6 }] }],
    })
    const outline = buildOutline(
      reportWith(document, [
        sectionWith(document, proxy, frameWith([['a', 1], ['b', 2]])),
        sectionWith(document, verified, frameWith([['c', 3], ['d', 4]])),
      ]),
      'Summary',
      'Other',
    )

    expect(outline.estimated).toBe(1)
  })

  it('gathers what the dashboard says about its own numbers into notes', () => {
    const panel = panelWith('coverage', {
      status: { label: 'Proxy', tone: 'warning' },
      metricRelationship: {
        source: { key: 'a', label: 'A' },
        target: { key: 'b', label: 'B' },
        type: 'derivation',
        note: 'Same amount, two names.',
      },
    })
    const document = documentWith([panel], {
      rows: [{ heading: 'Coverage', panels: [{ panelId: 'coverage', span: 12 }] }],
    })
    const outline = buildOutline(
      reportWith(document, [sectionWith(document, panel, frameWith([['a', 1], ['b', 2]]))]),
      'Summary',
      'Other',
    )

    expect(outline.notes.map(({ detail }) => detail)).toEqual(['Proxy', 'Same amount, two names.'])
  })
})
