import { describe, expect, it } from 'vitest'
import type { Frame } from '../../contract'
import { buildDistributionOption, type DistributionInput, type DistributionKind } from './distributions'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  card: '#fff', text: '#111', mutedText: '#666', border: '#ddd', divider: '#eee', selectedBorder: '#000',
  faintText: '#94a3b8', warn: '#d97706', warnSoft: '#fffbeb', accent: '#2563eb', trend: '#7c3aed',
  fontFamily: 'sans-serif', popoverShadow: '0 1px 2px rgba(0,0,0,0.1)', cardRadius: 8,
  type: { xs: 10, sm: 11, base: 12, md: 14 },
  colors: ['#2563eb'], seriesColor: () => undefined,
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
    expect(option).toMatchObject({ aria: { enabled: true }, backgroundColor: 'transparent' })
  })

  it('passes producer-computed five-number summaries in ECharts order', () => {
    const frame: Frame = {
      columns: ['product', 'min', 'q1', 'median', 'q3', 'max'].map((name) => ({ name, type: name === 'product' ? 'string' as const : 'number' as const })),
      rows: [['OSAGO', 1, 3, 5, 8, 20]],
    }
    const encoding = { category: 'product', lower: 'min', q1: 'q1', median: 'median', q3: 'q3', upper: 'max' } as DistributionInput['encoding']
    const chartInput = input('boxplot', frame, encoding)
    chartInput.labels = {
      previous: '', trend: '', movingAverage: () => '', estimate: '', ytd: '', forecast: '',
      forecastLower: () => '', forecastConfidence: () => '',
      boxplot: ['Мин', 'Квартиль 1', 'Медиана', 'Квартиль 3', 'Макс'],
      noData: 'No data',
    }
    const option = buildDistributionOption(chartInput, theme) as {
      series: Array<{ data: number[][] }>
      tooltip: { formatter: (raw: unknown) => string }
    }
    expect(option.series[0]?.data).toEqual([[1, 3, 5, 8, 20]])
    expect(option.tooltip.formatter({ name: 'OSAGO', data: [1, 3, 5, 8, 20] })).toContain('Медиана: 5')
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

describe('distribution chrome', () => {
  const frame: Frame = {
    columns: [{ name: 'x', type: 'string' }, { name: 'y', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [['Q1', 'North', 4], ['Q2', 'North', 9], ['Q1', 'South', 1]],
  }

  it('renders its tooltip through the shared chrome, at body level', () => {
    const option = buildDistributionOption(
      input('heatmap', frame, { category: 'x', series: 'y', value: 'value' }),
      theme,
    ) as { tooltip: { appendTo?: string; className?: string; confine?: boolean; extraCssText?: string } }

    expect(option.tooltip.appendTo).toBe('body')
    expect(option.tooltip.className).toBe('lens-echarts-tooltip')
    expect(option.tooltip.confine).toBe(true)
    expect(option.tooltip.extraCssText).toContain('z-index')
  })

  it('never ramps a heat cell down to the card it is drawn on', () => {
    const option = buildDistributionOption(
      input('heatmap', frame, { category: 'x', series: 'y', value: 'value' }),
      theme,
    ) as {
      visualMap: { inRange: { color: string[] }; text?: string[]; orient?: string; bottom?: number }
      series: Array<{ itemStyle?: { borderColor?: string; borderWidth?: number } }>
    }

    expect(option.visualMap.inRange.color[0]).toBe(theme.divider)
    expect(option.visualMap.inRange.color[0]).not.toBe(theme.card)
    // The legend states what a colour is worth, and stands under the plot
    // rather than over it.
    expect(option.visualMap.text).toEqual(['9', '0'])
    expect(option.visualMap.orient).toBe('horizontal')
    expect(option.series[0]?.itemStyle).toMatchObject({ borderColor: theme.card, borderWidth: 1 })
  })

  it('draws histogram bins adjacent, with their edges shown', () => {
    const bins: Frame = {
      columns: [{ name: 'bucket', type: 'string' }, { name: 'count', type: 'number' }],
      rows: Array.from({ length: 40 }, (_, index) => [`${index}–${index + 1}`, index]),
    }
    const option = buildDistributionOption(input('histogram', bins, { category: 'bucket', value: 'count' }), theme) as {
      xAxis: { axisLabel: { rotate?: number; interval?: number } }
      series: Array<{ barCategoryGap?: number; itemStyle?: { borderColor?: string } }>
    }

    expect(option.series[0]?.barCategoryGap).toBe(0)
    expect(option.series[0]?.itemStyle?.borderColor).toBe(theme.card)
    // Forty bins cannot print forty upright labels.
    expect(option.xAxis.axisLabel.rotate).toBe(45)
    expect(option.xAxis.axisLabel.interval).toBe(3)
  })
})
