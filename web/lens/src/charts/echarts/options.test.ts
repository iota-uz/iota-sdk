import type { EChartsOption } from 'echarts'
import { describe, expect, it, vi } from 'vitest'
import type { ChartInput } from '../adapter'
import { fallbackMarkKey } from '../keys'
import { buildChartOption, donutSliceLabel, middleEllipsis, niceLogMaximum, rawPercentPrecision, slicePercentLabel } from './options'
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
  ringKey?: string
  categoryKey?: string
  share?: number
  remainder?: boolean
  symbol?: string
}

interface TestSeries {
  id?: string
  percentPrecision?: number
  minAngle?: number
  label?: {
    show?: boolean
    position?: string
    formatter?: (params: { percent?: number; name?: string; value?: unknown; data?: { share?: number } }) => string
  }
  labelLayout?: { hideOverlap?: boolean }
  type?: string
  name?: string
  stack?: string
  silent?: boolean
  tooltip?: { show?: boolean }
  lineStyle?: { type?: string }
  endLabel?: { formatter?: string }
  areaStyle?: unknown
  radius?: string[]
  itemStyle?: { color?: string }
  data?: Array<TestDataItem | null>
  markLine?: {
    data?: Array<{ name?: string; xAxis?: unknown; yAxis?: number; lineStyle?: { type?: string }; label?: { formatter?: string } }>
  }
}

interface TestAxis {
  type?: string
  logBase?: number
  max?: number
  show?: boolean
  data?: string[]
  axisLabel?: { formatter?: (value: unknown) => string }
  triggerEvent?: boolean
}

interface TestTooltip {
  formatter?: (params: unknown) => string
  renderMode?: string
  valueFormatter?: (value: unknown) => string
}

interface TestGraphic {
  children?: Array<{ style?: { text?: string } }>
}

function testOption(option: EChartsOption) {
  return option as unknown as {
    animation: boolean
    series: TestSeries[]
    tooltip: TestTooltip
    xAxis: TestAxis
    yAxis: TestAxis
    grid?: { right?: number; containLabel?: boolean }
    media?: Array<{ query?: { maxWidth?: number }, option?: { series?: TestSeries[] } }>
    graphic?: TestGraphic[]
    dataZoom?: Array<{ type?: string }>
  }
}

function logarithmicInput(kind: 'bar' | 'hbar'): ChartInput {
  const chartInput = input(kind)
  chartInput.frame.rows = [
    ['tiny', 'Tiny', 'Revenue', 2],
    ['middle', 'Middle', 'Revenue', 50],
    ['large', 'Large', 'Revenue', 1200],
  ]
  chartInput.valueAxis = { scale: 'logarithmic', logBase: 10 }
  return chartInput
}

describe('slice percentages', () => {
  it('rounds the true share once, not the share ECharts already rounded', () => {
    // «Распределение риска»: 104 119 330 137 of 118 795 253 476 is 87.6459…%,
    // which reads 87.6. Rounded to ECharts' default two decimals first (87.65)
    // it reads 87.7, exposing the double rounding this helper prevents.
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
    // stored as 12.3499…, so one decimal reads 12.3. The server's %.1f agrees.
    [12.35, '12.3%'],
    [0.4999, ''],
    [3.99, ''],
    [4, '4.0%'],
    [undefined, ''],
  ])('formats %s as %s', (percent, expected) => {
    expect(slicePercentLabel(percent)).toBe(expected)
  })

  it('renders explicit partition rings with stable composite mark identity', () => {
    const chartInput = input('radial')
    chartInput.frame = {
      columns: chartInput.frame.columns,
      rows: [
        ['north', 'North', 'actual', 60],
        ['south', 'South', 'actual', 40],
        ['north', 'North', 'plan', 55],
        ['south', 'South', 'plan', 45],
      ],
    }
    chartInput.radial = {
      mode: 'partition',
      rings: [
        { key: 'plan', label: 'Plan', order: 2, total: 100 },
        { key: 'actual', label: 'Actual', order: 1, total: 100 },
      ],
    }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series.map((series) => series.id)).toEqual(['actual', 'plan'])
    // Equal bands around a hub that stays clear — the hub is where the total
    // badge sits, and a ring thicker than the hole reads as a filled disc.
    expect(chart.series.map((series) => series.radius)).toEqual([
      ['66.5%', '90%'],
      ['40%', '63.5%'],
    ])
    expect(chart.series[1]?.data?.[0]).toMatchObject({
      name: 'North',
      value: 55,
      nodeKey: 'radial:["plan","north"]',
      ringKey: 'plan',
      categoryKey: 'north',
    })
    expect(chart.series[0]?.data?.[0]?.itemStyle?.color).toBe(chart.series[1]?.data?.[0]?.itemStyle?.color)
  })

  it('never draws a sub-percent ring slice as an unclickable hairline', () => {
    // «Накопленная премия»: 99.1% collected against 0.9% still owed. The owed
    // share is the reason the ring exists, and inside the arc it has nowhere
    // to print and nothing to click.
    const chartInput = input('radial')
    chartInput.presentation = { sliceLabels: 'percent' }
    chartInput.frame = {
      columns: chartInput.frame.columns,
      rows: [
        ['collected', 'Собрано', 'payment', 99.1],
        ['receivable', 'Дебиторка', 'payment', 0.9],
      ],
    }
    chartInput.radial = { mode: 'partition', rings: [{ key: 'payment', label: 'Оплата', order: 1, total: 100 }] }

    const chart = testOption(buildChartOption(chartInput, theme))
    const [, thin] = (chart.series[0]?.data ?? []) as TestDataItem[]

    // Without a floor the 0.9% arc is a sub-pixel line — invisible, and
    // unclickable as a drill target. The widening is bounded well below the 4%
    // floor where slices start carrying their own labels, and the true share
    // still travels on the data item for the legend and the tooltip to print.
    expect(chart.series[0]?.minAngle).toBeGreaterThan(0)
    expect(chart.series[0]?.minAngle).toBeLessThanOrEqual(2)
    expect(thin?.share).toBeCloseTo(0.9, 5)
  })

  it('uses one preferred partition ring below 220px', () => {
    const chartInput = input('radial')
    chartInput.radial = {
      mode: 'partition',
      rings: [
        { key: 'Revenue', label: 'Revenue', order: 1, total: 2700 },
        { key: 'Cost', label: 'Cost', order: 2, total: 1500 },
      ],
    }

    const chart = testOption(buildChartOption(chartInput, theme))
    const responsive = chart.media?.[0]

    expect(responsive?.query?.maxWidth).toBe(220)
    expect(responsive?.option?.series?.[0]?.data).toHaveLength(2)
    expect(responsive?.option?.series?.[1]?.data).toEqual([])
  })

  it('renders progress arcs against an explicit maximum without assuming 100', () => {
    const chartInput = input('radial')
    chartInput.frame = {
      columns: chartInput.frame.columns,
      rows: [
        ['revenue', 'Revenue', '', 1200],
        ['cost', 'Cost', '', 700],
      ],
    }
    chartInput.radial = { mode: 'progress', max: 2000 }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series).toHaveLength(2)
    expect(chart.series[0]?.data?.[0]).toMatchObject({ nodeKey: 'revenue', value: 1200 })
    expect(chart.series[0]?.data?.[1]).toMatchObject({ remainder: true, value: 800 })
    expect(chart.series[1]?.data?.[1]).toMatchObject({ remainder: true, value: 1300 })
  })

  it('asks ECharts for an unrounded share', () => {
    const chart = testOption(buildChartOption(
      { ...input('pie'), presentation: { sliceLabels: 'percent' } },
      theme,
    ))

    expect(chart.series[0]?.percentPrecision).toBe(rawPercentPrecision)
    expect(chart.series[0]?.label?.formatter?.({ percent: 87.6459 })).toBe('87.6%')
  })

  it('measures slice shares against the frame total, not the rows', () => {
    const source = input('pie')
    const chart = testOption(buildChartOption(
      // Rows sum to 4200; the producer says the whole is 8400, because half of
      // it was collapsed into a bucket that is not in this frame.
      { ...source, frame: { ...source.frame, total: 8400 }, presentation: { sliceLabels: 'percent' } },
      theme,
    ))

    expect(chart.series[0]?.data?.[0]).toMatchObject({ share: 14.3 })
  })

  it('prefers our share over the share ECharts computed', () => {
    const chart = testOption(buildChartOption(
      { ...input('pie'), presentation: { sliceLabels: 'percent' } },
      theme,
    ))

    expect(chart.series[0]?.label?.formatter?.({ percent: 87.6459, data: { share: 12.5 } })).toBe('12.5%')
  })

  it('renders real-shaped expense amounts as shares of their truthful money total', () => {
    const total = 45_561_778_243.57
    const chartInput: ChartInput = {
      ...input('donut'),
      frame: {
        columns: [
          { name: 'ring_id', type: 'string' },
          { name: 'category_id', type: 'string' },
          { name: 'category', type: 'string' },
          { name: 'amount', type: 'number' },
        ],
        rows: [
          ['components', 'acquisition_cost', 'Acquisition', 1_068_717_254.18],
          ['components', 'operating_expenses', 'Operating', 29_812_997_444.63],
          ['components', 'reinsurance_cost', 'Reinsurance', 14_680_063_544.76],
        ],
        total,
      },
      encoding: { id: 'category_id', label: 'category', value: 'amount' },
      format: (_field, value) => `${(Number(value) / 1_000_000_000).toFixed(2)} млрд UZS`,
      presentation: { sliceLabels: 'percent' },
    }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series[0]?.data).toEqual(expect.arrayContaining([
      expect.objectContaining({ nodeKey: 'acquisition_cost', value: 1_068_717_254.18, share: 2.4 }),
      expect.objectContaining({ nodeKey: 'operating_expenses', value: 29_812_997_444.63, share: 65.4 }),
      expect.objectContaining({ nodeKey: 'reinsurance_cost', value: 14_680_063_544.76, share: 32.2 }),
    ]))
    expect(chart.series[0]?.label?.formatter?.({ data: { share: 65.4 } })).toBe('65.4%')
    expect(chart.graphic?.[0]?.children?.[1]?.style?.text).toBe('45.56 млрд UZS')
  })

  it('writes the category on the slice when asked, and only when it fits', () => {
    const chart = testOption(buildChartOption(
      { ...input('pie'), presentation: { sliceLabels: 'label' } },
      theme,
    ))
    const formatter = chart.series[0]?.label?.formatter

    expect(formatter?.({ name: '2024', data: { share: 28.6 } })).toBe('2024')
    // Too narrow an arc to hold any label at all.
    expect(formatter?.({ name: '2024', data: { share: 1.2 } })).toBe('')
    // Wide enough for a year, nowhere near enough for a product name.
    expect(formatter?.({ name: 'Обязательное страхование гражданской ответственности', data: { share: 5 } })).toBe('')
  })

  it('picks readable ink for the slice label from the slice fill', () => {
    const chart = testOption(buildChartOption(
      // `seriesColor` gives Revenue a dark green and Cost nothing, so the two
      // rows exercise both branches.
      { ...input('pie'), presentation: { sliceLabels: 'percent' } },
      theme,
    ))

    expect(chart.series[0]?.data?.[0]).toMatchObject({ label: { color: '#ffffff' } })
  })

  it('marks which slices expand and which are leaves', () => {
    const chart = testOption(buildChartOption(
      { ...input('pie'), expandable: (key: string) => key === 'jan-revenue' },
      theme,
    ))

    expect(chart.series[0]?.data?.[0]).toMatchObject({ cursor: 'pointer' })
    expect(chart.series[0]?.data?.[1]).toMatchObject({ cursor: 'default' })
  })

  it('measures each ring against its own declared total', () => {
    const source = input('radial')
    const chart = testOption(buildChartOption(
      {
        ...source,
        radial: {
          mode: 'partition',
          rings: [
            { key: 'Revenue', label: 'Revenue', order: 0, total: 5400 },
            { key: 'Cost', label: 'Cost', order: 1, total: 1500 },
          ],
        },
        presentation: { sliceLabels: 'percent' },
      },
      theme,
    ))

    // 1200 of a 5400 ring, not of the 2700 the two revenue rows sum to.
    expect(chart.series[0]?.data?.[0]).toMatchObject({ share: 22.2 })
    expect(chart.series[1]?.data?.[0]).toMatchObject({ share: 46.7 })
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

  it('gives every id-less multi-series mark a stable selectable key', () => {
    const chartInput = input('bar')
    chartInput.encoding = { category: 'category', series: 'series', value: 'value' }
    chartInput.selectedKey = undefined

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series[0]?.data?.[0]).toMatchObject({
      nodeKey: fallbackMarkKey('Jan', 'Revenue'),
      itemStyle: { borderWidth: 0 },
    })
    expect(chart.series[1]?.data?.[0]).toMatchObject({ nodeKey: fallbackMarkKey('Jan', 'Cost') })
    expect(chart.series[0]?.data?.[0]?.nodeKey).not.toBe(chart.series[1]?.data?.[0]?.nodeKey)
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

  it('uses the requested logarithmic value axis and base', () => {
    const chart = testOption(buildChartOption(logarithmicInput('hbar'), theme))

    expect(chart.xAxis.type).toBe('log')
    expect(chart.xAxis.logBase).toBe(10)
    expect(chart.xAxis.max).toBe(1200)
    expect(chart.yAxis.type).toBe('category')
    expect(chart.series[0]?.label).toMatchObject({ show: true, position: 'right' })
    expect(chart.series[0]?.label?.formatter?.({ value: 1200 })).toBe('$1200')
    expect(chart.series[0]?.labelLayout).toEqual({ hideOverlap: true })
  })

  it('keeps both ends of long horizontal category labels recoverable', () => {
    expect(middleEllipsis('Группа с очень длинным уточняющим названием для отчёта', 24)).toMatch(/^Группа с оче….*отчёта$/)
    expect(niceLogMaximum(1700)).toBe(1800)
  })

  it('does not interpret the ECharts category index as an ellipsis width', () => {
    const chart = testOption(buildChartOption(input('hbar'), theme))
    const formatter = chart.yAxis.axisLabel?.formatter
    const echartsFormatter = formatter as ((value: string, index: number) => string) | undefined
    expect(echartsFormatter?.('Apr', 0)).toBe('Apr')
    expect(echartsFormatter?.('Mar', 1)).toBe('Mar')
  })

  it('suppresses a fabricated value axis when every value is zero', () => {
    const chartInput = input('bar')
    chartInput.frame.rows = chartInput.frame.rows.map((row) => [...row.slice(0, 3), 0])
    const chart = testOption(buildChartOption(chartInput, theme))
    expect(chart.yAxis.show).toBe(false)
  })

  it('collapses a degenerate map domain to one swatch and value', () => {
    const chartInput = input('map')
    chartInput.frame.rows = [['north', 'North', 'Revenue', 42]]
    chartInput.map = {
      name: 'regions', featureProperty: 'code',
      geoJSON: { type: 'FeatureCollection', features: [{ type: 'Feature', properties: { code: 'north' }, geometry: { type: 'Polygon', coordinates: [] } }] },
    }
    const chart = buildChartOption(chartInput, theme) as Record<string, unknown>
    expect(chart.visualMap).toMatchObject({ show: false, min: 42, max: 42 })
    expect(chart.graphic).toBeDefined()
  })

  it('puts logarithmic vertical-bar values above their columns', () => {
    const chart = testOption(buildChartOption(logarithmicInput('bar'), theme))

    expect(chart.series[0]?.label).toMatchObject({ show: true, position: 'top' })
    expect(chart.series[0]?.label?.formatter?.({ value: 1200 })).toBe('$1200')
  })

  it('keeps data labels off ordinary linear bars', () => {
    const chart = testOption(buildChartOption(input('bar'), theme))

    expect(chart.series.every((series) => series.label === undefined)).toBe(true)
  })

  it.each(['bar', 'line'] as const)('enables formatted data labels for %s only when requested', (kind) => {
    const chartInput = input(kind)
    chartInput.presentation = { dataLabels: true }
    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series.every((series) => series.label?.show === true)).toBe(true)
    expect(chart.series[0]?.label?.formatter?.({ value: 1200 })).toBe('$1200')
  })

  it('places neighbouring line-series labels on opposite sides of their marks', () => {
    const chartInput = input('line')
    chartInput.presentation = { dataLabels: true }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series[0]?.label?.position).toBe('top')
    expect(chart.series[1]?.label?.position).toBe('bottom')
  })

  it('adds a muted prior-period series and zoom controls to time series', () => {
    const chartInput = input('line')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }
    chartInput.frame.columns.push({ name: 'previous', type: 'number' })
    chartInput.frame.rows = chartInput.frame.rows.map((row, index) => [row[0], `2026-0${index + 1}-01`, row[2], row[3], 900 + index * 100])
    chartInput.encoding = { ...chartInput.encoding, previous: 'previous' }
    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series[0]?.name).toContain('Previous')
    expect(chart.series[0]?.silent).toBe(true)
    expect(chart.series[0]?.lineStyle).toMatchObject({ type: 'dashed' })
    expect(chart.dataZoom?.map((zoom) => zoom.type)).toEqual(['inside', 'slider'])
  })

  it('falls back from log to linear for fewer than three categories or less than 100x spread', () => {
    const few = input('hbar')
    few.valueAxis = { scale: 'logarithmic', logBase: 10 }
    expect(testOption(buildChartOption(few, theme)).xAxis.type).toBe('value')

    const narrow = logarithmicInput('hbar')
    narrow.frame.rows = [
      ['a', 'A', 'Revenue', 10], ['b', 'B', 'Revenue', 20], ['c', 'C', 'Revenue', 99],
    ]
    expect(testOption(buildChartOption(narrow, theme)).xAxis.type).toBe('value')
  })

  it('makes category labels emit hover events for their full-name tooltip', () => {
    const chart = testOption(buildChartOption(input('hbar'), theme))
    expect(chart.yAxis.triggerEvent).toBe(true)
  })

  it('falls back to an available label column when the declared category is absent', () => {
    const chartInput = input('hbar')
    chartInput.encoding = { ...chartInput.encoding, category: 'future_category', label: 'category' }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.yAxis.data).toEqual(['Jan', 'Feb'])
    expect(chart.series[0]?.data).toHaveLength(2)
  })

  it('prints donut value and share labels and the total in its center', () => {
    const chartInput = input('donut')
    chartInput.tooltipTotalLabel = 'Итого'
    const chart = testOption(buildChartOption(chartInput, theme))
    const first = chart.series[0]?.data?.[0]

    expect(donutSliceLabel({ name: first?.name, data: first }, chartInput)).toBe('Jan\n$1200 · 28.6%')
    expect(chart.graphic).toHaveLength(1)
    expect(chart.tooltip.formatter?.({ name: 'Jan', data: first })).toContain('$1200 (28.6%)')
  })

  it('keeps outside donut labels readable in a narrow plot', () => {
    const chartInput = input('donut')
    chartInput.viewportWidth = 480
    const chart = testOption(buildChartOption(chartInput, theme))
    const first = chart.series[0]?.data?.[0]

    expect(donutSliceLabel({ name: first?.name, data: first }, chartInput)).toBe('Jan')
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

  it('does not expose ECharts synthetic series names for a single-series chart', () => {
    const chartInput = input('bar')
    chartInput.encoding = { id: 'id', category: 'category', value: 'value' }
    const chart = testOption(buildChartOption(chartInput, theme))

    const tooltip = chart.tooltip.formatter?.([
      { axisValueLabel: 'Jan', marker: '<span class="marker"></span>', seriesName: 'series0', value: 1200 },
    ]) ?? ''

    expect(tooltip).toContain('Jan')
    expect(tooltip).toContain('$1200')
    expect(tooltip).not.toContain('series0')
  })

  it('keeps numeric-looking categorical years literal and marks an incomplete period', () => {
    const chartInput = input('bar')
    chartInput.frame.rows = [['2025', '2025', 'Revenue', 1200]]
    chartInput.temporal = { period: { category: '2025', state: 'ytd', label: 'YTD' } }
    const chart = testOption(buildChartOption(chartInput, theme))

    const tooltip = chart.tooltip.formatter?.([
      { axisValueLabel: '2025', seriesName: 'Revenue', value: 1200 },
    ]) ?? ''

    expect(tooltip).toContain('2025 · YTD')
    expect(tooltip).not.toContain('2,025')
    expect(chart.series[0]?.data?.[0]?.itemStyle).toMatchObject({ borderWidth: 2 })
  })

  it('omits zero-valued stack entries and restores the localized column total', () => {
    const chartInput = input('bar')
    chartInput.presentation = { stack: true }
    chartInput.tooltipTotalLabel = 'Итого'
    const chart = testOption(buildChartOption(chartInput, theme))

    const tooltip = chart.tooltip.formatter?.([
      { axisValueLabel: 'Jan', marker: '<span class="marker"></span>', seriesName: 'Revenue', value: 1200 },
      { axisValueLabel: 'Jan', marker: '<span class="marker"></span>', seriesName: 'Cost', value: 0 },
    ]) ?? ''

    expect(tooltip).toContain('Jan')
    expect(tooltip).toContain('Revenue')
    expect(tooltip).not.toContain('Cost')
    expect(tooltip).toContain('Итого')
    expect(tooltip).toContain('$1200')
  })

  it('does not add an overlaid line series to a stacked column total', () => {
    const chartInput = input('bar')
    chartInput.presentation = { stack: true, lineSeries: ['Cost'] }
    chartInput.tooltipTotalLabel = 'Итого'
    const chart = testOption(buildChartOption(chartInput, theme))

    const tooltip = chart.tooltip.formatter?.([
      { axisValueLabel: 'Jan', seriesName: 'Revenue', value: 1200 },
      { axisValueLabel: 'Jan', seriesName: 'Cost', value: 700 },
    ]) ?? ''

    expect(tooltip).toContain('Cost')
    expect(tooltip).toContain('Итого')
    expect(tooltip).not.toContain('$1900')
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
    chartInput.categoryFormatDefined = true

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis.axisLabel?.formatter?.(time)).toBe(`category=${time}`)
    expect(chart.tooltip.renderMode).toBe('richText')
    expect(chart.tooltip.formatter?.([{ axisValue: time, seriesName: 'Revenue', value: [time, 1200] }]))
      .toBe(`category=${time}\nRevenue: value=1200`)
    expect(format).toHaveBeenCalledWith('category', time)
    expect(format).toHaveBeenCalledWith('value', 1200)
  })

  it('uses a readable calendar label when a time field has no explicit format', () => {
    const chartInput = input('line')
    const time = Date.parse('2026-01-01T00:00:00Z')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis.axisLabel?.formatter?.(time)).toBe('Jan 2026')
    expect(chart.tooltip.formatter?.([{ axisValue: time, seriesName: 'Revenue', value: [time, 1200] }]))
      .toContain('Jan 2026')
  })

  it('reserves enough right and bottom inset for labels on vertical bars', () => {
    const chartInput = input('bar')

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.xAxis).toMatchObject({ type: 'category' })
    expect(chart.grid).toMatchObject({ right: 64, bottom: 32, containLabel: true })
  })

  it('omits zero-valued series from time-axis tooltips too', () => {
    const chartInput = input('line')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }
    const chart = testOption(buildChartOption(chartInput, theme))

    const tooltip = chart.tooltip.formatter?.([
      { axisValue: 1, seriesName: 'Revenue', value: [1, 1200] },
      { axisValue: 1, seriesName: 'Cost', value: [1, 0] },
    ]) ?? ''

    expect(tooltip).toContain('Revenue')
    expect(tooltip).not.toContain('Cost')
  })

  it('omits an all-zero time-axis tooltip', () => {
    const chartInput = input('line')
    chartInput.frame.columns[1] = { name: 'category', type: 'time' }
    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.tooltip.formatter?.([
      { axisValue: 1, seriesName: 'Revenue', value: [1, 0] },
      { axisValue: 1, seriesName: 'Cost', value: [1, 0] },
    ])).toBe('')
  })

  it('stacks the parts of a whole and keeps another basis beside them', () => {
    const chartInput = input('bar')
    chartInput.presentation = { stack: true, lineSeries: ['Cost'] }

    const chart = testOption(buildChartOption(chartInput, theme))

    // 'Revenue' is a segment of the column; 'Cost' is measured on another
    // basis, so it runs over the stack instead of claiming to be part of it.
    expect(chart.series.map((series) => [series.type, series.stack])).toEqual([
      ['bar', 'total'],
      ['line', undefined],
    ])
  })

  it('draws each series in the colour its legend prints', () => {
    // A dashboard pins colours to the n-th series of a panel, and only the
    // panel can resolve those pins. Left to ECharts the lines walk a default
    // palette, and the legend beside them names a different colour.
    const chartInput = input('line')
    chartInput.seriesColor = (_label, index) => ['#111111', '#222222'][index]

    const chart = testOption(buildChartOption(chartInput, theme))

    // The panel resolver is authoritative — it is the very function the
    // legend calls — so both series take its colour even though the theme
    // also names 'Revenue' directly; a raw theme lookup only applies when the
    // panel has no resolver at all.
    expect(chart.series.map((series) => series.itemStyle?.color)).toEqual(['#111111', '#222222'])
  })

  it('renders temporal readability overlays as distinct, labelled chart layers', () => {
    const chartInput = input('line')
    chartInput.frame.columns.push(
      { name: 'regression', type: 'number' },
      { name: 'sma_3', type: 'number' },
      { name: 'annualized', type: 'number' },
      { name: 'forecast', type: 'number' },
      { name: 'forecast_lower', type: 'number' },
      { name: 'forecast_upper', type: 'number' },
    )
    chartInput.frame.rows = chartInput.frame.rows.map((row, index) => [
      ...row,
      1100 + index * 100,
      index < 2 ? null : 1000 + index * 100,
      index === 2 ? 1700 : null,
      index >= 2 ? 1600 + index * 100 : null,
      index >= 2 ? 1450 + index * 100 : null,
      index >= 2 ? 1750 + index * 100 : null,
    ])
    chartInput.temporal = {
      regression: { field: 'regression', label: 'Trend' },
      movingAverages: [{ window: 3, field: 'sma_3', label: 'SMA 3' }],
      referenceLines: [{ value: 1400, label: 'Threshold' }],
      period: { category: 'Feb', state: 'annualized', label: 'Estimate', annualizedField: 'annualized' },
      annotations: [{ at: 'Feb', label: 'Method changed' }],
      forecast: {
        start: 'Feb', valueField: 'forecast', lowerField: 'forecast_lower', upperField: 'forecast_upper', label: 'Projection',
      },
    }

    const chart = testOption(buildChartOption(chartInput, theme))

    expect(chart.series.map((series) => series.name)).toEqual(expect.arrayContaining([
      'Revenue · Trend', 'Revenue · SMA 3', 'Estimate', 'Revenue · Projection', 'Projection confidence',
    ]))
    expect(chart.series.find((series) => series.name === 'Revenue · Trend')?.lineStyle).toMatchObject({ type: 'dashed' })
    expect(chart.series.find((series) => series.name === 'Revenue · SMA 3')?.lineStyle).not.toMatchObject({ type: 'dashed' })
    expect(chart.series[0]?.markLine?.data).toEqual(expect.arrayContaining([
      expect.objectContaining({ name: 'Threshold', yAxis: 1400, lineStyle: { type: 'dashed', color: theme.mutedText, width: 1.5 } }),
      expect.objectContaining({ name: 'Method changed', xAxis: 'Feb', lineStyle: { type: 'dotted', color: theme.mutedText, width: 1.5 } }),
    ]))
    expect(chart.series.some((series) => series.name === 'Threshold' || series.name === 'Method changed')).toBe(false)
    expect(chart.series.find((series) => series.name === 'Projection lower')?.tooltip).toEqual({ show: false })
    expect(chart.series.find((series) => series.name === 'Projection confidence')?.tooltip).toEqual({ show: false })
    expect(chart.series[0]?.data?.[1]).toMatchObject({ symbol: 'emptyCircle', label: { formatter: 'Estimate' } })
  })

  it('uses localized generated temporal labels when producer labels are absent', () => {
    const chartInput = input('line')
    chartInput.encoding.previous = 'previous'
    chartInput.frame.columns.push(
      { name: 'previous', type: 'number' },
      { name: 'regression', type: 'number' },
      { name: 'sma', type: 'number' },
      { name: 'forecast', type: 'number' },
      { name: 'lower', type: 'number' },
      { name: 'upper', type: 'number' },
      { name: 'annualized', type: 'number' },
    )
    chartInput.frame.rows = chartInput.frame.rows.map((row) => [...row, 1, 2, 3, 4, 3, 5, 6])
    chartInput.labels = {
      previous: 'Прошлый', trend: 'Тренд', movingAverage: (window) => `Сск ${window}`,
      estimate: 'Оценка', ytd: 'С начала года', forecast: 'Прогноз',
      forecastLower: (forecast) => `${forecast}: низ`, forecastConfidence: (forecast) => `${forecast}: интервал`,
      boxplot: ['Мин', 'Q1', 'Медиана', 'Q3', 'Макс'],
    }
    chartInput.temporal = {
      regression: { field: 'regression' }, movingAverages: [{ window: 3, field: 'sma' }],
      period: { category: 'Feb', state: 'annualized', annualizedField: 'annualized' },
      forecast: { start: 'Feb', valueField: 'forecast', lowerField: 'lower', upperField: 'upper' },
    }

    const chart = testOption(buildChartOption(chartInput, theme))
    expect(chart.series.map((series) => series.name)).toEqual(expect.arrayContaining([
      'Revenue · Прошлый', 'Revenue · Тренд', 'Revenue · Сск 3', 'Оценка',
      'Revenue · Прогноз', 'Прогноз: низ', 'Прогноз: интервал',
    ]))
  })

  it('paints pie slices from the panel resolver when the theme names no colour', () => {
    // A slice is a row, so it resolves through the row-indexed half of the
    // panel's resolver — the same one its legend swatches come from.
    const chartInput = input('pie')
    chartInput.rowColor = (_label, index) => ['#111111', '#222222', '#333333', '#444444'][index]

    const chart = testOption(buildChartOption(chartInput, theme))
    const colors = (chart.series[0]?.data ?? []).map((item) => item?.itemStyle?.color)

    expect(colors).toEqual(['#111111', '#222222', '#333333', '#444444'])
  })
})
