import type { Composition, ContextPeriod, Placement } from './contract/types'

export const fullSpan = 12
export const minimumSpan = 3

export type PlacementDirection = 'left' | 'right' | 'up' | 'down'

export const spanOf = (placement: Pick<Placement, 'span'>): number => placement.span ?? fullSpan
const rowSpan = (row: Placement[]) => row.reduce((total, placement) => total + spanOf(placement), 0)

function locate(composition: Composition, id: string): { row: number; index: number } | undefined {
  for (let row = 0; row < composition.rows.length; row += 1) {
    const index = composition.rows[row]!.findIndex((placement) => placement.id === id)
    if (index >= 0) return { row, index }
  }
  return undefined
}

function withRows(composition: Composition, rows: Placement[][]): Composition {
  return { ...structuredClone(composition), rows: rows.filter((row) => row.length > 0) }
}

/** Splits rows wider than twelve columns, keeping order. */
function wrap(rows: Placement[][]): Placement[][] {
  return rows.flatMap((row) => {
    const wrapped: Placement[][] = [[]]
    for (const placement of row) {
      const current = wrapped[wrapped.length - 1]!
      if (current.length && rowSpan(current) + spanOf(placement) > fullSpan) wrapped.push([placement])
      else current.push(placement)
    }
    return wrapped
  })
}

/**
 * Moves a card one step. Left/right swaps with its neighbour or crosses into
 * the adjacent row where it fits; up/down moves it to the adjacent row, or to
 * a row of its own when that row is full. Order elsewhere is unchanged.
 */
export function movePlacement(composition: Composition, id: string, direction: PlacementDirection): Composition {
  const at = locate(composition, id)
  if (!at) return composition
  const rows = structuredClone(composition.rows)
  const row = rows[at.row]!
  const [placement] = row.splice(at.index, 1)
  const fits = (target: Placement[]) => rowSpan(target) + spanOf(placement!) <= fullSpan
  switch (direction) {
    case 'left':
      if (at.index > 0) row.splice(at.index - 1, 0, placement!)
      else if (at.row > 0 && fits(rows[at.row - 1]!)) rows[at.row - 1]!.push(placement!)
      else if (row.length > 0) rows.splice(at.row, 0, [placement!])
      else return composition
      break
    case 'right':
      if (at.index < row.length) row.splice(at.index + 1, 0, placement!)
      else if (at.row < rows.length - 1 && fits(rows[at.row + 1]!)) rows[at.row + 1]!.unshift(placement!)
      else if (row.length > 0) rows.splice(at.row + 1, 0, [placement!])
      else return composition
      break
    case 'up':
      if (at.row > 0 && fits(rows[at.row - 1]!)) rows[at.row - 1]!.splice(Math.min(at.index, rows[at.row - 1]!.length), 0, placement!)
      else if (at.row > 0) rows.splice(at.row - 1, 0, [placement!])
      else if (row.length > 0) rows.unshift([placement!])
      else return composition
      break
    case 'down':
      if (at.row < rows.length - 1 && fits(rows[at.row + 1]!)) rows[at.row + 1]!.splice(Math.min(at.index, rows[at.row + 1]!.length), 0, placement!)
      else if (at.row < rows.length - 1) rows.splice(at.row + 2, 0, [placement!])
      else if (row.length > 0) rows.push([placement!])
      else return composition
      break
  }
  return withRows(composition, rows)
}

/** Places a dragged card immediately before another, or in a row of its own above it when that row is full. */
export function movePlacementBefore(composition: Composition, id: string, targetId: string): Composition {
  if (id === targetId) return composition
  const at = locate(composition, id)
  if (!at) return composition
  const rows = structuredClone(composition.rows)
  const [placement] = rows[at.row]!.splice(at.index, 1)
  const cleaned = rows.filter((row) => row.length > 0)
  const target = locate({ ...composition, rows: cleaned }, targetId)
  if (!target) return composition
  const row = cleaned[target.row]!
  if (rowSpan(row) + spanOf(placement!) <= fullSpan) row.splice(target.index, 0, placement!)
  else cleaned.splice(target.row, 0, [placement!])
  return withRows(composition, cleaned)
}

/** Sets a card's width in columns; cards that no longer fit wrap onto the next row. */
export function resizePlacement(composition: Composition, id: string, span: number): Composition {
  const width = Math.max(minimumSpan, Math.min(fullSpan, Math.round(span)))
  const rows = structuredClone(composition.rows).map((row) => row.map((placement) => {
    if (placement.id !== id) return placement
    const next = { ...placement }
    if (width === fullSpan) delete next.span
    else next.span = width
    return next
  }))
  return withRows(composition, wrap(rows))
}

export function removePlacement(composition: Composition, id: string): Composition {
  return withRows(composition, composition.rows.map((row) => row.filter((placement) => placement.id !== id)))
}

export interface PlacementSettings {
  /** The card caption; empty restores the report title. */
  title: string
  span: number
  /** A card-local period, or undefined to inherit the dashboard's. */
  period?: ContextPeriod
}

/** Applies card-local settings. Other local filters, such as dimension constraints, are kept. */
export function configurePlacement(composition: Composition, id: string, settings: PlacementSettings): Composition {
  const rows = structuredClone(composition.rows).map((row) => row.map((placement) => {
    if (placement.id !== id) return placement
    const next: Placement = { ...placement }
    const title = settings.title.trim()
    if (title) next.title = title
    else delete next.title
    const filters = { ...next.filters }
    if (settings.period) filters.period = structuredClone(settings.period)
    else delete filters.period
    if (filters.period || filters.constraints?.length) next.filters = filters
    else delete next.filters
    return next
  }))
  return resizePlacement(withRows(composition, rows), id, settings.span)
}

export type CompositionChangeKind = 'add' | 'change' | 'remove'

export interface CompositionChange {
  kind: CompositionChangeKind
  placementId: string
  reportId: string
}

/** The placement-level difference between two compositions, in the proposed order then removals. */
export function diffCompositions(base: Composition, proposed: Composition): CompositionChange[] {
  const before = new Map(base.rows.flat().map((placement) => [placement.id, placement]))
  const position = (composition: Composition) => new Map(composition.rows.flatMap((row, rowIndex) => row.map((placement, index) => [placement.id, `${rowIndex}:${index}`])))
  const beforePosition = position(base)
  const afterPosition = position(proposed)
  const changes: CompositionChange[] = []
  for (const placement of proposed.rows.flat()) {
    const previous = before.get(placement.id)
    if (!previous) changes.push({ kind: 'add', placementId: placement.id, reportId: placement.reportId })
    else if (JSON.stringify(previous) !== JSON.stringify(placement) || beforePosition.get(placement.id) !== afterPosition.get(placement.id)) {
      changes.push({ kind: 'change', placementId: placement.id, reportId: placement.reportId })
    }
  }
  const kept = new Set(proposed.rows.flat().map((placement) => placement.id))
  for (const placement of base.rows.flat()) {
    if (!kept.has(placement.id)) changes.push({ kind: 'remove', placementId: placement.id, reportId: placement.reportId })
  }
  return changes
}
