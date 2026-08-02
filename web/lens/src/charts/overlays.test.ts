import { describe, expect, it } from 'vitest'
import type { PanelTemporal } from '../contract'
import type { ChartLabels } from './adapter'
import { activeOverlayIds, categoryDisplayFormatter, chartOverlays, overlayToneProperty } from './overlays'

const labels: ChartLabels = {
  previous: 'Previous',
  trend: 'Trend',
  movingAverage: (window) => `SMA ${window}`,
  estimate: 'Estimate',
  ytd: 'YTD',
  forecast: 'Forecast',
  forecastLower: (forecast) => `${forecast} lower`,
  forecastConfidence: (forecast) => `${forecast} confidence`,
  boxplot: ['Min', 'Q1', 'Median', 'Q3', 'Max'],
}

const temporal: PanelTemporal = {
  regression: { field: 'regression', label: 'Trend' },
  movingAverages: [{ window: 3, field: 'sma_3' }],
  referenceLines: [{ value: 100, label: 'Break-even' }, { value: 62.5 }],
  period: { category: '2026', state: 'ytd', label: 'Year to date', annualizedField: 'annualized' },
  annotations: [{ at: '2025', label: 'Method changed' }],
  forecast: { start: '2026', valueField: 'f', lowerField: 'l', upperField: 'u' },
}

function overlays(hidden?: ReadonlySet<string>) {
  return chartOverlays({
    temporal,
    hasComparison: true,
    labels,
    formatValue: (value) => `${value}%`,
    formatCategory: (value) => value,
    hidden,
  })
}

describe('chart overlays', () => {
  it('names every mark a chart draws that is not a row of its frame', () => {
    // The point of the list: six marks used to share one grey dashed idiom and
    // none of them could reach the legend, because the legend enumerates rows.
    expect(overlays().map(({ id, kind, label, detail }) => ({ id, kind, label, detail }))).toEqual([
      { id: 'incomplete', kind: 'incomplete', label: 'Year to date', detail: '2026' },
      { id: 'comparison', kind: 'comparison', label: 'Previous', detail: undefined },
      { id: 'reference:0', kind: 'reference', label: 'Break-even', detail: '100%' },
      // An unnamed threshold is its own value, and the legend does not print
      // the same number twice.
      { id: 'reference:1', kind: 'reference', label: '62.5%', detail: undefined },
      { id: 'annotation:0', kind: 'annotation', label: 'Method changed', detail: '2025' },
      { id: 'trend', kind: 'trend', label: 'Trend', detail: undefined },
      { id: 'average:3', kind: 'average', label: 'SMA 3', detail: undefined },
      { id: 'estimate', kind: 'estimate', label: 'Estimate', detail: '2026' },
      { id: 'forecast', kind: 'forecast', label: 'Forecast', detail: '2026' },
    ])
  })

  it('gives each meaning a stroke and a tone of its own', () => {
    const vocabulary = Object.fromEntries(overlays().map(({ kind, stroke, tone }) => [kind, `${tone}/${stroke}`]))

    expect(vocabulary).toEqual({
      incomplete: 'faint/hatch',
      comparison: 'series/dashed',
      reference: 'warn/hairline',
      annotation: 'accent/band',
      trend: 'trend/dashed',
      average: 'series/solid',
      estimate: 'muted/dotted',
      forecast: 'series/band',
    })
    // A fitted line is the one overlay that may never be mistaken for data, so
    // it is the one with an ink outside the categorical palette.
    expect(overlayToneProperty('trend')).toBe('--lens-overlay-trend')
    expect(overlayToneProperty('warn')).toBe('--lens-warn')
  })

  it('keeps a switched-off overlay listed, so the reader can switch it back on', () => {
    const switchedOff = overlays(new Set(['trend', 'reference:0']))

    expect(switchedOff.filter(({ active }) => !active).map(({ id }) => id)).toEqual(['reference:0', 'trend'])
    expect(activeOverlayIds(switchedOff).has('trend')).toBe(false)
    expect(activeOverlayIds(switchedOff).has('reference:1')).toBe(true)
  })

  it('prints a category the way the axis prints it', () => {
    const declared = categoryDisplayFormatter({ format: () => 'Jun 2026', isTime: true, hasDeclaredFormat: true })
    const bare = categoryDisplayFormatter({ format: (value) => String(value), isTime: true, hasDeclaredFormat: false, locale: 'en-US' })
    const plain = categoryDisplayFormatter({ format: (value) => String(value), isTime: false, hasDeclaredFormat: false })

    expect(declared('2026-06-01T00:00:00Z')).toBe('Jun 2026')
    // Without a declared format the legend falls back to the same month/year
    // rendering the time axis uses, rather than printing a raw timestamp.
    expect(bare('2026-06-01T00:00:00Z')).toBe('Jun 2026')
    expect(plain('2026')).toBe('2026')
  })
})
