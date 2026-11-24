// Graphs for the layout: big graphs with several sources, failure outcomes and flow constructs. They are
// the fixtures of the Tidy quality tests and of `?graph=multi` / `?graph=foreach` in the fixture host.
import type { AutomationGraph, CatalogNode, EditorCatalog, GraphEdge } from '../src/types'
import { catalog } from './catalog'

const object = { type: 'object', additionalProperties: false }
const expression = { type: 'string', minLength: 1, maxLength: 2000 }

const extra: CatalogNode[] = [
  {
    type: 'ai.recognize', title: 'Recognize', description: 'Reads a document; a doubtful answer takes low_confidence.', group: 'AI', behavior: 'call',
    params: { ...object, properties: { text: { type: 'string' } } }, outcomes: ['ok', 'low_confidence', 'failed'], outcome_rule: { fixed: ['ok', 'low_confidence', 'failed'] },
    expressions: [], templates: ['text'], output: null,
  },
  {
    type: 'parallel', title: 'Parallel', description: 'Starts every branch at once.', group: 'Flow', behavior: 'parallel',
    params: { ...object, properties: { branches: { type: 'array', items: { type: 'string' } } } }, outcomes: [], outcome_rule: { from: { param: 'branches' } },
    expressions: [], templates: [], output: null,
  },
  {
    type: 'join', title: 'Join', description: 'Waits for the branches of its parallel.', group: 'Flow', behavior: 'join',
    params: { ...object, properties: { of: { type: 'string' }, mode: { enum: ['all', 'any', 'n_of'] }, timeout: { type: 'string' } } },
    outcomes: ['joined'], outcome_rule: { fixed: ['joined'], optional: { param: 'timeout', outcome: 'timeout' } }, expressions: [], templates: [], output: null,
  },
  {
    type: 'foreach', title: 'For each', description: 'Runs the body once per item.', group: 'Flow', behavior: 'foreach',
    params: { ...object, properties: { items: expression } }, outcomes: ['item', 'done', 'partial_failed', 'failed'], outcome_rule: { fixed: ['item', 'done', 'partial_failed', 'failed'] },
    expressions: [{ param: 'items', type: 'list' }], templates: [], output: null,
  },
  {
    type: 'collect', title: 'Collect', description: 'Ends the body of a for each for one item.', group: 'Flow', behavior: 'collect',
    params: { ...object, properties: { of: { type: 'string' } } }, outcomes: [], outcome_rule: { fixed: [] }, expressions: [], templates: [], output: null,
  },
]

/** The fixture catalog plus the flow steps (parallel, join, for each, collect) and a step with a low_confidence outcome. */
export const layoutCatalog: EditorCatalog = { ...catalog, nodes: [...catalog.nodes, ...extra] }

type Params = Record<string, unknown>
const node = (id: string, type: string, params: Params = {}) => ({ id, type, params })
const edge = (from: string, outcome: string, to: string): GraphEdge => ({ from: { node: from, outcome }, to })
const mail: Params = { to: '{{ user.email }}', subject: 'Hello', body: 'Hi' }
const tg: Params = { connection: 'conn', chat: '@ops', text: 'Hi' }
const crm: Params = { client: 'c1' }

/** Four triggers, 26 steps: a column per trigger, steps shared by several, and failed / timeout / low_confidence outcomes. */
export const multiTrigger: AutomationGraph = {
  name: 'Four sources',
  run_as: 'owner',
  triggers: [
    { id: 'user_created', event: 'core.user.created.v1' },
    { id: 'client_created', event: 'crm.client.created.v1' },
    { id: 'schedule', event: 'core.schedule', params: { cron: '0 20 * * *', timezone: 'Asia/Tashkent' } },
    { id: 'manual', event: 'core.manual' },
  ],
  nodes: [
    // user_created
    node('a_check', 'branch', { expression: 'user.email != ""' }),
    node('a_mail', 'core.send_email', mail),
    node('a_tg', 'telegram.send_message', tg),
    node('a_wait', 'delay', { duration: '1d' }),
    node('a_tag', 'crm.update_client', crm),
    node('a_alert', 'telegram.send_message', tg),
    node('a_alert_end', 'end'),
    // client_created
    node('b_route', 'switch', { expression: 'client.kind', cases: [{ value: 'vip', outcome: 'vip' }, { value: 'regular', outcome: 'regular' }, { value: 'trial', outcome: 'trial' }] }),
    node('b_vip', 'crm.update_client', crm),
    node('b_mail', 'core.send_email', mail),
    node('b_pause', 'delay', { duration: '2h' }),
    node('b_end', 'end'),
    node('b_fail', 'end'),
    // schedule
    node('c_read', 'ai.recognize', { text: 'doc' }),
    node('c_check', 'branch', { expression: 'true' }),
    node('c_send', 'core.send_email', mail),
    node('c_review', 'telegram.send_message', tg),
    node('c_end', 'end'),
    node('c_fail', 'end'),
    // manual
    node('d_wait', 'wait', { event: 'core.user.updated.v1', timeout: '1h' }),
    node('d_update', 'crm.update_client', crm),
    node('d_timeout', 'end'),
    node('d_end', 'end'),
    // shared by several triggers
    node('s_crm', 'crm.update_client', crm),
    node('s_notify', 'telegram.send_message', tg),
    node('s_alert', 'core.send_email', mail),
    node('s_done', 'end'),
  ],
  edges: [
    edge('user_created', 'fired', 'a_check'),
    edge('a_check', 'true', 'a_mail'),
    edge('a_check', 'false', 'a_tg'),
    edge('a_mail', 'sent', 'a_wait'),
    edge('a_mail', 'failed', 'a_alert'),
    edge('a_tg', 'ok', 'a_wait'),
    edge('a_tg', 'failed', 'a_alert'),
    edge('a_wait', 'done', 'a_tag'),
    edge('a_tag', 'ok', 's_crm'),
    edge('a_tag', 'failed', 'a_alert'),
    edge('a_alert', 'ok', 'a_alert_end'),
    edge('a_alert', 'failed', 'a_alert_end'),

    edge('client_created', 'fired', 'b_route'),
    edge('b_route', 'vip', 'b_vip'),
    edge('b_route', 'regular', 'b_mail'),
    edge('b_route', 'trial', 'b_pause'),
    edge('b_route', 'default', 'b_end'),
    edge('b_vip', 'ok', 's_crm'),
    edge('b_vip', 'failed', 'b_fail'),
    edge('b_mail', 'sent', 's_crm'),
    edge('b_mail', 'failed', 'b_fail'),
    edge('b_pause', 'done', 'b_mail'),

    edge('schedule', 'fired', 'c_read'),
    edge('c_read', 'ok', 'c_check'),
    edge('c_read', 'low_confidence', 'c_review'),
    edge('c_read', 'failed', 'c_fail'),
    edge('c_check', 'true', 'c_send'),
    edge('c_check', 'false', 's_notify'),
    edge('c_send', 'sent', 's_notify'),
    edge('c_send', 'failed', 'c_fail'),
    edge('c_review', 'ok', 'c_end'),
    edge('c_review', 'failed', 'c_end'),

    edge('manual', 'fired', 'd_wait'),
    edge('d_wait', 'event', 'd_update'),
    edge('d_wait', 'timeout', 'd_timeout'),
    edge('d_update', 'ok', 's_notify'),
    edge('d_update', 'failed', 'd_end'),

    edge('s_crm', 'ok', 's_notify'),
    edge('s_crm', 'failed', 's_alert'),
    edge('s_notify', 'ok', 's_done'),
    edge('s_notify', 'failed', 's_alert'),
    edge('s_alert', 'sent', 's_done'),
    edge('s_alert', 'failed', 's_done'),
  ],
}

/** A parallel with its join, and a for each with its body and collect; every failure goes to one handler. */
export const foreachJoin: AutomationGraph = {
  name: 'Fan out and loop',
  run_as: 'owner',
  triggers: [{ id: 'start', event: 'core.manual' }],
  nodes: [
    node('fan', 'parallel', { branches: ['mail', 'chat', 'crm'] }),
    node('f_mail', 'core.send_email', mail),
    node('f_chat', 'telegram.send_message', tg),
    node('f_crm', 'crm.update_client', crm),
    node('f_join', 'join', { of: 'fan', mode: 'all', timeout: '10m' }),
    node('f_each', 'foreach', { items: 'event.items' }),
    node('f_item', 'core.send_email', mail),
    node('f_collect', 'collect', { of: 'f_each' }),
    node('f_summary', 'telegram.send_message', tg),
    node('f_review', 'telegram.send_message', tg),
    node('f_alert', 'core.send_email', mail),
    node('f_end', 'end'),
  ],
  edges: [
    edge('start', 'fired', 'fan'),
    edge('fan', 'mail', 'f_mail'),
    edge('fan', 'chat', 'f_chat'),
    edge('fan', 'crm', 'f_crm'),
    edge('f_mail', 'sent', 'f_join'),
    edge('f_mail', 'failed', 'f_alert'),
    edge('f_chat', 'ok', 'f_join'),
    edge('f_chat', 'failed', 'f_alert'),
    edge('f_crm', 'ok', 'f_join'),
    edge('f_crm', 'failed', 'f_alert'),
    edge('f_join', 'joined', 'f_each'),
    edge('f_join', 'timeout', 'f_alert'),
    edge('f_each', 'item', 'f_item'),
    edge('f_each', 'done', 'f_summary'),
    edge('f_each', 'partial_failed', 'f_review'),
    edge('f_each', 'failed', 'f_alert'),
    edge('f_item', 'sent', 'f_collect'),
    edge('f_item', 'failed', 'f_collect'),
    edge('f_summary', 'ok', 'f_end'),
    edge('f_summary', 'failed', 'f_alert'),
    edge('f_review', 'ok', 'f_end'),
    edge('f_review', 'failed', 'f_alert'),
    edge('f_alert', 'sent', 'f_end'),
    edge('f_alert', 'failed', 'f_end'),
  ],
}

/** A graph of `nodes` steps in layers of up to ten, each linked forward: for time and overlap tests. */
export function wide(nodes = 240): AutomationGraph {
  const graph: AutomationGraph = { name: 'Wide', run_as: 'owner', triggers: [{ id: 'start', event: 'core.user.created.v1' }], nodes: [], edges: [] }
  for (let i = 0; i < nodes; i += 1) graph.nodes.push(node(`n${i}`, i % 3 === 0 ? 'core.send_email' : i % 3 === 1 ? 'branch' : 'delay', i % 3 === 1 ? { expression: 'true' } : i % 3 === 0 ? mail : { duration: '1h' }))
  graph.edges.push(edge('start', 'fired', 'n0'))
  for (let i = 0; i < nodes; i += 1) {
    const outcomes = i % 3 === 0 ? ['sent', 'failed'] : i % 3 === 1 ? ['true', 'false'] : ['done']
    outcomes.forEach((outcome, k) => {
      const target = i + 1 + k * 3 + (i % 5)
      if (target < nodes) graph.edges.push(edge(`n${i}`, outcome, `n${target}`))
    })
  }
  return graph
}
