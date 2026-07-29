import type { EChartsOption } from 'echarts'
import type { Presentation } from '../../contract'
import { isVisualRegression } from '../../visualRegression'
import { radialNodeKey, type ChartInput } from '../adapter'
import { distributeShares, formatShare } from '../shares'
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
function dataItem(point: RowPoint, input: ChartInput, theme: EChartsTheme, expandKey?: string) {
  const selected = input.selectedKey !== undefined && point.nodeKey === input.selectedKey
  // A ring's mark key is a composite of ring and category, so the key the
  // drill tree knows this point by has to be passed in explicitly.
  const expandable = input.expandable?.(expandKey ?? point.nodeKey ?? point.category)
  return {
    value: point.value,
    nodeKey: point.nodeKey,
    // The only cue on the plot that this segment goes somewhere. Left alone
    // when the caller has no opinion, so charts without a drill tree keep the
    // chart-wide cursor they had.
    ...(expandable === undefined ? {} : { cursor: expandable ? 'pointer' : 'default' }),
    // A stable per-mark group id lets ECharts morph the same segment across a
    // perspective switch or a drill level instead of redrawing it.
    ...(point.nodeKey ? { groupId: point.nodeKey } : {}),
    itemStyle: {
      borderColor: selected ? theme.selectedBorder : undefined,
      borderWidth: selected ? 3 : 0,
    },
  }
}

/**
 * Morphing marks between two renders is motion; it is off under visual
 * regression (where all animation is pinned) and reduced motion. Selection
 * restyles never reach this — the adapter merges them in place without
 * replacing the series — so an outline never triggers a morph.
 */
function morphEnabled(): boolean {
  if (isVisualRegression()) return false
  return !(typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches)
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

/**
 * Marks every tooltip node this runtime creates. Living outside the chart
 * container, a tooltip is no longer torn down with it — the adapter needs a way
 * to find the nodes it is responsible for without touching anything else the
 * host page appended to `body`.
 */
export const tooltipClassName = 'lens-echarts-tooltip'

/** Tooltip settings shared by every chart kind. */
export function tooltipChrome(theme: EChartsTheme) {
  return {
    backgroundColor: theme.card,
    borderColor: theme.border,
    textStyle: { color: theme.text },
    appendTo: 'body',
    className: tooltipClassName,
    // Rendered at body level the tooltip escapes the card's clip, but it must
    // still stay on screen: confine clamps it to the window viewport so a wide
    // tooltip on a left-edge slice no longer overflows behind the sidebar
    // instead of merely flipping.
    confine: true,
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
export function slicePercentLabel(percent: number | undefined, locale?: string): string {
  const share = percent ?? 0
  return share >= 4 ? formatShare(share, locale) : ''
}

/**
 * Below this share a slice is too narrow to hold text of its own. The percent
 * label and the category label share the floor: what limits both is the arc,
 * not the string.
 */
const labelledSliceMinShare = 4

/**
 * A category label on the slice instead of a percent, for dimensions whose
 * labels are guaranteed short — a year, a quarter. The producer opts in; a
 * product name on a slice would be clipped to meaninglessness.
 *
 * Long labels are still possible (a producer can mislabel a dimension), so the
 * label is dropped rather than clipped once it cannot plausibly fit the arc.
 */
export function sliceCategoryLabel(name: string | undefined, share: number | undefined): string {
  const label = (name ?? '').trim()
  if (!label || (share ?? 0) < labelledSliceMinShare) return ''
  // One character needs roughly a degree of arc to stay legible at the radii
  // these rings are drawn at; a slice narrower than its own label reads as
  // corruption, so it goes to the legend instead.
  return label.length <= Math.max(2, Math.round(((share ?? 0) / 100) * 360)) ? label : ''
}

/**
 * The whole a pie's slices are shares of: the producer's total when it shipped
 * one, otherwise the rows themselves.
 *
 * The fallback is what ECharts would have computed anyway, so a producer that
 * says nothing keeps today's behaviour.
 */
function partitionTotal(points: RowPoint[], declared: number | undefined): number | undefined {
  if (declared !== undefined && Number.isFinite(declared) && declared !== 0) return declared
  const sum = points.reduce((total, point) => total + (typeof point.value === 'number' ? point.value : 0), 0)
  return sum === 0 ? undefined : sum
}

/** Reconciled shares for a set of points, indexed the same way. */
function pointShares(points: RowPoint[], total: number | undefined): (number | undefined)[] {
  return distributeShares(
    points.map((point) => (typeof point.value === 'number' ? point.value : undefined)),
    total,
  )
}

/**
 * The share a label should print. Ours, carried on the data item, in
 * preference to ECharts' `percent`: `percent` normalizes against the series it
 * was handed, which is the sum of the visible rows rather than the whole.
 */
function labelShare(params: unknown): { name: string; share: number | undefined } {
  const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
  const data = record.data && typeof record.data === 'object' ? record.data as Record<string, unknown> : {}
  const share = typeof data.share === 'number'
    ? data.share
    : (typeof record.percent === 'number' ? record.percent : undefined)
  return { name: text(record.name), share }
}

function sliceLabel(mode: Presentation['sliceLabels'], params: unknown, locale?: string): string {
  const { name, share } = labelShare(params)
  return mode === 'label' ? sliceCategoryLabel(name, share) : slicePercentLabel(share, locale)
}

/**
 * The smallest arc a ring will draw, in degrees. Under it a slice is a
 * sub-pixel hairline: invisible, and — because drill targets are the arcs
 * themselves — unclickable. Two degrees is 0.55% of the circle, so the widening
 * is bounded well under the 4% floor where callouts take over, and the callout
 * always prints the true share beside it.
 */
const minimumSliceAngle = 2

/**
 * Relative luminance per WCAG, for choosing ink against a slice fill.
 *
 * Only `#rgb`/`#rrggbb` are parsed: chart palettes are authored as hex in the
 * document theme, and an unparseable fill falls back to the dark ink, which is
 * the safe half of the guess on this palette's light end.
 */
function luminance(color: string | undefined): number | undefined {
  const hex = (color ?? '').trim().replace(/^#/, '')
  const expanded = hex.length === 3 ? hex.split('').map((digit) => digit + digit).join('') : hex
  if (!/^[0-9a-fA-F]{6}$/.test(expanded)) return undefined
  const channel = (offset: number) => {
    const value = parseInt(expanded.slice(offset, offset + 2), 16) / 255
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * channel(0) + 0.7152 * channel(2) + 0.0722 * channel(4)
}

/**
 * Ink for a label sitting on a slice. White on the palette's darker hues, the
 * theme's own dark ink on the lighter ones — a fixed white measures below 3:1
 * on the amber and teal entries, and a category label is wider than a percent,
 * so more of it lands on the fill.
 */
/**
 * The hover card for a slice: label, amount, and the share of the same total
 * the printed labels use. Rings prefix the ring's own name, since the same
 * category appears once per ring.
 */
function sliceTooltip(params: unknown, input: ChartInput, ringLabel?: string): string {
  const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
  const data = record.data && typeof record.data === 'object' ? record.data as Record<string, unknown> : {}
  const label = text(record.name)
  const value = input.format(input.encoding.value ?? '', data.value)
  const share = typeof data.share === 'number'
    ? data.share
    : (typeof record.percent === 'number' ? record.percent : undefined)
  const suffix = share === undefined ? '' : ` (${formatShare(share, input.locale)})`
  // The tooltip body is parsed as HTML, so a newline collapses to a space and
  // the ring name runs into the category it heads.
  const heading = ringLabel ? `${ringLabel}<br/>` : ''
  return `${heading}${label}: ${value}${suffix}`
}

function sliceLabelColor(fill: string | undefined, theme: EChartsTheme): string {
  const light = luminance(fill)
  // 0.4 is where white and #0f172a-class ink cross over on this palette;
  // above it the dark ink wins by a wide margin. An unmeasurable fill keeps
  // white, which is what every slice used before this.
  return light !== undefined && light > 0.4 ? theme.text : '#ffffff'
}

function pieOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const donut = input.kind === 'donut'
  const points = rowPoints(input)
  const fill = input.presentation?.fill === true
  const sliceLabels = input.presentation?.sliceLabels
  const insideLabels = sliceLabels === 'percent' || sliceLabels === 'label'
  // Shares are resolved here, once, against the authoritative total — not left
  // to ECharts' `params.percent`, which can only ever normalize the rows it was
  // handed and so drifts as soon as anything is collapsed or hidden.
  const shares = pointShares(points, partitionTotal(points, input.frame.total))
  // The legacy pie filled roughly 300px of card; these radii plus the taller
  // plot box below recover that presence without letting the circle touch the
  // legend or the total badge.
  const radius: [string, string] = donut
    ? (fill ? ['54%', '92%'] : ['50%', '82%'])
    : (fill ? ['0%', '92%'] : ['0%', '82%'])
  const label = insideLabels
    // Labels inside the slices remove the leader-line halo that shrinks the
    // plot, so the pie can fill the card.
    ? {
        position: 'inside' as const,
        fontWeight: 'bold' as const,
        // Slices under 4% cannot hold a legible label; the legend below
        // still names them.
        formatter: (params: unknown) => sliceLabel(sliceLabels, params, input.locale),
      }
    : { color: theme.text }
  return {
    ...baseOption(theme),
    tooltip: {
      trigger: 'item',
      ...tooltipChrome(theme),
      valueFormatter: valueFormatter(input),
      // Hovering answers "how much, and how much of the whole" without
      // committing to the full segment card, which is what the click opens.
      formatter: (params: unknown) => sliceTooltip(params, input),
    },
    series: [{
      type: 'pie',
      radius,
      center: ['50%', '50%'],
      selectedMode: false,
      universalTransition: { enabled: morphEnabled() },
      // ECharts rounds `percent` to two decimals before handing it over, and
      // rounding again to one decimal double-rounds: 87.6459 → 87.65 → 87.7,
      // where the true value reads 87.6. Ask for the raw share and round once.
      percentPrecision: rawPercentPrecision,
      minAngle: minimumSliceAngle,
      label,
      labelLine: insideLabels ? { show: false } : { lineStyle: { color: theme.border } },
      data: points.map((point, index) => {
        const item = dataItem(point, input, theme)
        const fill = input.colors?.[index] ?? theme.seriesColor(point.category) ?? input.seriesColor?.(point.category, index)
        return {
          ...item,
          name: point.category,
          share: shares[index],
          // Per item rather than per series: the readable ink depends on the
          // slice's own fill, and ECharts only accepts a literal here.
          label: insideLabels ? { color: sliceLabelColor(fill, theme) } : undefined,
          itemStyle: {
            ...item.itemStyle,
            color: fill,
          },
        }
      }),
    }],
  }
}

function ringRadius(index: number, count: number): [string, string] {
  const outer = 90 - (index * (68 / count))
  const inner = outer - (62 / count)
  return [`${Math.max(12, inner)}%`, `${outer}%`]
}

function pointColor(
  point: RowPoint,
  index: number,
  theme: EChartsTheme,
  colors?: string[],
  seriesColor?: (label: string, index: number) => string | undefined,
): string {
  return colors?.[index]
    ?? theme.seriesColor(point.nodeKey ?? '')
    ?? theme.seriesColor(point.category)
    ?? seriesColor?.(point.category, index)
    ?? theme.colors[index % theme.colors.length]
    ?? '#2563eb'
}

function radialPartitionOption(input: ChartInput, theme: EChartsTheme, points: RowPoint[]): EChartsOption {
  const rings = [...(input.radial?.rings ?? [])].sort((left, right) => (left.order ?? 0) - (right.order ?? 0))
  const categories = [...new Map(points.map((point) => [point.nodeKey ?? point.category, point])).values()]
  const categoryOrder = new Map(categories.map((point, index) => [point.nodeKey ?? point.category, index]))
  const sliceLabels = input.presentation?.sliceLabels
  const insideLabels = sliceLabels === 'percent' || sliceLabels === 'label'
  const series = rings.map((ring, ringIndex) => {
    const ringPoints = points
      .filter((point) => point.series === ring.key)
      .sort((left, right) =>
        (categoryOrder.get(left.nodeKey ?? left.category) ?? 0)
        - (categoryOrder.get(right.nodeKey ?? right.category) ?? 0))
    // A ring declares the whole it reconciles against, and the document build
    // already refuses a ring whose rows miss it by more than its tolerance —
    // so it is a better denominator than the rows, and the only one that keeps
    // the two rings of a donut comparable.
    const ringShares = pointShares(ringPoints, partitionTotal(ringPoints, ring.total))
    return {
      type: 'pie' as const,
      name: ring.label,
      id: ring.key,
      radius: ringRadius(ringIndex, rings.length),
      center: ['50%', '50%'],
      selectedMode: false,
      universalTransition: { enabled: morphEnabled() },
      percentPrecision: rawPercentPrecision,
      minAngle: minimumSliceAngle,
      label: insideLabels
        ? {
            position: 'inside' as const,
            fontWeight: 'bold' as const,
            formatter: (params: unknown) => sliceLabel(sliceLabels, params, input.locale),
          }
        : { show: false },
      labelLine: { show: false },
      data: ringPoints.map((point, index) => {
        const mark = { ...point, nodeKey: radialNodeKey(ring.key, point.nodeKey ?? point.category) }
        const item = dataItem(mark, input, theme, point.nodeKey ?? point.category)
        const fill = pointColor(point, categoryOrder.get(point.nodeKey ?? point.category) ?? 0, theme, input.colors, input.seriesColor)
        return {
          ...item,
          name: point.category,
          share: ringShares[index],
          ringKey: ring.key,
          categoryKey: point.nodeKey,
          label: insideLabels ? { color: sliceLabelColor(fill, theme) } : undefined,
          itemStyle: {
            ...item.itemStyle,
            color: fill,
            borderColor: item.itemStyle.borderColor ?? theme.card,
            borderWidth: item.itemStyle.borderWidth || 1,
          },
        }
      }),
    }
  })
  return {
    ...baseOption(theme),
    aria: { enabled: true },
    tooltip: {
      trigger: 'item',
      ...tooltipChrome(theme),
      formatter: (params: unknown) => {
        const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
        return sliceTooltip(params, input, text(record.seriesName))
      },
    },
    series,
    media: [{
      // ECharts evaluates media queries against the chart container, not the
      // browser viewport. Dashboard cards in desktop grids can be only
      // 260–480px wide, so a conventional mobile breakpoint hides every inner
      // ring on desktop. Keep the compact single-ring fallback for only the
      // genuinely unusable sliver case.
      query: { maxWidth: 220 },
      option: {
        series: series.map((ring, index) => index === 0
          ? { ...ring, radius: ['46%', '88%'] }
          : { ...ring, data: [], label: { show: false } }),
      },
    }],
  }
}

function radialProgressOption(input: ChartInput, theme: EChartsTheme, points: RowPoint[]): EChartsOption {
  const maximum = input.radial?.max ?? 0
  return {
    ...baseOption(theme),
    aria: { enabled: true },
    tooltip: {
      trigger: 'item',
      ...tooltipChrome(theme),
      formatter: (params: unknown) => {
        const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
        const data = record.data && typeof record.data === 'object' ? record.data as Record<string, unknown> : {}
        if (data.remainder === true) return ''
        return `${text(record.seriesName)}: ${input.format(input.encoding.value ?? '', data.value)} / ${input.format(input.encoding.value ?? '', maximum)}`
      },
    },
    series: points.map((point, index) => {
      const value = point.value === '-' ? 0 : point.value
      const item = dataItem(point, input, theme)
      return {
        type: 'pie' as const,
        name: point.category,
        radius: ringRadius(index, points.length),
        center: ['50%', '50%'],
        startAngle: 90,
        clockwise: true,
        selectedMode: false,
        silent: false,
        label: { show: false },
        labelLine: { show: false },
        animation: !isVisualRegression(),
        data: [
          {
            ...item,
            name: point.category,
            value,
            itemStyle: {
              ...item.itemStyle,
              color: pointColor(point, index, theme, input.colors, input.seriesColor),
              borderRadius: 8,
            },
          },
          {
            name: '',
            value: Math.max(0, maximum - value),
            remainder: true,
            tooltip: { show: false },
            itemStyle: { color: theme.divider },
            emphasis: { disabled: true },
          },
        ],
      }
    }),
  }
}

function radialOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const points = rowPoints(input)
  return input.radial?.mode === 'progress'
    ? radialProgressOption(input, theme, points)
    : radialPartitionOption(input, theme, points)
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
    theme.seriesColor(category) ?? input.seriesColor?.(category, index) ?? theme.colors[index % theme.colors.length]
  const series = seriesNames.map((name, index) => ({
    type: isBar ? 'bar' as const : 'line' as const,
    name: name || undefined,
    universalTransition: { enabled: morphEnabled() },
    barWidth: isBar && barWidth ? barWidth : undefined,
    // The panel's resolver knows about colours pinned to the n-th series of
    // this panel; ECharts' own palette does not, and left to itself it walks a
    // default sequence that has nothing to do with the legend beside it.
    itemStyle: { color: theme.seriesColor(name) ?? input.seriesColor?.(name, index) },
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
  // A horizontal bar hangs its category names in the plot's left margin, and
  // `containLabel` gives that margin whatever the longest name asks for. A
  // catalogue product name asks for the whole width, leaving the bars a sliver
  // — the chart then states nothing at all. Names are capped and the reading
  // stays with the bars; the full name is in the tooltip and in the table.
  const categoryAxis = {
    type: 'category' as const,
    data: categories,
    ...axisStyle(theme),
    ...(horizontal ? { axisLabel: { color: theme.mutedText, width: 260, overflow: 'truncate' as const } } : {}),
  }
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
  if (input.kind === 'radial') return radialOption(input, theme)
  return axisOption(input, theme)
}
