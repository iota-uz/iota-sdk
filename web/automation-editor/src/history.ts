// Undo/redo over snapshots of the graph together with its layout. One user action is one step.
// Saving does not clear the history.
import type { AutomationGraph, AutomationLayout } from './types'

export type Snapshot = { graph: AutomationGraph; layout: AutomationLayout }
export type History = { past: Snapshot[]; present: Snapshot; future: Snapshot[] }

export const HISTORY_LIMIT = 200

export function createHistory(present: Snapshot): History {
  return { past: [], present, future: [] }
}

/** Records a new present; the previous one becomes undoable and the redo stack is dropped. */
export function record(history: History, next: Snapshot): History {
  if (next.graph === history.present.graph && next.layout === history.present.layout) return history
  const past = [...history.past, history.present]
  if (past.length > HISTORY_LIMIT) past.splice(0, past.length - HISTORY_LIMIT)
  return { past, present: next, future: [] }
}

/** Replaces the present without a new step (an edit that continues the previous one). */
export function amend(history: History, next: Snapshot): History {
  return { ...history, present: next, future: [] }
}

export function undo(history: History): History {
  const previous = history.past[history.past.length - 1]
  if (!previous) return history
  return { past: history.past.slice(0, -1), present: previous, future: [history.present, ...history.future] }
}

export function redo(history: History): History {
  const next = history.future[0]
  if (!next) return history
  return { past: [...history.past, history.present], present: next, future: history.future.slice(1) }
}

export const canUndo = (history: History) => history.past.length > 0
export const canRedo = (history: History) => history.future.length > 0
