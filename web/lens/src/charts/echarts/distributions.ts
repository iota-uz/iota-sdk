import type { EChartsOption } from 'echarts'
import { isVisualRegression } from '../../visualRegression'
import type { ChartInput } from '../adapter'
import type { EChartsTheme } from './theme'

export type DistributionKind = 'histogram' | 'boxplot' | 'heatmap'
export type DistributionInput = Omit<ChartInput, 'kind'> & { kind: DistributionKind }

interface DistributionEncoding {
  label?: string
  category?: string
  series?: string
  value?: string
  lower?: string
  q1?: string
  median?: string
  q3?: string
  upper?: string
}

function index(input: DistributionInput, field: string | undefined): number {
  return field ? input.frame.columns.findIndex((column) => column.name === field) : -1
}

function text(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'bigint' ? String(value) : ''
}

function number(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

function chrome(theme: EChartsTheme): Pick<EChartsOption, 'animation' | 'animationDuration' | 'aria' | 'backgroundColor' | 'color' | 'textStyle' | 'tooltip'> {
  return {
    animation: !isVisualRegression(),
    animationDuration: 220,
    aria: { enabled: true },
    backgroundColor: 'transparent',
    color: theme.colors,
    textStyle: { color: theme.text, fontFamily: theme.fontFamily },
    tooltip: { backgroundColor: theme.card, borderColor: theme.border, textStyle: { color: theme.text } },
  }
}

function categoryAxis(data: string[], theme: EChartsTheme) {
  return {
    type: 'category' as const, data,
    axisLabel: { color: theme.mutedText, hideOverlap: true },
    axisLine: { lineStyle: { color: theme.border } },
    axisTick: { lineStyle: { color: theme.border } },
  }
}

function valueAxis(theme: EChartsTheme, formatter?: (value: unknown) => string) {
  return {
    type: 'value' as const,
    axisLabel: { color: theme.mutedText, formatter },
    splitLine: { lineStyle: { color: theme.divider } },
  }
}

function histogramOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  const encoding = input.encoding as DistributionEncoding
  const categoryField = encoding.category ?? encoding.label ?? ''
  const categoryIndex = index(input, categoryField)
  const valueIndex = index(input, encoding.value)
  return {
    ...chrome(theme), grid: { left: 12, right: 16, top: 20, bottom: 12, containLabel: true },
    tooltip: { ...chrome(theme).tooltip, trigger: 'axis', valueFormatter: (value) => input.format(encoding.value ?? '', value) },
    xAxis: categoryAxis(input.frame.rows.map((row) => text(row[categoryIndex])), theme),
    yAxis: valueAxis(theme, (value) => (input.formatAxis ?? input.format)(encoding.value ?? '', value)),
    series: [{ type: 'bar', barCategoryGap: '2%', data: input.frame.rows.map((row) => number(row[valueIndex]) ?? 0) }],
  }
}

function boxPlotOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  const encoding = input.encoding as DistributionEncoding
  const categoryField = encoding.category ?? encoding.label ?? ''
  const categoryIndex = index(input, categoryField)
  const fields = [encoding.lower, encoding.q1, encoding.median, encoding.q3, encoding.upper]
  const indexes = fields.map((field) => index(input, field))
  return {
    ...chrome(theme), grid: { left: 12, right: 16, top: 20, bottom: 12, containLabel: true },
    tooltip: {
      ...chrome(theme).tooltip, trigger: 'item',
      formatter: (raw: unknown) => {
        const params = raw as { name?: string; data?: unknown[] }
        const values = params.data ?? []
        const labels = input.labels?.boxplot ?? ['Min', 'Q1', 'Median', 'Q3', 'Max']
        return [params.name ?? '', ...values.map((value, position) => `${labels[position]}: ${input.format(fields[position] ?? '', value)}`)].join('<br/>')
      },
    },
    xAxis: categoryAxis(input.frame.rows.map((row) => text(row[categoryIndex])), theme),
    yAxis: valueAxis(theme, (value) => (input.formatAxis ?? input.format)(encoding.median ?? '', value)),
    series: [{ type: 'boxplot', data: input.frame.rows.map((row) => indexes.map((fieldIndex) => number(row[fieldIndex]) ?? 0)) }],
  }
}

function heatmapOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  const encoding = input.encoding as DistributionEncoding
  const xIndex = index(input, encoding.category)
  const yIndex = index(input, encoding.series)
  const valueIndex = index(input, encoding.value)
  const x = [...new Set(input.frame.rows.map((row) => text(row[xIndex])))]
  const y = [...new Set(input.frame.rows.map((row) => text(row[yIndex])))]
  const values = input.frame.rows.map((row) => number(row[valueIndex]) ?? 0)
  const data = input.frame.rows.map((row, rowIndex) => [x.indexOf(text(row[xIndex])), y.indexOf(text(row[yIndex])), values[rowIndex]])
  return {
    ...chrome(theme), grid: { left: 12, right: 64, top: 20, bottom: 12, containLabel: true },
    tooltip: {
      ...chrome(theme).tooltip,
      formatter: (raw: unknown) => {
        const params = raw as { data?: [number, number, number] }
        const cell = params.data ?? [0, 0, 0]
        return `${y[cell[1]]} × ${x[cell[0]]}<br/>${input.format(encoding.value ?? '', cell[2])}`
      },
    },
    xAxis: categoryAxis(x, theme), yAxis: categoryAxis(y, theme),
    visualMap: {
      min: Math.min(0, ...values), max: Math.max(0, ...values), calculable: true, orient: 'vertical', right: 0,
      textStyle: { color: theme.mutedText }, inRange: { color: [theme.card, theme.colors[0] ?? '#2563eb'] },
    },
    series: [{ type: 'heatmap', data, label: { show: input.presentation?.dataLabels === true } }],
  }
}

export function buildDistributionOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  if (input.kind === 'histogram') return histogramOption(input, theme)
  if (input.kind === 'boxplot') return boxPlotOption(input, theme)
  return heatmapOption(input, theme)
}
