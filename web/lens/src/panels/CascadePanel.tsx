import type { CSSProperties, KeyboardEvent, MouseEvent } from 'react'
import type { Frame, Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate } from '../runtime'
import { usePanelNavigation } from './actions'
import { columnIndex, displayText, panelField } from './data'
import { PanelFrame } from './PanelFrame'

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
  width: number
  /** Explicit semantic tone overriding the direction default; absent = default. */
  tone?: CascadeTone
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
  const toneField = panelField(panel, 'tone')
  const labelIndex = columnIndex(frame, labelField)
  const valueIndex = columnIndex(frame, valueField)
  const cutIndex = columnIndex(frame, cutField)
  const cutLabelIndex = columnIndex(frame, cutLabelField)
  const finalIndex = columnIndex(frame, finalField)
  const toneIndex = columnIndex(frame, toneField)
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
      tone: toneIndex >= 0 ? asTone(row[toneIndex]) : undefined,
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
  /** Explicit semantic tone overriding the direction default; absent = default. */
  tone?: CascadeTone
  /**
   * The frame row backing this column, for per-row actions. The synthetic
   * closing total repeats the last stage the cascade already offers, so it
   * carries none and stays inert.
   */
  rowIndex?: number
}

interface WaterfallTick {
  value: number
  label: string
  top: number
}

interface WaterfallModel {
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

function buildWaterfallModel(
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
    rowIndex?: number
  }> = [{
    label: first.label,
    from: 0,
    to: first.value,
    value: first.value,
    kind: 'start',
    tone: first.tone,
    rowIndex: first.rowIndex,
  }]
  for (let index = 1; index < stages.length; index += 1) {
    const previousStage = stages[index - 1]
    const currentStage = stages[index]
    if (!previousStage || !currentStage) continue
    const previous = previousStage.value
    const current = currentStage.value
    const value = current - previous
    // A final=true row is a closing TOTAL checkpoint, not a stage-to-stage
    // movement. The canonical closing row restates the running total it closes
    // (cut=0), so a delta bar here would be a zero-height "+0" duplicate of the
    // synthesized `end` total below. Skip it; it is drawn once as that end bar.
    // A final row carrying a genuine movement stays a real deduction/addition
    // and renders here as before. The running totals arrive as floating-point
    // sums, so the restated delta is a residual (~1e-5) rather than exactly 0;
    // these are UZS amounts in the billions where any real movement is >= 1e6,
    // so a sub-unit (< 1 UZS) floor separates residual noise from real deltas.
    if (currentStage.final && Math.abs(value) < 1) continue
    raw.push({
      label: currentStage.cutLabel || currentStage.label,
      from: previous,
      to: current,
      value,
      kind: value < 0 ? 'decrease' : 'increase',
      tone: currentStage.tone,
      rowIndex: currentStage.rowIndex,
    })
  }
  if (stages.length > 1) {
    // Prefer an explicit final=true stage as the closing total; fall back to the
    // last stage for frames that never mark one (back-compat, unchanged output).
    const closing = stages.find((stage) => stage.final) ?? stages.at(-1)
    if (closing) {
      raw.push({
        label: closing.label,
        from: 0,
        to: closing.value,
        value: closing.value,
        kind: 'end',
        tone: closing.tone,
      })
    }
  }

  let minimum = 0
  let maximum = 0
  raw.forEach((item) => {
    minimum = Math.min(minimum, item.from, item.to)
    maximum = Math.max(maximum, item.from, item.to)
  })
  const step = niceCeiling(Math.max(1, maximum - minimum) / 3)
  const plotMinimum = minimum < 0 ? Math.floor(minimum / step) * step : 0
  const plotMaximum = Math.max(step, Math.ceil(maximum / step) * step)
  const plotRange = Math.max(1, plotMaximum - plotMinimum)
  const y = (value: number) => Math.max(0, Math.min(100, (plotMaximum - value) / plotRange * 100))

  const items = raw.map((item) => {
    const top = y(Math.max(item.from, item.to))
    const bottom = y(Math.min(item.from, item.to))
    return {
      label: item.label,
      value: item.value,
      formattedValue: item.kind === 'start' || item.kind === 'end'
        ? formatValue(item.value)
        : signedChange(item.value, formatValue),
      top,
      height: Math.max(1.5, bottom - top),
      connectorTop: y(item.to),
      zero: y(0),
      kind: item.kind,
      tone: item.tone,
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
        <div
          aria-label={translate('cascade.stages', '{name} stages', { name: panel.title })}
          className="lens-waterfall"
          data-lens-waterfall
          // An image exposes no children to assistive tech; once the columns
          // are activatable the container must group them instead.
          role={anyInteractive ? 'group' : 'img'}
          style={{
            '--lens-waterfall-count': waterfall.items.length,
            '--lens-waterfall-zero': `${waterfall.zero}%`,
          } as CSSProperties}
        >
          <div className="lens-waterfall-chart">
            <div className="lens-waterfall-axis" aria-hidden="true">
              {waterfall.ticks.map((tick) => (
                <span key={tick.value} style={{ top: `${tick.top}%` }}>{tick.label}</span>
              ))}
            </div>
            <div className="lens-waterfall-plot">
              {waterfall.ticks.map((tick) => (
                <span
                  className="lens-waterfall-gridline"
                  key={`grid-${tick.value}`}
                  style={{ top: `${tick.top}%` }}
                />
              ))}
              <div className="lens-waterfall-zero" />
              {waterfall.items.map((item, index) => {
                const interaction = stageInteraction(item.rowIndex, item.label)
                return (
                  <div className="lens-waterfall-column" key={`${item.label}-${index}`}>
                    {index < waterfall.items.length - 1 && (
                      <span
                        className="lens-waterfall-connector"
                        style={{ top: `${item.connectorTop}%` }}
                      />
                    )}
                    <div
                      className="lens-waterfall-bar"
                      data-kind={item.kind}
                      data-tone={item.tone}
                      style={{
                        top: `${item.top}%`,
                        height: `${item.height}%`,
                        ...(interaction ? { cursor: 'pointer' } : {}),
                      }}
                      {...interaction}
                    >
                      <strong>{item.formattedValue}</strong>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
          <div className="lens-waterfall-labels">
            {waterfall.items.map((item, index) => (
              <span key={`${item.label}-label-${index}`}>{item.label}</span>
            ))}
          </div>
        </div>
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
                    <span>{stage.label}</span>
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
