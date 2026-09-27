import { describe, expect, it } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument, type QueryRequest, type QueryResponse } from '../contract'
import { buildPrintReport, type PrintQuery } from './print'

describe('buildPrintReport', () => {
  it('keeps every ordinary dashboard panel as a root audit section', async () => {
    const document = parseDocument(fixture)
    const report = await buildPrintReport(document)

    expect(report.sections).toHaveLength(1)
    expect(report.sections[0]).toMatchObject({
      root: true,
      breadcrumb: ['Total'],
      frame: document.frames['panel:total'],
    })
  })

  it('gives every panel that opens one drawer its own copy of the reading', async () => {
    const drawerAction = {
      kind: 'open_drawer' as const,
      urlTemplate: '/drawer',
      params: [],
      payload: {},
    }
    const document = parseDocument({
      ...fixture,
      panels: [
        { ...fixture.panels[0], id: 'hero', actions: [drawerAction], terminal: false },
        { ...fixture.panels[0], id: 'strip', actions: [drawerAction], terminal: false },
      ],
      layout: {
        rows: [{ panels: [{ panelId: 'hero', span: 6 }, { panelId: 'strip', span: 6 }] }],
      },
    })
    const drawer = parseDocument({
      ...fixture,
      meta: { ...fixture.meta, dashboardId: 'drawer' },
      panels: [{ ...fixture.panels[0], id: 'numerator', actions: [] }],
      layout: { rows: [{ panels: [{ panelId: 'numerator', span: 12 }] }] },
    })
    let loads = 0
    const report = await buildPrintReport(document, undefined, 2_000, () => {
      loads += 1
      return Promise.resolve(drawer)
    })

    // Fetched once, attached twice: neither tile prints without its calculation.
    expect(loads).toBe(1)
    expect(report.sections.filter(({ owner }) => owner === 'hero')).toHaveLength(1)
    expect(report.sections.filter(({ owner }) => owner === 'strip')).toHaveLength(1)
  })

  it('loads dynamic children and flattens their query paths in order', async () => {
    const document = parseDocument({
      ...fixture,
      panels: [{ ...fixture.panels[0], drillRoot: 'root', terminal: false }],
      drill: {
        inlineDepth: 0,
        edges: {
          root: {
            path: ['root'],
            label: 'Years',
            children: [],
            perspectives: [],
            dynamicChildren: {
              key: { kind: 'field', name: 'id' },
              label: { kind: 'field', name: 'label' },
              target: { kind: 'literal', value: 'detail' },
            },
          },
          detail: {
            path: ['root', 'detail'],
            label: 'Products',
            children: [],
            perspectives: [],
          },
        },
      },
    })
    const requests: Array<Array<string>> = []
    const query: PrintQuery = (request: QueryRequest): Promise<QueryResponse> => {
      requests.push([...request.path])
      if (request.path.length === 1) {
        return Promise.resolve({
          frames: {
            years: {
              columns: [
                { name: 'id', type: 'string' },
                { name: 'label', type: 'string' },
                { name: 'value', type: 'number' },
              ],
              rows: [['2025', '2025', 42]],
              children: [{ key: '2025', path: ['root', '2025'], label: '2025', target: 'detail' }],
            },
          },
        })
      }
      return Promise.resolve({
        frames: {
          products: {
            columns: document.frames['panel:total']!.columns,
            rows: [['Motor', 42]],
          },
        },
      })
    }

    const report = await buildPrintReport(document, query)

    expect(requests).toEqual([['root'], ['root', '2025', 'detail']])
    expect(report.sections.map(({ breadcrumb }) => breadcrumb)).toEqual([
      ['Years'],
      ['Years', '2025', 'Products'],
    ])
  })

  it('expands every perspective of a fork instead of printing a choice screen', async () => {
    const document = parseDocument({
      ...fixture,
      panels: [{ ...fixture.panels[0], drillRoot: 'root', terminal: false }],
      drill: {
        inlineDepth: 0,
        edges: {
          root: {
            path: ['root'],
            label: 'Breakdown',
            children: [],
            perspectives: [{ id: 'periods' }, { id: 'products' }],
          },
          periods: {
            path: ['root', 'periods'],
            label: 'By period',
            children: [],
            perspectives: [{ id: 'periods' }],
          },
          products: {
            path: ['root', 'products'],
            label: 'By product',
            children: [],
            perspectives: [{ id: 'products' }],
          },
        },
      },
      perspectives: [
        {
          id: 'periods',
          explorerId: 'premium',
          branchKey: 'root',
          key: 'periods',
          label: 'By period',
          semantics: 'series',
          root: 'periods',
        },
        {
          id: 'products',
          explorerId: 'premium',
          branchKey: 'root',
          key: 'products',
          label: 'By product',
          semantics: 'partition',
          root: 'products',
        },
      ],
    })
    const perspectives: Array<string | undefined> = []
    const query: PrintQuery = (request: QueryRequest): Promise<QueryResponse> => {
      perspectives.push(request.perspective)
      return Promise.resolve({
        frames: {
          [request.perspective ?? 'unknown']: {
            columns: document.frames['panel:total']!.columns,
            rows: [[request.perspective, 42]],
          },
        },
      })
    }

    const report = await buildPrintReport(document, query)

    expect(perspectives).toEqual(['periods', 'products'])
    expect(report.sections.map(({ perspective }) => perspective?.label)).toEqual(['By period', 'By product'])
  })

  it('loads drawer documents and appends their panels as flat audit sections', async () => {
    const document = parseDocument({
      ...fixture,
      panels: [{
        ...fixture.panels[0],
        actions: [{
          kind: 'open_drawer',
          method: 'GET',
          urlTemplate: '/drawer/premium',
          params: [],
          payload: {},
        }],
        terminal: false,
      }],
    })
    const nested = parseDocument({
      ...fixture,
      snapshotId: 'nested-snapshot',
      meta: { ...fixture.meta, dashboardId: 'premium-detail', title: 'Premium detail' },
      drawer: { title: 'Premium detail' },
      panels: [{ ...fixture.panels[0], id: 'detail', title: 'By product', actions: [] }],
    })
    const loaded: Array<string> = []

    const report = await buildPrintReport(
      document,
      undefined,
      2_000,
      (src) => {
        loaded.push(src)
        return Promise.resolve(nested)
      },
      new URL('https://example.test/analytics/profitability?year=2026'),
    )

    expect(loaded).toEqual(['https://example.test/drawer/premium'])
    expect(report.sections.map(({ breadcrumb }) => breadcrumb)).toEqual([
      ['Total'],
      ['Total', 'Premium detail', 'By product'],
    ])
    expect(report.sections[1]?.document.snapshotId).toBe('nested-snapshot')
    expect(report.sections[1]?.root).toBe(false)
  })

  it('prints row-scoped evidence without reopening a drawer for every row', async () => {
    const document = parseDocument({
      ...fixture,
      panels: [{
        ...fixture.panels[0],
        actions: [{
          kind: 'open_drawer',
          method: 'GET',
          urlTemplate: '/drawer/products/{id}',
          params: [{ name: 'id', source: { kind: 'field', name: 'category' } }],
          payload: {},
        }],
        terminal: false,
      }],
    })
    let loads = 0

    const report = await buildPrintReport(
      document,
      undefined,
      2_000,
      () => {
        loads += 1
        return Promise.reject(new Error('row-scoped drawer must not load'))
      },
      new URL('https://example.test/analytics/profitability'),
    )

    expect(loads).toBe(0)
    expect(report.sections).toHaveLength(1)
    expect(report.sections[0]?.frame?.rows).toEqual(document.frames['panel:total']?.rows)
  })

  it('returns a clearly marked partial report when preparation is cancelled', async () => {
    const document = parseDocument({
      ...fixture,
      panels: [{ ...fixture.panels[0], drillRoot: 'root', terminal: false }],
      drill: {
        inlineDepth: 0,
        edges: {
          root: {
            path: ['root'],
            label: 'Deferred detail',
            children: [],
            perspectives: [],
          },
        },
      },
    })
    const controller = new AbortController()
    controller.abort()

    const report = await buildPrintReport(
      document,
      () => Promise.reject(new Error('must not query after cancellation')),
      2_000,
      undefined,
      new URL('https://example.test/analytics/profitability'),
      controller.signal,
    )

    expect(report.truncated).toBe(true)
    expect(report.sections).toEqual([])
  })
})
