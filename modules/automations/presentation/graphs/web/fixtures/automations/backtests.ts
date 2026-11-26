import type { RPC } from '../../Automations.solid.generated'

/** The backtests of the fixture: one graph with a classifier (BT_GRAPH), its datasets and backtests. */
export const BT_GRAPH = '00000000-0000-4000-8000-0000000000b1'
export const DS_JOURNAL = '00000000-0000-4000-8000-0000000000d1'
export const DS_HAND = '00000000-0000-4000-8000-0000000000d2'
export const BT_V2 = '00000000-0000-4000-8000-0000000000e1'
export const BT_V3 = '00000000-0000-4000-8000-0000000000e2'
export const BT_RUNNING = '00000000-0000-4000-8000-0000000000e3'

type R<K extends keyof RPC> = RPC[K]['result']
type Example = R<'automations.dataset_examples'>['items'][number]
type Result = R<'automations.backtest_results'>['items'][number]
type Report = R<'automations.backtest_report'>
type Dataset = R<'automations.backtests'>['datasets'][number]
type Backtest = R<'automations.backtests'>['backtests'][number]

export const messageEvent = {
  key: 'telegram.message.v1', title: 'Telegram message received', group: 'Telegram', subject: 'conversation',
  payload: { type: 'object', properties: { text: { type: 'string', title: 'Text' }, from: { type: 'string', title: 'From' } }, required: ['text'] },
  trigger_params: null, connected: true,
}

export const jevNode = {
  type: 'ai.jev', title: 'Classify (Jev)', description: 'Chooses one of the given choices with a model.', group: 'AI', behavior: 'call', connected: true,
  params: { type: 'object', properties: { instructions: { type: 'string' }, choices: { type: 'array', items: { type: 'object', properties: { value: { type: 'string' } } } } } },
  outcomes: [], outcome_rule: { from: { param: 'choices', field: 'value' }, fixed: ['low_confidence', 'failed'] }, expressions: [], templates: ['instructions'], output: null,
}

export function routeGraph(version: number) {
  const choices = version >= 3 ? [{ value: 'sales' }, { value: 'support' }, { value: 'refund' }] : [{ value: 'sales' }, { value: 'support' }]
  return {
    name: 'Route client messages', run_as: 'owner' as const,
    triggers: [{ id: 'msg', event: 'telegram.message.v1' }],
    nodes: [
      { id: 'intent', type: 'ai.jev', params: { instructions: 'What does the client want?', choices } },
      { id: 'to_sales', type: 'core.send_email', params: { to: 'sales@acme.test', subject: 'Lead', body: '{{ event.text }}' } },
      { id: 'to_support', type: 'core.send_email', params: { to: 'support@acme.test', subject: 'Ticket', body: '{{ event.text }}' } },
      { id: 'fin', type: 'end', params: {} },
    ],
    edges: [
      { from: { node: 'msg', outcome: 'fired' }, to: 'intent' },
      { from: { node: 'intent', outcome: 'sales' }, to: 'to_sales' },
      { from: { node: 'intent', outcome: 'support' }, to: 'to_support' },
      ...(version >= 3 ? [{ from: { node: 'intent', outcome: 'refund' }, to: 'to_support' }] : []),
      { from: { node: 'intent', outcome: 'low_confidence' }, to: 'fin' },
      { from: { node: 'intent', outcome: 'failed' }, to: 'fin' },
      { from: { node: 'to_sales', outcome: 'sent' }, to: 'fin' }, { from: { node: 'to_sales', outcome: 'failed' }, to: 'fin' },
      { from: { node: 'to_support', outcome: 'sent' }, to: 'fin' }, { from: { node: 'to_support', outcome: 'failed' }, to: 'fin' },
    ],
  }
}

const texts: [string, string, string | null, 'manual' | 'expected' | null][] = [
  ['I want to buy 20 licenses', 'sales', 'sales', 'manual'],
  ['The invoice is wrong, please refund', 'support', 'refund', 'manual'],
  ['How do I reset my password?', 'support', 'support', 'manual'],
  ['Do you have a discount for schools?', 'sales', 'sales', null],
  ['Cancel my order and return the money', 'support', 'refund', 'manual'],
  ['Can we schedule a demo next week?', 'sales', null, null],
]

function example(i: number, ds: string): Example {
  const [text, , label, source] = texts[i]!
  return {
    id: `00000000-0000-4000-8000-0000000c${ds.slice(-2)}${String(i).padStart(2, '0')}`, source: ds === DS_HAND ? 'synthetic' : 'journal', title: ds === DS_HAND ? text : '',
    event_kind: 'telegram.message.v1', event: { kind: 'telegram.message.v1', text, from: `client-${i + 1}` },
    subject: { entity: 'conversation', key: `c-${i + 1}` }, snapshot_approximate: i === 3, has_snapshot: true,
    occurred_at: `2026-11-${String(10 + i).padStart(2, '0')}T08:${String(10 + i).padStart(2, '0')}:00Z`,
    labels: label ? { intent: { outcome: label, source: ds === DS_HAND ? 'expected' : source ?? 'manual' } } : {},
  }
}

export interface BacktestFake {
  datasets: Dataset[]
  backtests: Backtest[]
  examples: Map<string, Example[]>
}

export function seedBacktests(empty: boolean): BacktestFake {
  const examples = new Map<string, Example[]>([[DS_JOURNAL, texts.map((_, i) => example(i, DS_JOURNAL))], [DS_HAND, [0, 1, 2].map((i) => example(i, DS_HAND))]])
  if (empty) return { datasets: [], backtests: [], examples: new Map() }
  return {
    datasets: [
      { id: DS_JOURNAL, name: 'Last 30 days of Telegram', description: '', examples: 6, labeled: 4, created_by: 'Dilnoza Karimova', created_at: '2026-11-18T08:00:00Z',
        imports: [{ id: 'i-1', kind: 'journal', status: 'done', imported: 6, duplicates: 0, overflow: 2, unreadable: 0, error: null, created_at: '2026-11-18T08:00:00Z', finished_at: '2026-11-18T08:01:00Z' }] },
      { id: DS_HAND, name: 'Tricky cases', description: '', examples: 3, labeled: 3, created_by: 'Sales assistant', created_at: '2026-11-19T08:00:00Z', imports: [] },
    ],
    backtests: [
      { id: BT_RUNNING, dataset_id: DS_JOURNAL, dataset_name: 'Last 30 days of Telegram', version: 3, decisions: 'live', status: 'running', total: 6, done: 2, live_calls: 2, error: null,
        created_by: 'Dilnoza Karimova', by_agent: false, created_at: '2026-11-20T09:55:00Z', finished_at: null },
      { id: BT_V3, dataset_id: DS_JOURNAL, dataset_name: 'Last 30 days of Telegram', version: 3, decisions: 'cached', status: 'done', total: 6, done: 6, live_calls: 4, error: null,
        created_by: 'Dilnoza Karimova', by_agent: false, created_at: '2026-11-20T09:00:00Z', finished_at: '2026-11-20T09:02:00Z' },
      { id: BT_V2, dataset_id: DS_JOURNAL, dataset_name: 'Last 30 days of Telegram', version: 2, decisions: 'cached', status: 'done', total: 6, done: 6, live_calls: 0, error: null,
        created_by: 'Sales assistant', by_agent: true, created_at: '2026-11-19T12:00:00Z', finished_at: '2026-11-19T12:01:00Z' },
    ],
    examples,
  }
}

/** What v2 (two choices) and v3 (with refund) decide for each message. */
function decided(version: number, i: number): string {
  const [, v2] = texts[i]!
  if (version < 3) return v2
  return i === 1 || i === 4 ? 'refund' : v2
}

export function resultsOf(f: BacktestFake, b: Backtest): Result[] {
  return (f.examples.get(b.dataset_id) ?? []).slice(0, b.done).map((ex, i) => {
    const outcome = decided(b.version, i)
    const label = ex.labels.intent
    const send = outcome === 'sales' ? 'to_sales' : 'to_support'
    return {
      example: ex, end: 'ended', outcomes: { msg: 'fired', intent: outcome, [send]: 'sent', fin: 'end' },
      decisions: [{ node: 'intent', outcome, status: b.decisions === 'live' || i % 2 === 0 ? 'live' : 'cached', cost_usd: 0.0021 }],
      effects: [{ node: send, params: { to: send === 'to_sales' ? 'sales@acme.test' : 'support@acme.test', subject: send === 'to_sales' ? 'Lead' : 'Ticket', body: (ex.event as { text: string }).text } }],
      trace: [], mismatches: label && label.outcome !== outcome ? ['intent'] : [],
    }
  })
}

export function reportOf(f: BacktestFake, b: Backtest): Report {
  const results = resultsOf(f, b)
  const outcomes = [...new Set(results.flatMap((r) => [r.outcomes.intent!, ...(r.example.labels.intent ? [r.example.labels.intent.outcome] : [])]))].sort()
  const labeled = results.filter((r) => r.example.labels.intent)
  const metrics = outcomes.map((o) => {
    const tp = labeled.filter((r) => r.outcomes.intent === o && r.example.labels.intent!.outcome === o).length
    const fp = labeled.filter((r) => r.outcomes.intent === o && r.example.labels.intent!.outcome !== o).length
    const fn = labeled.filter((r) => r.outcomes.intent !== o && r.example.labels.intent!.outcome === o).length
    const support = labeled.filter((r) => r.example.labels.intent!.outcome === o).length
    return { outcome: o, tp, fp, fn, support, precision: tp + fp > 0 ? tp / (tp + fp) : null, recall: support > 0 ? tp / support : null }
  })
  const correct = labeled.filter((r) => r.mismatches.length === 0).length
  const distribution: Record<string, number> = {}
  for (const r of results) distribution[r.outcomes.intent!] = (distribution[r.outcomes.intent!] ?? 0) + 1
  const effects = new Map<string, number>()
  for (const r of results) for (const e of r.effects) effects.set(e.node, (effects.get(e.node) ?? 0) + 1)
  const live = results.filter((r) => r.decisions[0]!.status === 'live').length
  return {
    backtest: b, examples: results.length, triggered: results.length, ends: { ended: results.length },
    classifiers: [{ node: 'intent', decision: true, labeled: labeled.length, correct, undecided: 0, accuracy: labeled.length > 0 ? correct / labeled.length : null, outcomes: metrics, distribution }],
    mismatches: results.filter((r) => r.mismatches.length > 0).map((r) => ({ example_id: r.example.id, title: r.example.title, occurred_at: r.example.occurred_at, node: 'intent', expected: r.example.labels.intent!.outcome, actual: r.outcomes.intent!, label_source: r.example.labels.intent!.source })),
    mismatch_total: results.filter((r) => r.mismatches.length > 0).length,
    effects: [...effects].sort().map(([node, count]) => ({ node, examples: count, count, samples: [] })),
    cost: { live_calls: live, cached_calls: results.length - live, missing: 0, live_usd: live * 0.0021, cached_usd: (results.length - live) * 0.0021, input_tokens: 1200, output_tokens: 40, unknown_cost: 0 },
  }
}
