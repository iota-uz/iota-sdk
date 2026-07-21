import type { EChartsOption } from 'echarts'
import { describe, expect, it, vi } from 'vitest'
import type { ChartInput } from '../adapter'
import { buildChartOption, rawPercentPrecision, slicePercentLabel } from './options'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  card: '#fff',
  text: '#334155',
  mutedText: '#64748b',
  border: '#e2e8f0',
  divider: '#f1f5f9',
  selectedBorder: '#0f172a',
  fontFamily: 'Inter',
  colors: ['#2563eb', '#059669'],
  seriesColor: (name) => name === 'Revenue' ? '#059669' : undefined,
}

function input(kind: ChartInput['kind']): ChartInput {
  return {
    kind,
    frame: {
      columns: [
        { name: 'id', type: 'string' },
        { name: 'category', type: 'string' },
        { name: 'series', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['jan-revenue', 'Jan', 'Revenue', 1200],
        ['jan-cost', 'Jan', 'Cost', 700],
        ['feb-revenue', 'Feb', 'Revenue', 1500],
        ['feb-cost', 'Feb', 'Cost', 800],
      ],
    },
    encoding: { id: 'id', label: 'category', category: 'category', series: 'series', value: 'value' },
    format: (_field, value) => `$${String(value)}`,
    theme: { palette: { success: '#059669' }, series: { Revenue: 'success' } },
    selectedKey: 'feb-revenue',
  }
}

interface TestDataItem {
  name?: string
  nodeKey?: string
  itemStyle?: { borderColor?: string; borderWidth?: number; color?: string; opacity?: number }
  value?: unknown
}

interface TestSeries {
  percentPrecision?: number
  label?: { formatter?: (params: { percent?: number }) => string }
  type?: string
  name?: string
  areaStyle?: unknown
  radius?: string[]
  itemStyle?: { color?: string }
  data?: Array<TestDataItem | null>
}

interface TestAxis {
  type?: string
  data?: string[]
  axisLabel?: { formatter?: (value: unknown) => string }
}

interface TestTooltip {
  formatter?: (params: unknown) => string
  renderMode?: string
  valueFormatter?: (value: unknown) => string
}

function testOption(option: EChartsOption) {
  return option as unknown as {
    animation: boolean
    series: TestSeries[]
    tooltip: TestTooltip
    xAxis: TestAxis
    yAxis: TestAxis
  }
}

describe('slice percentages', () => {
  it('rounds the true share once, not the share ECharts already rounded', () => {
    // «Распределение риска»: 104 119 330 137 of 118 795 253 476 is 87.6459…%,
    // which reads 87.6. Rounded to ECharts' default two decimals first (87.65)
    // it reads 87.7 — the double rounding the legacy renderer never had.
    const share = (100 * 104_119_330_137) / (104_119_330_137 + 14_675_923_339)
    expect(slicePercentLabel(share)).toBe('87.6%')
    expect(slicePercentLabel(100 - share)).toBe('12.4%')
    expect(slicePercentLabel(Number(share.toFixed(2)))).toBe('87.7%')
  })

  it.each([
    [12.351, '12.4%'],
    [12.349, '12.3%'],
    [87.6459, '87.6%'],
    [87.66, '87.7%'],
    // A literal x.x5 resolves by the binary value it actually holds — 12.35 is
    // stored as 12.3499…, so one decimal reads 12.3. Go's %.1f agrees, which
    // is what keeps the two renderers printing the same number.
    [12.35, '12.3%'],
    [0.4999, ''],
    [3.99, ''],
    [4, '4.0%'],
    [undefined, ''],
  ])('formats %s as %s', (percent, expected) => {
    expect(slicePercentLabel(percent)).toBe(expected)
  })

  it('asks ECharts for an unrounded share', () => {
    const chart = testOption(buildChartOption(
      { ...input('pie'), presentation: { sliceLabels: 'percent' } },
      theme,
    ))

    expect(chart.series[0]?.percentPrecision).toBe(rawPercentPrecision)
    expect(chart.series[0]?.label?.formatter?.({ percent: 87.6459 })).toBe('87.6%')
  })
})

describe('buildChartOption', () => {
  it('disables animation in visual regression mode', () => {
    document.documentElement.dataset.lensVr = 'true'
    const chart = testOption(buildChartOption(input('bar'), theme))
    delete document.documentElement.dataset.lensVr

    expect(chart.animation).toBe(false)
  })

  it.each([
    ['pie', ['0%', '82%']],
    ['donut', ['50%', '82%']],
  ] as const)('maps %s labels, values, stable keys, and radius', (kind, radius) => {
    const chart = testOption(buildChartOption(input(kind), theme))
    const series = chart.series[0]

    expect(series?.type).toBe('pie')
    expect(series?.radius).toEqual(radius)
    expect(series?.data?.[0]).toMatchObject({ name: 'Jan', value: 1200, nodeKey: 'jan-revenue' })
    // Selection outlines the chosen mark; the rest keep their colour, so a
    // pick never reads as the chart having changed.
    expect(series?.data?.[2]).toMatchObject({ nodeKey: 'feb-revenue', itemStyle: { borderWidth: 3 } })
    expect(series?.data?.[0]?.itemStyle?.opacity).toBeUndefined()
    expect(series?.data?.[0]).toMatchObject({ itemStyle: { borderWidth: 0 } })
  })

  it('does not select id-less points when no selection exists', () => {
    const chartInput = input('bar')
    chartInput.encoding = { category: 'category', series: 'series', value: 'value' }
    chartInput.selectedKey = undefined

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series[0]?.data?.[0]).toMatchObject({
      nodeKey: undefined,
      itemStyle: { borderWidth: 0 },
    })
    expect(chart.series[0]?.data?.[0]?.itemStyle?.borderColor).toBeUndefined()
  })

  it.each([
    ['bar', 'category', 'value'],
    ['hbar', 'value', 'category'],
  ] as const)('maps %s categories and grouped series to the correct axes', (kind, xType, yType) => {
    const chart = testOption(buildChartOption(input(kind), theme))

    expect(chart.xAxis.type).toBe(xType)
    expect(chart.yAxis.type).toBe(yType)
    expect(chart.series.every((series) => series.type === 'bar')).toBe(true)
    expect(chart.series.map((series) => series.name)).toEqual(['Revenue', 'Cost'])
    expect(chart.series[0]?.data?.[1]).toMatchObject({ value: 1500, nodeKey: 'feb-revenue' })
  })

  it.each(['bar', 'line'] as const)('applies configured series brand colors to %s series', (kind) => {
    const chart = testOption(buildChartOption(input(kind), theme))

    expect(chart.series[0]?.itemStyle?.color).toBe('#059669')
  })

  it.each([
    ['line', false],
    ['area', true],
  ] as const)('maps %s to line series with the expected fill', (kind, hasArea) => {
    const chart = testOption(buildChartOption(input(kind), theme))

    expect(chart.series.every((series) => series.type === 'line')).toBe(true)
    expect(chart.series[0]?.areaStyle !== undefined).toBe(hasArea)
    expect(chart.yAxis.axisLabel?.formatter?.(1200)).toBe('$1200')
  })

  it('uses formatAxis for value-axis ticks while tooltips keep the full format', () => {
    const chartInput = input('bar')
    chartInput.format = (_field, value) => `$${String(value)}`
    chartInput.formatAxis = (_field, value) => `${String(value)} compact`

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.yAxis.axisLabel?.formatter?.(1200)).toBe('1200 compact')
    expect(chart.tooltip.valueFormatter?.(1200)).toBe('$1200')
  })

  it('falls back to format for axis ticks when formatAxis is absent', () => {
    const chart = testOption(buildChartOption(input('bar'), theme))

    expect(chart.yAxis.axisLabel?.formatter?.(1200)).toBe('$1200')
  })

  it.each(['line', 'area'] as const)('uses sorted timestamp pairs on a time axis for %s', (kind) => {
    const chartInput = input(kind)
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }
    chartInput.frame.rows = [
      ['late', '2026-04-10T00:00:00Z', 'Revenue', 300],
      ['early', '2026-01-01T00:00:00Z', 'Revenue', 100],
      ['middle', '2026-01-03T00:00:00Z', 'Revenue', 200],
    ]

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis.type).toBe('time')
    expect(chart.xAxis.data).toBeUndefined()
    expect(chart.series[0]?.data?.map((item) => item?.value)).toEqual([
      [Date.parse('2026-01-01T00:00:00Z'), 100],
      [Date.parse('2026-01-03T00:00:00Z'), 200],
      [Date.parse('2026-04-10T00:00:00Z'), 300],
    ])
  })

  it('keeps non-time line categories unchanged', () => {
    const chart = testOption(buildChartOption(input('line'), theme))

    expect(chart.xAxis).toMatchObject({ type: 'category', data: ['Jan', 'Feb'] })
    expect(chart.series[0]?.data?.map((item) => item?.value)).toEqual([1200, 1500])
  })

  it('keeps bars categorical even when the category column is time', () => {
    const chartInput = input('bar')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis).toMatchObject({ type: 'category', data: ['Jan', 'Feb'] })
  })

  it('delegates time axis and tooltip formatting to the chart input', () => {
    const chartInput = input('line')
    const format = vi.fn((field: string, value: unknown) => `${field}=${String(value)}`)
    const time = Date.parse('2026-01-01T00:00:00Z')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }
    chartInput.format = format

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis.axisLabel?.formatter?.(time)).toBe(`category=${time}`)
    expect(chart.tooltip.renderMode).toBe('richText')
    expect(chart.tooltip.formatter?.([{ axisValue: time, seriesName: 'Revenue', value: [time, 1200] }]))
      .toBe(`category=${time}\nRevenue: value=1200`)
    expect(format).toHaveBeenCalledWith('category', time)
    expect(format).toHaveBeenCalledWith('value', 1200)
  })
})
