import type { EChartsOption } from 'echarts'
import { isVisualRegression } from '../../visualRegression'
import type { ChartInput } from '../adapter'
import type { EChartsTheme } from './theme'
import { tooltipChrome } from './tooltip'

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

/**
 * The three distribution kinds arrived with a tooltip of their own — a
 * background, a border and an ink — and so were the only charts in the runtime
 * not rendered at `body` level: a histogram tooltip near the card edge was
 * clipped by the card's own overflow, sat under the expanded-panel dialog, and
 * could not be found by the adapter's tooltip cleanup, which knows the runtime's
 * tooltips by the class only `tooltipChrome` applies. Every rationale in that
 * function's doc comment applied here and none of it was in force.
 */
function chrome(theme: EChartsTheme): Pick<EChartsOption, 'animation' | 'animationDuration' | 'aria' | 'backgroundColor' | 'color' | 'textStyle' | 'tooltip'> {
  return {
    animation: !isVisualRegression(),
    animationDuration: 220,
    aria: { enabled: true },
    backgroundColor: 'transparent',
    color: theme.colors,
    textStyle: { color: theme.text, fontFamily: theme.fontFamily },
    tooltip: tooltipChrome(theme),
  }
}

function categoryAxis(data: string[], theme: EChartsTheme, extra?: Record<string, unknown>) {
  return {
    type: 'category' as const, data,
    axisLabel: { color: theme.mutedText, hideOverlap: true, ...extra },
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

/**
 * Above this many bins an axis cannot print a horizontal label per bin, and a
 * histogram routinely has thirty to fifty. Past it the labels turn and the axis
 * prints every n-th edge instead of every one.
 */
const uprightBinLabels = 12

function histogramOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  const encoding = input.encoding as DistributionEncoding
  const categoryField = encoding.category ?? encoding.label ?? ''
  const categoryIndex = index(input, categoryField)
  const valueIndex = index(input, encoding.value)
  const bins = input.frame.rows.map((row) => text(row[categoryIndex]))
  // A histogram's bars are adjacent by definition — they tile the range — so
  // they touch, and the edge between two bins is drawn rather than left as a
  // gap. Before, the 2% gap did the work of both and did neither: the bars read
  // as a categorical bar chart drawn slightly too tightly.
  const binEdge = { borderColor: theme.card, borderWidth: 1 }
  const dense = bins.length > uprightBinLabels
  return {
    ...chrome(theme), grid: { left: 12, right: 16, top: 20, bottom: 12, containLabel: true },
    tooltip: { ...chrome(theme).tooltip, trigger: 'axis', valueFormatter: (value) => input.format(encoding.value ?? '', value) },
    xAxis: categoryAxis(bins, theme, dense
      ? {
        rotate: 45,
        interval: Math.ceil(bins.length / uprightBinLabels) - 1,
        fontSize: theme.type.xs,
      }
      : undefined),
    yAxis: valueAxis(theme, (value) => (input.formatAxis ?? input.format)(encoding.value ?? '', value)),
    series: [{
      type: 'bar',
      barCategoryGap: 0,
      itemStyle: binEdge,
      data: input.frame.rows.map((row) => number(row[valueIndex]) ?? 0),
    }],
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
  const minimum = Math.min(0, ...values)
  const maximum = Math.max(0, ...values)
  const formatScale = (value: number) => (input.formatAxis ?? input.format)(encoding.value ?? '', value)
  return {
    // The scale legend moves out from over the plot and onto the floor of the
    // panel, where the map's already is; the plot gets the 64px back.
    ...chrome(theme), grid: { left: 12, right: 16, top: 20, bottom: 28, containLabel: true },
    tooltip: {
      ...chrome(theme).tooltip,
      formatter: (raw: unknown) => {
        const params = raw as { data?: [number, number, number] }
        const cell = params.data ?? [0, 0, 0]
        return `${y[cell[1]]} × ${x[cell[0]]}<br/>${input.format(encoding.value ?? '', cell[2])}`
      },
    },
    xAxis: categoryAxis(x, theme), yAxis: categoryAxis(y, theme),
    // The ramp used to start at `theme.card`: the low end of the scale was
    // literally the panel background, so the quietest cells were
    // indistinguishable from cells that were never filled and the matrix had no
    // visible grid at all. It starts at the divider — the same floor the map's
    // choropleth ramps from — and the endpoints are printed and formatted, so a
    // colour on this legend can be read as a quantity.
    visualMap: {
      type: 'continuous', min: minimum, max: maximum, calculable: false,
      orient: 'horizontal', left: 'center', bottom: 0, itemWidth: 12, itemHeight: 96,
      show: minimum !== maximum,
      text: [formatScale(maximum), formatScale(minimum)],
      textStyle: { color: theme.mutedText, fontSize: theme.type.xs },
      inRange: { color: [theme.divider, input.theme.palette.accent ?? theme.colors[0] ?? theme.accent] },
    },
    series: [{
      type: 'heatmap',
      data,
      // A one-pixel card-coloured gutter is what makes a heatmap read as a
      // matrix of cells rather than as a continuous wash.
      itemStyle: { borderColor: theme.card, borderWidth: 1 },
      label: { show: input.presentation?.dataLabels === true },
    }],
  }
}

export function buildDistributionOption(input: DistributionInput, theme: EChartsTheme): EChartsOption {
  if (input.kind === 'histogram') return histogramOption(input, theme)
  if (input.kind === 'boxplot') return boxPlotOption(input, theme)
  return heatmapOption(input, theme)
}
