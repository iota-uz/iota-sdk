import { describe, expect, it } from 'vitest'
import { sample } from '../../fixture/graphs'
import { approval, approvalWaiting, sampleRuns, sampleV2 } from '../../fixture/runs'
import type { RunData } from '../types'
import { byteSize, failureMessage, hasData, isFinal, isLive, missingSecret, overlaysEqual, runOverlay, sameRun, testOverlay, traceOverlay } from './model'

const runs = sampleRuns()
const byId = (suffix: string) => runs.find((r) => r.run_id.endsWith(suffix))!

describe('runOverlay', () => {
  it('a parallel with one branch waiting: exactly one waiting node, both branches passed up to it, the join without a state', () => {
    const o = runOverlay(approval, approvalWaiting)
    const waiting = [...o.states].filter(([, s]) => s === 'waiting').map(([id]) => id)
    expect(waiting).toEqual(['answer'])
    expect(o.states.get('fan')).toBe('passed')
    expect(o.states.get('notify')).toBe('passed')
    expect(o.states.get('ask')).toBe('passed')
    expect(o.states.has('joined')).toBe(false)
    expect(o.states.has('done')).toBe(false)
    expect(o.states.get('start')).toBe('passed')
    expect([...o.taken].sort()).toEqual(['ask.ok', 'fan.mail', 'notify.sent', 'start.fired'])
  })

  it('a live run: the step in progress is running and nothing is skipped yet', () => {
    const o = runOverlay(sample, byId('r1'))
    expect(o.states.get('has_email')).toBe('passed')
    expect(o.states.get('send_email')).toBe('running')
    expect(o.states.has('delay')).toBe(false)
    expect(o.states.has('end')).toBe(false)
  })

  it('a failed run: the last attempt decides, the badge counts attempts, unreached steps are skipped', () => {
    const o = runOverlay(sample, byId('r2'), 'ru')
    expect(o.states.get('send_email')).toBe('failed')
    expect(o.badges.get('send_email')).toBe('попытка 2')
    expect(o.notes?.get('send_email')).toBe('The mail server refused the message')
    expect(o.states.get('delay')).toBe('skipped')
    expect(o.states.get('end')).toBe('skipped')
    expect(o.taken.has('has_email.true')).toBe(true)
  })

  it('a run of version 2 is drawn with its own graph: the step removed in version 3 is there, and the failed branch carries its message', () => {
    const o = runOverlay(sampleV2, byId('r3'))
    expect(o.states.get('legacy_note')).toBe('passed')
    expect(o.states.get('send_email')).toBe('passed')
    expect(o.taken.has('send_email.failed')).toBe(true)
    expect(o.notes?.get('send_email')).toBe('Address rejected by the provider')
    expect(o.states.get('delay')).toBe('skipped')
  })

  it('a blocked call is waiting; a cancelled token marks its step cancelled', () => {
    expect(runOverlay(sample, byId('r4')).states.get('send_email')).toBe('waiting')
    const cancelled = runOverlay(sample, byId('r5'))
    expect(cancelled.states.get('delay')).toBe('cancelled')
    expect(cancelled.states.get('end')).toBe('skipped')
  })

  it('a for each counts its items', () => {
    const run: RunData = {
      ...byId('r3'), state: 'done',
      steps: [1, 2, 3].map((i) => ({ node: 'send_email', token_id: `item-${i}`, attempt: 1, input: {}, outcome: 'sent', output: {}, logs: [], error: null, started_at: '2026-11-20T09:00:00Z', ended_at: '2026-11-20T09:00:01Z' })),
      tokens: [], waits: [],
    }
    expect(runOverlay(sample, run).badges.get('send_email')).toBe('3×')
  })

  it('equal answers give equal overlays (a poll that changed nothing redraws nothing)', () => {
    const a = runOverlay(sample, byId('r2'))
    const b = runOverlay(sample, JSON.parse(JSON.stringify(byId('r2'))) as RunData)
    expect(overlaysEqual(a, b)).toBe(true)
    expect(overlaysEqual(a, runOverlay(sample, byId('r1')))).toBe(false)
    expect(sameRun(byId('r2'), JSON.parse(JSON.stringify(byId('r2'))) as RunData)).toBe(true)
  })
})

describe('trace and test overlays', () => {
  it('draws a dry run path, a trigger that did not fire and a wait', () => {
    const o = traceOverlay(sample, [
      { node: 'user_created', kind: 'triggered' },
      { node: 'schedule', kind: 'not_triggered', path: 'triggers[1].filter', message: 'event.at > now' },
      { node: 'has_email', kind: 'step', outcome: 'true' },
      { node: 'send_email', kind: 'step', outcome: 'sent', status: 'captured', effects: [{ to: 'x' }] },
      { node: 'delay', kind: 'wait' },
    ])
    expect(o.states.get('user_created')).toBe('passed')
    expect(o.states.get('schedule')).toBe('skipped')
    expect(o.states.get('delay')).toBe('waiting')
    expect(o.taken.has('user_created.fired')).toBe(true)
    expect(o.taken.has('send_email.sent')).toBe(true)
  })

  it('marks a tested step', () => {
    expect(testOverlay('send_email', { outcome: 'sent' }).states.get('send_email')).toBe('passed')
    const failed = testOverlay('send_email', { outcome: 'failed', error: { reason: 'secret_missing', message: 'secret crm.token is not set' } })
    expect(failed.states.get('send_email')).toBe('failed')
    expect(failed.notes?.get('send_email')).toBe('secret crm.token is not set')
  })
})

describe('helpers', () => {
  it('knows live and final states', () => {
    expect(isLive('running') && isLive('waiting')).toBe(true)
    expect(isLive('blocked') || isLive('done')).toBe(false)
    expect(['done', 'failed', 'cancelled'].every((s) => isFinal(s as never))).toBe(true)
    expect(isFinal('blocked')).toBe(false)
  })
  it('reads a failed branch message and a missing secret', () => {
    expect(failureMessage({ outcome: 'failed', output: { error: { message: 'boom' } }, error: null })).toBe('boom')
    expect(failureMessage({ outcome: 'sent', output: {}, error: { code: 'x', message: 'err' } })).toBe('err')
    expect(missingSecret({ reason: 'secret_missing', message: 'secret crm.token is not set' })).toBe('crm.token')
    expect(missingSecret({ reason: 'timeout', message: 'slow' })).toBeNull()
  })
  it('measures output and knows an empty step', () => {
    expect(byteSize({ a: 'я' })).toBe(JSON.stringify({ a: 'я' }).length + 1)
    expect(hasData({ node: 'n', token_id: 't', attempt: 1, input: {}, outcome: 'ok', output: null, logs: [], error: null, started_at: '', ended_at: '' })).toBe(false)
    expect(hasData({ node: 'n', token_id: 't', attempt: 1, input: {}, outcome: 'ok', output: null, logs: ['x'], error: null, started_at: '', ended_at: '' })).toBe(true)
  })
})
