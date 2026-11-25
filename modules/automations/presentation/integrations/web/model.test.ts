import { describe, expect, it } from 'vitest'
import {
  MSG, availablePresets, connectionPreset, knownPresets, presetOptions, changedCredentials, oauthErrorText, reasonText, errorCode, groupByCategory, missingRequired, newCredentials, saveErrorText, schemaFields, slugify, warningText,
  translator, usageHref, usedByText, type ConnectorView,
  MASKED_VALUE, SECRET_NAME_RE, secretFieldErrorText, secretFormErrors, secretMeta, secretName, secretSaveErrorText, sortSecrets, type SecretView,
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

describe('warningText', () => {
  it('shows the phrase of the code, whatever bot the confirmation is bound to', () => {
    const t = (key: string) => key
    expect(warningText(t, 'foreign_webhook')).toBe('Form.Warnings.foreign_webhook')
    expect(warningText(t, 'polling_unknown@222222')).toBe('Form.Warnings.polling_unknown')
  })
})

describe('sign-in presets of a connector with a form', () => {
  const connector = {
    kind: 'imap', oauth_presets: ['gmail'],
    credential_schema: { type: 'object', properties: { preset: { type: 'string', enum: ['custom', 'gmail', 'm365'], 'x-oauth-client': { gmail: 'google_oauth', m365: 'microsoft_oauth' } } } },
  } as unknown as ConnectorView

  it('knows the providers, and offers only those the application has', () => {
    expect(knownPresets(connector)).toEqual(['gmail', 'm365'])
    expect(availablePresets(connector)).toEqual(['gmail'])
    expect(presetOptions(connector, ['custom', 'gmail', 'm365'])).toEqual(['custom', 'gmail'])
    expect(presetOptions({ ...connector, oauth_presets: [] } as ConnectorView, ['custom', 'gmail', 'm365'])).toEqual(['custom'])
  })

  it('tells a sign-in connection by its preset, even when its provider is not offered now', () => {
    expect(connectionPreset(connector, { fields: { preset: 'm365' } })).toBe('m365')
    expect(connectionPreset(connector, { fields: { preset: 'custom' } })).toBeUndefined()
    expect(connectionPreset(connector, { fields: {} })).toBeUndefined()
    expect(connectionPreset({ kind: 'x', credential_schema: {} } as unknown as ConnectorView, { fields: { preset: 'gmail' } })).toBeUndefined()
  })
})

describe('secrets', () => {
  const st = translator((k) => ({
    [MSG + 'Form.Required']: 'Fill in this field.',
    [MSG + 'Form.Invalid']: 'Check this value.',
    [MSG + 'Secrets.Form.NameFormat']: 'Use API_KEY.',
    [MSG + 'Secrets.Form.NameTaken']: 'Taken.',
    [MSG + 'Secrets.Form.VersionConflict']: 'Changed meanwhile.',
    [MSG + 'Secrets.Form.Unavailable']: 'Store unavailable.',
    [MSG + 'Secrets.Form.Failed']: 'Could not save.',
    [MSG + 'Secrets.Created']: 'Created {date}',
    [MSG + 'Secrets.Rotated']: 'Rotated {date}',
    [MSG + 'Secrets.LastUsed']: 'Last used {date}',
    [MSG + 'Secrets.NeverUsed']: 'Not used yet',
  } as Record<string, string>)[k] ?? k)
  const secret = (extra: Partial<SecretView> = {}): SecretView => ({
    id: 's1', name: 'API_KEY', description: '', version: 1, created_at: '2026-11-02T10:00:00Z', updated_at: '2026-11-02T10:00:00Z',
    rotated_at: '2026-11-02T10:00:00Z', last_used_at: null, ...extra,
  })

  it('accepts names as the store keeps them', () => {
    for (const ok of ['API_KEY', 'A', 'X1_2']) expect(SECRET_NAME_RE.test(ok), ok).toBe(true)
    for (const bad of ['api_key', '1KEY', '_KEY', 'KEY-1', 'A'.repeat(65)]) expect(SECRET_NAME_RE.test(bad), bad).toBe(false)
    expect(secretName('api key')).toBe('API_KEY')
    expect(secretName('crm-token')).toBe('CRM_TOKEN')
  })

  it('refuses an empty value and a bad name before a request, per mode', () => {
    expect(secretFormErrors('create', '', '')).toEqual({ name: 'required', value: 'required' })
    expect(secretFormErrors('create', 'api', 'x')).toEqual({ name: 'secret_name' })
    expect(secretFormErrors('edit', 'API_KEY', '')).toEqual({})
    expect(secretFormErrors('rotate', '', '')).toEqual({ value: 'required' })
    expect(secretFieldErrorText(st, 'secret_name')).toBe('Use API_KEY.')
    expect(secretFieldErrorText(st, 'taken')).toBe('Taken.')
    expect(secretFieldErrorText(st, 'anything')).toBe('Check this value.')
  })

  it('explains a refused save without a raw code', () => {
    expect(secretSaveErrorText(st, { ok: false, error: 'name_taken' })).toBe('Taken.')
    expect(secretSaveErrorText(st, { ok: false, error: 'version_conflict' })).toBe('Changed meanwhile.')
    expect(secretSaveErrorText(st, { ok: false, error: 'unavailable' })).toBe('Store unavailable.')
    expect(secretSaveErrorText(st, { ok: false })).toBe('Could not save.')
  })

  it('tells the safe facts: created, rotated only when it was, last used', () => {
    expect(secretMeta(st, secret(), 'en')).toEqual(['Created Nov 2, 2026', 'Not used yet'])
    expect(secretMeta(st, secret({ rotated_at: '2026-11-10T08:00:00Z', last_used_at: '2026-11-20T09:00:00Z' }), 'en'))
      .toEqual(['Created Nov 2, 2026', 'Rotated Nov 10, 2026', 'Last used Nov 20, 2026'])
    expect(sortSecrets([secret({ name: 'B' }), secret({ name: 'A' })]).map((s) => s.name)).toEqual(['A', 'B'])
    expect(MASKED_VALUE).not.toMatch(/[A-Za-z0-9]/)
  })
})
