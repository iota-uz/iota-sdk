import type { EChartsOption } from 'echarts'
import type { Presentation } from '../../contract'
import { isVisualRegression } from '../../visualRegression'
import { radialNodeKey, type ChartInput } from '../adapter'
import { fallbackMarkKey } from '../keys'
import { shouldUseLogarithmicScale } from '../scales'
import { distributeShares, formatShare } from '../shares'
import type { EChartsTheme } from './theme'
import { buildDistributionOption, type DistributionInput } from './distributions'

type ChartValue = number | '-'

interface RowPoint {
  category: string
  nodeKey?: string
  rowIndex: number
  series: string
  timestamp?: number
  value: ChartValue
  previous: ChartValue
}

function columnIndex(input: ChartInput, field: string | undefined): number {
  return field ? input.frame.columns.findIndex((column) => column.name === field) : -1
}

function availableEncodingField(input: ChartInput, ...fields: Array<string | undefined>): string | undefined {
  return fields.find((field) => field !== undefined && columnIndex(input, field) >= 0)
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
  // Producers may declare the semantic category field before it is present in
  // a progressive frame. Fall back to the first field that actually exists;
  // otherwise every bar receives the empty category and is painted on top of
  // the preceding one.
  const categoryField = availableEncodingField(input, input.encoding.category, input.encoding.label)
  const categoryIndex = columnIndex(input, categoryField)
  const valueIndex = columnIndex(input, input.encoding.value)
  const previousIndex = columnIndex(input, input.encoding.previous)
  const idIndex = columnIndex(input, input.encoding.id)
  const seriesIndex = columnIndex(input, input.encoding.series)

  return input.frame.rows.map((row, rowIndex) => {
    const category = text(row[categoryIndex])
    const series = seriesIndex >= 0 ? text(row[seriesIndex]) : ''
    return {
      category,
      // A partition radial adds its ring to the category key below; composing
      // the series here as well would wrap the ring twice.
      nodeKey: (idIndex >= 0 ? text(row[idIndex]) : '') || fallbackMarkKey(
        category,
        input.radial?.mode === 'partition' ? '' : series,
      ),
      rowIndex,
      series,
      timestamp: timestamp(row[categoryIndex]),
      value: chartValue(row[valueIndex]),
      previous: chartValue(row[previousIndex]),
    }
  })
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

function numericTooltipValue(value: unknown): number | undefined {
  const raw = tooltipValue(value)
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  if (typeof raw === 'string' && raw.trim() !== '') {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function nonZeroTooltipRecords(params: unknown): Record<string, unknown>[] {
  const entries = Array.isArray(params) ? params : [params]
  return entries
    .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object')
    // A sparse stack can contain dozens of declared series at a given
    // category. Zero rows add no information and used to be omitted by the
    // server source; printing them turns a useful tooltip into a catalogue.
    .filter((entry) => numericTooltipValue(entry.value) !== 0)
}

function timeTooltipFormatter(input: ChartInput, categoryField: string, showSeriesName: boolean) {
  const valueField = input.encoding.value ?? ''
  return (params: unknown) => {
    const records = nonZeroTooltipRecords(params)
    if (records.length === 0) return ''
    const axisValue = records[0]?.axisValue
    const header = typeof axisValue === 'number'
      ? timeLabel(input, categoryField, axisValue)
      : input.format(categoryField, axisValue)
    const reference = records.reduce((maximum, entry) => Math.max(maximum, Math.abs(numericTooltipValue(entry.value) ?? 0)), 0)
    const lines = records.map((entry) => {
      const seriesName = text(entry.seriesName)
      const formatted = input.formatTooltip?.(valueField, tooltipValue(entry.value), reference)
        ?? input.format(valueField, tooltipValue(entry.value))
      return showSeriesName && seriesName ? `${seriesName}: ${formatted}` : formatted
    })
    return [header, ...lines].join('\n')
  }
}

function escapeTooltipHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function categoryTooltipFormatter(
  input: ChartInput,
  categoryField: string,
  stacked: boolean,
  lineSeries: ReadonlySet<string>,
  showSeriesName: boolean,
) {
  const valueField = input.encoding.value ?? ''
  return (params: unknown) => {
    const records = nonZeroTooltipRecords(params)
    if (records.length === 0) return ''
    const rawCategory = records[0]?.axisValueLabel ?? records[0]?.axisValue
    // A categorical year is an identifier, not a quantity. Passing the string
    // "2025" through a generic numeric fallback produced "2 025" in the
    // tooltip while the axis correctly kept "2025".
    const formattedCategory = typeof rawCategory === 'string'
      ? rawCategory
      : input.format(categoryField, rawCategory)
    const period = input.temporal?.period
    const periodLabel = period && String(rawCategory) === period.category
      ? (period.label || (period.state === 'annualized'
        ? (input.labels?.estimate ?? 'Estimate')
        : (input.labels?.ytd ?? 'YTD')))
      : ''
    const header = periodLabel ? `${formattedCategory} · ${periodLabel}` : formattedCategory
    const lines = records.map((entry) => {
      const marker = typeof entry.marker === 'string' ? entry.marker : ''
      const seriesName = text(entry.seriesName)
      const formatted = input.format(valueField, tooltipValue(entry.value))
      const label = showSeriesName ? escapeTooltipHTML(seriesName) : ''
      return `<div>${marker}${label}<span style="float:right;margin-left:24px;font-weight:600">${escapeTooltipHTML(formatted)}</span></div>`
    })
    if (stacked) {
      const total = records.reduce((sum, entry) => {
        if (lineSeries.has(text(entry.seriesName))) return sum
        return sum + (numericTooltipValue(entry.value) ?? 0)
      }, 0)
      const label = input.tooltipTotalLabel ?? 'Total'
      lines.push(`<div style="margin-top:6px;padding-top:6px;border-top:1px solid currentColor;font-weight:600">${escapeTooltipHTML(label)}<span style="float:right;margin-left:24px">${escapeTooltipHTML(input.format(valueField, total))}</span></div>`)
    }
    return `<div><div style="margin-bottom:6px">${escapeTooltipHTML(header)}</div>${lines.join('')}</div>`
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
    aria: { enabled: true },
    backgroundColor: 'transparent',
    color: theme.colors,
    textStyle: { color: theme.text, fontFamily: theme.fontFamily },
  }
}

export function buildMapOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  if (!input.map) throw new Error('map chart requires registered geometry')
  const idIndex = columnIndex(input, input.encoding.id)
  const labelIndex = columnIndex(input, input.encoding.label)
  const valueIndex = columnIndex(input, input.encoding.value)
  const featureLabels = new Map<string, string>()
  for (const feature of input.map.geoJSON.features) {
    const key = text(feature.properties[input.map.featureProperty])
    const label = input.map.labelProperty ? text(feature.properties[input.map.labelProperty]) : key
    if (key) featureLabels.set(key, label || key)
  }
  const frameLabels = new Map<string, string>()
  const values: number[] = []
  const data = input.frame.rows.map((row) => {
    const key = text(row[idIndex])
    const raw = chartValue(row[valueIndex])
    if (typeof raw === 'number') values.push(raw)
    const frameLabel = labelIndex >= 0 ? text(row[labelIndex]) : ''
    if (key && frameLabel) frameLabels.set(key, frameLabel)
    return { name: key, value: raw, nodeKey: key, displayLabel: frameLabel || featureLabels.get(key) || key }
  })
  const min = values.length > 0 ? Math.min(...values) : 0
  const max = values.length > 0 ? Math.max(...values) : 0
  const rangeMin = min
  const rangeMax = max === min && max <= 0 ? 0 : max
  return {
    ...baseOption(theme),
    tooltip: {
      ...tooltipChrome(theme),
      trigger: 'item',
      formatter: (params: unknown) => {
        const record = params as { data?: { displayLabel?: string; value?: unknown }; name?: string }
        const key = record.name ?? ''
        const label = record.data?.displayLabel || frameLabels.get(key) || featureLabels.get(key) || key
        return `${escapeTooltipHTML(label)}<br><strong>${escapeTooltipHTML(input.format(input.encoding.value ?? '', record.data?.value))}</strong>`
      },
    },
    visualMap: {
      type: 'continuous', min: rangeMin, max: rangeMax, calculable: false, orient: 'horizontal',
      show: min !== max,
      left: 'center', bottom: 4, itemWidth: 12, itemHeight: 96,
      text: [input.formatAxis?.(input.encoding.value ?? '', rangeMax) ?? input.format(input.encoding.value ?? '', rangeMax), input.formatAxis?.(input.encoding.value ?? '', rangeMin) ?? input.format(input.encoding.value ?? '', rangeMin)],
      textStyle: { color: theme.mutedText, fontSize: 10 },
      inRange: { color: [theme.divider, input.theme.palette.accent ?? theme.colors[0]] },
    },
    ...(min === max && values.length > 0 ? {
      graphic: [{
        type: 'group', left: 'center', bottom: 8, children: [
          { type: 'rect', shape: { x: 0, y: 1, width: 24, height: 10 }, style: { fill: input.theme.palette.accent ?? theme.colors[0] } },
          { type: 'text', style: { x: 31, y: 0, text: input.formatAxis?.(input.encoding.value ?? '', max) ?? input.format(input.encoding.value ?? '', max), fill: theme.mutedText, fontSize: 10 } },
        ],
      }],
    } : {}),
    series: [{
      type: 'map',
      map: input.map.name,
      nameProperty: input.map.featureProperty,
      roam: false,
      selectedMode: 'single',
      data,
      label: {
        show: true,
        color: theme.text,
        fontSize: 10,
        // Also reserve a small clear zone between labels whose glyph bounds
        // merely touch; without it dense neighbouring names read as one line.
        padding: [4, 8],
        formatter: (params: { name?: string }) =>
          frameLabels.get(params.name ?? '') ?? featureLabels.get(params.name ?? '') ?? params.name ?? '',
      },
      // Region centroids are fixed by the boundary geometry. Moving their
      // labels would visually associate names with the wrong region, so keep
      // the centroid position and suppress only labels that collide. The
      // region name and value remain available from the item tooltip.
      labelLayout: { hideOverlap: true },
      itemStyle: { areaColor: theme.divider, borderColor: theme.card, borderWidth: 1.5 },
      emphasis: {
        label: { show: true, color: theme.selectedBorder, fontWeight: 600 },
        itemStyle: { borderColor: theme.selectedBorder, borderWidth: 2 },
      },
      select: { itemStyle: { borderColor: theme.selectedBorder, borderWidth: 3 } },
    }],
  }
}

/**
 * ECharts pre-rounds `params.percent` to `percentPrecision` decimals; asking
 * for more precision than any label prints keeps the single rounding step in
 * our hands.
 */
export const rawPercentPrecision = 10

/** The label a pie slice carries: one rounding, and nothing under 4%. */
export function slicePercentLabel(percent: number | undefined, locale?: string, decimalSeparator?: string): string {
  const share = percent ?? 0
  return share >= 4 ? formatShare(share, locale, decimalSeparator) : ''
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

function sliceLabel(mode: Presentation['sliceLabels'], params: unknown, locale?: string, decimalSeparator?: string): string {
  const { name, share } = labelShare(params)
  return mode === 'label' ? sliceCategoryLabel(name, share) : slicePercentLabel(share, locale, decimalSeparator)
}

export function donutSliceLabel(params: unknown, input: ChartInput): string {
  const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
  const data = record.data && typeof record.data === 'object' ? record.data as Record<string, unknown> : {}
  const share = typeof data.share === 'number'
    ? data.share
    : (typeof record.percent === 'number' ? record.percent : undefined)
  if (input.viewportWidth !== undefined && input.viewportWidth < 500) return text(record.name)
  const value = input.format(input.encoding.value ?? '', data.value)
  return `${text(record.name)}\n${value} · ${formatShare(share, input.locale, input.shareDecimalSeparator)}`
}

function timeLabel(input: ChartInput, field: string, value: number): string {
  const formatted = input.format(field, value)
  if (input.categoryFormatDefined) return formatted
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return formatted
  return new Intl.DateTimeFormat(input.locale, { month: 'short', year: 'numeric', timeZone: 'UTC' }).format(date)
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
  const suffix = share === undefined ? '' : ` (${formatShare(share, input.locale, input.shareDecimalSeparator)})`
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
      formatter: (params: unknown) => sliceLabel(sliceLabels, params, input.locale, input.shareDecimalSeparator),
    }
    : donut
      ? {
        color: theme.text,
        formatter: (params: unknown) => donutSliceLabel(params, input),
        lineHeight: 16,
      }
      : { color: theme.text }
  const total = partitionTotal(points, input.frame.total)
  return {
    ...baseOption(theme),
    graphic: donut && total !== undefined ? [{
      type: 'group',
      left: 'center',
      top: 'middle',
      children: [
        { type: 'text', style: { text: input.tooltipTotalLabel ?? 'Total', fill: theme.mutedText, font: `12px ${theme.fontFamily}`, align: 'center' }, left: 'center', top: -12 },
        { type: 'text', style: { text: input.format(input.encoding.value ?? '', total), fill: theme.text, font: `600 16px ${theme.fontFamily}`, align: 'center' }, left: 'center', top: 5 },
      ],
    }] : undefined,
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
        const fill = pointColor(point, index, theme, input.rowColor)
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

/** Radius of the innermost hole, as a share of the plot box. */
const ringHubRadius = 40
/** Radius the outermost ring reaches. */
const ringOuterRadius = 90
/** Blank space between two adjacent rings. */
const ringGap = 3

/**
 * Bands for `count` concentric rings, outermost first.
 *
 * Every ring gets the same thickness and the hub stays clear, which is what
 * makes the figure read as rings at all. Dividing the whole radius among the
 * rings instead — the first rule here — left a two-ring donut with a hole
 * narrower than its own inner band: the inner ring came out a filled disc with
 * a dot punched in it, and the total badge that sits in the hub landed on top
 * of the slices rather than inside them.
 */
function ringRadius(index: number, count: number): [string, string] {
  const rings = Math.max(1, count)
  const band = (ringOuterRadius - ringHubRadius - (ringGap * (rings - 1))) / rings
  const outer = ringOuterRadius - (index * (band + ringGap))
  return [`${outer - band}%`, `${outer}%`]
}

function pointColor(
  point: RowPoint,
  index: number,
  theme: EChartsTheme,
  rowColor?: (label: string, index: number, nodeKey?: string) => string | undefined,
): string {
  return rowColor?.(point.category, index, point.nodeKey)
    ?? theme.seriesColor(point.nodeKey ?? '')
    ?? theme.seriesColor(point.category)
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
        const fill = pointColor(point, categoryOrder.get(point.nodeKey ?? point.category) ?? 0, theme, input.rowColor)
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
              color: pointColor(point, index, theme, input.rowColor),
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

function temporalFieldData(
  input: ChartInput,
  points: RowPoint[],
  field: string,
  seriesName: string,
  categories: string[],
  timeAxis: boolean,
): Array<ChartValue | [number, ChartValue] | null> {
  const fieldIndex = columnIndex(input, field)
  if (fieldIndex < 0) return []
  const matching = points.filter((point) => point.series === seriesName)
  if (timeAxis) {
    return matching
      .filter((point): point is RowPoint & { timestamp: number } => point.timestamp !== undefined)
      .sort((left, right) => left.timestamp - right.timestamp)
      .map((point) => [point.timestamp, chartValue(input.frame.rows[point.rowIndex]?.[fieldIndex])])
  }
  return categories.map((category) => {
    const point = matching.find((candidate) => candidate.category === category)
    return point ? chartValue(input.frame.rows[point.rowIndex]?.[fieldIndex]) : null
  })
}

function confidenceBandData(
  input: ChartInput,
  points: RowPoint[],
  lowerField: string,
  upperField: string,
  seriesName: string,
  categories: string[],
  timeAxis: boolean,
): Array<ChartValue | [number, ChartValue] | null> {
  const lowerIndex = columnIndex(input, lowerField)
  const upperIndex = columnIndex(input, upperField)
  if (lowerIndex < 0 || upperIndex < 0) return []
  const difference = (point: RowPoint): ChartValue => {
    const lower = chartValue(input.frame.rows[point.rowIndex]?.[lowerIndex])
    const upper = chartValue(input.frame.rows[point.rowIndex]?.[upperIndex])
    return typeof lower === 'number' && typeof upper === 'number' ? Math.max(0, upper - lower) : '-'
  }
  const matching = points.filter((point) => point.series === seriesName)
  if (timeAxis) {
    return matching
      .filter((point): point is RowPoint & { timestamp: number } => point.timestamp !== undefined)
      .sort((left, right) => left.timestamp - right.timestamp)
      .map((point) => [point.timestamp, difference(point)])
  }
  return categories.map((category) => {
    const point = matching.find((candidate) => candidate.category === category)
    return point ? difference(point) : null
  })
}

function incompletePeriodItem<T extends Record<string, unknown>>(
  item: T,
  point: RowPoint,
  input: ChartInput,
  theme: EChartsTheme,
): T & Record<string, unknown> {
  const period = input.temporal?.period
  if (!period || point.category !== period.category) return item
  const label = period.label || (period.state === 'annualized'
    ? (input.labels?.estimate ?? 'Estimate')
    : (input.labels?.ytd ?? 'YTD'))
  const itemStyle = item.itemStyle && typeof item.itemStyle === 'object'
    ? item.itemStyle as Record<string, unknown>
    : {}
  return {
    ...item,
    symbol: 'emptyCircle',
    symbolSize: 12,
    label: { show: true, formatter: label, color: theme.text, position: 'top' },
    itemStyle: {
      ...itemStyle,
      borderWidth: 2,
      decal: {
        color: theme.mutedText,
        dashArrayX: [1, 0],
        dashArrayY: [2, 4],
        rotation: -Math.PI / 4,
        symbol: 'rect',
      },
    },
  }
}

function axisOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  const points = rowPoints(input)
  const categories = [...new Set(points.map((point) => point.category))]
  const seriesNames = [...new Set(points.map((point) => point.series))]
  const formatter = valueFormatter(input)
  const compactFormatter = axisValueFormatter(input)
  const isBar = input.kind === 'bar' || input.kind === 'hbar'
  const horizontal = input.kind === 'hbar'
  const logarithmic = shouldUseLogarithmicScale(input.frame, input.encoding, input.valueAxis)
  const numericPointValues = points.flatMap((point) => typeof point.value === 'number' ? [point.value] : [])
  const logMaximum = logarithmic && numericPointValues.length > 0
    ? niceLogMaximum(Math.max(...numericPointValues))
    : undefined
  const showSeriesName = Boolean(input.encoding.series)
  const categoryField = availableEncodingField(input, input.encoding.category, input.encoding.label) ?? ''
  const categoryIsTime = input.frame.columns.find((column) => column.name === categoryField)?.type === 'time'
  const timeAxis = !isBar && categoryIsTime
  const colorByCategory = isBar && input.presentation?.colorBy === 'category'
  const barWidth = input.presentation?.barWidthPx
  // A stack states that its segments add up to the column, so only the series
  // that really are parts of the whole may join it: anything the producer
  // named as a line is drawn over the columns instead, on the same axis but
  // outside the sum.
  const lineSeries = new Set(input.presentation?.lineSeries ?? [])
  const stacked = isBar && input.presentation?.stack === true
  const categoryColor = (category: string, index: number) =>
    input.seriesColor?.(category, index) ?? theme.seriesColor(category) ?? theme.colors[index % theme.colors.length]
  const currentSeries = seriesNames.map((name, index) => ({
    type: isBar && !lineSeries.has(name) ? 'bar' as const : 'line' as const,
    name: name || undefined,
    universalTransition: { enabled: morphEnabled() },
    stack: stacked && !lineSeries.has(name) ? 'total' : undefined,
    barWidth: isBar && !lineSeries.has(name) && barWidth ? barWidth : undefined,
    // Bar length on a logarithmic axis communicates order of magnitude, not
    // the literal value. Print that value on the mark so restoring the scale
    // never makes the chart less informative than its linear counterpart.
    label: (input.presentation?.dataLabels === true || logarithmic) && !lineSeries.has(name)
      ? {
        show: true,
        // Neighbouring line series often carry close values at the same
        // category. Alternating sides preserves both readings instead of
        // painting two formatted values on top of each other.
        position: isBar
          ? (horizontal ? 'right' as const : 'top' as const)
          : (index % 2 === 0 ? 'top' as const : 'bottom' as const),
        color: theme.text,
        formatter: (params: { value?: unknown }) => compactFormatter(tooltipValue(params.value)),
      }
      : undefined,
    labelLayout: (input.presentation?.dataLabels === true || logarithmic) && !lineSeries.has(name)
      ? { hideOverlap: true }
      : undefined,
    // The panel's resolver knows about colours pinned to the n-th series of
    // this panel; ECharts' own palette does not, and left to itself it walks a
    // default sequence that has nothing to do with the legend beside it.
    itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name) },
    areaStyle: input.kind === 'area' ? { opacity: 0.18 } : undefined,
    showSymbol: !isBar || lineSeries.has(name),
    data: timeAxis
      ? points
        .filter((point): point is RowPoint & { timestamp: number } => point.series === name && point.timestamp !== undefined)
        .sort((left, right) => left.timestamp - right.timestamp)
        .map((point) => incompletePeriodItem(
          { ...dataItem(point, input, theme), value: [point.timestamp, point.value] },
          point,
          input,
          theme,
        ))
      : categories.map((category, index) => {
        const point = points.find((candidate) => candidate.category === category && candidate.series === name)
        if (!point) return null
        const item = dataItem(point, input, theme)
        return incompletePeriodItem(
          colorByCategory ? { ...item, itemStyle: { ...item.itemStyle, color: categoryColor(category, index) } } : item,
          point,
          input,
          theme,
        )
      }),
  }))
  const comparisonSeries = input.encoding.previous ? seriesNames.map((name, index) => ({
    type: isBar && !lineSeries.has(name) ? 'bar' as const : 'line' as const,
    name: `${name ? `${name} · ` : ''}${input.labels?.previous ?? 'Previous'}`,
    silent: true,
    z: 0,
    barGap: isBar ? '-100%' : undefined,
    barWidth: isBar && barWidth ? barWidth : undefined,
    itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name), opacity: 0.24 },
    lineStyle: { opacity: 0.38, type: 'dashed' as const, width: 2 },
    areaStyle: input.kind === 'area' ? { opacity: 0.06 } : undefined,
    showSymbol: false,
    data: timeAxis
      ? points
        .filter((point): point is RowPoint & { timestamp: number } => point.series === name && point.timestamp !== undefined)
        .sort((left, right) => left.timestamp - right.timestamp)
        .map((point) => [point.timestamp, point.previous])
      : categories.map((category) => points.find((point) => point.category === category && point.series === name)?.previous ?? null),
  })) : []
  const regression = input.temporal?.regression
  const regressionSeries = regression ? seriesNames.map((name, index) => ({
    type: 'line' as const,
    name: `${name ? `${name} · ` : ''}${regression.label || input.labels?.trend || 'Trend'}`,
    silent: true,
    z: 4,
    showSymbol: false,
    itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name) },
    lineStyle: { type: 'dashed' as const, width: 2.5, opacity: 0.9 },
    data: temporalFieldData(input, points, regression.field, name, categories, timeAxis),
  })) : []
  const movingAverageSeries = (input.temporal?.movingAverages ?? []).flatMap((average) => seriesNames.map((name, index) => ({
    type: 'line' as const,
    name: `${name ? `${name} · ` : ''}${average.label || input.labels?.movingAverage(average.window) || `SMA ${average.window}`}`,
    silent: true,
    z: 5,
    showSymbol: false,
    itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name) },
    lineStyle: { width: 3, opacity: 0.95 },
    data: temporalFieldData(input, points, average.field, name, categories, timeAxis),
  })))
  const annualizedSeries = input.temporal?.period?.annualizedField ? seriesNames.map((name) => ({
    type: 'line' as const,
    name: input.temporal?.period?.label || input.labels?.estimate || 'Estimate',
    silent: true,
    z: 6,
    showSymbol: true,
    symbol: 'diamond',
    symbolSize: 11,
    itemStyle: { color: theme.mutedText },
    lineStyle: { type: 'dotted' as const, width: 2, color: theme.mutedText },
    data: temporalFieldData(input, points, input.temporal!.period!.annualizedField!, name, categories, timeAxis),
  })) : []
  const forecastSeries = input.temporal?.forecast ? seriesNames.flatMap((name, index) => {
    const forecast = input.temporal!.forecast!
    const color = input.seriesColor?.(name, index) ?? theme.seriesColor(name) ?? theme.colors[index % theme.colors.length]
    const stack = `lens-forecast-${index}`
    const forecastLabel = forecast.label || input.labels?.forecast || 'Forecast'
    return [
      {
        type: 'line' as const,
        name: `${name ? `${name} · ` : ''}${forecastLabel}`,
        silent: true,
        z: 5,
        showSymbol: false,
        itemStyle: { color },
        lineStyle: { type: 'dashed' as const, width: 2.5, color },
        data: temporalFieldData(input, points, forecast.valueField, name, categories, timeAxis),
      },
      {
        type: 'line' as const,
        name: input.labels?.forecastLower(forecastLabel) ?? `${forecastLabel} lower`,
        silent: true,
        tooltip: { show: false },
        stack,
        stackStrategy: 'all' as const,
        showSymbol: false,
        lineStyle: { opacity: 0 },
        areaStyle: { opacity: 0 },
        data: temporalFieldData(input, points, forecast.lowerField, name, categories, timeAxis),
      },
      {
        type: 'line' as const,
        name: input.labels?.forecastConfidence(forecastLabel) ?? `${forecastLabel} confidence`,
        silent: true,
        tooltip: { show: false },
        stack,
        stackStrategy: 'all' as const,
        showSymbol: false,
        lineStyle: { opacity: 0 },
        areaStyle: { color, opacity: 0.14 },
        data: confidenceBandData(input, points, forecast.lowerField, forecast.upperField, name, categories, timeAxis),
      },
    ]
  }) : []
  const verticalMarkers = [
    ...(input.temporal?.annotations ?? []).map(({ at, label }) => ({ at, label, type: 'dotted' as const })),
    ...(input.temporal?.forecast ? [{
      at: input.temporal.forecast.start,
      label: input.temporal.forecast.label || input.labels?.forecast || 'Forecast',
      type: 'dashed' as const,
    }] : []),
  ]
  const markLineData = [
    ...(input.temporal?.referenceLines ?? []).map((reference) => {
      const label = reference.label || input.format(input.encoding.value ?? '', reference.value)
      return {
        name: label,
        yAxis: reference.value,
        lineStyle: { type: 'dashed' as const, color: theme.mutedText, width: 1.5 },
        label: { show: true, formatter: label, color: theme.mutedText, position: 'end' as const },
      }
    }),
    ...verticalMarkers.map((marker) => ({
      name: marker.label,
      xAxis: timeAxis ? (timestamp(marker.at) ?? marker.at) : marker.at,
      lineStyle: { type: marker.type, color: theme.mutedText, width: marker.type === 'dashed' ? 2 : 1.5 },
      label: { show: true, formatter: marker.label, color: theme.mutedText, position: 'end' as const },
    })),
  ]
  // Mark lines annotate the axes without contributing synthetic values to the
  // data extent. In particular, adding a vertical annotation must never
  // rescale the values it is meant to explain.
  const annotatedCurrentSeries = currentSeries.map((series, index) => index === 0 && markLineData.length > 0
    ? { ...series, markLine: { silent: true, symbol: ['none', 'none'], data: markLineData } }
    : series)
  const series = [
    ...comparisonSeries,
    ...annotatedCurrentSeries,
    ...regressionSeries,
    ...movingAverageSeries,
    ...annualizedSeries,
    ...forecastSeries,
  ]
  // A horizontal bar hangs its category names in the plot's left margin, and
  // `containLabel` gives that margin whatever the longest name asks for. A
  // catalogue product name asks for the whole width, leaving the bars a sliver
  // — the chart then states nothing at all. Names are capped and the reading
  // stays with the bars; the full name is in the tooltip and in the table.
  const categoryAxis = {
    type: 'category' as const,
    data: categories,
    triggerEvent: true,
    ...axisStyle(theme),
    // ECharts passes the category index as the formatter's second argument;
    // passing middleEllipsis directly therefore treated the index as a width
    // and mangled already-short labels such as "Apr".
    ...(horizontal ? { axisLabel: { color: theme.mutedText, width: 260, formatter: (value: string) => middleEllipsis(String(value)), overflow: 'truncate' as const } } : {}),
  }
  const temporalAxis = {
    type: 'time' as const,
    triggerEvent: true,
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter: (value: number) => timeLabel(input, categoryField, value) },
  }
  const valueAxis = {
    type: logarithmic ? 'log' as const : 'value' as const,
    show: points.some((point) => typeof point.value === 'number' && point.value !== 0),
    logBase: logarithmic ? (input.valueAxis?.logBase || 10) : undefined,
    max: logMaximum,
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter: axisValueFormatter(input), hideOverlap: true },
  }

  return {
    ...baseOption(theme),
    // In VR mode the grid inset is pinned: containLabel derives it from
    // canvas text measurement, which lands on a rounding boundary for the
    // variable font and shifts the whole plot by 1px between runs.
    grid: isVisualRegression()
      ? { left: 96, right: (input.temporal?.referenceLines?.length ?? 0) > 0 ? 168 : 32, top: 24, bottom: timeAxis ? 58 : 32, containLabel: false }
      : {
        left: 16,
        right: (input.temporal?.referenceLines?.length ?? 0) > 0 ? 152 : categoryIsTime ? 64 : horizontal && logarithmic ? 88 : 16,
        top: 24,
        bottom: timeAxis ? 52 : 12,
        containLabel: true,
      },
    dataZoom: timeAxis ? [
      { type: 'inside', xAxisIndex: 0, filterMode: 'none' },
      { type: 'slider', xAxisIndex: 0, filterMode: 'none', height: 18, bottom: 8 },
    ] : undefined,
    tooltip: {
      trigger: 'axis',
      renderMode: timeAxis ? 'richText' : undefined,
      ...tooltipChrome(theme),
      formatter: timeAxis
        ? timeTooltipFormatter(input, categoryField, showSeriesName)
        : categoryTooltipFormatter(input, categoryField, stacked, lineSeries, showSeriesName),
      valueFormatter: timeAxis ? undefined : formatter,
    },
    xAxis: horizontal ? valueAxis : timeAxis ? temporalAxis : categoryAxis,
    yAxis: horizontal ? categoryAxis : valueAxis,
    series,
  }
}

/** Preserve both identifying ends of long catalogue names on narrow axes. */
export function middleEllipsis(value: string, limit = 36): string {
  if (value.length <= limit) return value
  const available = limit - 1
  const left = Math.ceil(available / 2)
  return `${value.slice(0, left)}…${value.slice(value.length - (available - left))}`
}

/** Clamp log whitespace to the next fifth of the leading magnitude. */
export function niceLogMaximum(maximum: number): number {
  if (!Number.isFinite(maximum) || maximum <= 0) return maximum
  const magnitude = 10 ** Math.floor(Math.log10(maximum))
  const step = magnitude / 5
  return Math.ceil(maximum / step) * step
}

export function buildChartOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  if (input.kind === 'map') return buildMapOption(input, theme)
  if (input.kind === 'histogram' || input.kind === 'boxplot' || input.kind === 'heatmap') {
    return buildDistributionOption(input as DistributionInput, theme)
  }
  if (input.kind === 'pie' || input.kind === 'donut') return pieOption(input, theme)
  if (input.kind === 'radial') return radialOption(input, theme)
  return axisOption(input, theme)
}
