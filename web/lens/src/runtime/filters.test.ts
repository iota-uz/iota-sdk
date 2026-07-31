import { describe, expect, it } from 'vitest'
import type { Action, DashboardDocument, Filter } from '../contract'
import {
  compareValues,
  currentPeriodValue,
  declaredFilters,
  filterActionURL,
  filterParamNames,
  periodValues,
  readFilterValues,
  sameFilterValues,
  srcWithFilterParams,
  writeFilterValues,
} from './filters'

function documentWithFilters(filters: Array<Filter>): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'snap',
    meta: { dashboardId: 'dash', title: 'Dash', generatedAt: '2026-07-22T00:00:00Z', locale: 'en' },
    layout: { rows: [] },
    panels: [],
    frames: {},
    drill: { edges: {}, inlineDepth: 0 },
    perspectives: [],
    filters,
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

const periodFilter: Filter = {
  id: 'period',
  kind: 'period',
  label: 'Period',
  period: {
    startParam: 'ActualRangeStart',
    endParam: 'ActualRangeEnd',
    value: { start: '2026-01-01', end: '2026-07-22' },
    allowEmpty: true,
    presets: [
      { id: 'year-2026', label: '2026', value: { start: '2026-01-01', end: '2026-12-31' } },
      { id: 'all', label: 'All time', value: { start: '', end: '' } },
    ],
  },
}

const doc = documentWithFilters([periodFilter])

describe('readFilterValues', () => {
  it('reads declared params, keeping empty strings distinct from absence', () => {
    expect(readFilterValues(doc, new URL('https://x.test/dash'))).toEqual({})
    expect(readFilterValues(doc, new URL('https://x.test/dash?ActualRangeStart=2026-02-01&ActualRangeEnd=2026-03-01')))
      .toEqual({ ActualRangeStart: '2026-02-01', ActualRangeEnd: '2026-03-01' })
    expect(readFilterValues(doc, new URL('https://x.test/dash?ActualRangeStart=&ActualRangeEnd=')))
      .toEqual({ ActualRangeStart: '', ActualRangeEnd: '' })
  })

  it('requires both boundaries', () => {
    expect(readFilterValues(doc, new URL('https://x.test/dash?ActualRangeStart=2026-02-01'))).toEqual({})
  })

  it('drops values the declaration cannot have produced', () => {
    for (const search of [
      '?ActualRangeStart=2026-2-1&ActualRangeEnd=2026-03-01',
      '?ActualRangeStart=garbage&ActualRangeEnd=2026-03-01',
      '?ActualRangeStart=2026-05-01&ActualRangeEnd=2026-03-01',
    ]) {
      expect(readFilterValues(doc, new URL(`https://x.test/dash${search}`))).toEqual({})
    }
  })

  it('honors declared min/max bounds', () => {
    const bounded = documentWithFilters([{
      ...periodFilter,
      period: { ...periodFilter.period!, min: '2025-01-01', max: '2026-12-31' },
    }])
    expect(readFilterValues(bounded, new URL('https://x.test/dash?ActualRangeStart=2024-01-01&ActualRangeEnd=2026-01-01')))
      .toEqual({})
    expect(readFilterValues(bounded, new URL('https://x.test/dash?ActualRangeStart=2025-01-01&ActualRangeEnd=2026-01-01')))
      .toEqual({ ActualRangeStart: '2025-01-01', ActualRangeEnd: '2026-01-01' })
  })

  it('rejects empty boundaries unless the declaration allows them', () => {
    const strict = documentWithFilters([{
      ...periodFilter,
      period: { ...periodFilter.period!, allowEmpty: false },
    }])
    expect(readFilterValues(strict, new URL('https://x.test/dash?ActualRangeStart=&ActualRangeEnd='))).toEqual({})
  })
})

describe('writeFilterValues', () => {
  it('round-trips through a URL and preserves unrelated params', () => {
    const url = new URL('https://x.test/dash?path=a&path=b&_f=product%3Aone&_f=region%3Atwo&other=1')
    const values = { ActualRangeStart: '2026-02-01', ActualRangeEnd: '2026-03-01' }
    const next = writeFilterValues(url, doc, values)
    expect(readFilterValues(doc, next)).toEqual({ ...values, _f: ['product:one', 'region:two'] })
    expect(next.searchParams.getAll('path')).toEqual(['a', 'b'])
    expect(next.searchParams.getAll('_f')).toEqual(['product:one', 'region:two'])
    expect(next.searchParams.get('other')).toBe('1')
  })

  it('clears declared params when a value is absent', () => {
    const url = new URL('https://x.test/dash?ActualRangeStart=2026-02-01&ActualRangeEnd=2026-03-01')
    const next = writeFilterValues(url, doc, {})
    expect(next.search).toBe('')
  })

  it('writes the present-but-empty all-time form', () => {
    const next = writeFilterValues(new URL('https://x.test/dash'), doc, { ActualRangeStart: '', ActualRangeEnd: '' })
    expect(next.search).toBe('?ActualRangeStart=&ActualRangeEnd=')
    expect(readFilterValues(doc, next)).toEqual({ ActualRangeStart: '', ActualRangeEnd: '' })
  })
})

describe('value helpers', () => {
  it('lists declared filters and param names', () => {
    expect(declaredFilters(doc)).toHaveLength(1)
    expect(filterParamNames(doc)).toEqual(['ActualRangeStart', 'ActualRangeEnd'])
    expect(declaredFilters(documentWithFilters([]))).toEqual([])
  })

  it('falls back to the declared normalized value', () => {
    expect(currentPeriodValue(periodFilter.period!, {})).toEqual({ start: '2026-01-01', end: '2026-07-22' })
    expect(currentPeriodValue(periodFilter.period!, { ActualRangeStart: '2026-02-01', ActualRangeEnd: '2026-03-01' }))
      .toEqual({ start: '2026-02-01', end: '2026-03-01' })
    expect(currentPeriodValue(periodFilter.period!, { ActualRangeStart: '', ActualRangeEnd: '' }))
      .toEqual({ start: '', end: '' })
  })

  it('maps a period value to its parameters', () => {
    expect(periodValues(periodFilter.period!, { start: '2026-02-01', end: '2026-03-01' }))
      .toEqual({ ActualRangeStart: '2026-02-01', ActualRangeEnd: '2026-03-01' })
  })

  it('compares value maps with empty-string awareness', () => {
    expect(sameFilterValues({}, {})).toBe(true)
    expect(sameFilterValues({ a: '' }, {})).toBe(false)
    expect(sameFilterValues({ a: '' }, { a: '' })).toBe(true)
    expect(sameFilterValues({ a: '1' }, { a: '2' })).toBe(false)
  })
})

describe('srcWithFilterParams', () => {
  it('replaces driven params and preserves the rest of the src', () => {
    const src = '/analytics/profitability/lens/document?ActualRangeStart=2026-01-01&ActualRangeEnd=2026-07-22&LargeLossThreshold=5'
    const driven = new Set(['ActualRangeStart', 'ActualRangeEnd'])
    expect(srcWithFilterParams(src, driven, { ActualRangeStart: '2026-02-01', ActualRangeEnd: '2026-03-01' }))
      .toBe('/analytics/profitability/lens/document?LargeLossThreshold=5&ActualRangeStart=2026-02-01&ActualRangeEnd=2026-03-01')
  })

  it('removes driven params that have no current value', () => {
    const src = '/doc?ActualRangeStart=2026-01-01&ActualRangeEnd=2026-07-22'
    const driven = new Set(['ActualRangeStart', 'ActualRangeEnd'])
    expect(srcWithFilterParams(src, driven, {})).toBe('/doc')
  })

  it('keeps the all-time empty form on the src', () => {
    expect(srcWithFilterParams('/doc', [], { ActualRangeStart: '', ActualRangeEnd: '' }))
      .toBe('/doc?ActualRangeStart=&ActualRangeEnd=')
  })
})

describe('comparison and cube interaction state', () => {
  it('encodes custom comparison values and leaves preset ranges implicit', () => {
    const compare = {
      modeParam: 'compare', startParam: 'compare_start', endParam: 'compare_end', compareTo: 'period',
      value: { mode: 'off' as const },
    }
    expect(compareValues(compare, { mode: 'previous_period' })).toEqual({ compare: 'previous_period' })
    expect(compareValues(compare, { mode: 'custom', start: '2026-01-01', end: '2026-01-31' })).toEqual({
      compare: 'custom', compare_start: '2026-01-01', compare_end: '2026-01-31',
    })
  })

  it('toggles cross-filter values on the same repeated _f stack and preserves drill group-by', () => {
    const action: Action = {
      kind: 'cross_filter', urlTemplate: '/sales', params: [], payload: {},
      filter: { dimension: 'product', value: { kind: 'field', name: 'product_id' } },
    }
    const current = new URL('https://x.test/sales?_f=region%3Atashkent&_groupby=region')
    const selected = filterActionURL(action, { product_id: 'osago' }, current)!
    expect(selected.searchParams.getAll('_f')).toEqual(['region:tashkent', 'product:osago'])
    expect(selected.searchParams.get('_groupby')).toBe('region')
    expect(filterActionURL(action, { product_id: 'osago' }, selected)!.searchParams.getAll('_f')).toEqual(['region:tashkent'])
  })

  it('adds cube drill group-by without replacing the shared filter stack', () => {
    const action: Action = {
      kind: 'cube_drill', urlTemplate: '/sales', params: [], payload: {},
      filter: { dimension: 'product', groupBy: 'region', value: { kind: 'field', name: 'product_id' } },
    }
    const next = filterActionURL(action, { product_id: 'osago' }, new URL('https://x.test/sales?_f=channel%3Aagent'))!
    expect(next.searchParams.getAll('_f')).toEqual(['channel:agent', 'product:osago'])
    expect(next.searchParams.get('_groupby')).toBe('region')
  })
})
