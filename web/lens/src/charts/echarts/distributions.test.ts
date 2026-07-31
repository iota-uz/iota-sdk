import { describe, expect, it } from 'vitest'
import type { Frame } from '../../contract'
import { buildDistributionOption, type DistributionInput, type DistributionKind } from './distributions'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  card: '#fff', text: '#111', mutedText: '#666', border: '#ddd', divider: '#eee', selectedBorder: '#000',
  fontFamily: 'sans-serif', colors: ['#2563eb'], seriesColor: () => undefined,
}

function input(kind: DistributionKind, frame: Frame, encoding: DistributionInput['encoding']): DistributionInput {
  return { kind, frame, encoding, format: (_field, value) => String(value), theme: { palette: {}, series: {} } }
}

describe('distribution chart options', () => {
  it('renders histogram buckets without recomputing them', () => {
    const frame: Frame = {
      columns: [{ name: 'bucket', type: 'string' }, { name: 'count', type: 'number' }], rows: [['0–10', 4], ['10–20', 7]],
    }
    const option = buildDistributionOption(input('histogram', frame, { category: 'bucket', value: 'count' }), theme) as { series: Array<{ data: number[] }> }
    expect(option.series[0]?.data).toEqual([4, 7])
  })

  it('passes producer-computed five-number summaries in ECharts order', () => {
    const frame: Frame = {
      columns: ['product', 'min', 'q1', 'median', 'q3', 'max'].map((name) => ({ name, type: name === 'product' ? 'string' as const : 'number' as const })),
      rows: [['OSAGO', 1, 3, 5, 8, 20]],
    }
    const encoding = { category: 'product', lower: 'min', q1: 'q1', median: 'median', q3: 'q3', upper: 'max' } as DistributionInput['encoding']
    const option = buildDistributionOption(input('boxplot', frame, encoding), theme) as { series: Array<{ data: number[][] }> }
    expect(option.series[0]?.data).toEqual([[1, 3, 5, 8, 20]])
  })

  it('maps category × series cells and derives a value scale', () => {
    const frame: Frame = {
      columns: [{ name: 'region', type: 'string' }, { name: 'product', type: 'string' }, { name: 'count', type: 'number' }],
      rows: [['Tashkent', 'OSAGO', 12], ['Samarkand', 'OSAGO', 7], ['Tashkent', 'KASKO', 4]],
    }
    const option = buildDistributionOption(input('heatmap', frame, { category: 'region', series: 'product', value: 'count' }), theme) as {
      series: Array<{ data: number[][] }>; visualMap: { max: number }
    }
    expect(option.series[0]?.data).toEqual([[0, 0, 12], [1, 0, 7], [0, 1, 4]])
    expect(option.visualMap.max).toBe(12)
  })
})
