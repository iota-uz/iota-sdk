import type {
  Availability,
  Confidence,
  MetricFlowStage,
  MetricFlowStageRole,
  MetricHierarchyRow,
  MetricRelationshipConfig,
  MetricRelationshipDirection,
  MetricRelationshipEnd,
  Panel,
} from '../contract'
import type { KeyedJoin, ResolvedElement } from './data'

/* -------------------------------------------------------------------------- */
/* The grammar of the three metric panels — which operator a stage prints, how */
/* deep a hierarchy row sits, which end of a relationship derives which —      */
/* expressed as pure functions over the contract.                             */
/*                                                                            */
/* Both the interactive panel and the printed report render these structures, */
/* and they must not disagree: a formula that reads «− 6.81» on screen and     */
/* «+ 6.81» on paper is worse than one that prints nothing. Nothing here       */
/* translates or formats beyond the caller's own formatter, so the grammar has */
/* exactly one definition.                                                    */
/* -------------------------------------------------------------------------- */

/** Stages that carry an operator and therefore print an unsigned magnitude. */
const OPERATOR_ROLES: ReadonlySet<MetricFlowStageRole> = new Set<MetricFlowStageRole>(['add', 'subtract'])

/** A stage's signed contribution to the reconciliation sum (grammar, not display). */
export function contribution(role: MetricFlowStageRole, value: number): number {
  return role === 'subtract' ? -value : value
}

export interface ElementQuality {
  confidence?: Confidence
  availability?: Availability
  /** The value slot shows an em dash rather than a number. */
  showDash: boolean
}

/**
 * Quality of one resolved element, with the locked precedence: the frame
 * overrides the structural declaration, and a value that cannot exist is at
 * least "unavailable" — a more specific producer status survives.
 */
export function elementQuality(
  element: ResolvedElement,
  structural: { confidence?: Confidence; availability?: Availability },
  panel: Panel,
): ElementQuality {
  const confidence = element.confidence ?? structural.confidence ?? panel.confidence
  const declared = element.availability ?? structural.availability ?? panel.availability
  const missing = element.value.kind === 'unavailable'
  const availability: Availability | undefined = missing
    ? (declared && declared !== 'available' ? declared : 'unavailable')
    : declared
  return {
    ...(confidence ? { confidence } : {}),
    ...(availability ? { availability } : {}),
    showDash: missing || (availability !== undefined && availability !== 'available'),
  }
}

export interface FlowStageView extends ElementQuality {
  stage: MetricFlowStage
  role: MetricFlowStageRole
  /** Rendered operator glyph, or '' for an input stage. */
  operator: string
  /** Formatted magnitude: unsigned under an operator, signed otherwise. */
  text: string
}

/**
 * The formula, stage by stage. An absent amount must not rewrite the declared
 * grammar: an `add` stage keeps its `+` and a `subtract` stage its `−`, because
 * sign normalization only applies to a value that is actually shown.
 */
export function flowStageViews(
  panel: Panel,
  join: KeyedJoin,
  formatValue: (value: number) => string,
): Array<FlowStageView> {
  if (join.kind !== 'ok') return []
  return (panel.metricFlow?.stages ?? []).map((stage) => {
    const element = join.resolve(stage.key)
    const quality = elementQuality(element, stage, panel)
    const numeric = element.value.kind === 'value' ? element.value.value : 0
    const isOperand = OPERATOR_ROLES.has(stage.role)
    let operator = ''
    if (quality.showDash && stage.role === 'add') operator = '+'
    else if (quality.showDash && stage.role === 'subtract') operator = '−'
    else if (isOperand) operator = contribution(stage.role, numeric) >= 0 ? '+' : '−'
    else if (stage.role === 'intermediate' || stage.role === 'result') operator = '='
    return {
      ...quality,
      stage,
      role: stage.role,
      operator,
      text: formatValue(isOperand ? Math.abs(numeric) : numeric),
    }
  })
}

/**
 * How far the declared result stands from the sum of its operands, or nothing
 * when the flow declares no reconciliation, balances inside its tolerance, or
 * carries an unresolved operand — an incomplete sum would report a discrepancy
 * that does not exist.
 */
export function flowReconcileDelta(panel: Panel, join: KeyedJoin): number | undefined {
  const reconcile = panel.metricFlow?.reconcile
  if (join.kind !== 'ok' || reconcile === undefined) return undefined
  let sum = 0
  let result: number | undefined
  for (const stage of panel.metricFlow?.stages ?? []) {
    const value = join.resolve(stage.key).value
    if (stage.role === 'result') {
      if (value.kind !== 'value') return undefined
      result = value.value
      continue
    }
    if (stage.role === 'intermediate') continue
    if (value.kind !== 'value') return undefined
    sum += contribution(stage.role, value.value)
  }
  if (result === undefined) return undefined
  const delta = result - sum
  return Math.abs(delta) <= (reconcile.tolerance ?? 0) ? undefined : delta
}

export interface HierarchyRowView extends ElementQuality {
  row: MetricHierarchyRow
  depth: number
  isParent: boolean
  valueText: string
  shareText?: string
  /** Whole-to-parts check for a parent row: balanced, or off by this much. */
  reconcile?: { balanced: boolean; delta: number }
}

/**
 * The hierarchy, row by row. `parent` is authoritative for depth; the wire may
 * carry a derived `depth`, and a detached parent reference must not send the
 * walk running.
 */
export function hierarchyRowViews(
  panel: Panel,
  join: KeyedJoin,
  formatValue: (value: number) => string,
  formatShare: (value: number) => string,
): Array<HierarchyRowView> {
  if (join.kind !== 'ok') return []
  const resolve = join.resolve
  const rows = panel.metricHierarchy?.rows ?? []
  const reconcile = panel.metricHierarchy?.reconcile
  const byKey = new Map(rows.map((row) => [row.key, row]))
  const parentKeys = new Set(rows.map((row) => row.parent).filter((key): key is string => Boolean(key)))
  const childrenByParent = new Map<string, Array<MetricHierarchyRow>>()
  for (const row of rows) {
    if (!row.parent) continue
    const list = childrenByParent.get(row.parent) ?? []
    list.push(row)
    childrenByParent.set(row.parent, list)
  }

  const depthOf = (row: MetricHierarchyRow): number => {
    if (typeof row.depth === 'number') return row.depth
    let depth = 0
    let current: MetricHierarchyRow | undefined = row
    const seen = new Set<string>()
    while (current?.parent && !seen.has(current.key)) {
      seen.add(current.key)
      current = byKey.get(current.parent)
      if (!current) break
      depth += 1
    }
    return depth
  }

  return rows.map((row) => {
    const element = resolve(row.key)
    const quality = elementQuality(element, row, panel)
    const children = childrenByParent.get(row.key)
    let summary: HierarchyRowView['reconcile']
    if (reconcile && children && children.length > 0 && element.value.kind === 'value') {
      let sum = 0
      let complete = true
      for (const child of children) {
        const childValue = resolve(child.key).value
        if (childValue.kind !== 'value') { complete = false; break }
        sum += childValue.value
      }
      if (complete) {
        const delta = element.value.value - sum
        summary = { balanced: Math.abs(delta) <= (reconcile.tolerance ?? 0), delta }
      }
    }
    return {
      ...quality,
      row,
      depth: depthOf(row),
      isParent: parentKeys.has(row.key),
      valueText: element.value.kind === 'value' ? formatValue(element.value.value) : '—',
      ...(element.share !== undefined ? { shareText: formatShare(element.share) } : {}),
      ...(summary ? { reconcile: summary } : {}),
    }
  })
}

export interface RelationshipEndView extends ElementQuality {
  end: MetricRelationshipEnd
  valueText: string
}

export function relationshipEndView(
  panel: Panel,
  join: KeyedJoin | undefined,
  end: MetricRelationshipEnd | undefined,
  formatValue: (value: number) => string,
): RelationshipEndView | undefined {
  if (!end) return undefined
  const element: ResolvedElement = join?.kind === 'ok' ? join.resolve(end.key) : { value: { kind: 'unavailable' } }
  return {
    ...elementQuality(element, end, panel),
    end,
    valueText: element.value.kind === 'value' ? formatValue(element.value.value) : '—',
  }
}

export const relationshipTypeFallback: Record<MetricRelationshipConfig['type'], string> = {
  association: 'Association',
  derivation: 'Derivation',
  reconciliation: 'Reconciliation',
}

export interface RelationshipGlyphs {
  horizontal: string
  vertical: string
}

/**
 * Association and reconciliation are symmetric by nature — a neutral
 * double-headed connector that never implies containment or derivation.
 */
export function connectorGlyphs(config: MetricRelationshipConfig): RelationshipGlyphs {
  const direction: MetricRelationshipDirection = config.direction ?? 'bidirectional'
  if (config.type !== 'derivation' || direction === 'bidirectional') {
    return { horizontal: '⇄', vertical: '⇵' }
  }
  if (direction === 'target_to_source') return { horizontal: '←', vertical: '↑' }
  return { horizontal: '→', vertical: '↓' }
}

/** The two ends in the order the relationship's own sentence names them. */
export function relationshipOperands(
  config: MetricRelationshipConfig | undefined,
  source: string,
  target: string,
): { source: string; target: string } {
  // Derivation names the deriving end first, so the arrow and the sentence
  // cannot point opposite ways.
  if (config?.type === 'derivation' && config.direction === 'target_to_source') {
    return { source: target, target: source }
  }
  return { source, target }
}
