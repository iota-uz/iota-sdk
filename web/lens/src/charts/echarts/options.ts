import type { EChartsOption } from 'echarts'
import { isVisualRegression } from '../../visualRegression'
import type { ChartInput } from '../adapter'
import type { EChartsTheme } from './theme'

type ChartValue = number | '-'

interface RowPoint {
  category: string
  nodeKey?: string
  series: string
  timestamp?: number
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

function timestamp(value: unknown): number | undefined {
  if (value instanceof Date) {
    const parsed = value.getTime()
    return Number.isFinite(parsed) ? parsed : undefined
  }
  if (typeof value === 'number') return Number.isFinite(value) ? value : undefined
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : undefined
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
    timestamp: timestamp(row[categoryIndex]),
    value: chartValue(row[valueIndex]),
  }))
}

/**
 * Selection is an outline on the chosen mark, never a wash over the others.
 *
 * Fading the rest to a third of their colour turned a pick into something that
 * reads as the chart having changed — the palette shifts, percentage labels
 * printed inside the marks stop being legible, and next to the popover the
 * whole thing looks like it drilled. The mark that was clicked is named in the
 * popover; an outline is all the confirmation the plot has to carry.
 */
function dataItem(point: RowPoint, input: ChartInput, theme: EChartsTheme) {
  const selected = input.selectedKey !== undefined && point.nodeKey === input.selectedKey
  return {
    value: point.value,
    nodeKey: point.nodeKey,
    itemStyle: {
      borderColor: selected ? theme.selectedBorder : undefined,
      borderWidth: selected ? 3 : 0,
    },
  }
}

function valueFormatter(input: ChartInput) {
  const field = input.encoding.value ?? ''
  return (value: unknown) => input.format(field, value)
}

function axisValueFormatter(input: ChartInput) {
  const field = input.encoding.value ?? ''
  const resolver = input.formatAxis ?? input.format
  return (value: unknown) => resolver(field, value)
}

function tooltipValue(value: unknown): unknown {
  return Array.isArray(value) ? (value as unknown[])[1] : value
}

function timeTooltipFormatter(input: ChartInput, categoryField: string) {
  const valueField = input.encoding.value ?? ''
  return (params: unknown) => {
    const entries = Array.isArray(params) ? params : [params]
    const records = entries.filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object')
    const header = input.format(categoryField, records[0]?.axisValue)
    const lines = records.map((entry) => {
      const seriesName = text(entry.seriesName)
      const formatted = input.format(valueField, tooltipValue(entry.value))
      return seriesName ? `${seriesName}: ${formatted}` : formatted
    })
    return [header, ...lines].join('\n')
  }
}

/**
 * Tooltips render at `body` level, not inside the chart container: a panel
 * card clips its own overflow, so a tooltip anchored near the card edge was
 * cut off. From `body` ECharts flips it against the viewport instead, and the
 * pinned z-index keeps it above the expanded-panel dialog, which portals to
 * `body` too.
 */
export const tooltipZIndex = 2147483600

/** Tooltip settings shared by every chart kind. */
export function tooltipChrome(theme: EChartsTheme) {
  return {
    backgroundColor: theme.card,
    borderColor: theme.border,
    textStyle: { color: theme.text },
    appendTo: 'body',
    // Confinement is what the card used to impose; against the viewport the
    // tooltip may flip freely.
    confine: false,
    extraCssText: `z-index: ${tooltipZIndex};`,
    // A moving tooltip is unscreenshotable; VR pins it in place.
    transitionDuration: isVisualRegression() ? 0 : undefined,
  }
}

function baseOption(theme: EChartsTheme): EChartsOption {
  return {
    animation: !isVisualRegression(),
    animationDuration: 250,
    backgroundColor: 'transparent',
    color: theme.colors,
    textStyle: { color: theme.text, fontFamily: theme.fontFamily },
  }
}

/**
 * ECharts pre-rounds `params.percent` to `percentPrecision` decimals; asking
 * for more precision than any label prints keeps the single rounding step in
 * our hands.
 */
export const rawPercentPrecision = 10

/** The label a pie slice carries: one rounding, and nothing under 4%. */
export function slicePercentLabel(percent: number | undefined): string {
  const share = percent ?? 0
  return share >= 4 ? `${share.toFixed(1)}%` : ''
}

function pieOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const donut = input.kind === 'donut'
  const points = rowPoints(input)
  const fill = input.presentation?.fill === true
  const insideLabels = input.presentation?.sliceLabels === 'percent'
  // The legacy pie filled roughly 300px of card; these radii plus the taller
  // plot box below recover that presence without letting the circle touch the
  // legend or the total badge.
  const radius: [string, string] = donut
    ? (fill ? ['54%', '92%'] : ['50%', '82%'])
    : (fill ? ['0%', '92%'] : ['0%', '82%'])
  const label = insideLabels
    // Percent labels inside the slices remove the leader-line halo that
    // shrinks the plot, so the pie can fill the card.
    ? {
        position: 'inside' as const,
        color: '#ffffff',
        fontWeight: 'bold' as const,
        // Slices under 4% cannot hold a legible label; the legend below
        // still names them.
        formatter: (params: { percent?: number }) => slicePercentLabel(params.percent),
      }
    : { color: theme.text }
  return {
    ...baseOption(theme),
    tooltip: {
      trigger: 'item',
      ...tooltipChrome(theme),
      valueFormatter: valueFormatter(input),
    },
    series: [{
      type: 'pie',
      radius,
      center: ['50%', '50%'],
      selectedMode: false,
      // ECharts rounds `percent` to two decimals before handing it over, and
      // rounding again to one decimal double-rounds: 87.6459 → 87.65 → 87.7,
      // where the true value reads 87.6. Ask for the raw share and round once.
      percentPrecision: rawPercentPrecision,
      label,
      labelLine: insideLabels ? { show: false } : { lineStyle: { color: theme.border } },
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
  const categoryField = input.encoding.category ?? input.encoding.label ?? ''
  const timeAxis = !isBar && input.frame.columns.find((column) => column.name === categoryField)?.type === 'time'
  const colorByCategory = isBar && input.presentation?.colorBy === 'category'
  const barWidth = input.presentation?.barWidthPx
  const categoryColor = (category: string, index: number) =>
    theme.seriesColor(category) ?? theme.colors[index % theme.colors.length]
  const series = seriesNames.map((name) => ({
    type: isBar ? 'bar' as const : 'line' as const,
    name: name || undefined,
    barWidth: isBar && barWidth ? barWidth : undefined,
    itemStyle: { color: theme.seriesColor(name) },
    areaStyle: input.kind === 'area' ? { opacity: 0.18 } : undefined,
    showSymbol: !isBar,
    data: timeAxis
      ? points
        .filter((point): point is RowPoint & { timestamp: number } => point.series === name && point.timestamp !== undefined)
        .sort((left, right) => left.timestamp - right.timestamp)
        .map((point) => ({ ...dataItem(point, input, theme), value: [point.timestamp, point.value] }))
      : categories.map((category, index) => {
        const point = points.find((candidate) => candidate.category === category && candidate.series === name)
        if (!point) return null
        const item = dataItem(point, input, theme)
        if (!colorByCategory) return item
        return { ...item, itemStyle: { ...item.itemStyle, color: categoryColor(category, index) } }
      }),
  }))
  const categoryAxis = { type: 'category' as const, data: categories, ...axisStyle(theme) }
  const temporalAxis = {
    type: 'time' as const,
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter: (value: number) => input.format(categoryField, value) },
  }
  const valueAxis = {
    type: 'value' as const,
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter: axisValueFormatter(input), hideOverlap: true },
  }

  return {
    ...baseOption(theme),
    // In VR mode the grid inset is pinned: containLabel derives it from
    // canvas text measurement, which lands on a rounding boundary for the
    // variable font and shifts the whole plot by 1px between runs.
    grid: isVisualRegression()
      ? { left: 96, right: 32, top: 24, bottom: 32, containLabel: false }
      : { left: 16, right: 16, top: 24, bottom: 12, containLabel: true },
    tooltip: {
      trigger: 'axis',
      renderMode: timeAxis ? 'richText' : undefined,
      ...tooltipChrome(theme),
      formatter: timeAxis ? timeTooltipFormatter(input, categoryField) : undefined,
      valueFormatter: timeAxis ? undefined : formatter,
    },
    xAxis: horizontal ? valueAxis : timeAxis ? temporalAxis : categoryAxis,
    yAxis: horizontal ? categoryAxis : valueAxis,
    series,
  }
}

export function buildChartOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  if (input.kind === 'pie' || input.kind === 'donut') return pieOption(input, theme)
  return axisOption(input, theme)
}
