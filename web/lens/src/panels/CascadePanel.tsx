import type { KeyboardEvent, MouseEvent } from 'react'
import type { Frame, Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate } from '../runtime'
import { usePanelNavigation } from './actions'
import { columnIndex, displayText, panelField } from './data'
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

export interface CascadeStage {
  label: string
  value: number
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
  const maximum = Math.max(1, ...frame.rows.map((row) => Math.max(0, numeric(row[valueIndex]))))

  return frame.rows.map((row, index) => {
    const value = numeric(row[valueIndex])
    const cut = numeric(row[cutIndex])
    const rawWidth = value > 0 ? Math.min(100, value / maximum * 100) : 0
    return {
      label: displayText(row[labelIndex], `Stage ${index + 1}`),
      value,
      formattedValue: formatValue(value),
      cut,
      formattedCut: signedCut(cut, formatCut),
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
   * The frame row backing this column, for per-row actions. The synthetic
   * closing total repeats the last stage the cascade already offers, so it
   * carries none and stays inert.
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

export function buildWaterfallModel(
  stages: CascadeStage[],
  formatValue: (value: unknown) => string,
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
    rowIndex?: number
  }> = [{
    label: first.label,
    from: 0,
    to: first.value,
    value: first.value,
    kind: 'start',
    tone: first.tone,
    annotation: first.annotation,
    // An opening total is not a movement, so it has no part-of-a-movement to
    // band off. Only the deduction/addition bars carry a split.
    split: 0,
    splitLabel: '',
    rowIndex: first.rowIndex,
  }]
  const magnitude = Math.max(...stages.map((stage) => Math.abs(stage.value)), 1)
  const residual = magnitude * 1e-6
  for (let index = 1; index < stages.length; index += 1) {
    const previousStage = stages[index - 1]
    const currentStage = stages[index]
    if (!previousStage || !currentStage) continue
    const previous = previousStage.value
    const current = currentStage.value
    const value = current - previous
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
  if (stages.length > 1 && raw.at(-1)?.kind !== 'end') {
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
    const height = Math.max(1.5, bottom - top)
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
    const splittable = splitMagnitude > residual && splitMagnitude < magnitude - residual
    return {
      label: item.label,
      value: item.value,
      formattedValue: item.kind === 'start' || item.kind === 'end'
        ? formatValue(item.value)
        : signedChange(item.value, formatValue),
      top,
      height,
      // Share of the bar, not of the plot: the band is painted inside the bar,
      // so its percentage resolves against the bar's own box. Scaling by the
      // bar's plot height too shrank every band by that height again — a 24.5%
      // share of a bar 18% tall drew as 4.5% of it.
      splitHeight: splittable ? 100 * (splitMagnitude / magnitude) : undefined,
      formattedSplit: splittable ? formatValue(splitMagnitude) : undefined,
      splitLabel: splittable ? item.splitLabel : undefined,
      // Only a floating bar leaves a balance under it; the totals stand on zero
      // already. A bar dipping below zero leaves nothing, hence the clamp.
      underlayHeight: item.kind === 'start' || item.kind === 'end'
        ? undefined
        : Math.max(0, zero - bottom) || undefined,
      connectorTop: y(item.to),
      zero,
      kind: item.kind,
      checkpoint: item.kind === 'end' && index < raw.length - 1 ? true : undefined,
      tone: item.tone,
      annotation: item.annotation,
      rowIndex: item.rowIndex,
    }
  })
  const ticks: WaterfallTick[] = []
  for (let value = plotMinimum; value <= plotMaximum + step / 10; value += step) {
    ticks.push({ value, label: formatValue(value), top: y(value) })
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
  const stages = frame.data ? buildCascadeStages(panel, frame.data, formatValue, formatCut) : []
  const waterfall = panel.presentation?.bridgeLayout === 'waterfall'
    ? buildWaterfallModel(stages, formatValue)
    : { items: [], ticks: [], zero: 100 }

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

  return (
    <PanelFrame panel={panel} frame={frame}>
      {panel.presentation?.bridgeLayout === 'waterfall' ? (
        <WaterfallPlot
          interaction={(item) => stageInteraction(item.rowIndex, item.label)}
          label={translate('cascade.stages', '{name} stages', { name: panel.title })}
          model={waterfall}
          // An image exposes no children to assistive tech; once the columns
          // are activatable the container must group them instead.
          role={anyInteractive ? 'group' : 'img'}
        />
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
                    <strong data-direction={stage.cut > 0 ? 'down' : stage.cut < 0 ? 'up' : 'flat'}>{stage.formattedCut}</strong>
                  </div>
                )}
                <div
                  className={`lens-cascade-stage${stage.final ? ' lens-cascade-stage-final' : ''}`}
                  data-final={stage.final || undefined}
                  data-tone={stage.tone}
                  style={interaction ? { cursor: 'pointer' } : undefined}
                  {...interaction}
                >
                  <div className="lens-cascade-stage-label">
                    <span className="lens-cascade-stage-title">
                      <span>{stage.label}</span>
                      {stage.annotation && (
                        <small className="lens-cascade-stage-annotation">{stage.annotation}</small>
                      )}
                    </span>
                    <strong data-negative={stage.value < 0 || undefined}>{stage.formattedValue}</strong>
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
