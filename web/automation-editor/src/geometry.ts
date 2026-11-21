// Card geometry is computed, never measured: a card's width follows its outcomes, its height is fixed,
// and every handle position is declared from these numbers. Cards do not grow with their content.
import type { XY } from './types'

export const GRID = 16

export const CARD_HEIGHT = 96
export const CARD_MIN_WIDTH = 224
export const CARD_MAX_WIDTH = 640
export const CARD_PADDING = 12
export const OUTCOME_GAP = 8
export const OUTCOME_HEIGHT = 24
export const OUTCOME_ROW_Y = 60
const CHAR_WIDTH = 7
const OUTCOME_MIN_WIDTH = 48
const OUTCOME_LABEL_PADDING = 20

export const HANDLE_SIZE = 12
export const TARGET_HANDLE = 'in'

export function snap(value: number, grid = GRID): number {
  return Math.round(value / grid) * grid
}

export function snapXY(point: XY, grid = GRID): XY {
  return { x: snap(point.x, grid), y: snap(point.y, grid) }
}

export function outcomeWidth(outcome: string): number {
  return Math.max(OUTCOME_MIN_WIDTH, Math.ceil(outcome.length * CHAR_WIDTH + OUTCOME_LABEL_PADDING))
}

export type OutcomeSlot = { outcome: string; x: number; width: number; handle: XY }

export type CardGeometry = {
  width: number
  height: number
  target: XY | null
  outcomes: OutcomeSlot[]
}

/** Width from outcomes (snapped up to the grid), fixed height, and every handle relative to the card. */
export function cardGeometry(outcomes: readonly string[], hasTarget: boolean): CardGeometry {
  const widths = outcomes.map(outcomeWidth)
  const content = widths.reduce((sum, w) => sum + w, 0) + OUTCOME_GAP * Math.max(0, outcomes.length - 1)
  const raw = Math.max(CARD_MIN_WIDTH, content + CARD_PADDING * 2)
  const width = Math.min(CARD_MAX_WIDTH, Math.ceil(raw / GRID) * GRID)
  // Too many outcomes for the widest card: shrink the slots evenly; labels are truncated with an ellipsis.
  const available = width - CARD_PADDING * 2 - OUTCOME_GAP * Math.max(0, outcomes.length - 1)
  const scale = content - OUTCOME_GAP * Math.max(0, outcomes.length - 1) > available
    ? available / Math.max(1, content - OUTCOME_GAP * Math.max(0, outcomes.length - 1))
    : 1
  const scaled = widths.map((w) => Math.floor(w * scale))
  const used = scaled.reduce((sum, w) => sum + w, 0) + OUTCOME_GAP * Math.max(0, outcomes.length - 1)
  let x = Math.round((width - used) / 2)
  const slots = outcomes.map((outcome, index) => {
    const w = scaled[index] ?? OUTCOME_MIN_WIDTH
    const slot: OutcomeSlot = { outcome, x, width: w, handle: { x: x + w / 2, y: CARD_HEIGHT } }
    x += w + OUTCOME_GAP
    return slot
  })
  return {
    width,
    height: CARD_HEIGHT,
    target: hasTarget ? { x: 0, y: CARD_HEIGHT / 2 } : null,
    outcomes: slots,
  }
}

export type Rect = { x: number; y: number; width: number; height: number }

export function rectsOverlap(a: Rect, b: Rect, margin = 0): boolean {
  return a.x < b.x + b.width + margin && a.x + a.width + margin > b.x && a.y < b.y + b.height + margin && a.y + a.height + margin > b.y
}

export function rectContains(outer: Rect, inner: Rect): boolean {
  return inner.x >= outer.x && inner.y >= outer.y && inner.x + inner.width <= outer.x + outer.width && inner.y + inner.height <= outer.y + outer.height
}

export function normalizeRect(a: XY, b: XY): Rect {
  return { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), width: Math.abs(a.x - b.x), height: Math.abs(a.y - b.y) }
}
