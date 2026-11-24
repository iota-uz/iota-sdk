// Runs of the fixture graphs (#1347): the data of unit tests, of the fixture host's Runs tab and of
// its screenshots. Times are fixed around 2026-11-20 09:00 UTC (the VR clock).
import type { AutomationGraph, GraphEdge, RunData, RunStep, RunSummary } from '../src/types'
import { sample } from './graphs'

const edge = (from: string, outcome: string, to: string): GraphEdge => ({ from: { node: from, outcome }, to })

/** A parallel whose second branch waits for an event before the join. */
export const approval: AutomationGraph = {
  name: 'Approve a new client',
  run_as: 'owner',
  triggers: [{ id: 'start', event: 'crm.client.created.v1' }],
  nodes: [
    { id: 'fan', type: 'parallel', params: { branches: ['mail', 'approve'] } },
    { id: 'notify', type: 'core.send_email', params: { to: 'sales@acme.test', subject: 'New client', body: 'Hi' } },
    { id: 'ask', type: 'telegram.send_message', params: { connection: 'conn_sales', chat: '@ops', text: 'Approve?' } },
    { id: 'answer', type: 'wait', params: { event: 'crm.client.updated.v1', timeout: '1d' } },
    { id: 'joined', type: 'join', params: { of: 'fan', mode: 'all' } },
    { id: 'done', type: 'end', params: {} },
  ],
  edges: [
    edge('start', 'fired', 'fan'),
    edge('fan', 'mail', 'notify'),
    edge('fan', 'approve', 'ask'),
    edge('notify', 'sent', 'joined'),
    edge('ask', 'ok', 'answer'),
    edge('answer', 'event', 'joined'),
    edge('joined', 'joined', 'done'),
  ],
}

/** Version 2 of the sample graph: it had a step version 3 removed (a run of v2 still draws it). */
export const sampleV2: AutomationGraph = {
  ...sample,
  nodes: [...sample.nodes, { id: 'legacy_note', type: 'core.send_email', params: { to: 'ops@acme.test', subject: 'Old note', body: 'v2 only' } }],
  edges: [...sample.edges.filter((e) => !(e.from.node === 'send_email' && e.from.outcome === 'failed')), edge('send_email', 'failed', 'legacy_note')],
}

const at = (minutes: number) => new Date(Date.UTC(2026, 10, 20, 9, 0, 0) + minutes * 60_000).toISOString()

const step = (node: string, token: string, outcome: string | null, minute: number, extra: Partial<RunStep> = {}): RunStep => ({
  node, token_id: token, attempt: 1, input: {}, outcome, output: {}, logs: [], error: null,
  started_at: at(minute), ended_at: outcome === null ? null : at(minute + 0.1), ...extra,
})

const summary = (run_id: string, version: number, state: RunSummary['state'], minute: number, extra: Partial<RunSummary> = {}): RunSummary => ({
  run_id, version, state, subject: { entity: 'core.user', key: '42', label: 'Dilnoza Karimova' }, trigger: 'user_created',
  error: null, created_at: at(minute), updated_at: at(minute + 1), current_nodes: [], ...extra,
})

/** A run of the parallel graph: the mail branch is done, the approval branch waits for its event. */
export const approvalWaiting: RunData = {
  ...summary('00000000-0000-4000-8000-00000000a001', 1, 'waiting', -10, { trigger: 'start', subject: { entity: 'crm.client', key: 'c-7', label: 'Acme LLC' } }),
  // As the engine writes it: the parallel's step is "fork", its token waits for the branches it made.
  tokens: [
    { token_id: 't0', node: 'fan', state: 'waiting' },
    { token_id: 't-mail', node: 'joined', state: 'waiting', parent: 't0', branch: 'mail' },
    { token_id: 't-approve', node: 'answer', state: 'waiting', parent: 't0', branch: 'approve' },
  ],
  steps: [
    step('fan', 't0', 'fork', -10),
    step('notify', 't-mail', 'sent', -10, { input: { to: 'sales@acme.test' }, output: { message_id: 'm-1' } }),
    step('ask', 't-approve', 'ok', -9, { output: { message_id: 17 } }),
  ],
  waits: [{ node: 'answer', token_id: 't-approve', kind: 'event', since: at(-9), timeout_at: at(1430), wait_id: 'w-1' }],
}

const secretLog = 'calling api with token ***'

/** The runs of the sample graph, newest first. */
export function sampleRuns(): RunData[] {
  return [
    {
      ...summary('00000000-0000-4000-8000-0000000000r1', 3, 'running', -1, { current_nodes: ['send_email'] }),
      tokens: [{ token_id: 'a', node: 'send_email', state: 'active' }],
      steps: [step('has_email', 'a', 'true', -1, { input: { expression: 'user.email != ""' }, output: { value: true } }), step('send_email', 'a', null, -0.5)],
      waits: [],
    },
    {
      ...summary('00000000-0000-4000-8000-0000000000r2', 3, 'failed', -30, { error: 'The mail server refused the message' }),
      tokens: [{ token_id: 'b', node: 'send_email', state: 'done' }],
      steps: [
        step('has_email', 'b', 'true', -30, { output: { value: true } }),
        step('send_email', 'b', null, -29, { attempt: 1, ended_at: at(-29), error: { code: 'effect_failed', message: 'Timed out' } }),
        step('send_email', 'b', null, -28, {
          attempt: 2, ended_at: at(-28), error: { code: 'effect_failed', message: 'The mail server refused the message' },
          input: { to: 'dilnoza@acme.test', subject: 'Welcome, Dilnoza' },
          logs: ['connecting to smtp.acme.test', secretLog, 'SMTP 550 mailbox unavailable', '<b>not html</b>'],
        }),
      ],
      waits: [],
    },
    {
      ...summary('00000000-0000-4000-8000-0000000000r3', 2, 'done', -120, { subject: { entity: 'core.user', key: '7', label: 'Aziz Rakhimov' } }),
      tokens: [{ token_id: 'c', node: 'legacy_note', state: 'done' }],
      steps: [
        step('has_email', 'c', 'true', -120),
        step('send_email', 'c', 'failed', -119, { output: { error: { message: 'Address rejected by the provider', code: 'rejected' } } }),
        step('legacy_note', 'c', 'sent', -118, { output: { message_id: 'm-9' } }),
      ],
      waits: [],
    },
    {
      ...summary('00000000-0000-4000-8000-0000000000r4', 3, 'blocked', -240),
      tokens: [{ token_id: 'd', node: 'send_email', state: 'waiting' }],
      steps: [step('has_email', 'd', 'true', -240)],
      waits: [{ node: 'send_email', token_id: 'd', kind: 'call', since: at(-239), timeout_at: null, wait_id: 'call-1' }],
    },
    {
      ...summary('00000000-0000-4000-8000-0000000000r5', 3, 'cancelled', -600, { subject: null }),
      tokens: [{ token_id: 'e', node: 'delay', state: 'cancelled' }],
      // A cancel closes the open step with the outcome "cancelled".
      steps: [step('has_email', 'e', 'true', -600), step('send_email', 'e', 'sent', -599, { output: { message_id: 'm-3' } }), step('delay', 'e', 'cancelled', -598)],
      waits: [],
    },
  ]
}

export function toSummary(run: RunData): RunSummary {
  const { tokens: _t, steps: _s, waits: _w, ...rest } = run
  return rest
}
