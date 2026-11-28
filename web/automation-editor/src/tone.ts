// An outcome's tone: what colours its handle, its label and its edge (matches the reference editor).
// Success-like outcomes are green, failures red, detours amber, everything else neutral.
export type Tone = 'ok' | 'warn' | 'bad' | 'neutral'

const OK = new Set([
  'fired', 'sent', 'done', 'true', 'ok', 'success', 'succeeded', 'passed', 'matched', 'created', 'updated', 'found',
  'approved', 'approve', 'answered', 'handled', 'assigned', 'started', 'completed', 'delivered', 'result', 'continue', 'ready',
])
const BAD = new Set([
  'failed', 'error', 'errored', 'refused', 'rejected', 'reject', 'blocked', 'stopped', 'cancelled', 'invalid', 'unavailable', 'denied',
])
const WARN = new Set([
  'false', 'none', 'default', 'skipped', 'timeout', 'timed_out', 'waiting', 'stale', 'not_found', 'no_recipients', 'no_contact',
  'no_match', 'empty', 'expired', 'partial', 'low_confidence', 'question', 'attention',
])

export function toneOf(outcome: string): Tone {
  if (OK.has(outcome)) return 'ok'
  if (BAD.has(outcome)) return 'bad'
  if (WARN.has(outcome) || outcome.startsWith('no_')) return 'warn'
  return 'neutral'
}
