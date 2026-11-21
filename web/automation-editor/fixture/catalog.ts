// A catalog in the shape of pkg/automation/graph CatalogView, close to the built-in nodes.
import type { CatalogNode, EditorCatalog } from '../src/types'

const expression = { type: 'string', minLength: 1, maxLength: 2000, description: 'CEL; it may span lines' }
const duration = { type: 'string', pattern: '^[1-9][0-9]{0,5}(s|m|h|d)$', 'x-max-ms': 2592000000, description: '30s, 20m, 2h or 7d; at most 30 days' }

const nodes: CatalogNode[] = [
  {
    type: 'branch', title: 'Branch', description: 'A CEL expression of type bool: outcome true or false.', group: 'Logic', behavior: 'branch',
    params: { type: 'object', properties: { expression }, required: ['expression'], additionalProperties: false },
    outcomes: ['true', 'false'], outcome_rule: { fixed: ['true', 'false'] }, expressions: [{ param: 'expression', type: 'bool' }], templates: [], output: null,
  },
  {
    type: 'switch', title: 'Switch', description: 'The outcome of the first case with that value, else default.', group: 'Logic', behavior: 'switch',
    params: {
      type: 'object', required: ['expression', 'cases'], additionalProperties: false,
      properties: {
        expression,
        cases: { type: 'array', minItems: 1, maxItems: 20, 'x-unique': 'value', items: { type: 'object', required: ['value', 'outcome'], additionalProperties: false, properties: { value: { type: 'string', maxLength: 200 }, outcome: { type: 'string', pattern: '^[a-z][a-z0-9_]{0,39}$' } } }, default: [{ value: 'a', outcome: 'a' }] },
      },
    },
    outcomes: [], outcome_rule: { from: { param: 'cases', field: 'outcome' }, fixed: ['default'] }, expressions: [{ param: 'expression', type: 'string' }], templates: [], output: null,
  },
  {
    type: 'wait', title: 'Wait for event', description: 'Sleeps until an event about the subject passes the filter, or the timeout.', group: 'Logic', behavior: 'wait',
    params: { type: 'object', required: ['event'], additionalProperties: false, properties: { event: { enum: ['core.user.updated.v1', 'crm.client.updated.v1'] }, filter: expression, timeout: duration } },
    outcomes: ['event'], outcome_rule: { fixed: ['event'], optional: { param: 'timeout', outcome: 'timeout' } }, expressions: [{ param: 'filter', type: 'bool' }], templates: [], output: null,
  },
  {
    type: 'delay', title: 'Delay', description: 'Waits for a duration.', group: 'Logic', behavior: 'wait',
    params: { type: 'object', required: ['duration'], additionalProperties: false, properties: { duration: { ...duration, default: '1h' } } },
    outcomes: ['done'], outcome_rule: { fixed: ['done'] }, expressions: [], templates: [], output: null,
  },
  {
    type: 'end', title: 'End', description: 'Ends the run.', group: 'Logic', behavior: 'end',
    params: { type: 'object', additionalProperties: false, properties: { reason: { type: 'string', maxLength: 500 } } },
    outcomes: [], outcome_rule: { fixed: [] }, expressions: [], templates: [], output: null,
  },
  {
    type: 'core.send_email', title: 'Send email', description: 'Sends an email to a user or an address.', group: 'Messages', behavior: 'effect',
    params: {
      type: 'object', required: ['to', 'subject', 'body'], additionalProperties: false,
      properties: {
        to: { type: 'string', title: 'To', maxLength: 320 },
        subject: { type: 'string', title: 'Subject', maxLength: 200 },
        body: { type: 'string', title: 'Body', maxLength: 10000 },
        reply_to: { type: ['string', 'null'], title: 'Reply to' },
        priority: { enum: ['low', 'normal', 'high'], default: 'normal' },
      },
    },
    outcomes: ['sent', 'failed'], outcome_rule: { fixed: ['sent', 'failed'] }, expressions: [], templates: ['subject', 'body'],
    output: { type: 'object', properties: { message_id: { type: 'string' }, sent_at: { type: 'string', format: 'date-time' } } },
  },
  {
    type: 'telegram.send_message', title: 'Telegram message', description: 'Sends a Telegram message through a connected bot.', group: 'Telegram', behavior: 'call',
    params: {
      type: 'object', required: ['connection', 'chat', 'text'], additionalProperties: false,
      properties: { connection: { type: 'string' }, chat: { type: 'string', title: 'Chat' }, text: { type: 'string', title: 'Text', maxLength: 4096 }, silent: { type: 'boolean', title: 'Silent' } },
    },
    outcomes: ['ok', 'failed'], outcome_rule: { fixed: ['ok', 'failed'] }, expressions: [], templates: ['text'], connector: 'telegram',
    output: { type: 'object', properties: { message_id: { type: 'integer' } } },
  },
  {
    type: 'crm.update_client', title: 'Update client', description: 'Updates fields of a CRM client.', group: 'CRM', behavior: 'effect',
    params: {
      type: 'object', required: ['client'], additionalProperties: false,
      properties: {
        client: { type: 'string', title: 'Client' },
        fields: { type: ['object', 'null'], title: 'Fields', properties: { phone: { type: 'string' }, note: { type: 'string', maxLength: 2000 } } },
        tags: { type: 'array', items: { type: 'string' }, title: 'Tags' },
        extra: {},
      },
    },
    outcomes: ['ok', 'failed'], outcome_rule: { fixed: ['ok', 'failed'] }, expressions: [], templates: [], output: { type: 'object', properties: { client: { type: 'object', properties: { id: { type: 'string' }, name: { type: 'string' } } } } },
  },
  {
    type: 'ai.classify', title: 'Classify text', description: 'Asks a model to pick a label.', group: 'AI', behavior: 'call', connected: false,
    params: { type: 'object', properties: { text: { type: 'string' } } }, outcomes: ['ok'], outcome_rule: { fixed: ['ok'] }, expressions: [], templates: ['text'], output: null,
  },
]

export const catalog: EditorCatalog = {
  revision: 'fixture-1',
  context_schema: { type: 'object', properties: { user: { type: 'object', properties: { id: { type: 'integer' }, email: { type: 'string' }, first_name: { type: 'string' } } } } },
  events: [
    { key: 'core.user.created.v1', title: 'User created', group: 'Users', subject: 'core.user', payload: { type: 'object', properties: { user: { type: 'object', properties: { id: { type: 'integer' }, email: { type: 'string' } } } } }, trigger_params: null },
    { key: 'core.user.updated.v1', title: 'User updated', group: 'Users', subject: 'core.user', payload: { type: 'object', properties: { changed: { type: 'array', items: { type: 'string' } } } }, trigger_params: { type: 'object', properties: { fields: { type: 'array', items: { type: 'string' }, maxItems: 50 } }, additionalProperties: false } },
    { key: 'crm.client.created.v1', title: 'Client created', group: 'CRM', subject: 'crm.client', payload: { type: 'object', properties: { client: { type: 'object', properties: { id: { type: 'string' }, name: { type: 'string' } } } } }, trigger_params: null },
    { key: 'core.schedule', title: 'On a schedule', group: 'Time', subject: 'none', payload: { type: 'object', properties: { at: { type: 'string' } } }, trigger_params: null, schedule: true },
    { key: 'core.manual', title: 'Manual start', group: 'Time', subject: 'none', payload: { type: 'object' }, trigger_params: null, manual: true },
    { key: 'telegram.message.v1', title: 'Telegram message received', group: 'Telegram', subject: 'conversation', payload: { type: 'object' }, trigger_params: null, connected: false },
  ],
  nodes,
}

export const emptyCatalog: EditorCatalog = { ...catalog, events: [] }
