import { describe, expect, it } from 'vitest'
import { ManageApi } from './api'
import { exampleText, groupByKind, helpDocUrl, matches, pickNamespace, typeDocs } from './reference'
import type { ReferenceItem, ReferenceModel } from './types'

const doc = (signature: string, example: unknown = {}) => ({ signature, schema: { type: 'object' }, example })
const item = (id: string, kind: string, extra: Partial<ReferenceItem> = {}): ReferenceItem => ({ id, kind, ...extra })

const model: ReferenceModel = {
  full: true,
  namespaces: [
    { name: 'acme.loyalty', package: { namespace: 'acme.loyalty', version: '1.2.0' }, items: [item('acme.loyalty.award', 'command')] },
    { name: 'core', native: true, items: [item('core.user', 'entity')] },
  ],
}

describe('pickNamespace', () => {
  it('prefers the user pick, then the selected package, then the first namespace', () => {
    expect(pickNamespace(model, 'core', 'acme.loyalty')?.name).toBe('core')
    expect(pickNamespace(model, undefined, 'acme.loyalty')?.name).toBe('acme.loyalty')
    expect(pickNamespace(model, undefined, 'unknown')?.name).toBe('acme.loyalty')
  })

  it('falls through a pick that is no longer in the model', () => {
    expect(pickNamespace(model, 'gone', 'core')?.name).toBe('core')
  })

  it('has nothing for an empty or missing model', () => {
    expect(pickNamespace(undefined, 'core', 'core')).toBeUndefined()
    expect(pickNamespace({ full: false, namespaces: [] }, undefined, undefined)).toBeUndefined()
  })
})

describe('helpDocUrl', () => {
  it('points at the namespace document of the Help Center', () => {
    expect(helpDocUrl('acme.loyalty')).toBe('/help/doc/extensions/reference/acme.loyalty')
    expect(helpDocUrl('a/b')).toBe('/help/doc/extensions/reference/a%2Fb')
  })
})

describe('groupByKind', () => {
  it('orders kinds as the reference does and keeps unknown kinds last', () => {
    const groups = groupByKind([
      item('b', 'query'), item('a', 'entity'), item('c', 'mystery'), item('d', 'command'), item('e', 'query'),
    ])
    expect(groups.map((g) => g.kind)).toEqual(['entity', 'command', 'query', 'mystery'])
    expect(groups[2]?.items.map((i) => i.id)).toEqual(['b', 'e'])
  })

  it('is empty without items', () => {
    expect(groupByKind([])).toEqual([])
  })
})

describe('typeDocs', () => {
  it('lists the typed parts in reading order, signals last', () => {
    const it = item('x', 'command', {
      output: doc('out'), input: doc('in'), record: doc('rec'),
      signals: [{ name: 'stop', payload: doc('sig') }],
    })
    expect(typeDocs(it).map(([label, d]) => `${label}:${d.signature}`)).toEqual(['record:rec', 'input:in', 'output:out', 'signal stop:sig'])
  })
})

describe('matches', () => {
  const award = item('acme.loyalty.award', 'command', { label: 'Award points', permission: 'Loyalty.Manage' })
  it('matches on id, label, kind and permission, ignoring case', () => {
    for (const q of ['award', 'POINTS', 'command', 'loyalty.manage', '  ']) expect(matches(award, q)).toBe(true)
    expect(matches(award, 'nothing')).toBe(false)
  })
})

describe('exampleText', () => {
  it('pretty-prints JSON', () => {
    expect(exampleText({ a: [1] })).toBe('{\n  "a": [\n    1\n  ]\n}')
    expect(exampleText(undefined)).toBe('null')
  })
})

describe('ManageApi.reference', () => {
  const respond = (body: unknown): { calls: string[]; api: ManageApi } => {
    const calls: string[] = []
    const fetchStub = (async (input: RequestInfo | URL) => {
      calls.push(String(input))
      return new Response(JSON.stringify(body), { status: 200, headers: { 'content-type': 'application/json' } })
    }) as typeof fetch
    return { calls, api: new ManageApi({ base: '/_ext/manage', fetch: fetchStub }) }
  }

  it('reads the reference from the management API', async () => {
    const { calls, api } = respond(model)
    expect(await api.reference()).toEqual(model)
    expect(calls).toEqual(['/_ext/manage/reference'])
  })

  it('passes the locale', async () => {
    const { calls, api } = respond(model)
    await api.reference('ru-RU')
    expect(calls).toEqual(['/_ext/manage/reference?locale=ru-RU'])
  })

  it('reports a denied session', async () => {
    const api = new ManageApi({
      base: '/_ext/manage',
      fetch: (async () => new Response(JSON.stringify({ error: { code: 'unauthenticated', message: 'sign in' } }), { status: 401 })) as typeof fetch,
    })
    await expect(api.reference()).rejects.toMatchObject({ status: 401, code: 'unauthenticated', isDenied: true })
  })
})
