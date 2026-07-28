import type { Encoding, FieldFormat, Frame, Panel, PanelKind, Semantics } from '../contract'
import type { PrintSection } from '../runtime/print'

/** Panel kinds ChartHost can draw on its own, without a dashboard provider. */
export const chartKinds = new Set<PanelKind>(['pie', 'donut', 'radial', 'bar', 'hbar', 'line', 'area'])

/**
 * Panel kinds whose printed form is a formula rather than a chart with an
 * evidence table: the structure — operators, indent, direction — is the
 * reading, and a two-column table of the same numbers is not a substitute.
 */
export const formulaKinds = new Set<PanelKind>(['metric_flow', 'metric_hierarchy', 'metric_relationship'])

export type ChartKind = 'pie' | 'donut' | 'radial' | 'bar' | 'hbar' | 'line' | 'area'

export function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

export function text(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'string') return value || '—'
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return '—'
}

export function indexOf(frame: Frame, field: string | undefined): number {
  return field ? frame.columns.findIndex(({ name }) => name === field) : -1
}

export function formatsForEncoding(panel: Panel, encoding: Encoding): Record<string, FieldFormat> {
  const formats = { ...panel.format }
  for (const [role, target] of Object.entries(encoding) as Array<[keyof Encoding, string | undefined]>) {
    const source = panel.encoding[role]
    if (target && source && panel.format[source] && !formats[target]) formats[target] = panel.format[source]!
  }
  return formats
}

export function viewFor(semantics: Semantics, preferred: PanelKind): PanelKind {
  if (semantics === 'partition') {
    return ['pie', 'donut', 'bar', 'hbar'].includes(preferred) ? preferred : 'donut'
  }
  if (semantics === 'reconciliation') return 'cascade'
  if (semantics === 'series') return 'line'
  return 'table'
}

/**
 * The panel a section prints as: the authored panel for a root section, and the
 * drill level's own view, encoding and formats below it.
 */
export function sectionPanel(section: PrintSection): Panel {
  const semantics = section.perspective?.semantics ?? section.panel.semantics
  const encoding = section.level.encoding ?? section.panel.encoding
  return {
    ...section.panel,
    kind: section.root ? section.panel.kind : (section.level.view ?? viewFor(semantics, section.panel.kind)),
    title: section.level.label.trim() || section.panel.title,
    semantics,
    encoding,
    format: formatsForEncoding(section.panel, encoding),
    presentation: section.level.presentation ?? section.panel.presentation,
  }
}

/**
 * Row-identical frames print once. Drill levels frequently restate their
 * parent's rows, and two perspectives over the same cut resolve to the same
 * numbers; a printed audit gains nothing from the repetition.
 */
export function frameSignature(frame: Frame | undefined): string | undefined {
  if (!frame || frame.rows.length === 0) return undefined
  return JSON.stringify([frame.columns.map(({ name }) => name), frame.rows])
}
