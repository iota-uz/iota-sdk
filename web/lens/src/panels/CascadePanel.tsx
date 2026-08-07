import { useCallback, type KeyboardEvent, type MouseEvent } from 'react'
import type { Frame, Panel } from '../contract'
import { axisUnit, useAxisFormat, useFormat, useFormatExact, usePanelFrame, useTranslate } from '../runtime'
import { rawValueText } from '../runtime/format'
import { usePanelNavigation } from './actions'
import { columnIndex, displayText, finiteNumber, panelField } from './data'
import { PanelFrame } from './PanelFrame'
import { WaterfallPlot } from './WaterfallPlot'

/* eslint-disable react-refresh/only-export-components */

const widthFloor = 2

function numeric(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

/**
 * What a stage prints where its amount would be, when there is no amount.
 *
 * The same em dash the metric panels put in an `unavailable` value slot, so a
 * board that mixes a flow, a hierarchy and a bridge says "not known" one way.
 * Spoken as the translated «Unavailable», never as "em dash".
 */
export const unknownValueText = '—'

function boolean(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  return typeof value === 'string' && value.trim().toLowerCase() === 'true'
}

/**
 * The explicit per-stage semantic tone carried by the Encoding.Tone column. It
 * overrides the flow-direction default color so a stage reads by meaning (a cost
 * is negative/red) rather than by direction (a decrease is green).
 */
export type CascadeTone = 'neutral' | 'positive' | 'negative' | 'inflow'

const CASCADE_TONES: ReadonlySet<string> = new Set<CascadeTone>([
  'neutral', 'positive', 'negative', 'inflow',
])

/** A recognized tone, or undefined so the renderer keeps the direction default. */
function asTone(value: unknown): CascadeTone | undefined {
  return typeof value === 'string' && CASCADE_TONES.has(value) ? (value as CascadeTone) : undefined
}

function signedCut(value: number, format: (value: unknown) => string): string {
  if (value > 0) return `−${format(value)}`
  if (value < 0) return `+${format(-value)}`
  return format(0)
}

function signedChange(value: number, format: (value: unknown) => string): string {
  if (value > 0) return `+${format(value)}`
  if (value < 0) return `−${format(-value)}`
  return format(0)
}

/** The same sign convention over a formatter that may have nothing to say. */
function signedChangeExact(value: number, format: (value: unknown) => string | undefined): string | undefined {
  const magnitude = format(Math.abs(value))
  if (magnitude === undefined) return undefined
  return value === 0 ? magnitude : `${value > 0 ? '+' : '−'}${magnitude}`
}

export interface CascadeStage {
  label: string
  /**
   * The running total this stage stands at — meaningful only when `hasValue`.
   * It stays a plain number so every layout measure downstream keeps its type;
   * an unknown stage carries 0 and must never be read as one.
   */
  value: number
  /**
   * False when the frame carries no amount for this stage: a null cell, an
   * empty string, a non-finite number, or a column the producer left out.
   *
   * "No data for this line" and "this line is zero" are different statements on
   * a bridge, and coercing the first into the second is how a stage with
   * nothing behind it drew a full-height deduction down to zero. Every consumer
   * that reads `value` must ask this first.
   */
  hasValue: boolean
  formattedValue: string
  cut: number
  formattedCut: string
  cutLabel: string
  final: boolean
  /** Optional producer-authored context shown as a compact stage badge. */
  annotation: string
  width: number
  /** Explicit semantic tone overriding the direction default; absent = default. */
  tone?: CascadeTone
  /** The part of this stage's own movement that differs in kind from the rest. */
  split: number
  /** What that part is called. */
  splitLabel: string
  /** Index of the frame row this stage was built from (per-row actions). */
  rowIndex: number
}

export function buildCascadeStages(
  panel: Panel,
  frame: Frame,
  formatValue: (value: unknown) => string,
  formatCut: (value: unknown) => string,
): CascadeStage[] {
  const labelField = panelField(panel, 'label') ?? 'label'
  const valueField = panelField(panel, 'value') ?? 'value'
  const cutField = panelField(panel, 'cut') ?? 'cut'
  const cutLabelField = panelField(panel, 'cutLabel') ?? 'cutLabel'
  const finalField = panelField(panel, 'final') ?? 'final'
  const annotationField = panelField(panel, 'annotation')
  const toneField = panelField(panel, 'tone')
  const splitField = panelField(panel, 'split')
  const splitLabelField = panelField(panel, 'splitLabel')
  const labelIndex = columnIndex(frame, labelField)
  const valueIndex = columnIndex(frame, valueField)
  const cutIndex = columnIndex(frame, cutField)
  const cutLabelIndex = columnIndex(frame, cutLabelField)
  const finalIndex = columnIndex(frame, finalField)
  const annotationIndex = columnIndex(frame, annotationField)
  const toneIndex = columnIndex(frame, toneField)
  const splitIndex = columnIndex(frame, splitField)
  const splitLabelIndex = columnIndex(frame, splitLabelField)
  // `finiteNumber` rather than `numeric`, and only here: the stage value is the
  // one field where "absent" is a statement the panel has to make. A cut or a
  // split that fails to parse is still an undivided, unmoved bar, which is what
  // the 0 those keep already draws.
  const stageValue = (row: ReadonlyArray<unknown>) => finiteNumber(row[valueIndex])
  const maximum = Math.max(1, ...frame.rows.map((row) => Math.max(0, stageValue(row) ?? 0)))

  return frame.rows.map((row, index) => {
    const resolved = stageValue(row)
    const hasValue = resolved !== undefined
    const value = resolved ?? 0
    const cut = numeric(row[cutIndex])
    const rawWidth = hasValue && value > 0 ? Math.min(100, value / maximum * 100) : 0
    return {
      label: displayText(row[labelIndex], `Stage ${index + 1}`),
      value,
      hasValue,
      formattedValue: hasValue ? formatValue(value) : unknownValueText,
      cut,
      // A stage whose total is unknown has an unknown movement: the frame's own
      // `cut` for such a row is a placeholder, and printing «0 UZS» beside it
      // would restate the very lie the em dash above it just refused to tell.
      formattedCut: hasValue ? signedCut(cut, formatCut) : unknownValueText,
      cutLabel: displayText(row[cutLabelIndex], ''),
      final: boolean(row[finalIndex]),
      annotation: annotationIndex >= 0 ? displayText(row[annotationIndex], '') : '',
      tone: toneIndex >= 0 ? asTone(row[toneIndex]) : undefined,
      split: splitIndex >= 0 ? numeric(row[splitIndex]) : 0,
      splitLabel: splitLabelIndex >= 0 ? displayText(row[splitLabelIndex], '') : '',
      width: rawWidth > 0 ? Math.max(widthFloor, rawWidth) : 0,
      rowIndex: index,
    }
  })
}

export interface WaterfallItem {
  label: string
  value: number
  formattedValue: string
  /**
   * The unabbreviated figure, when the field is compact and the plot's own
   * label is «−41,20 млрд UZS». The tooltip is the surface a reader opens to
   * look closer, so it is where the whole number belongs. Absent when nothing
   * was abbreviated away — then `formattedValue` already is the whole number.
   */
  exactValue?: string
  /** The same, for the split band. */
  exactSplit?: string
  /**
   * This column's machine value — plain digits in the unit the figure is drawn
   * in, for the clipboard. The reader copies the number they can see, not the
   * tiyin behind it.
   */
  rawValue?: string
  top: number
  height: number
  connectorTop: number
  zero: number
  kind: 'start' | 'increase' | 'decrease' | 'end'
  /**
   * True on a total that the cascade carries on past — a milestone rather than
   * the finish. Two identically filled dark columns would otherwise read as two
   * endings; the checkpoint takes a lighter treatment so the eye still knows
   * which bar the cascade stops at.
   */
  checkpoint?: boolean
  /**
   * True on a column that draws no movement bar: a step whose movement is
   * exactly nothing, or one whose amount is not known at all (see `unknown`).
   *
   * A cascade's bars carry a `min-height`, so a step that did not happen drew
   * the same 3px coloured rule as a step of −1,74 млн: on a P&L bridge "not
   * applicable" and "a real small number" looked identical, which is the one
   * thing a bridge may never say. Both are drawn as a hairline on the running
   * total instead of as a coloured movement off it.
   */
  noMovement?: boolean
  /**
   * True when the producer sent no amount for this stage.
   *
   * The third statement a bridge has to be able to make, and the one it used to
   * be unable to: not "this step is zero" but "nobody knows what this step is".
   * Coerced to zero it became a movement down to the running total — a stage
   * annotated «Нет данных» rendered a −149,00 млрд bar that was, exactly, the
   * preceding total. It shares `noMovement`'s hairline (there is no movement to
   * draw either way) and is told apart from a real zero by a dashed rule and an
   * em dash where the figure would be.
   */
  unknown?: boolean
  annotation: string
  /** Explicit semantic tone overriding the direction default; absent = default. */
  tone?: CascadeTone
  /**
   * The band drawn at the leading (upper) end of this bar for the part of the
   * movement that differs in kind from the rest. Height is a percentage of the
   * plot, like every other measure here, so it composes with `height`. Absent
   * when the row declares no split or declares one the bar cannot hold.
   */
  splitHeight?: number
  /** Already-formatted amount of that band. */
  formattedSplit?: string
  /** The band's own machine value, ready for the clipboard. */
  rawSplit?: string
  /** What the band is called; may be empty even when the band is drawn. */
  splitLabel?: string
  /**
   * How far the bar's foot sits above the zero line, as a plot percentage. It
   * is the remaining balance this deduction leaves behind, drawn as a translucent
   * accent block so the eye reads "the red bites out of the blue". Absent for
   * the opening and closing totals, which already stand on zero.
   */
  underlayHeight?: number
  /**
   * The frame row backing this column, for per-row actions.
   *
   * The synthetic closing total repeats a stage the cascade already offers, so
   * it inherits that stage's row rather than carrying none: a column that
   * displays the producer's «Настроить» badge and answers nothing when a reader
   * clicks it is worse than no column. Absent only when there is no backing row
   * at all.
   */
  rowIndex?: number
}

export interface WaterfallTick {
  value: number
  label: string
  top: number
}

export interface WaterfallModel {
  items: WaterfallItem[]
  ticks: WaterfallTick[]
  zero: number
}

function niceCeiling(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1
  const power = 10 ** Math.floor(Math.log10(value))
  const fraction = value / power
  if (fraction <= 1) return power
  if (fraction <= 2) return 2 * power
  if (fraction <= 5) return 5 * power
  return 10 * power
}

/** Round numbers a money axis is allowed to step by, per power of ten. */
const axisStepLadder = [1, 1.5, 2, 2.5, 3, 4, 5, 7.5]

/**
 * The step whose gridlines cover the data with the least dead space above it.
 *
 * Deriving one step from `range / 3` and rounding it up compounds two
 * roundings: a 655 bn column asked for a 218 bn step, got 500 bn, and ended up
 * under a 1 trn ceiling — a third of the plot spent on air, which is exactly
 * the height the bars were supposed to gain. Searching the step ladder for the
 * tightest ceiling instead lands on 100 bn and a 700 bn top. Between two steps
 * that fit equally well the coarser one wins, so a tight axis never costs a
 * thicket of gridlines.
 */
export function waterfallAxisStep(minimum: number, maximum: number): number {
  const span = Math.max(1, maximum - minimum)
  const fallback = niceCeiling(span / 3)
  let best: number | undefined
  let bestSpan = Number.POSITIVE_INFINITY
  for (let power = 10 ** Math.floor(Math.log10(span / 7)); power <= span; power *= 10) {
    for (const rung of axisStepLadder) {
      const step = rung * power
      if (!Number.isFinite(step) || step <= 0) continue
      const top = maximum > 0 ? Math.ceil(maximum / step) * step : 0
      const bottom = minimum < 0 ? Math.floor(minimum / step) * step : 0
      const divisions = Math.round((top - bottom) / step)
      if (divisions < 2 || divisions > 7) continue
      const plotSpan = top - bottom
      if (plotSpan < bestSpan || (plotSpan === bestSpan && step > (best ?? 0))) {
        bestSpan = plotSpan
        best = step
      }
    }
  }
  return best ?? fallback
}

/**
 * @param formatValue what a column prints on itself: the full, exact amount.
 * @param formatTick what a gridline prints: the compact axis form. They were
 * one formatter, so all eight gridlines of the P&L bridge repeated «млрд UZS»
 * while every ECharts axis in the runtime had long since dropped the unit and
 * stated it once. The unit is the plot's to state, not the tick's.
 */
export function buildWaterfallModel(
  stages: CascadeStage[],
  formatValue: (value: unknown) => string,
  formatTick: (value: unknown) => string = formatValue,
  // Two optional companions to `formatValue`, both for the tooltip: the
  // unabbreviated figure a compact plot label hides, and the machine value the
  // copy button puts on the clipboard. Grouped rather than trailing positional
  // so the printed report keeps calling this with three arguments.
  { formatExact = () => undefined, rawValue = () => undefined }: {
    formatExact?: (value: unknown) => string | undefined
    rawValue?: (value: unknown) => string | undefined
  } = {},
): WaterfallModel {
  const first = stages[0]
  if (!first) return { items: [], ticks: [], zero: 100 }
  const raw: Array<{
    label: string
    from: number
    to: number
    value: number
    kind: WaterfallItem['kind']
    tone?: CascadeTone
    annotation: string
    split: number
    splitLabel: string
    unknown?: boolean
    rowIndex?: number
  }> = [{
    label: first.label,
    from: 0,
    to: first.hasValue ? first.value : 0,
    value: first.hasValue ? first.value : 0,
    kind: 'start',
    tone: first.tone,
    annotation: first.annotation,
    // An opening total is not a movement, so it has no part-of-a-movement to
    // band off. Only the deduction/addition bars carry a split.
    split: 0,
    splitLabel: '',
    unknown: first.hasValue ? undefined : true,
    rowIndex: first.rowIndex,
  }]
  /**
   * The last total the cascade actually knows it stands at, and the height every
   * following movement is measured from.
   *
   * Reading it off `stages[index - 1]` made one missing amount everybody's
   * problem: the gap stage became a movement to zero, and the stage after it a
   * movement back up from zero, so a single hole corrupted the rest of the
   * bridge. An unknown stage leaves this untouched, so the damage stops at the
   * column that has nothing to say. An unknown OPENING total leaves the cascade
   * with no origin at all — zero is the only anchor left, and the known totals
   * that follow are still drawn at their own heights.
   */
  let running = first.hasValue ? first.value : 0
  const magnitude = Math.max(...stages.map((stage) => (stage.hasValue ? Math.abs(stage.value) : 0)), 1)
  const residual = magnitude * 1e-6
  for (let index = 1; index < stages.length; index += 1) {
    const currentStage = stages[index]
    if (!currentStage) continue
    // A stage with no amount is not a movement of any size. It is drawn as a
    // gap standing on the running total — keeping its name, its badge and its
    // frame row, because "we do not compute this yet" is a thing the panel is
    // there to say — and it leaves the total alone for the next known stage.
    if (!currentStage.hasValue) {
      raw.push({
        label: currentStage.cutLabel || currentStage.label,
        from: running,
        to: running,
        value: 0,
        // A final row keeps the closing branch even with nothing to close, so
        // it holds its declared place, keeps its row (and therefore its drill),
        // and no synthetic closing is appended after it.
        kind: currentStage.final ? 'end' : 'increase',
        tone: currentStage.tone,
        annotation: currentStage.annotation,
        split: 0,
        splitLabel: '',
        unknown: true,
        rowIndex: currentStage.rowIndex,
      })
      continue
    }
    const previous = running
    const current = currentStage.value
    const value = current - previous
    running = current
    // A final=true row is a TOTAL checkpoint, not a stage-to-stage movement.
    // The canonical checkpoint row restates the running total it closes
    // (cut=0), so a delta bar here would be a zero-height "+0" duplicate; it is
    // drawn instead as a full-height `end` bar standing on zero, IN PLACE. A
    // cascade may declare several — a statutory result mid-chart that the
    // remaining stages then carry on to a second, final total — and each one
    // marks its own position rather than being hoisted to the end. A final row
    // carrying a genuine movement stays a real deduction/addition and renders
    // below as before. The running totals arrive as floating-point sums, so
    // suppress only residuals relative to the scale of the data.
    if (currentStage.final && Math.abs(value) < residual) {
      raw.push({
        label: currentStage.label,
        from: 0,
        to: current,
        value: current,
        kind: 'end',
        tone: currentStage.tone,
        annotation: currentStage.annotation,
        split: 0,
        splitLabel: '',
        rowIndex: currentStage.rowIndex,
      })
      continue
    }
    raw.push({
      label: currentStage.cutLabel || currentStage.label,
      from: previous,
      to: current,
      value,
      kind: value < 0 ? 'decrease' : 'increase',
      tone: currentStage.tone,
      annotation: currentStage.annotation,
      split: currentStage.split,
      splitLabel: currentStage.splitLabel,
      rowIndex: currentStage.rowIndex,
    })
  }
  // A cascade that declares no closing row still ends somewhere, so the last
  // running total is restated as one. Two cases must not reach here: a stage
  // already drawn as an `end` (restating it produced the twin «Расчётный
  // результат» columns, the second of them inert), and a last stage with no
  // amount (there is no total to restate, only a second gap). What is
  // synthesized repeats a real stage, so it carries that stage's row and badge
  // — a column showing «Настроить» that answers nothing when clicked is worse
  // than no column at all.
  const last = raw.at(-1)
  if (stages.length > 1 && last && last.kind !== 'end' && !last.unknown) {
    const closing = stages.at(-1)
    if (closing) {
      raw.push({
        label: closing.label,
        from: 0,
        to: closing.value,
        value: closing.value,
        kind: 'end',
        tone: closing.tone,
        annotation: closing.annotation,
        split: 0,
        splitLabel: '',
        rowIndex: closing.rowIndex,
      })
    }
  }

  let minimum = 0
  let maximum = 0
  raw.forEach((item) => {
    minimum = Math.min(minimum, item.from, item.to)
    maximum = Math.max(maximum, item.from, item.to)
  })
  const step = waterfallAxisStep(minimum, maximum)
  const plotMinimum = minimum < 0 ? Math.floor(minimum / step) * step : 0
  const plotMaximum = Math.max(step, Math.ceil(maximum / step) * step)
  const plotRange = Math.max(1, plotMaximum - plotMinimum)
  const y = (value: number) => Math.max(0, Math.min(100, (plotMaximum - value) / plotRange * 100))

  const zero = y(0)
  const items = raw.map((item, index) => {
    const top = y(Math.max(item.from, item.to))
    const bottom = y(Math.min(item.from, item.to))
    // A movement of exactly zero gets no height at all; the stylesheet gives it
    // a hairline. Reserving the same 1.5% floor a real movement gets is what
    // made a zero indistinguishable from the smallest genuine step. A stage
    // with no amount takes the same hairline for the stronger reason: there is
    // no movement to draw, and any height at all would be one invented here.
    const unknown = item.unknown === true
    const noMovement = unknown || (item.kind !== 'start' && item.kind !== 'end' && item.value === 0)
    const height = noMovement ? 0 : Math.max(1.5, bottom - top)
    // A split is a portion OF the movement. Outside (0, |movement|) it is not
    // one — a producer that sends the whole movement, more than it, or nothing
    // is describing an undivided bar, and that is what we draw. Guarding here
    // rather than at the wire keeps a bad number from silently becoming a band
    // taller than the bar it lives in.
    //
    // The bounds carry the same residual tolerance the closing-total check uses,
    // and for the same reason: a movement is a difference of running totals, so
    // a split that equals it exactly in the producer's arithmetic arrives a few
    // ulps off here. 178.30 - 150.00 is 28.300000000000011, and a strict "<"
    // against a declared 28.30 would band the entire bar.
    const splitMagnitude = Math.abs(item.split)
    const magnitude = Math.abs(item.value)
    const splittable = !unknown && splitMagnitude > residual && splitMagnitude < magnitude - residual
    return {
      label: item.label,
      value: item.value,
      // An unknown column prints the em dash and nothing else: there is no
      // exact form of a figure that does not exist, and no machine value to put
      // on the clipboard, so the tooltip carries neither and shows no copy
      // button rather than offering to copy a zero.
      formattedValue: unknown
        ? unknownValueText
        : item.kind === 'start' || item.kind === 'end'
          ? formatValue(item.value)
          : signedChange(item.value, formatValue),
      exactValue: unknown
        ? undefined
        : item.kind === 'start' || item.kind === 'end'
          ? formatExact(item.value)
          : signedChangeExact(item.value, formatExact),
      rawValue: unknown ? undefined : rawValue(item.value),
      top,
      height,
      // Share of the bar, not of the plot: the band is painted inside the bar,
      // so its percentage resolves against the bar's own box. Scaling by the
      // bar's plot height too shrank every band by that height again — a 24.5%
      // share of a bar 18% tall drew as 4.5% of it.
      splitHeight: splittable ? 100 * (splitMagnitude / magnitude) : undefined,
      formattedSplit: splittable ? formatValue(splitMagnitude) : undefined,
      exactSplit: splittable ? formatExact(splitMagnitude) : undefined,
      rawSplit: splittable ? rawValue(splitMagnitude) : undefined,
      splitLabel: splittable ? item.splitLabel : undefined,
      // Only a floating bar leaves a balance under it; the totals stand on zero
      // already. A bar dipping below zero leaves nothing, hence the clamp.
      underlayHeight: item.kind === 'start' || item.kind === 'end'
        ? undefined
        : Math.max(0, zero - bottom) || undefined,
      connectorTop: y(item.to),
      zero,
      kind: item.kind,
      // A total nobody could compute is not an interim total the reader passes
      // through, so it does not take the hollow-column treatment or put the
      // legend that explains it on the panel.
      checkpoint: item.kind === 'end' && !unknown && index < raw.length - 1 ? true : undefined,
      noMovement: noMovement || undefined,
      unknown: unknown || undefined,
      tone: item.tone,
      annotation: item.annotation,
      rowIndex: item.rowIndex,
    }
  })
  const ticks: WaterfallTick[] = []
  for (let value = plotMinimum; value <= plotMaximum + step / 10; value += step) {
    ticks.push({ value, label: formatTick(value), top: y(value) })
  }
  return { items, ticks, zero: y(0) }
}

export function buildWaterfallItems(
  stages: CascadeStage[],
  formatValue: (value: unknown) => string,
): WaterfallItem[] {
  return buildWaterfallModel(stages, formatValue).items
}

export interface CascadePanelProps {
  panel: Panel
}

export function CascadePanel({ panel }: CascadePanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const valueField = panelField(panel, 'value') ?? 'value'
  const cutField = panelField(panel, 'cut') ?? 'cut'
  const formatValue = useFormat(panel.format[valueField])
  const formatCut = useFormat(panel.format[cutField] ?? panel.format[valueField])
  const formatTick = useAxisFormat(panel.format[valueField])
  // The plot draws compact figures and the tooltip un-abbreviates them; the
  // clipboard gets the machine value behind both.
  const formatExact = useFormatExact(panel.format[valueField])
  const rawValue = useCallback(
    (value: unknown) => rawValueText(value, panel.format[valueField]),
    [panel.format, valueField],
  )
  const unit = axisUnit(panel.format[valueField])
  const stages = frame.data ? buildCascadeStages(panel, frame.data, formatValue, formatCut) : []
  const waterfall = panel.presentation?.bridgeLayout === 'waterfall'
    ? buildWaterfallModel(stages, formatValue, formatTick, { formatExact, rawValue })
    : { items: [], ticks: [], zero: 100 }
  // The hollow column and the solid one are a real distinction — a total the
  // cascade carries on past, and the one it stops at — stated nowhere on the
  // panel. An encoding no one can look up is decoration, and this panel had no
  // legend, caption or footer of any kind to look it up in.
  const hasCheckpoint = waterfall.items.some((item) => item.checkpoint)

  // Per-stage navigation: the panel-wide navigate/open-drawer action resolved
  // against the stage's own frame row, exactly like chart marks. A stage whose
  // row carries no destination stays the inert block it always was.
  const navigation = usePanelNavigation(panel)
  const stageURL = (rowIndex: number | undefined): string | undefined => {
    if (rowIndex === undefined || !navigation.action || !frame.data) return undefined
    return navigation.urlForRow(frame.data, frame.data.rows[rowIndex])
  }
  const stageInteraction = (rowIndex: number | undefined, label: string) => {
    const href = stageURL(rowIndex)
    if (!href) return undefined
    const activate = (event: MouseEvent<HTMLElement> | KeyboardEvent<HTMLElement>) => {
      navigation.activate(href, event.currentTarget as HTMLElement)
    }
    return {
      role: 'button' as const,
      tabIndex: 0,
      'aria-label': translate('cascade.openStage', 'Open {name}', { name: label }),
      onClick: activate,
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => {
        if (event.key !== 'Enter' && event.key !== ' ') return
        event.preventDefault()
        activate(event)
      },
    }
  }
  const anyInteractive = Boolean(navigation.action) &&
    stages.some((stage) => stageURL(stage.rowIndex) !== undefined)
  // An em dash is a glyph, not a word: a reader who cannot see it is owed the
  // sentence it stands for. The same key the metric panels speak an absent
  // amount with, so the board has one word for "not known".
  const unavailable = translate('availability.unavailable', 'Unavailable')

  return (
    <PanelFrame panel={panel} frame={frame}>
      {panel.presentation?.bridgeLayout === 'waterfall' ? (
        <WaterfallPlot
          actionHint={translate('chart.drillHint', 'Select to explore')}
          axisUnit={unit}
          interaction={(item) => stageInteraction(item.rowIndex, item.label)}
          label={translate('cascade.stages', '{name} stages', { name: panel.title })}
          model={waterfall}
          // An image exposes no children to assistive tech; once the columns
          // are activatable the container must group them instead.
          role={anyInteractive ? 'group' : 'img'}
          unknownLabel={unavailable}
        >
          {hasCheckpoint && (
            <p className="lens-waterfall-key">
              <span className="lens-waterfall-key-item" data-mark="checkpoint">
                {translate('cascade.checkpoint', 'Interim total')}
              </span>
              <span className="lens-waterfall-key-item" data-mark="result">
                {translate('cascade.result', 'Closing total')}
              </span>
            </p>
          )}
        </WaterfallPlot>
      ) : (
        <div
          aria-label={translate('cascade.stages', '{name} stages', { name: panel.title })}
          className="lens-cascade"
          role="list"
        >
          {stages.map((stage, index) => {
            const interaction = stageInteraction(stage.rowIndex, stage.label)
            return (
              <div className="lens-cascade-step" key={`${stage.label}-${index}`} role="listitem">
                {index > 0 && stage.cutLabel && (
                  <div className="lens-cascade-connector">
                    <span>{stage.cutLabel}</span>
                    <strong
                      data-direction={!stage.hasValue ? 'unknown' : stage.cut > 0 ? 'down' : stage.cut < 0 ? 'up' : 'flat'}
                    >
                      {stage.formattedCut}
                    </strong>
                  </div>
                )}
                <div
                  className={`lens-cascade-stage${stage.final ? ' lens-cascade-stage-final' : ''}`}
                  data-final={stage.final || undefined}
                  data-tone={stage.tone}
                  data-unknown={!stage.hasValue || undefined}
                  {...interaction}
                >
                  <div className="lens-cascade-stage-label">
                    <span className="lens-cascade-stage-title">
                      <span>{stage.label}</span>
                      {stage.annotation && (
                        <small className="lens-cascade-stage-annotation">{stage.annotation}</small>
                      )}
                    </span>
                    <strong data-negative={(stage.hasValue && stage.value < 0) || undefined}>
                      {stage.formattedValue}
                      {!stage.hasValue && <span className="lens-sr-only">{unavailable}</span>}
                    </strong>
                  </div>
                  <div className="lens-cascade-track" aria-hidden="true">
                    <span style={{ width: `${stage.width}%` }} />
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </PanelFrame>
  )
}
