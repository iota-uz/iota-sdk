import type { Frame, Panel, Formula } from '../contract'
import { buildKeyedJoin, columnIndex, finiteNumber, panelField } from '../panels/data'
import { elementQuality } from '../panels/metricViews'
import { formatFieldValue } from '../runtime/format'

/** One variable of a formula, ready to show: its text, source and value. */
export interface FormulaVariableView {
  key: string
  label: string
  source?: string
  /** Formatted value in the figure's own slice; undefined when it is not there. */
  value?: string
}

/**
 * Resolves each variable's value from the frame that already carries the
 * figure — same filters, period and row — so hovering a symbol can never show a
 * number from a different slice than the one printed next to it. A variable
 * with no reachable value keeps its place in the list without one; a host that
 * rendered the page on the server may hand the text over in `variable.value`.
 */
export function resolveFormulaVariables(
  formula: Formula,
  panel: Panel,
  frame: Frame | undefined,
  locale: string,
  row = 0,
): FormulaVariableView[] {
  let join: ReturnType<typeof buildKeyedJoin> | undefined
  const stageJoin = () => {
    if (!frame) return undefined
    join ??= buildKeyedJoin(panel, frame, { missingColumn: String, duplicateKey: String })
    return join
  }
  return (formula.variables ?? []).map((variable) => {
    const view: FormulaVariableView = { key: variable.key, label: variable.label, source: variable.source, value: variable.value || undefined }
    const ref = variable.ref
    if (!ref || !frame) return view
    if (ref.kind === 'stage') {
      const resolved = stageJoin()
      if (resolved?.kind === 'ok') {
        const element = resolved.resolve(ref.name)
        const structural = panel.metricFlow?.stages.find((stage) => stage.key === ref.name)
          ?? panel.metricHierarchy?.rows.find((row) => row.key === ref.name)
          ?? {}
        // An unavailable stage prints a dash on the panel; it must not print a
        // number here.
        if (elementQuality(element, structural, panel).showDash) {
          // Unavailable in this slice: neither the frame nor a server-supplied
          // fallback may state a number for it.
          view.value = undefined
        } else if (element.value.kind === 'value') {
          const field = panelField(panel, 'value')
          view.value = formatFieldValue(element.value.value, field ? panel.format[field] : undefined, locale)
        }
      }
      return view
    }
    const index = columnIndex(frame, ref.name)
    const cell = index >= 0 ? frame.rows[row]?.[index] : undefined
    if (cell === undefined || cell === null) return view
    const numeric = finiteNumber(cell)
    view.value = formatFieldValue(numeric ?? cell, variable.format ?? panel.format[ref.name], locale)
    return view
  })
}

/** The figure itself, formatted — the line shown under the formula. */
export function formulaValueLine(panel: Panel, frame: Frame | undefined, locale: string, row = 0): string | undefined {
  const field = panelField(panel, 'value')
  if (!frame || !field) return undefined
  // A flow's figure is its declared result stage, not whichever row comes
  // first; with none (or several) there is no single figure to state.
  if (panel.kind === 'metric_flow') {
    const results = panel.metricFlow?.stages.filter((stage) => stage.role === 'result') ?? []
    if (results.length !== 1) return undefined
    const join = buildKeyedJoin(panel, frame, { missingColumn: String, duplicateKey: String })
    if (join.kind !== 'ok') return undefined
    const element = join.resolve(results[0]!.key)
    if (element.value.kind !== 'value' || elementQuality(element, results[0]!, panel).showDash) return undefined
    return formatFieldValue(element.value.value, panel.format[field], locale)
  }
  if (panel.kind === 'metric_hierarchy' || panel.kind === 'metric_relationship') return undefined
  const index = columnIndex(frame, field)
  const cell = index >= 0 ? frame.rows[row]?.[index] : undefined
  if (cell === undefined || cell === null) return undefined
  return formatFieldValue(finiteNumber(cell) ?? cell, panel.format[field], locale)
}
