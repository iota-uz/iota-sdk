import { describe, expect, it } from 'vitest'
import type { EditorCatalog } from '@iota-uz/sdk/automation-editor'
import {
  counted, editorLocale, errorText, graphPath, graphStatus, graphStatusText, highlight, inputFields, inputValue, manualTriggers, readFilters,
  relativeTime, runStatusKey, translator, writeFilters,
} from './model'

const messages: Record<string, string> = {
  'Automations.Graphs.Status.Enabled': 'On v{version}',
  'Automations.Graphs.Status.Disabled': 'Off',
  'Automations.Graphs.Status.Pending': 'New version v{version}',
  'Automations.Graphs.Status.PendingEnabled': 'On v{enabled} · new version v{version}',
  'Automations.Graphs.Delete.Runs.Singular': '{count} запуск',
  'Automations.Graphs.Delete.Runs.Paucal': '{count} запуска',
  'Automations.Graphs.Delete.Runs.Multiple': '{count} запусков',
  'Automations.Graphs.Delete.Runs.Plural': '{count} запуска',
  'Automations.Graphs.Errors.Forbidden': 'No right',
  'Automations.Graphs.Errors.Generic': 'Could not {action}.',
}
const t = translator((k) => messages[k] ?? k)

describe('graph status', () => {
  it('names the status in words with the version (AL1)', () => {
    expect(graphStatusText(t, graphStatus({ enabled: true, enabled_version: 3, latest_version: 3, pending_version: false }))).toBe('On v3')
    expect(graphStatusText(t, graphStatus({ enabled: false, enabled_version: null, latest_version: 2, pending_version: false }))).toBe('Off')
    expect(graphStatusText(t, graphStatus({ enabled: true, enabled_version: 1, latest_version: 2, pending_version: true }))).toBe('On v1 · new version v2')
    expect(graphStatusText(t, graphStatus({ enabled: false, enabled_version: null, latest_version: 2, pending_version: true }))).toBe('New version v2')
  })
})

describe('counted', () => {
  it('takes the plural form of the language', () => {
    expect(counted(t, 'Delete.Runs', 1, 'ru')).toBe('1 запуск')
    expect(counted(t, 'Delete.Runs', 3, 'ru')).toBe('3 запуска')
    expect(counted(t, 'Delete.Runs', 5, 'ru')).toBe('5 запусков')
  })
})

describe('filters in the URL (D1, D2)', () => {
  it('reads and writes query and status, keeping other parameters', () => {
    expect(readFilters('?q=%20digest%20&status=attention')).toEqual({ query: 'digest', status: 'attention' })
    expect(readFilters('?status=bogus')).toEqual({ query: '', status: '' })
    expect(writeFilters('https://erp.test/automations?x=1&status=enabled', { query: 'a b', status: '' })).toBe('/automations?x=1&q=a+b')
  })
})

describe('errors (C1, C2)', () => {
  it('turns codes into phrases and never shows a raw code', () => {
    expect(errorText(t, { code: 'forbidden' }, 'save')).toBe('No right')
    expect(errorText(t, { details: { code: 'forbidden' } }, 'save')).toBe('No right')
    expect(errorText(t, { code: 'VALIDATION_FAILED_UNKNOWN' }, 'save')).toBe('Could not save.')
  })
})

describe('highlight', () => {
  it('keeps the text and marks keywords, strings, comments and numbers', () => {
    const src = 'const x = "a\\"b" // hi\nreturn 42 /* c */'
    const tokens = highlight(src)
    expect(tokens.map((x) => x.text).join('')).toBe(src)
    expect(tokens.filter((x) => x.kind === 'keyword').map((x) => x.text)).toEqual(['const', 'return'])
    expect(tokens.find((x) => x.kind === 'string')?.text).toBe('"a\\"b"')
    expect(tokens.filter((x) => x.kind === 'comment').map((x) => x.text)).toEqual(['// hi', '/* c */'])
    expect(tokens.find((x) => x.kind === 'number')?.text).toBe('42')
  })
  it('never treats markup as anything but text', () => {
    const src = '<img src=x onerror=alert(1)>'
    expect(highlight(src).map((x) => x.text).join('')).toBe(src)
  })
})

describe('manual start input (AL9)', () => {
  const schema = { type: 'object', required: ['amount'], properties: { amount: { type: 'integer', title: 'Amount' }, note: { type: 'string' }, urgent: { type: 'boolean' }, kind: { enum: ['a', 'b'] }, meta: { type: 'object' } } }
  it('builds fields from the schema', () => {
    expect(inputFields(schema).map((f) => [f.name, f.kind, f.required])).toEqual([
      ['amount', 'integer', true], ['note', 'text', false], ['urgent', 'boolean', false], ['kind', 'enum', false], ['meta', 'json', false],
    ])
  })
  it('reads values by type and reports the fields it cannot read', () => {
    const fields = inputFields(schema)
    expect(inputValue(fields, { amount: '12', note: ' hi ', urgent: true, meta: '{"a":1}' })).toEqual({ input: { amount: 12, note: 'hi', urgent: true, meta: { a: 1 } }, errors: {} })
    expect(inputValue(fields, { amount: '1.5', meta: '{' }).errors).toEqual({ amount: 'number', meta: 'json' })
    expect(inputValue(fields, {}).errors).toEqual({ amount: 'required' })
  })
  it('sends an enum value with its type', () => {
    const fields = inputFields({ type: 'object', properties: { level: { type: 'integer', enum: [1, 2] }, tag: { enum: ['1', 'b'] } } })
    expect(fields[0]!.options).toEqual(['1', '2'])
    expect(inputValue(fields, { level: '1', tag: '0' }).input).toEqual({ level: 2, tag: '1' })
  })
  it('finds the manual triggers of a graph', () => {
    const catalog = { events: [{ key: 'automation.manual.v1', title: 'Manual start', manual: true }, { key: 'x.v1', title: 'X' }] } as unknown as EditorCatalog
    const graph = { name: 'g', run_as: 'owner' as const, nodes: [], edges: [], triggers: [{ id: 'm', event: 'automation.manual.v1', params: { input_schema: schema } }, { id: 'e', event: 'x.v1' }] }
    expect(manualTriggers(graph, catalog)).toEqual([{ id: 'm', title: 'Manual start', schema }])
  })
})

describe('small helpers', () => {
  it('maps languages, paths, statuses and times', () => {
    expect(editorLocale('uz-Cyrl')).toBe('uz-Cyrl')
    expect(editorLocale('ru')).toBe('ru')
    expect(editorLocale('pt-BR')).toBe('en')
    expect(graphPath('/automations/', 'a b', { tab: 'runs' })).toBe('/automations/a%20b?tab=runs')
    expect(graphPath('/automations', 'id', { run: 'r' })).toBe('/automations/id/runs/r')
    expect(runStatusKey('filter_failed')).toBe('Run.FilterFailed')
    const now = new Date('2026-11-24T10:00:00Z')
    expect(relativeTime('2026-11-24T09:55:00Z', now, 'en')).toBe('5 minutes ago')
    expect(relativeTime('2026-11-24T09:59:50Z', now, 'en')).toBe('now')
  })
})
