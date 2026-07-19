import type { EChartsOption } from 'echarts'
import type { ChartInput } from '../adapter'
import type { EChartsTheme } from './theme'

type ChartValue = number | '-'

interface RowPoint {
  category: string
  nodeKey?: string
  series: string
  value: ChartValue
}

function columnIndex(input: ChartInput, field: string | undefined): number {
  return field ? input.frame.columns.findIndex((column) => column.name === field) : -1
}

function text(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return ''
}

function chartValue(value: unknown): ChartValue {
  if (typeof value === 'number') return Number.isFinite(value) ? value : '-'
  if (typeof value === 'string') {
    const number = Number(value)
    return value.trim() !== '' && Number.isFinite(number) ? number : '-'
  }
  return '-'
}

function rowPoints(input: ChartInput): RowPoint[] {
  const categoryField = input.encoding.category ?? input.encoding.label
  const categoryIndex = columnIndex(input, categoryField)
  const valueIndex = columnIndex(input, input.encoding.value)
  const idIndex = columnIndex(input, input.encoding.id)
  const seriesIndex = columnIndex(input, input.encoding.series)

  return input.frame.rows.map((row) => ({
    category: text(row[categoryIndex]),
    nodeKey: idIndex >= 0 ? text(row[idIndex]) || undefined : undefined,
    series: seriesIndex >= 0 ? text(row[seriesIndex]) : '',
    value: chartValue(row[valueIndex]),
  }))
}

function dataItem(point: RowPoint, input: ChartInput, theme: EChartsTheme) {
  const dimmed = input.selectedKey !== undefined && point.nodeKey !== input.selectedKey
  const selected = input.selectedKey !== undefined && point.nodeKey === input.selectedKey
  return {
    value: point.value,
    nodeKey: point.nodeKey,
    itemStyle: {
      opacity: dimmed ? 0.35 : 1,
      borderColor: selected ? theme.selectedBorder : undefined,
      borderWidth: selected ? 2 : 0,
    },
  }
}

function valueFormatter(input: ChartInput) {
  const field = input.encoding.value ?? ''
  return (value: unknown) => input.format(field, value)
}

function baseOption(theme: EChartsTheme): EChartsOption {
  return {
    animationDuration: 250,
    backgroundColor: 'transparent',
    color: theme.colors,
    textStyle: { color: theme.text, fontFamily: theme.fontFamily },
  }
}

function pieOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const donut = input.kind === 'donut'
  const points = rowPoints(input)
  return {
    ...baseOption(theme),
    tooltip: {
      trigger: 'item',
      backgroundColor: theme.card,
      borderColor: theme.border,
      textStyle: { color: theme.text },
      valueFormatter: valueFormatter(input),
    },
    series: [{
      type: 'pie',
      radius: donut ? ['48%', '72%'] : ['0%', '72%'],
      selectedMode: false,
      label: { color: theme.text },
      labelLine: { lineStyle: { color: theme.border } },
      data: points.map((point) => {
        const item = dataItem(point, input, theme)
        return {
          ...item,
          name: point.category,
          itemStyle: {
            ...item.itemStyle,
            color: theme.seriesColor(point.category),
          },
        }
      }),
    }],
  }
}

function axisStyle(theme: EChartsTheme) {
  return {
    axisLabel: { color: theme.mutedText },
    axisLine: { lineStyle: { color: theme.border } },
    axisTick: { lineStyle: { color: theme.border } },
    splitLine: { lineStyle: { color: theme.divider } },
  }
}

function axisOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const points = rowPoints(input)
  const categories = [...new Set(points.map((point) => point.category))]
  const seriesNames = [...new Set(points.map((point) => point.series))]
  const formatter = valueFormatter(input)
  const isBar = input.kind === 'bar' || input.kind === 'hbar'
  const horizontal = input.kind === 'hbar'
  const series = seriesNames.map((name) => ({
    type: isBar ? 'bar' as const : 'line' as const,
    name: name || undefined,
    itemStyle: { color: theme.seriesColor(name) },
    areaStyle: input.kind === 'area' ? { opacity: 0.18 } : undefined,
    showSymbol: !isBar,
    data: categories.map((category) => {
      const point = points.find((candidate) => candidate.category === category && candidate.series === name)
      return point ? dataItem(point, input, theme) : null
    }),
  }))
  const categoryAxis = { type: 'category' as const, data: categories, ...axisStyle(theme) }
  const valueAxis = {
    type: 'value' as const,
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter },
  }

  return {
    ...baseOption(theme),
    grid: { left: 16, right: 16, top: 24, bottom: 12, containLabel: true },
    tooltip: {
      trigger: 'axis',
      backgroundColor: theme.card,
      borderColor: theme.border,
      textStyle: { color: theme.text },
      valueFormatter: formatter,
    },
    xAxis: horizontal ? valueAxis : categoryAxis,
    yAxis: horizontal ? categoryAxis : valueAxis,
    series,
  }
}

export function buildChartOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  if (input.kind === 'pie' || input.kind === 'donut') return pieOption(input, theme)
  return axisOption(input, theme)
}
