// Pure functions of the runs view (#1347): the overlay of a run and of a dry run, run states, steps by
// node and the output size limit. No DOM, no Solid: unit tested and usable by a host.
import { edgeKey, FIRED } from '../graph'
import { pick } from '../locale'
import type { AutomationGraph, EditorLocale, ItemState, Overlay, RunData, RunState, RunStep, TestNodeResult, TraceStep } from '../types'

/** States a run is still moving in: the run view polls them. */
export const LIVE_STATES: readonly RunState[] = ['running', 'waiting']
export const FINAL_STATES: readonly RunState[] = ['done', 'failed', 'cancelled']
export const ALL_STATES: readonly RunState[] = ['running', 'waiting', 'blocked', 'failed', 'done', 'cancelled']
export const POLL_MS = 2000
/** Output above this size is folded with «Load in full». */
export const OUTPUT_FOLD_BYTES = 256 * 1024

/** The outcome of the step a token records when it arrives at a join or a collect. */
export const ARRIVED = 'arrived'

export const isLive = (state: RunState) => LIVE_STATES.includes(state)
export const isFinal = (state: RunState) => FINAL_STATES.includes(state)

const ATTEMPT: Record<EditorLocale, (n: number) => string> = {
  en: (n) => `attempt ${n}`,
  ru: (n) => `попытка ${n}`,
  uz: (n) => `${n}-urinish`,
  'uz-Cyrl': (n) => `${n}-уриниш`,
}

const RANK: Record<ItemState, number> = { waiting: 6, running: 5, failed: 4, cancelled: 3, passed: 2, skipped: 1 }
const raise = (states: Map<string, ItemState>, id: string, state: ItemState) => {
  const current = states.get(id)
  if (!current || RANK[state] > RANK[current]) states.set(id, state)
}

/** The message of a failed branch: output.error.message, else the step's error. */
export function failureMessage(step: Pick<RunStep, 'output' | 'error' | 'outcome'>): string | null {
  const out = step.output as { error?: { message?: unknown } } | null | undefined
  if (step.outcome === 'failed' && out && typeof out === 'object' && out.error && typeof out.error.message === 'string') return out.error.message
  return step.error?.message ?? null
}

/** The last attempt of each token at each node. */
function lastAttempts(steps: RunStep[]): Map<string, RunStep[]> {
  const byKey = new Map<string, RunStep>()
  for (const step of steps) {
    const key = `${step.node}\u0000${step.token_id}`
    const seen = byKey.get(key)
    if (!seen || step.attempt >= seen.attempt) byKey.set(key, step)
  }
  const byNode = new Map<string, RunStep[]>()
  for (const step of byKey.values()) {
    const list = byNode.get(step.node)
    if (list) list.push(step)
    else byNode.set(step.node, [step])
  }
  return byNode
}

/**
 * The overlay of a run on the graph of its version:
 * - a node with a finished step is passed, or failed when the step has an error;
 * - a node with an active token is running; a node with a wait is waiting;
 * - a node the run did not reach by its end is skipped (a node of a run still moving has no state);
 * - the edges the run took are taken ("node.outcome"), the trigger that started it included;
 * - the badge counts attempts (> 1) and the steps of a for each (> 1 token);
 * - a failed branch carries its message (output.error.message) as the card's note.
 */
export function runOverlay(graph: AutomationGraph, run: RunData, locale: EditorLocale = 'en'): Overlay {
  const states = new Map<string, ItemState>()
  const badges = new Map<string, string>()
  const taken = new Set<string>()
  const notes = new Map<string, string>()
  const nodes = new Set(graph.nodes.map((n) => n.id))

  if (graph.triggers.some((t) => t.id === run.trigger)) {
    states.set(run.trigger, 'passed')
    taken.add(edgeKey(run.trigger, FIRED))
  }
  const attempt = pick(ATTEMPT, locale)
  const edges = new Set(graph.edges.map((e) => edgeKey(e.from.node, e.from.outcome)))
  const children = new Map<string, RunData['tokens']>()
  for (const token of run.tokens) {
    if (!token.parent) continue
    const list = children.get(token.parent)
    if (list) list.push(token)
    else children.set(token.parent, [token])
  }
  // A branch that reached a join (or a for each item that reached its collect) records a step with the
  // outcome "arrived" on that node: the token's arrival, not the node's own decision. The join is passed,
  // failed or timed out by its own step only.
  const steps = run.steps.filter((s) => s.outcome !== ARRIVED)
  for (const [node, latest] of lastAttempts(steps)) {
    for (const step of latest) {
      // A cancelled run closes its open steps with the outcome "cancelled" (no error).
      const cancelled = step.outcome === 'cancelled' && !edges.has(edgeKey(node, 'cancelled'))
      if (step.ended_at === null) raise(states, node, 'running')
      else if (step.error) raise(states, node, 'failed')
      else if (cancelled) raise(states, node, 'cancelled')
      else raise(states, node, 'passed')
      if (step.outcome && edges.has(edgeKey(node, step.outcome))) taken.add(edgeKey(node, step.outcome))
      // A parallel records "fork" and a for each "each": the edges taken are those of the tokens they made.
      for (const child of children.get(step.token_id) ?? []) {
        const outcome = child.branch ? child.branch : child.foreach_index !== null && child.foreach_index !== undefined ? 'item' : null
        if (outcome && edges.has(edgeKey(node, outcome))) taken.add(edgeKey(node, outcome))
      }
      const note = step.outcome === 'failed' || step.error ? failureMessage(step) : null
      if (note) notes.set(node, note)
    }
    const all = steps.filter((s) => s.node === node)
    const maxAttempt = Math.max(...all.map((s) => s.attempt))
    const parts: string[] = []
    if (latest.length > 1) parts.push(`${latest.length}×`)
    if (maxAttempt > 1) parts.push(attempt(maxAttempt))
    if (parts.length > 0) badges.set(node, parts.join(' · '))
  }
  for (const token of run.tokens) {
    if (!token.node) continue
    if (token.state === 'active') raise(states, token.node, 'running')
    else if (token.state === 'cancelled' && !states.has(token.node)) states.set(token.node, 'cancelled')
  }
  for (const wait of run.waits) raise(states, wait.node, 'waiting')
  if (isFinal(run.state)) {
    for (const id of nodes) if (!states.has(id)) states.set(id, 'skipped')
  }
  return { states, badges, taken, notes }
}

/** The overlay of a dry run's trace: the same states as a run's. */
export function traceOverlay(graph: AutomationGraph, trace: TraceStep[]): Overlay {
  const states = new Map<string, ItemState>()
  const taken = new Set<string>()
  const notes = new Map<string, string>()
  const triggers = new Set(graph.triggers.map((t) => t.id))
  for (const step of trace) {
    switch (step.kind) {
      case 'triggered':
        raise(states, step.node, 'passed')
        taken.add(edgeKey(step.node, FIRED))
        break
      case 'not_triggered':
        if (triggers.has(step.node)) raise(states, step.node, 'skipped')
        break
      case 'step':
        raise(states, step.node, step.outcome === 'failed' ? 'failed' : 'passed')
        if (step.outcome) taken.add(edgeKey(step.node, step.outcome))
        if (step.outcome === 'failed') {
          const note = failureMessage({ output: step.output, error: step.message ? { code: '', message: step.message } : null, outcome: 'failed' })
          if (note) notes.set(step.node, note)
        }
        break
      case 'wait':
        raise(states, step.node, 'waiting')
        break
      case 'woke':
      case 'timeout':
      case 'joined':
        raise(states, step.node, 'passed')
        if (step.outcome) taken.add(edgeKey(step.node, step.outcome))
        break
      case 'cancelled':
        raise(states, step.node, 'cancelled')
        break
      case 'error':
        if (step.node) {
          raise(states, step.node, 'failed')
          if (step.message) notes.set(step.node, step.message)
        }
        break
      default:
        break
    }
  }
  // A woken wait is passed: the later step wins over the earlier waiting.
  for (const step of trace) if (step.kind === 'woke' || step.kind === 'timeout') states.set(step.node, 'passed')
  return { states, badges: new Map(), taken, notes }
}

/** The overlay of a Test node result: the tested node passed or failed, and the outcome it took. */
export function testOverlay(node: string, result: TestNodeResult): Overlay {
  const failed = Boolean(result.error) || result.outcome === 'failed'
  const notes = new Map<string, string>()
  if (failed) {
    const message = failureMessage({ output: result.output, error: result.error ? { code: result.error.reason, message: result.error.message } : null, outcome: result.outcome })
    if (message) notes.set(node, message)
  }
  return {
    states: new Map([[node, failed ? 'failed' : 'passed']]),
    badges: new Map(),
    taken: new Set(result.outcome ? [edgeKey(node, result.outcome)] : []),
    notes,
  }
}

/** Two overlays draw the same: a poll that changed nothing keeps the old one (P4). */
export function overlaysEqual(a: Overlay | null | undefined, b: Overlay | null | undefined): boolean {
  if (a === b) return true
  if (!a || !b) return false
  const maps = <T,>(x: Map<string, T> | undefined, y: Map<string, T> | undefined) => {
    const xs = x ?? new Map<string, T>()
    const ys = y ?? new Map<string, T>()
    if (xs.size !== ys.size) return false
    for (const [k, v] of xs) if (ys.get(k) !== v) return false
    return true
  }
  if (a.taken.size !== b.taken.size) return false
  for (const k of a.taken) if (!b.taken.has(k)) return false
  return maps(a.states, b.states) && maps(a.badges, b.badges) && maps(a.notes, b.notes)
}

/** The steps of a node in order (attempts and for each items). */
export function stepsOf(run: RunData, node: string): RunStep[] {
  return run.steps.filter((s) => s.node === node)
}

/** The node a run's row names: where it stands now, else where it failed. */
export function rowNode(run: { current_nodes?: string[]; state: RunState }, data?: RunData | null): string | null {
  if (run.current_nodes && run.current_nodes.length > 0) return run.current_nodes[0]!
  if (data) {
    const failed = [...data.steps].reverse().find((s) => s.error)
    if (failed) return failed.node
  }
  return null
}

/** Serialized size of a value in bytes (UTF-8), what the fold of a large output compares. */
export function byteSize(value: unknown): number {
  if (value === undefined) return 0
  const text = typeof value === 'string' ? value : JSON.stringify(value) ?? ''
  return new TextEncoder().encode(text).length
}

/** Whether a step carries anything to show. */
export function hasData(step: RunStep): boolean {
  const empty = (v: unknown) => v === undefined || v === null || (typeof v === 'object' && Object.keys(v as object).length === 0)
  return !step.hidden && (!empty(step.input) || !empty(step.output) || step.logs.length > 0 || Boolean(step.error))
}

/** A RunData answer that equals the one on screen (P4: a poll that changed nothing redraws nothing). */
export function sameRun(a: RunData | null | undefined, b: RunData | null | undefined): boolean {
  if (!a || !b) return a === b
  return a.updated_at === b.updated_at && a.state === b.state && a.steps.length === b.steps.length &&
    a.waits.length === b.waits.length && JSON.stringify(a) === JSON.stringify(b)
}

/** The secret a Test node lacks, from its error ("secret NAME is not set"). */
export function missingSecret(error: TestNodeResult['error'] | undefined): string | null {
  if (!error || error.reason !== 'secret_missing') return null
  const m = /secret\s+(\S+)/i.exec(error.message)
  return m ? m[1]! : ''
}
