import type { Availability, Confidence, Encoding, Frame, Panel, Theme } from '../contract'

const fallbackSeries = ['#2563eb', '#059669', '#d97706', '#7c3aed', '#0891b2', '#dc2626']

/**
 * Resolves a datum's color the way the chart adapter does, so a React-rendered
 * legend cannot disagree with the plot: the panel's own `panelId:index` series
 * entry first, then a series entry keyed by label, then the palette order, then
 * the panel accent.
 *
 * `positional: false` drops the `panelId:index` entry. That entry pins a color
 * to the n-th row of the panel's *own* frame, so once the panel is showing a
 * drill level it describes rows that are no longer on screen — the legend was
 * printing the root's second color next to the level's second slice, which the
 * plot (which only ever resolves by label) had drawn in a different color.
 */
export function seriesColorResolver(
  theme: Theme,
  panel: Panel,
  { positional = true }: { positional?: boolean } = {},
): (label: string, index: number) => string | undefined {
  const palette = Object.values(theme.palette).filter((color) => color.trim() !== '')
  const colors = palette.length > 0 ? palette : fallbackSeries
  const resolve = (value: string | undefined) => (value ? theme.palette[value] ?? value : undefined)
  return (label, index) => (positional ? resolve(theme.series[`${panel.id}:${index}`]) : undefined)
    ?? resolve(theme.series[label])
    ?? colors[index % colors.length]
    ?? panel.accent
}

export function columnIndex(frame: Frame | undefined, field: string | undefined): number {
  if (!frame || !field?.trim()) return -1
  return frame.columns.findIndex((column) => column.name === field)
}

export function cell(frame: Frame | undefined, field: string | undefined, row = 0): unknown {
  const index = columnIndex(frame, field)
  return index >= 0 ? frame?.rows[row]?.[index] : undefined
}

export function displayText(value: unknown, fallback: string): string {
  if (typeof value === 'string' && value.trim()) return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return fallback
}

export const encodingRoles: ReadonlyArray<keyof Encoding> = [
  'label', 'value', 'id', 'series', 'category', 'cut', 'cutLabel', 'final',
  'tone', 'share', 'confidence', 'availability',
]

export function panelField(panel: Panel, role: keyof Encoding): string | undefined {
  return panel.encoding[role]?.trim() || undefined
}

/* -------------------------------------------------------------------------- */
/* Keyed join for the metric panels (flow / hierarchy / relationship).        */
/*                                                                            */
/* Values live in the dataset frame, joined to the declared structure by key: */
/* `encoding.id` names the key column, `encoding.value` the value column, and */
/* optional `share` / `confidence` / `availability` columns override each      */
/* element's structural defaults. The join is deterministic (by key, never by */
/* row order) and enforces the locked missing-data semantics.                 */
/* -------------------------------------------------------------------------- */

/** A single element's resolved numeric value, or an explicit unavailable slot. */
export type ResolvedNumber =
  | { kind: 'value'; value: number }
  | { kind: 'unavailable' }

/** Everything the frame contributes for one structural element. */
export interface ResolvedElement {
  value: ResolvedNumber
  /** Optional producer-supplied share, already a display magnitude. */
  share?: number
  /** Frame override of the element's structural confidence. */
  confidence?: Confidence
  /** Frame override of the element's structural availability. */
  availability?: Availability
}

/**
 * The frame either yields a deterministic resolver keyed by element key, or the
 * panel is mis-contracted (a configured column is absent, or a key is
 * duplicated) and must be surfaced as a panel-level error — never as a
 * per-element unavailable, which would hide a wiring mistake behind a plausible
 * business state.
 */
export type KeyedJoin =
  | { kind: 'ok'; resolve: (key: string) => ResolvedElement }
  | { kind: 'contract-error'; message: string }

const UNAVAILABLE_ELEMENT: ResolvedElement = { value: { kind: 'unavailable' } }

const CONFIDENCE_VALUES: ReadonlySet<string> = new Set<Confidence>([
  'verified', 'calculated', 'proxy', 'requires_reconciliation',
])

const AVAILABILITY_VALUES: ReadonlySet<string> = new Set<Availability>([
  'available', 'config_required', 'empty_source', 'unavailable',
])

/** A finite number, coerced from a numeric string; undefined for null/NaN/±Inf. */
function finiteNumber(value: unknown): number | undefined {
  if (typeof value === 'number') return Number.isFinite(value) ? value : undefined
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : undefined
  }
  return undefined
}

function asConfidence(value: unknown): Confidence | undefined {
  return typeof value === 'string' && CONFIDENCE_VALUES.has(value) ? (value as Confidence) : undefined
}

function asAvailability(value: unknown): Availability | undefined {
  return typeof value === 'string' && AVAILABILITY_VALUES.has(value) ? (value as Availability) : undefined
}

/** A stable string key from a primitive cell; non-primitives cannot be a key. */
function keyText(value: unknown): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return ''
}

export interface KeyedJoinMessages {
  /** A configured column is not present in the frame. */
  missingColumn: (column: string) => string
  /** Two frame rows resolve to the same key. */
  duplicateKey: (key: string) => string
}

/**
 * Resolves a value column index, distinguishing "not configured" from
 * "configured but absent from the frame": the former is legitimate (structural
 * default applies), the latter is a contract error.
 */
function roleIndex(frame: Frame, field: string | undefined): number | 'unset' | 'missing' {
  if (!field) return 'unset'
  const index = columnIndex(frame, field)
  return index >= 0 ? index : 'missing'
}

/**
 * Builds the keyed join for a metric panel.
 *
 * An empty frame (no rows) is not a contract error: the declared structure
 * renders with every element `unavailable`, which the panels present as their
 * "empty structure" state — deliberately distinct from the loading skeleton.
 */
export function buildKeyedJoin(panel: Panel, frame: Frame, messages: KeyedJoinMessages): KeyedJoin {
  // No rows → the structure stands on its own with unavailable values. The
  // column contract is only meaningful once there is data to bind.
  if (frame.rows.length === 0) {
    return { kind: 'ok', resolve: () => UNAVAILABLE_ELEMENT }
  }

  const idField = panelField(panel, 'id')
  const valueField = panelField(panel, 'value')
  const idIndex = roleIndex(frame, idField)
  const valueIndex = roleIndex(frame, valueField)
  // id and value are required roles: undeclared or absent is a wiring error.
  if (idIndex === 'unset' || idIndex === 'missing') return { kind: 'contract-error', message: messages.missingColumn(idField ?? 'id') }
  if (valueIndex === 'unset' || valueIndex === 'missing') return { kind: 'contract-error', message: messages.missingColumn(valueField ?? 'value') }

  const shareIndex = roleIndex(frame, panelField(panel, 'share'))
  const confidenceIndex = roleIndex(frame, panelField(panel, 'confidence'))
  const availabilityIndex = roleIndex(frame, panelField(panel, 'availability'))
  // Optional roles: absent-when-configured is still a contract error; unset is fine.
  if (shareIndex === 'missing') return { kind: 'contract-error', message: messages.missingColumn('share') }
  if (confidenceIndex === 'missing') return { kind: 'contract-error', message: messages.missingColumn('confidence') }
  if (availabilityIndex === 'missing') return { kind: 'contract-error', message: messages.missingColumn('availability') }

  // Narrowed to number here (-1 marks "not configured").
  const share = shareIndex === 'unset' ? -1 : shareIndex
  const confidence = confidenceIndex === 'unset' ? -1 : confidenceIndex
  const availability = availabilityIndex === 'unset' ? -1 : availabilityIndex

  // Deterministic by key: duplicate keys are rejected rather than last-write-wins.
  const byKey = new Map<string, ReadonlyArray<unknown>>()
  for (const row of frame.rows) {
    const key = keyText(row[idIndex])
    if (byKey.has(key)) return { kind: 'contract-error', message: messages.duplicateKey(key) }
    byKey.set(key, row)
  }

  const resolve = (key: string): ResolvedElement => {
    const row = byKey.get(key)
    if (!row) return UNAVAILABLE_ELEMENT
    const number = finiteNumber(row[valueIndex])
    return {
      value: number === undefined ? { kind: 'unavailable' } : { kind: 'value', value: number },
      share: share >= 0 ? finiteNumber(row[share]) : undefined,
      confidence: confidence >= 0 ? asConfidence(row[confidence]) : undefined,
      availability: availability >= 0 ? asAvailability(row[availability]) : undefined,
    }
  }

  return { kind: 'ok', resolve }
}
