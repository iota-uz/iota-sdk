import type { EChartsOption } from 'echarts'
import { describe, expect, it } from 'vitest'
import type { ChartInput } from '../adapter'
import { buildChartOption } from './options'
import type { EChartsTheme } from './theme'

const theme: EChartsTheme = {
  mode: 'light',
  background: '#f6f7f9',
  card: '#fff',
  text: '#334155',
  mutedText: '#64748b',
  border: '#e2e8f0',
  divider: '#f1f5f9',
  accent: '#2563eb',
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
  itemStyle?: { opacity?: number }
  value?: unknown
}

interface TestSeries {
  type?: string
  name?: string
  areaStyle?: unknown
  radius?: string[]
  data?: Array<TestDataItem | null>
}

interface TestAxis {
  type?: string
  data?: string[]
  axisLabel?: { formatter?: (value: unknown) => string }
}

function testOption(option: EChartsOption) {
  return option as unknown as {
    series: TestSeries[]
    xAxis: TestAxis
    yAxis: TestAxis
  }
}

describe('buildChartOption', () => {
  it.each([
    ['pie', ['0%', '72%']],
    ['donut', ['48%', '72%']],
  ] as const)('maps %s labels, values, stable keys, and radius', (kind, radius) => {
    const chart = testOption(buildChartOption(input(kind), theme))
    const series = chart.series[0]

    expect(series?.type).toBe('pie')
    expect(series?.radius).toEqual(radius)
    expect(series?.data?.[0]).toMatchObject({ name: 'Jan', value: 1200, nodeKey: 'jan-revenue' })
    expect(series?.data?.[2]).toMatchObject({ nodeKey: 'feb-revenue', itemStyle: { opacity: 1 } })
    expect(series?.data?.[0]).toMatchObject({ itemStyle: { opacity: 0.35 } })
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

  it.each([
    ['line', false],
    ['area', true],
  ] as const)('maps %s to line series with the expected fill', (kind, hasArea) => {
    const chart = testOption(buildChartOption(input(kind), theme))

    expect(chart.series.every((series) => series.type === 'line')).toBe(true)
    expect(chart.series[0]?.areaStyle !== undefined).toBe(hasArea)
    expect(chart.yAxis.axisLabel?.formatter?.(1200)).toBe('$1200')
  })
})
