import type { EChartsOption } from 'echarts'
import { compactChartLabelWidth } from '../../breakpoints'
import type { Presentation } from '../../contract'
import { isVisualRegression } from '../../visualRegression'
import { radialNodeKey, type ChartInput } from '../adapter'
import { fallbackMarkKey } from '../keys'
import { activeOverlayIds, categoryDisplayFormatter, chartOverlays, overlayId } from '../overlays'
import { linearScaleObscuresValues, shouldUseLogarithmicScale } from '../scales'
import { distributeShares, formatShare } from '../shares'
import { measureTextWidth } from '../text'
import type { EChartsTheme } from './theme'
import { tooltipChrome } from './tooltip'
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
  const seen = new Set<string>()
  return entries
    .filter((entry): entry is Record<string, unknown> => Boolean(entry) && typeof entry === 'object')
    // A sparse stack can contain dozens of declared series at a given
    // category. Zero rows add no information and used to be omitted by the
    // server source; printing them turns a useful tooltip into a catalogue.
    .filter((entry) => numericTooltipValue(entry.value) !== 0)
    // A series with a gap at this category has nothing to say about it. The
    // incomplete-period tail deliberately creates such a gap — the final
    // segment is drawn by its own dashed series — and a "—" beside the name
    // states only that the runtime split the line.
    .filter((entry) => numericTooltipValue(entry.value) !== undefined)
    // That split leaves the boundary point in two series at once, under the
    // same name and with the same value. It is one reading, so it prints once.
    .filter((entry) => {
      const key = `${text(entry.seriesName)} ${String(numericTooltipValue(entry.value))}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
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

/**
 * One tooltip row: a swatch and a name on the left, the amount hard right.
 *
 * Three formats used to coexist on one dashboard — the map's `name<br><b>value
 * </b>`, the donut's `Click: 908 (50,7%)`, and the axis tooltip's aligned
 * table — so the same act of hovering produced three different objects
 * depending on which panel was under the pointer. This is the row all three
 * print now; only what they put in it differs.
 */
function tooltipRow(label: string, value: string, marker = ''): string {
  return `<div>${marker}${escapeTooltipHTML(label)}<span style="float:right;margin-left:24px;font-weight:600">${escapeTooltipHTML(value)}</span></div>`
}

/** A heading and its rows, as one tooltip body. */
function tooltipCard(header: string, rows: string[]): string {
  const heading = header ? `<div style="margin-bottom:6px">${escapeTooltipHTML(header)}</div>` : ''
  return `<div>${heading}${rows.join('')}</div>`
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
      return tooltipRow(showSeriesName ? seriesName : '', formatted, marker)
    })
    if (stacked) {
      const total = records.reduce((sum, entry) => {
        if (lineSeries.has(text(entry.seriesName))) return sum
        return sum + (numericTooltipValue(entry.value) ?? 0)
      }, 0)
      const label = input.tooltipTotalLabel ?? 'Total'
      lines.push(`<div style="margin-top:6px;padding-top:6px;border-top:1px solid currentColor;font-weight:600">${escapeTooltipHTML(label)}<span style="float:right;margin-left:24px">${escapeTooltipHTML(input.format(valueField, total))}</span></div>`)
    }
    return tooltipCard(header, lines)
  }
}

export { tooltipChrome, tooltipClassName, tooltipZIndex } from './tooltip'

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

/** `#rgb`/`#rrggbb` as three channels, or nothing when it is neither. */
function channels(color: string | undefined): [number, number, number] | undefined {
  const hex = (color ?? '').trim().replace(/^#/, '')
  const expanded = hex.length === 3 ? hex.split('').map((digit) => digit + digit).join('') : hex
  if (!/^[0-9a-fA-F]{6}$/.test(expanded)) return undefined
  return [0, 2, 4].map((offset) => parseInt(expanded.slice(offset, offset + 2), 16)) as [number, number, number]
}

/**
 * `ratio` of `color` over `over`, resolved to a literal.
 *
 * Canvas takes a colour, not an expression: `color-mix()` reaches the
 * stylesheet but never the plot, and ECharts' own colour parser would refuse
 * it. Tints a chart paints are therefore mixed here, from the same tokens the
 * sheet mixes from.
 */
export function mixColor(color: string | undefined, over: string | undefined, ratio: number): string | undefined {
  const front = channels(color)
  const back = channels(over)
  if (!front || !back) return color
  const blend = front.map((value, index) => Math.round(value * ratio + back[index]! * (1 - ratio)))
  return `#${blend.map((value) => value.toString(16).padStart(2, '0')).join('')}`
}

export function buildMapOption(input: ChartInput, theme: EChartsTheme): EChartsOption {
  if (!input.map) throw new Error('map chart requires registered geometry')
  const idIndex = columnIndex(input, input.encoding.id)
  const labelIndex = columnIndex(input, input.encoding.label)
  const valueIndex = columnIndex(input, input.encoding.value)
  const featureLabels = new Map<string, string>()
  for (const feature of input.map.geoJSON.features) {
    const key = text(feature.properties[input.map.featureProperty])
    // Locale name, then the geometry's own default name, then the join key. A
    // region the boundary file never translated is named in whatever language
    // it does carry rather than shown as «UZ-AN».
    const label = (input.map.labelProperty ? text(feature.properties[input.map.labelProperty]) : '')
      || (input.map.fallbackLabelProperty ? text(feature.properties[input.map.fallbackLabelProperty]) : '')
      || key
    if (key) featureLabels.set(key, label || key)
  }
  const frameLabels = new Map<string, string>()
  const values: number[] = []
  for (const row of input.frame.rows) {
    const raw = chartValue(row[valueIndex])
    if (typeof raw === 'number') values.push(raw)
  }
  // Shade by rank, not by amount — the same rule the shaded table column
  // follows. On this map one region carries 31 415 policies and the next
  // carries 10: linear in the value, thirteen of fourteen regions land on the
  // palest end and the map says nothing the numeral had not. Ranking answers
  // what a reader actually asks of a choropleth — which are the big ones, in
  // order — whatever the distances between them.
  const sorted = [...values].sort((left, right) => left - right)
  const rank = (value: number): number => {
    if (sorted.length <= 1) return 1
    const first = sorted.indexOf(value)
    if (first < 0) return 0
    const last = sorted.lastIndexOf(value)
    return ((first + last) / 2) / (sorted.length - 1)
  }
  const data = input.frame.rows.map((row) => {
    const key = text(row[idIndex])
    const raw = chartValue(row[valueIndex])
    const frameLabel = labelIndex >= 0 ? text(row[labelIndex]) : ''
    if (key && frameLabel) frameLabels.set(key, frameLabel)
    return {
      name: key,
      // `value` is the number the visual mapping reads; the amount it stands
      // for travels beside it and is what the tooltip prints.
      value: typeof raw === 'number' ? rank(raw) : raw,
      amount: raw,
      nodeKey: key,
      displayLabel: frameLabel || featureLabels.get(key) || key,
    }
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
        const record = params as { data?: { displayLabel?: string; amount?: unknown }; name?: string }
        const key = record.name ?? ''
        const label = record.data?.displayLabel || frameLabels.get(key) || featureLabels.get(key) || key
        // A region the frame never joined has no reading, and an em-dash cannot
        // say whether that means "nothing happened here" or "this row did not
        // arrive". Only a value that exists is formatted; the rest say so.
        const raw = record.data?.amount
        const value = raw === undefined || raw === null || raw === '-'
          ? (input.labels?.noData ?? 'No data')
          : input.format(input.encoding.value ?? '', raw)
        return tooltipCard('', [tooltipRow(label, value)])
      },
    },
    visualMap: {
      // The domain is the rank the series now carries; the two ends are still
      // labelled with the real amounts they stand for.
      type: 'continuous', min: 0, max: 1, calculable: false, orient: 'horizontal',
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
      // Hover had no `areaColor`, so ECharts painted its factory default —
      // bright amber inside a 2px black outline — on a blue dashboard, which
      // reads as a warning state on whichever region the pointer crossed. The
      // accent tint is the same one selection uses, one step lighter.
      emphasis: {
        label: { show: true, color: theme.selectedBorder, fontWeight: 600 },
        itemStyle: {
          areaColor: mixColor(input.theme.palette.accent ?? theme.colors[0] ?? theme.accent, theme.card, 0.28),
          borderColor: input.theme.palette.accent ?? theme.accent,
          borderWidth: 1.5,
        },
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
 * The geometry a slice label has to fit inside, for the ring it is drawn on.
 *
 * A label sits at the middle of the ring's band, written horizontally, so what
 * limits it is the chord the slice spans at that radius. Both terms of that
 * chord were previously ignored: the old rule compared a *character count*
 * against the slice's share of the circle, which is the same test on a 240px
 * donut and a 900px one, and on «Ш» and «і».
 */
interface RingGeometry {
  /** Plot box width, in pixels; the radii below are fractions of half of it. */
  plotWidth: number
  /** Inner and outer radius of the ring, as fractions of the plot radius. */
  band: [number, number]
  fontSize: number
  fontFamily: string
}

/** Chord length in pixels at the middle of `band` for a slice of `share`. */
function sliceChordWidth(share: number, geometry: RingGeometry): number {
  const radius = geometry.plotWidth / 2 * ((geometry.band[0] + geometry.band[1]) / 2)
  const angle = (share / 100) * 2 * Math.PI
  return 2 * radius * Math.sin(Math.min(angle, Math.PI) / 2)
}

/**
 * A category label on the slice instead of a percent, for dimensions whose
 * labels are guaranteed short — a year, a quarter. The producer opts in; a
 * product name on a slice would be clipped to meaninglessness.
 *
 * Long labels are still possible (a producer can mislabel a dimension), so the
 * label is dropped rather than clipped once it cannot fit the arc. Without a
 * measurable geometry — a plot box of zero width, a server render — the label
 * is kept: dropping it would hide data on the strength of a measurement that
 * was never taken.
 */
export function sliceCategoryLabel(
  name: string | undefined,
  share: number | undefined,
  geometry?: RingGeometry,
): string {
  const label = (name ?? '').trim()
  if (!label || (share ?? 0) < labelledSliceMinShare) return ''
  if (!geometry || !(geometry.plotWidth > 0)) return label
  const available = sliceChordWidth(share ?? 0, geometry)
  return measureTextWidth(label, geometry.fontSize, geometry.fontFamily) <= available ? label : ''
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

function sliceLabel(
  mode: Presentation['sliceLabels'],
  params: unknown,
  locale?: string,
  decimalSeparator?: string,
  geometry?: RingGeometry,
): string {
  const { name, share } = labelShare(params)
  return mode === 'label' ? sliceCategoryLabel(name, share, geometry) : slicePercentLabel(share, locale, decimalSeparator)
}

/**
 * The callout beside a slice: what it is, and how much of the whole it is.
 *
 * The share used to be dropped on a narrow plot, which is where the name alone
 * says least — a screenshot of a compact donut carried no quantity at all, and
 * the only place the percentage existed was a hover the picture cannot record.
 * The amount is the part that goes: it is the longest of the three, the legend
 * and the centre total both carry it, and it is what pushed «Payme» to «Pa…».
 */
export function donutSliceLabel(params: unknown, input: ChartInput): string {
  const record = params && typeof params === 'object' ? params as Record<string, unknown> : {}
  const data = record.data && typeof record.data === 'object' ? record.data as Record<string, unknown> : {}
  const share = typeof data.share === 'number'
    ? data.share
    : (typeof record.percent === 'number' ? record.percent : undefined)
  const formattedShare = formatShare(share, input.locale, input.shareDecimalSeparator)
  if (input.viewportWidth !== undefined && input.viewportWidth < compactChartLabelWidth) {
    return `${text(record.name)}\n${formattedShare}`
  }
  const value = input.format(input.encoding.value ?? '', data.value)
  return `${text(record.name)}\n${value} · ${formattedShare}`
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
  const suffix = share === undefined ? '' : ` · ${formatShare(share, input.locale, input.shareDecimalSeparator)}`
  // The same row every other tooltip prints: the ring, if there is one, heads
  // the card; the category and its amount are the row.
  return tooltipCard(ringLabel ?? '', [tooltipRow(label, `${value}${suffix}`)])
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
  //
  // A ring that hangs its labels outside itself cannot also fill the card. At
  // 82% the callouts had 9% of the box a side to live in, and ECharts, which
  // clips a callout to the container rather than dropping it, cut «Payme» to
  // «Pa…» on every panel under ~1100px. Labels outside therefore buy their room
  // from the ring, once, instead of being truncated to fit what is left.
  const radius: [string, string] = donut
    ? (fill ? ['54%', '92%'] : insideLabels ? ['50%', '82%'] : ['40%', '64%'])
    : (fill ? ['0%', '92%'] : insideLabels ? ['0%', '82%'] : ['0%', '64%'])
  const geometry: RingGeometry | undefined = input.viewportWidth
    ? {
      plotWidth: input.viewportWidth,
      band: [Number.parseFloat(radius[0]) / 100, Number.parseFloat(radius[1]) / 100],
      fontSize: theme.type.base,
      fontFamily: theme.fontFamily,
    }
    : undefined
  const label = insideLabels
    // Labels inside the slices remove the leader-line halo that shrinks the
    // plot, so the pie can fill the card.
    ? {
      position: 'inside' as const,
      fontWeight: 'bold' as const,
      // Slices under 4% cannot hold a legible label; the legend below
      // still names them.
      formatter: (params: unknown) => sliceLabel(sliceLabels, params, input.locale, input.shareDecimalSeparator, geometry),
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
    const band = ringRadius(ringIndex, rings.length)
    const geometry: RingGeometry | undefined = input.viewportWidth
      ? {
        plotWidth: input.viewportWidth,
        band: [Number.parseFloat(band[0]) / 100, Number.parseFloat(band[1]) / 100],
        fontSize: theme.type.base,
        fontFamily: theme.fontFamily,
      }
      : undefined
    return {
      type: 'pie' as const,
      name: ring.label,
      id: ring.key,
      radius: band,
      center: ['50%', '50%'],
      selectedMode: false,
      universalTransition: { enabled: morphEnabled() },
      percentPrecision: rawPercentPrecision,
      minAngle: minimumSliceAngle,
      label: insideLabels
        ? {
          position: 'inside' as const,
          fontWeight: 'bold' as const,
          formatter: (params: unknown) => sliceLabel(sliceLabels, params, input.locale, input.shareDecimalSeparator, geometry),
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

/**
 * The mark for the period that has not finished yet.
 *
 * It used to print the period's name beside the mark, once per series: on a
 * three-series panel that is three copies of "с начала года" stacked on each
 * other and on the data markers. The name is now stated once, by the panel —
 * in the legend and in the caption over the plot — and the mark itself only
 * says "this value is partial": a hatched fill on a column, an open ring on a
 * line, with the tinted band behind them covering the whole period.
 */
function incompletePeriodItem<T extends Record<string, unknown>>(
  item: T,
  point: RowPoint,
  input: ChartInput,
  theme: EChartsTheme,
  active: boolean,
): T & Record<string, unknown> {
  const period = input.temporal?.period
  if (!active || !period || point.category !== period.category) return item
  const itemStyle = item.itemStyle && typeof item.itemStyle === 'object'
    ? item.itemStyle as Record<string, unknown>
    : {}
  return {
    ...item,
    symbol: 'emptyCircle',
    symbolSize: 10,
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

/**
 * The plot's inset from the card edge, before axis labels are measured.
 *
 * One object, read twice. The runtime hands the left and bottom insets to
 * `containLabel`, which grows them by whatever the axis labels measure; the
 * visual-regression profile cannot use that measurement, because canvas text
 * metrics for a variable font land on a rounding boundary and shift the whole
 * plot by a pixel between otherwise identical runs. So VR pins the same grid
 * with the measurement replaced by a fixed allowance.
 *
 * The two used to be separate literals, and three commits edited only the
 * runtime one: visual regression was guarding a geometry no reader ever saw.
 */
const pinnedAxisLabelBox = { width: 80, height: 22 }

/** The fill of a tinted region. */
interface BandStyle {
  color: string
  opacity: number
}

/** Both ends of one `markArea` region; ECharts requires exactly two. */
type MarkAreaBand = [{ xAxis: string | number; itemStyle: BandStyle }, { xAxis: string | number }]

/**
 * The value bounds a chart should hold when a derived overlay leaves the range
 * of the data it was derived from.
 *
 * A linear regression over «Поступления денежных средств» extrapolates below
 * zero. Left to itself the axis grows to −20bn to keep the fitted line whole,
 * and the measured series — the reason the panel exists — collapses into the
 * top third of the plot. The data (and any stated threshold, which the reader
 * must be able to see) defines the frame; an extrapolation that leaves it is
 * clipped at the edge, where it visibly runs out of the plot.
 *
 * Only the side an overlay would have pushed is pinned, and never tighter than
 * the baseline ECharts would have chosen anyway: pinning both ends would turn a
 * zero-based axis into one that starts at the data's floor, and a non-zero
 * baseline exaggerates every movement on the plot. An ordinary chart, and the
 * end no overlay reaches, keep ECharts' own axis.
 */
export function clampedValueBounds(
  data: readonly number[],
  overlay: readonly number[],
): { min?: number; max?: number } | undefined {
  if (data.length === 0 || overlay.length === 0) return undefined
  const dataMin = Math.min(...data)
  const dataMax = Math.max(...data)
  const overlayMin = Math.min(...overlay)
  const overlayMax = Math.max(...overlay)
  if (overlayMin >= dataMin && overlayMax <= dataMax) return undefined
  const span = dataMax - dataMin
  if (!Number.isFinite(span) || span <= 0) return undefined
  const step = 10 ** Math.floor(Math.log10(span)) / 2
  const pad = span * 0.05
  const floor = (value: number) => Math.floor(value / step) * step
  const ceil = (value: number) => Math.ceil(value / step) * step
  return {
    // Readings that never go negative keep the zero baseline they are read
    // against, whatever the fitted line does below it.
    ...(overlayMin < dataMin ? { min: dataMin >= 0 ? 0 : floor(dataMin - pad) } : {}),
    ...(overlayMax > dataMax ? { max: ceil(dataMax + pad) } : {}),
  }
}

/** Every finite number in one frame column. */
function columnValues(input: ChartInput, field: string | undefined): number[] {
  const index = columnIndex(input, field)
  if (index < 0) return []
  return input.frame.rows.flatMap((row) => {
    const value = chartValue(row[index])
    return typeof value === 'number' ? [value] : []
  })
}

/**
 * The typical distance between two categories on a time axis, used to give a
 * band around a single instant the same width one category occupies.
 */
function medianTimeStep(timestamps: readonly number[]): number {
  const sorted = [...new Set(timestamps)].sort((left, right) => left - right)
  if (sorted.length < 2) return 0
  const steps = sorted.slice(1).map((value, index) => value - sorted[index]!).sort((left, right) => left - right)
  return steps[Math.floor(steps.length / 2)] ?? 0
}

/**
 * The axis labels that carry an annotation, as the axis prints them.
 *
 * The mark for an event is a dot on its own tick, and hovering that tick names
 * it — so the adapter, which owns the axis hover bubble, needs the same
 * mapping the axis formatter uses.
 */
export function annotatedAxisValues(input: ChartInput, shown?: ReadonlySet<string>): string[] {
  const categoryField = availableEncodingField(input, input.encoding.category, input.encoding.label) ?? ''
  const categoryIsTime = input.frame.columns.find((column) => column.name === categoryField)?.type === 'time'
  const timeAxis = input.kind !== 'bar' && input.kind !== 'hbar' && categoryIsTime
  return (input.temporal?.annotations ?? [])
    .filter((_, index) => !shown || shown.has(overlayId.annotation(index)))
    .map(({ at }) => {
      const parsed = timestamp(at)
      return timeAxis && parsed !== undefined ? timeLabel(input, categoryField, parsed) : at
    })
}

/**
 * The annotations standing at one axis value, for the tick's hover bubble.
 * The value arrives as ECharts hands it over: a category string on a category
 * axis, a timestamp on a time one.
 */
export function annotationsAtAxisValue(input: ChartInput, raw: unknown): string[] {
  return (input.temporal?.annotations ?? [])
    .filter(({ at }) => {
      if (String(raw) === at) return true
      const parsed = timestamp(at)
      return parsed !== undefined && typeof raw === 'number' && parsed === raw
    })
    .map(({ label }) => label)
}

/**
 * Widest a single bar may be drawn.
 *
 * A bar's width encodes nothing — its length does — so a chart with one or two
 * categories was handing each of them a third of the card and painting a
 * 330×190px slab that says exactly what a number says. Capped, a one-category
 * result reads as one bar on an axis instead of as a filled panel.
 */
const maximumBarWidth = 96

/**
 * Clear space between two neighbouring categories, as a share of the slot.
 *
 * ECharts' own default is computed from the series count and collapses towards
 * zero as categories multiply: four quarters of one colour merged into a single
 * continuous green block with no quarter boundaries visible anywhere in it.
 */
const barCategoryGap = '28%'

/**
 * The most marks that can carry a printed value.
 *
 * Above it `labelLayout: { hideOverlap: true }` starts dropping labels by
 * collision, which is non-deterministic in reading order: the chart then shows
 * figures for an arbitrary subset of its bars, which reads as missing data
 * rather than as a layout decision. Below it every mark gets its value or none
 * does. Twelve is where a 500px panel stops being able to hold a formatted
 * money label per bar without them touching.
 */
export const maximumLabelledMarks = 12

/**
 * Whether this panel prints its values on the marks.
 *
 * One rule, three reasons for it. A producer can ask (`dataLabels`). A
 * logarithmic axis has to, because a log bar's length states an order of
 * magnitude and not a value. And a linear axis whose spread it cannot show has
 * to, because the alternative is eleven months drawn as eleven baselines — the
 * same diagnosis as the log case, on a panel whose producer never asked for a
 * log axis and should not have to.
 *
 * All three are then subject to the same ceiling: a label the reader cannot
 * rely on being there is worse than no label at all.
 */
export function shouldPrintDataLabels(options: {
  explicit?: boolean
  logarithmic: boolean
  obscured: boolean
  isBar: boolean
  stacked: boolean
  markCount: number
}): boolean {
  // A stacked segment is bounded by its neighbours, not by the plot; its label
  // has nowhere to go but on top of the segment above it.
  if (options.stacked) return options.explicit === true
  const wanted = options.explicit === true
    || options.logarithmic
    || (options.isBar && options.obscured)
    // A handful of bars can always afford their own figures, and a policy of
    // "labels when they fit" is the only one under which a reader can tell the
    // absence of a label from the absence of a value.
    || (options.isBar && options.markCount <= maximumLabelledMarks)
  return wanted && options.markCount <= maximumLabelledMarks
}

/**
 * The stride at which a category axis prints its ticks.
 *
 * ECharts decides this from measured label widths, which is why two panels
 * stacked over the same quarters — the premium bars and the ratio line beneath
 * them — labelled every third quarter and every second one respectively, and
 * could not be read against each other at all. Deriving the stride from the
 * category count alone makes it a property of the domain, so two panels sharing
 * a domain share a grid whatever their labels happen to measure.
 */
export function categoryTickInterval(count: number, maximumTicks = 8): number {
  if (count <= maximumTicks) return 0
  return Math.ceil(count / maximumTicks) - 1
}

/**
 * How much of a horizontal bar chart's width its category names may take.
 *
 * The cap was a flat 260px. In a 407px panel that is two thirds of the canvas:
 * the zero tick disappeared, nine of eleven bars drew at zero width, and the
 * longest bar ended exactly at the plot edge — a chart stating nothing, on a
 * panel whose neighbour at the same width was fine. What differed was label
 * length, so the allowance is now a share of the plot rather than a constant.
 */
export function categoryLabelWidth(plotWidth: number | undefined): number {
  if (!plotWidth || plotWidth <= 0) return 260
  return Math.max(72, Math.min(260, Math.round(plotWidth * 0.38)))
}

/**
 * The same allowance expressed as the string length that fits in it.
 *
 * `axisLabel.width` clips the drawn glyphs; this shortens the string before it
 * is drawn, so a name that does not fit loses its middle rather than its end
 * and keeps both identifying halves. The two have to be derived from one
 * number or the ellipsis lands somewhere other than the clip.
 *
 * It is also the honest test for "the label decision changed": the allowance
 * moves with every pixel of the box, but what a reader sees only changes when
 * this does. The mounted adapter rebuilds on that, not on the raw width.
 */
export function categoryLabelLimit(plotWidth: number | undefined): number {
  return Math.max(8, Math.round(categoryLabelWidth(plotWidth) / 6.5))
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
  const colorBySequence = isBar && input.presentation?.colorBy === 'sequence'
  const barWidth = input.presentation?.barWidthPx
  // A stack states that its segments add up to the column, so only the series
  // that really are parts of the whole may join it: anything the producer
  // named as a line is drawn over the columns instead, on the same axis but
  // outside the sum.
  const lineSeries = new Set(input.presentation?.lineSeries ?? [])
  const stacked = isBar && input.presentation?.stack === true
  const dataLabels = shouldPrintDataLabels({
    explicit: input.presentation?.dataLabels,
    logarithmic,
    obscured: linearScaleObscuresValues(input.frame, input.encoding),
    isBar,
    stacked,
    markCount: points.filter((point) => typeof point.value === 'number').length,
  })
  const categoryColor = (category: string, index: number) =>
    input.seriesColor?.(category, index) ?? theme.seriesColor(category) ?? theme.colors[index % theme.colors.length]
  /**
   * One hue, stepped in lightness along the order of the categories.
   *
   * An ordered dimension drawn in the categorical palette — ten age bands in
   * teal, cyan, sky, red, orange, purple — says the bands are unrelated, and
   * makes the shape of the distribution harder to read than the bar lengths
   * alone. The ramp runs from a light tint of the accent to the accent itself,
   * so the sequence is visible and its direction is unambiguous.
   */
  const sequenceColor = (index: number, count: number) => {
    const accent = input.theme.palette.accent ?? theme.colors[0] ?? theme.accent
    const step = count <= 1 ? 1 : index / (count - 1)
    return mixColor(accent, theme.card, 0.25 + step * 0.75) ?? accent
  }
  // One list of overlays, shared with the panel's legend, so a mark cannot be
  // drawn here under a vocabulary the legend does not print.
  const overlays = chartOverlays({
    temporal: input.temporal,
    hasComparison: Boolean(input.encoding.previous),
    labels: input.labels,
    formatValue: (value) => input.format(input.encoding.value ?? '', value),
    formatCategory: categoryDisplayFormatter({
      format: (value) => input.format(categoryField, value),
      isTime: categoryIsTime,
      hasDeclaredFormat: Boolean(input.categoryFormatDefined),
      locale: input.locale,
    }),
    hidden: input.hiddenOverlays,
  })
  const shown = activeOverlayIds(overlays)
  const period = input.temporal?.period
  const periodShown = Boolean(period) && shown.has(overlayId.incomplete)
  // The final segment of a line is drawn dashed when the last category is the
  // one that has not finished. That means the body of the line stops one point
  // short and a dashed tail carries the last two points, which is only possible
  // when the incomplete period really is at the end of the axis.
  const lastTimestamp = Math.max(...points.flatMap((point) => point.timestamp === undefined ? [] : [point.timestamp]))
  const incompleteIsLast = periodShown && !isBar && (timeAxis
    ? timestamp(period!.category) === lastTimestamp
    : categories[categories.length - 1] === period!.category)
  const seriesData = seriesNames.map((name) => (timeAxis
    ? points
      .filter((point): point is RowPoint & { timestamp: number } => point.series === name && point.timestamp !== undefined)
      .sort((left, right) => left.timestamp - right.timestamp)
      .map((point) => incompletePeriodItem(
        { ...dataItem(point, input, theme), value: [point.timestamp, point.value] },
        point,
        input,
        theme,
        periodShown,
      ))
    : categories.map((category, categoryIndex) => {
      const point = points.find((candidate) => candidate.category === category && candidate.series === name)
      if (!point) return null
      const item = dataItem(point, input, theme)
      const distributed = colorByCategory
        ? categoryColor(category, categoryIndex)
        : colorBySequence ? sequenceColor(categoryIndex, categories.length) : undefined
      return incompletePeriodItem(
        distributed ? { ...item, itemStyle: { ...item.itemStyle, color: distributed } } : item,
        point,
        input,
        theme,
        periodShown,
      )
    })))
  const currentSeries = seriesNames.map((name, index) => ({
    type: isBar && !lineSeries.has(name) ? 'bar' as const : 'line' as const,
    name: name || undefined,
    universalTransition: { enabled: morphEnabled() },
    stack: stacked && !lineSeries.has(name) ? 'total' : undefined,
    barWidth: isBar && !lineSeries.has(name) && barWidth ? barWidth : undefined,
    barMaxWidth: isBar && !lineSeries.has(name) && !barWidth ? maximumBarWidth : undefined,
    barCategoryGap: isBar && !lineSeries.has(name) ? barCategoryGap : undefined,
    // Bar length on a logarithmic axis communicates order of magnitude, not
    // the literal value. Print that value on the mark so restoring the scale
    // never makes the chart less informative than its linear counterpart.
    label: dataLabels && !lineSeries.has(name)
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
    // With the mark count capped above, `hideOverlap` is a last resort against
    // two labels that still touch rather than the rule that decides which
    // values the reader is shown.
    labelLayout: dataLabels && !lineSeries.has(name) ? { hideOverlap: true } : undefined,
    // The panel's resolver knows about colours pinned to the n-th series of
    // this panel; ECharts' own palette does not, and left to itself it walks a
    // default sequence that has nothing to do with the legend beside it.
    itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name) },
    areaStyle: input.kind === 'area' ? { opacity: 0.18 } : undefined,
    showSymbol: !isBar || lineSeries.has(name),
    // The body of the line stops one point short when the last period is
    // unfinished; the dashed tail below carries that final segment. The
    // boundary point belongs to both, and the tooltip prints it once.
    data: incompleteIsLast
      ? [...(seriesData[index] ?? []).slice(0, -1), null]
      : seriesData[index] ?? [],
  }))
  /**
   * The unfinished end of a line, drawn dashed so the segment that is still
   * being filled in cannot be read as a completed movement.
   *
   * It carries the same name as the series it continues: the two are one
   * reading, and the axis tooltip deduplicates the point they share.
   */
  const incompleteTailSeries = incompleteIsLast ? seriesNames.flatMap((name, index) => {
    const data = seriesData[index] ?? []
    const color = input.seriesColor?.(name, index) ?? theme.seriesColor(name) ?? theme.colors[index % theme.colors.length]
    const tail = data.slice(-2)
    if (tail.length < 2) return []
    return [{
      type: 'line' as const,
      name: name || undefined,
      id: `lens-incomplete-tail-${index}`,
      z: 3,
      showSymbol: true,
      itemStyle: { color },
      lineStyle: { type: 'dashed' as const, width: 2, color },
      areaStyle: input.kind === 'area' ? { opacity: 0.18, color } : undefined,
      data: timeAxis ? tail : categories.map((_, position) => position >= data.length - 2 ? data[position] : null),
    }]
  }) : []
  const comparisonSeries = input.encoding.previous && shown.has(overlayId.comparison) ? seriesNames.map((name, index) => ({
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
  const regression = shown.has(overlayId.trend) ? input.temporal?.regression : undefined
  // A fitted line is not a measurement, and it used to say so only by being
  // dashed — in the measured series' own colour, which is the strongest claim
  // a chart can make that two lines are the same quantity. The wide dash is
  // now the fitted idiom everywhere. The ink is the trend ink when the panel
  // draws a single series, because then the hue carries no identity to lose;
  // with several series the hue is the only thing that says which trend
  // belongs to which line, so it is kept and the dash carries the meaning.
  const trendColor = (name: string, index: number) => seriesNames.length > 1
    ? (input.seriesColor?.(name, index) ?? theme.seriesColor(name) ?? theme.colors[index % theme.colors.length])
    : theme.trend
  const regressionSeries = regression ? seriesNames.map((name, index) => ({
    type: 'line' as const,
    name: `${name ? `${name} · ` : ''}${regression.label || input.labels?.trend || 'Trend'}`,
    silent: true,
    z: 4,
    showSymbol: false,
    itemStyle: { color: trendColor(name, index) },
    lineStyle: { type: [10, 5] as [number, number], width: 2, color: trendColor(name, index), opacity: 0.95 },
    data: temporalFieldData(input, points, regression.field, name, categories, timeAxis),
  })) : []
  const movingAverageSeries = (input.temporal?.movingAverages ?? [])
    .filter((average) => shown.has(overlayId.average(average.window)))
    .flatMap((average) => seriesNames.map((name, index) => ({
      type: 'line' as const,
      name: `${name ? `${name} · ` : ''}${average.label || input.labels?.movingAverage(average.window) || `SMA ${average.window}`}`,
      silent: true,
      z: 5,
      showSymbol: false,
      itemStyle: { color: input.seriesColor?.(name, index) ?? theme.seriesColor(name) },
      lineStyle: { width: 3, opacity: 0.95 },
      data: temporalFieldData(input, points, average.field, name, categories, timeAxis),
    })))
  const annualized = shown.has(overlayId.estimate) ? input.temporal?.period?.annualizedField : undefined
  const annualizedSeries = annualized ? seriesNames.map((name) => ({
    type: 'line' as const,
    name: input.labels?.estimate || 'Estimate',
    silent: true,
    z: 6,
    showSymbol: true,
    symbol: 'diamond',
    symbolSize: 11,
    itemStyle: { color: theme.mutedText },
    // A dot, against the fitted line's wide dash: both are derived, and this
    // one is derived from a period that has not happened yet.
    lineStyle: { type: 'dotted' as const, width: 2, color: theme.mutedText },
    data: temporalFieldData(input, points, annualized, name, categories, timeAxis),
  })) : []
  const forecastSeries = input.temporal?.forecast && shown.has(overlayId.forecast) ? seriesNames.flatMap((name, index) => {
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
  /**
   * A stated threshold: one warning-coloured hairline running the width of the
   * plot, with the value on a chip at the axis end of it.
   *
   * It used to be a grey dash indistinguishable from the four derived overlays
   * around it, and it printed its whole name inside the plot at the right-hand
   * end — for which the grid reserved 152px of width whether or not anything
   * was ever drawn there. The name belongs to the legend; the plot keeps the
   * number.
   */
  const markLineData = (input.temporal?.referenceLines ?? [])
    .filter((_, index) => shown.has(overlayId.reference(index)))
    .map((reference) => {
      const value = input.format(input.encoding.value ?? '', reference.value)
      return {
        name: reference.label || value,
        yAxis: reference.value,
        lineStyle: { type: 'solid' as const, color: theme.warn, width: 1 },
        label: {
          show: true,
          position: 'insideStartTop' as const,
          formatter: value,
          color: theme.warn,
          backgroundColor: theme.warnSoft,
          borderColor: theme.warn,
          borderWidth: 1,
          borderRadius: 4,
          padding: [2, 5] as [number, number],
          fontSize: 10,
          fontWeight: 600 as const,
        },
      }
    })
  const axisValue = (raw: string): string | number => timeAxis ? (timestamp(raw) ?? raw) : raw
  const halfStep = timeAxis
    ? medianTimeStep(points.flatMap((point) => point.timestamp === undefined ? [] : [point.timestamp])) / 2
    : 0
  const bandAt = (raw: string, style: BandStyle): MarkAreaBand => {
    const value = axisValue(raw)
    return typeof value === 'number'
      ? [{ xAxis: value - halfStep, itemStyle: style }, { xAxis: value + halfStep }]
      // On a category axis a single category is both ends of the band; the
      // host series below is a bar, and a bar's marker positioning resolves
      // those ends to the band's own edges rather than to its centre twice.
      : [{ xAxis: value, itemStyle: style }, { xAxis: value }]
  }
  const bandFrom = (raw: string, style: BandStyle): MarkAreaBand => {
    const start = axisValue(raw)
    const last = timeAxis
      ? lastTimestamp + halfStep
      : categories[categories.length - 1] ?? start
    return typeof start === 'number'
      ? [{ xAxis: start - halfStep, itemStyle: style }, { xAxis: last }]
      : [{ xAxis: start, itemStyle: style }, { xAxis: last }]
  }
  /**
   * The regions. An event and a projection are true of a span of the axis, not
   * of a value, so each is a tinted band rather than another full-height line
   * competing with the data for the reader's attention.
   */
  const markAreaData = [
    ...(periodShown ? [bandAt(period!.category, { color: theme.faintText, opacity: 0.12 })] : []),
    ...(input.temporal?.annotations ?? [])
      .filter((_, index) => shown.has(overlayId.annotation(index)))
      .map((annotation) => bandAt(annotation.at, { color: theme.accent, opacity: 0.09 })),
    ...(input.temporal?.forecast && shown.has(overlayId.forecast)
      ? [bandFrom(input.temporal.forecast.start, { color: theme.faintText, opacity: 0.08 })]
      : []),
  ]
  // Mark lines and areas annotate the axes without contributing synthetic
  // values to the data extent. In particular, adding a vertical annotation
  // must never rescale the values it is meant to explain.
  const markLine = markLineData.length > 0
    ? { silent: true, symbol: ['none', 'none'], data: markLineData }
    : undefined
  const markArea = markAreaData.length > 0 ? { silent: true, data: markAreaData } : undefined
  const annotatedCurrentSeries = currentSeries.map((series, index) => index === 0 && (markLine || (markArea && isBar))
    ? { ...series, ...(markLine ? { markLine } : {}), ...(markArea && isBar ? { markArea } : {}) }
    : series)
  /**
   * A band on a category axis needs a bar's sense of where a category starts
   * and ends; a line series resolves both ends of a one-category band to the
   * same point and draws nothing. A bar panel already has a bar to hang the
   * band on. A line panel gets this: an empty, silent bar series that exists
   * only to own the regions. It carries no data, so it cannot take a column
   * slot away from anything — and a line panel has no columns to share.
   */
  const bandHostSeries = markArea && !isBar ? [{
    type: 'bar' as const,
    id: 'lens-overlay-bands',
    silent: true,
    z: 0,
    animation: false,
    tooltip: { show: false },
    itemStyle: { opacity: 0 },
    data: [],
    markArea,
  }] : []
  const series = [
    ...bandHostSeries,
    ...comparisonSeries,
    ...annotatedCurrentSeries,
    ...incompleteTailSeries,
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
  // An annotated category is marked on the axis itself — a dot before its
  // name, in the annotation's own colour — and hovering that tick names the
  // event. The plot keeps the band and stays free of text.
  const annotatedTicks = new Set(annotatedAxisValues(input, shown).map((value) => String(value)))
  const tickLabel = (value: string) => annotatedTicks.has(value) ? `{annotated|●} ${value}` : value
  const annotatedRich = { annotated: { color: theme.accent, fontSize: 9 } }
  // The names a horizontal bar hangs in the plot's left margin get a share of
  // the plot, not a constant, and the string is shortened to that share.
  const nameWidth = categoryLabelWidth(input.viewportWidth)
  const categoryAxis = {
    type: 'category' as const,
    data: categories,
    triggerEvent: true,
    // A category axis puts index 0 at the origin, which on a vertical axis is
    // the bottom — so a frame sorted largest-first was drawn smallest-first
    // and a ranked breakdown read backwards to anyone scanning it top-down.
    // Rows arrive in the order the producer meant them to be read; on a
    // horizontal bar that order runs downwards.
    ...(horizontal ? { inverse: true } : {}),
    ...axisStyle(theme),
    // One stride for a given number of categories, so two panels over the same
    // domain print the same grid.
    ...(horizontal ? {} : { axisLabel: { color: theme.mutedText, interval: categoryTickInterval(categories.length) } }),
    ...(annotatedTicks.size > 0 && !horizontal
      ? {
        axisLabel: {
          color: theme.mutedText,
          interval: categoryTickInterval(categories.length),
          formatter: (value: string) => tickLabel(String(value)),
          rich: annotatedRich,
        },
      }
      : {}),
    // ECharts passes the category index as the formatter's second argument;
    // passing middleEllipsis directly therefore treated the index as a width
    // and mangled already-short labels such as "Apr".
    ...(horizontal
      ? {
        axisLabel: {
          color: theme.mutedText,
          width: nameWidth,
          formatter: (value: string) => middleEllipsis(String(value), categoryLabelLimit(input.viewportWidth)),
          overflow: 'truncate' as const,
        },
      }
      : {}),
  }
  const temporalAxis = {
    type: 'time' as const,
    triggerEvent: true,
    ...axisStyle(theme),
    axisLabel: {
      color: theme.mutedText,
      formatter: (value: number) => tickLabel(timeLabel(input, categoryField, value)),
      ...(annotatedTicks.size > 0 ? { rich: annotatedRich } : {}),
    },
  }
  // An extrapolation may not redefine the frame the measurements are read in.
  const clamped = logarithmic ? undefined : clampedValueBounds(
    [
      ...numericPointValues,
      ...(input.encoding.previous && shown.has(overlayId.comparison) ? columnValues(input, input.encoding.previous) : []),
      // A threshold is a stated fact about the same axis, so the frame has to
      // hold it even when no reading comes near it.
      ...(input.temporal?.referenceLines ?? [])
        .filter((_, index) => shown.has(overlayId.reference(index)))
        .map((reference) => reference.value),
    ],
    [
      ...(regression ? columnValues(input, regression.field) : []),
      ...(annualized ? columnValues(input, annualized) : []),
      ...(input.temporal?.forecast && shown.has(overlayId.forecast)
        ? [input.temporal.forecast.valueField, input.temporal.forecast.lowerField, input.temporal.forecast.upperField]
          .flatMap((field) => columnValues(input, field))
        : []),
    ],
  )
  // The ticks dropped the currency so that eight gridlines would not repeat it;
  // nothing was ever given it in exchange, which is how a revenue axis came to
  // read «35 млн» with the unit findable only by hovering a mark. It is stated
  // once, at the end of the axis, where a unit belongs.
  const valueAxis = {
    type: logarithmic ? 'log' as const : 'value' as const,
    show: points.some((point) => typeof point.value === 'number' && point.value !== 0),
    logBase: logarithmic ? (input.valueAxis?.logBase || 10) : undefined,
    min: clamped?.min,
    max: logMaximum ?? clamped?.max,
    ...(input.valueUnit
      ? {
        name: input.valueUnit,
        nameLocation: 'end' as const,
        nameGap: horizontal ? 16 : 10,
        nameTextStyle: {
          color: theme.mutedText,
          fontSize: theme.type.xs,
          align: horizontal ? ('center' as const) : ('right' as const),
        },
      }
      : {}),
    ...axisStyle(theme),
    axisLabel: { color: theme.mutedText, formatter: axisValueFormatter(input), hideOverlap: true },
  }

  // One grid, described once. A reference line no longer reserves width for a
  // name it prints in the legend, so the line runs edge to edge.
  const gridInset = {
    left: 16,
    right: horizontal ? (logarithmic ? 88 : 16) : 64,
    top: 24,
    bottom: timeAxis ? 52 : horizontal ? 12 : 32,
  }
  return {
    ...baseOption(theme),
    // The VR profile cannot use `containLabel`: it derives the inset from
    // canvas text measurement, which lands on a rounding boundary for the
    // variable font and shifts the whole plot by 1px between runs. It pins
    // *this* grid with a fixed allowance in place of that measurement, rather
    // than describing a second geometry that no reader ever sees.
    grid: isVisualRegression()
      ? {
        ...gridInset,
        left: gridInset.left + pinnedAxisLabelBox.width,
        bottom: gridInset.bottom + pinnedAxisLabelBox.height,
        containLabel: false,
      }
      : { ...gridInset, containLabel: true },
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
