import { describe, expect, it } from 'vitest'
import {
  MSG, changedCredentials, oauthErrorText, reasonText, errorCode, groupByCategory, missingRequired, newCredentials, saveErrorText, schemaFields, slugify,
  translator, usageHref, usedByText, type ConnectorView,
} from './model'

const messages: Record<string, string> = {
  [MSG + 'UsedBy.Singular']: '{count} automation',
  [MSG + 'UsedBy.Paucal']: '{count} автоматизациях (few)',
  [MSG + 'UsedBy.Plural']: '{count} automations',
  [MSG + 'UsedBy.None']: 'Not used',
  [MSG + 'Form.TestFailed']: 'Refused: {message}',
  [MSG + 'Form.Failed']: 'Could not save.',
  [MSG + 'Form.Invalid']: 'Check this value.',
}
const t = translator((k) => messages[k] ?? k)

const connector = (kind: string, category: ConnectorView['category']): ConnectorView => ({
  kind, title: kind, icon: '', description: '', category, auth: 'api_key', credential_schema: {}, nodes: [], platform: null, connections: [],
})

describe('model', () => {
  it('groups connectors by category in a fixed order and drops empty groups', () => {
    const groups = groupByCategory([connector('openai', 'ai'), connector('smtp', 'email'), connector('tg', 'messaging'), connector('x', 'other')])
    expect(groups.map((g) => g.category)).toEqual(['messaging', 'email', 'ai', 'other'])
  })

  it('pluralizes by the language', () => {
    expect(usedByText(t, 0, 'en')).toBe('Not used')
    expect(usedByText(t, 1, 'en')).toBe('1 automation')
    expect(usedByText(t, 3, 'en')).toBe('3 automations')
    expect(usedByText(t, 3, 'ru')).toBe('3 автоматизациях (few)')
    // A missing form falls back to other.
    expect(usedByText(t, 5, 'ru')).toBe('5 automations')
  })

  it('reads form fields from the credentials schema', () => {
    const fields = schemaFields({
      type: 'object', required: ['token', 'host'],
      properties: {
        token: { type: 'string', title: 'Bot token', 'x-secret': true },
        host: { type: 'string', examples: ['smtp.example.com'] },
        port: { type: 'integer' },
        tls: { type: 'boolean' },
        mode: { type: 'string', enum: ['a', 'b'] },
      },
    })
    expect(fields.map((f) => [f.name, f.kind, f.required])).toEqual([
      ['token', 'secret', true], ['host', 'text', true], ['port', 'number', false], ['tls', 'boolean', false], ['mode', 'enum', false],
    ])
    expect(fields[0]!.title).toBe('Bot token')
    expect(fields[1]!.title).toBe('Host')
    expect(fields[1]!.placeholder).toBe('smtp.example.com')
    expect(schemaFields(null)).toEqual([])
  })

  it('derives a valid system name', () => {
    expect(slugify('Sales bot', 'telegram_bot')).toBe('sales_bot')
    expect(slugify('Бот продаж', 'telegram_bot')).toBe('bot_prodazh')
    expect(slugify('123', 'telegram_bot')).toBe('telegram_bot_123')
    expect(slugify('', 'smtp')).toBe('smtp')
    expect(slugify('a'.repeat(60), 'x')).toHaveLength(40)
  })

  it('sends only what changed and never an untouched secret', () => {
    const fields = schemaFields({ properties: { token: { type: 'string', 'x-secret': true }, chat: { type: 'string' } } })
    expect(changedCredentials(fields, { chat: 'a' }, { chat: 'a' }, new Set())).toBeUndefined()
    expect(changedCredentials(fields, { chat: 'b' }, { chat: 'a' }, new Set())).toEqual({ chat: 'b' })
    expect(changedCredentials(fields, { chat: 'a', token: 'new' }, { chat: 'a' }, new Set(['token']))).toEqual({ chat: 'a', token: 'new' })
    expect(changedCredentials(fields, { chat: 'a', token: 'typed' }, { chat: 'a' }, new Set())).toBeUndefined()
    expect(newCredentials(fields, { token: 's', chat: '' })).toEqual({ token: 's' })
    const numeric = schemaFields({ properties: { chat: { type: 'integer' } } })
    expect(newCredentials(numeric, { chat: '-1001234567890123456' })).toEqual({ chat: '-1001234567890123456' })
  })

  it('checks required fields, counting a set secret as filled', () => {
    const fields = schemaFields({ required: ['token', 'chat'], properties: { token: { type: 'string', 'x-secret': true }, chat: { type: 'string' } } })
    expect(missingRequired(fields, {}, () => false)).toEqual({ token: 'required', chat: 'required' })
    expect(missingRequired(fields, { chat: 'x' }, () => true)).toEqual({})
  })

  it('explains refusals in words and reads RPC error codes', () => {
    const tr = translator((k) => ({ ...messages, [MSG + 'Reasons.auth.Text']: 'the key was refused', [MSG + 'Reasons.auth.Fix']: 'Replace it.' })[k] ?? k)
    expect(saveErrorText(tr, { ok: false, error: 'test_failed', reason: 'auth' })).toBe('Refused: the key was refused. Replace it.')
    expect(reasonText(tr, 'not-a-reason')).toContain('Reasons.unknown')
    expect(oauthErrorText(tr, 'weird')).toBe('OAuth.exchange')
    expect(saveErrorText(t, { ok: false })).toBe('Could not save.')
    expect(errorCode({ code: 'unknown', details: { code: 'not_found' } })).toBe('not_found')
    expect(errorCode({ code: 'permission_denied' })).toBe('permission_denied')
    expect(errorCode(undefined)).toBeUndefined()
  })

  it('links a use to the graph and its node', () => {
    expect(usageHref('/automations/', { automation_id: 'a1', name: 'n', version: 2, enabled: true, node_ids: ['send'] })).toBe('/automations/a1?node=send')
    expect(usageHref('/automations', { automation_id: 'a1', name: 'n', version: 2, enabled: true, node_ids: [] })).toBe('/automations/a1')
  })
})
