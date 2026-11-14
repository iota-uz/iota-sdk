// Runtime behavior behind the audit fixes (type-level behavior lives in typetests/audit.ts).
import { describe, expect, it } from 'vitest'
import * as pub from '../src/index.ts'
import {
  $, ContractError, WireError, column, command, compileFilter, defineSlot, domainError, encode, entity, field, href, issue, operation,
  overlay, ref, route, slotRef, t, validation, type ValidationIssue,
} from '../src/index.ts'
import { foreign, nativeEntity, nativeList } from '../src/kit.ts'
import { cell, isError, issuesOf, navigation, page, RemoteError, slot } from '../src/client.ts'
import { collectManifest, ManifestError } from '../src/manifest.ts'
import { fixture } from '../src/testing.ts'
import { implement } from '../src/server.ts'
import * as kit from '../typetests/kit.ts'
import * as c from '../typetests/acme/contracts.ts'

const ns = { namespace: 'acme.insurance', server: {}, client: {}, contracts: {} }
const problems = (input: Parameters<typeof collectManifest>[0]): readonly string[] => {
  try {
    collectManifest(input)
  } catch (e) {
    expect(e).toBeInstanceOf(ManifestError)
    return (e as ManifestError).problems
  }
  throw new Error('expected collectManifest to throw')
}

describe('1. contracts cannot be forged by ID', () => {
  it('keeps foreign, nativeEntity, nativeList and contract off the public entry', () => {
    for (const name of ['foreign', 'nativeEntity', 'nativeList', 'contract']) expect(name in pub).toBe(false)
    expect(typeof foreign).toBe('function')
  })

  it('rejects a locally built contract outside the namespace even when it is only implemented', () => {
    const Fake = operation('core.users.rename', { input: t.object({}) })
    const impl = implement(Fake, async () => {})
    expect(problems({ ...ns, server: { impl } })).toEqual([expect.stringMatching(/"core.users.rename" is outside namespace "acme.insurance"; contracts of other packages come from the developer kit/)])
  })

  it('rejects a contract that claims to be foreign without coming through the kit', () => {
    const Fake = { ...operation('core.users.rename', { input: t.object({}) }), foreign: true as const }
    const impl = implement(Fake, async () => {})
    expect(problems({ ...ns, contracts: { Fake }, server: { impl } })).toEqual(
      expect.arrayContaining([expect.stringMatching(/marked foreign but was not issued by the developer kit/)]),
    )
  })

  it('rejects a kit contract that shadows the package own namespace', () => {
    const Shadow = foreign(operation('acme.insurance.shadow', { input: t.object({}) }))
    expect(problems({ ...ns, contracts: { Shadow } })).toEqual([expect.stringMatching(/inside this package's own namespace/)])
  })

  it('accepts kit contracts, which are referenced and never emitted', () => {
    const impl = implement(kit.RenameUser, async () => {})
    const manifest = collectManifest({ ...ns, contracts: { ...kit }, server: { impl } })
    expect(manifest.resources).toEqual([])
    expect(manifest.implementations).toEqual([{ export: 'impl', implements: 'core.users.rename', kind: 'operation' }])
  })
})

describe('2. falsy defaults are emitted', () => {
  it('emits false, empty string, empty array, zero and null defaults', () => {
    const E = entity('acme.insurance.defaults', {
      flag: field.bool().default(false),
      name: field.text().default(''),
      tags: field.json<string[]>().default([]),
      points: field.int().default(0),
      note: field.text().nullable().default(null),
      plain: field.text(),
    })
    const byId = Object.fromEntries(E.resource.fields!.map((f) => [f.id, f]))
    expect(byId.flag).toEqual({ id: 'flag', type: { kind: 'bool' }, default: false })
    expect(byId.name).toEqual({ id: 'name', type: { kind: 'string' }, default: '' })
    expect(byId.tags).toEqual({ id: 'tags', type: { kind: 'json' }, default: [] })
    expect(byId.points).toEqual({ id: 'points', type: { kind: 'int' }, default: 0 })
    expect(byId.note).toEqual({ id: 'note', type: { kind: 'string', nullable: true }, default: null })
    expect('default' in byId.plain!).toBe(false)
    expect(JSON.parse(JSON.stringify(E.resource)).fields[0]).toEqual({ id: 'flag', type: { kind: 'bool' }, default: false })
  })

  it('emits the default of an overlay field and keeps unique/index flags next to it', () => {
    const O = overlay('acme.insurance.defaultsOverlay', kit.Client, { on: field.bool().index().default(false) })
    expect(O.resource.fields).toEqual([{ id: 'on', type: { kind: 'bool' }, index: true, default: false }])
  })

  it('compiles the access of an overlay: field permissions and row rules reach the manifest', () => {
    const Secret = pub.permission('acme.insurance.seeSecret', { label: { en: 'See secrets' } })
    const O = overlay(
      'acme.insurance.secretOverlay',
      kit.Client,
      { note: field.text().optional(), score: field.int().optional() },
      { access: { fields: { note: [Secret] }, read: [{ when: Secret, rows: 'all' }, { rows: 'none' }] } },
    )
    expect(O.resource.access).toEqual({
      read: [{ permission: 'acme.insurance.seeSecret', all: true }, {}],
      fields: { note: ['acme.insurance.seeSecret'] },
    })
    expect(overlay('acme.insurance.openOverlay', kit.Client, { on: field.bool().default(false) }).resource.access).toBeUndefined()
  })

  it('rejects a default the column cannot hold', () => {
    expect(() => field.int().default('0' as never)).toThrow(/invalid field default/)
    expect(() => field.text({ maxLength: 2 }).default('abc')).toThrow(/longer than 2/)
    expect(() => field.text().default(null as never)).toThrow(/invalid field default/)
  })
})

describe('4. refs', () => {
  it('validates the key against the native key type', () => {
    expect(ref(kit.User, '42')).toEqual({ entity: 'core.user', key: '42' })
    expect(() => ref(kit.User, 'abc')).toThrow(/not a valid int64 key/)
    expect(() => ref(kit.User, '9223372036854775808')).toThrow(/int64/)
    expect(ref(kit.Client, '123e4567-e89b-42d3-a456-426614174000').entity).toBe('crm.client')
    expect(() => ref(kit.Client, '42')).toThrow(/not a valid uuid key/)
    expect(() => ref(c.Policy, 'not-a-uuid')).toThrow(/not a valid uuid key/)
    expect(() => ref(kit.User, '')).toThrow()
  })

  it('rejects $.user on a direct field that is not a ref to core.user', () => {
    expect(() =>
      entity('acme.insurance.bad1', { a: field.text() }, { access: { read: [{ rows: { a: $.user as never } }] } }),
    ).toThrow(/\$\.user can only be compared with a ref to core\.user, but "a" is not one/)
    expect(() =>
      entity('acme.insurance.bad2', { o: field.ref(c.Customer) }, { access: { read: [{ rows: { o: { in: [$.user] } as never } }] } }),
    ).toThrow(/core\.user/)
    expect(() =>
      entity('acme.insurance.bad3', { a: field.text() }, { access: { read: [{ rows: { or: [{ a: $.user as never }] } }] } }),
    ).toThrow(/core\.user/)
    const ok = entity('acme.insurance.ok', { owner: field.ref(kit.User) }, { access: { read: [{ rows: { owner: $.user } }] } })
    expect(ok.resource.access?.read).toEqual([{ rows: { fields: { owner: { eq: { $: 'user' } } } } }])
  })

  it('reserves `ref` as a system column', () => {
    expect(() => entity('acme.insurance.refcol', { ref: field.text() })).toThrow(/reserved/)
  })
})

describe('5. JSON on the wire', () => {
  it('rejects NaN and infinities, top-level and nested', () => {
    expect(() => encode(t.json(), Number.NaN)).toThrow(/non-finite/)
    expect(() => encode(t.json(), Number.POSITIVE_INFINITY)).toThrow(/non-finite/)
    expect(() => encode(t.json(), Number.NEGATIVE_INFINITY)).toThrow(/non-finite/)
    expect(() => encode(t.object({ j: t.json() }), { j: { a: [1, Number.NaN] } })).toThrow(/^j\.a\.1: invalid json: non-finite number/)
    expect(encode(t.json(), { a: [1, 2.5, -0, null, 'x', true] })).toEqual({ a: [1, 2.5, -0, null, 'x', true] })
  })

  it('rejects null for a json type unless it is nullable, like the Go validator', () => {
    expect(() => encode(t.json(), null as never)).toThrow(/value is required/)
    expect(encode(t.json().nullable(), null)).toBeNull()
  })
})

describe('6. optional / nullable semantics', () => {
  it('emits optional fields as nullable and omittable', () => {
    const E = entity('acme.insurance.opt', { a: field.text().optional(), b: field.int().optional().default(5), c: field.text().nullable() })
    expect(E.resource.fields).toEqual([
      { id: 'a', type: { kind: 'string', optional: true, nullable: true } },
      { id: 'b', type: { kind: 'int', optional: true, nullable: true }, default: 5 },
      { id: 'c', type: { kind: 'string', nullable: true } },
    ])
  })
})

describe('7. t.row and t.fields', () => {
  const customer = fixture(c.Customer, { name: 'Ali', phone: '+998901234567' })
  const policy = fixture(c.Policy, { number: 'P-1', holder: customer.ref, premium: '10.00' as never, expiresOn: '2027-01-01' as never })

  it('describes the row the data layer returns, ref included', () => {
    const schema = t.row(c.Policy)
    expect(schema.type.kind).toBe('object')
    const names = schema.type.properties!.map((p) => p.name)
    expect(names.slice(0, 5)).toEqual(['key', 'createdAt', 'updatedAt', 'rowVersion', 'ref'])
    expect(schema.type.properties!.find((p) => p.name === 'ref')!.type).toEqual({ kind: 'ref', entity: 'acme.insurance.policy' })
    expect(schema.type.properties!.find((p) => p.name === 'notes')!.type).toEqual({ kind: 'string', nullable: true })
    const encoded = encode(schema, policy.record)
    expect(encoded).toMatchObject({ key: policy.ref.key, ref: policy.ref, number: 'P-1', notes: null, status: 'draft' })
  })

  it('encodes rows against the schema and rejects a row of another shape', () => {
    expect(() => encode(t.row(c.Customer), policy.record as never)).toThrow(WireError)
    expect(() => encode(t.row(c.Policy), { ...policy.record, ref: customer.ref as never })).toThrow(/expected ref to acme.insurance.policy/)
    expect(() => encode(t.row(c.Policy), { ...policy.record, bogus: 1 } as never)).toThrow(/unknown property/)
  })

  it('uses the key type of native entities and overlays', () => {
    expect(t.row(kit.User).type.properties![0]).toEqual({ name: 'key', type: { kind: 'int64' } })
    expect(t.row(c.ClientExtra).type.properties![0]).toEqual({ name: 'baseKey', type: { kind: 'uuid' } })
  })

  it('t.fields projects fields as read from a row, optionally picked', () => {
    const all = t.fields(c.Customer)
    expect(all.type.properties!.map((p) => p.name)).toEqual(['name', 'phone', 'owner'])
    const some = t.fields(c.Customer, ['name', 'phone'])
    expect(some.type.properties).toEqual([
      { name: 'name', type: { kind: 'string' } },
      { name: 'phone', type: { kind: 'string', nullable: true } },
    ])
    expect(encode(some, { name: 'a', phone: null })).toEqual({ name: 'a', phone: null })
    expect(() => encode(some, { name: 'a' } as never)).toThrow(/^phone: value is required/)
    expect(() => t.fields(c.Customer, ['nope' as never])).toThrow(/no field "nope"/)
  })
})

describe('8. routes', () => {
  it('requires params to match the :segments of the path', () => {
    expect(() => route('acme.insurance.a', { path: '/a/:id/:n', params: t.object({ id: t.uuid() }) as never })).toThrow(/":n" has no schema/)
    expect(() => route('acme.insurance.b', { path: '/a/:id', params: t.object({ id: t.uuid(), extra: t.string() }) as never })).toThrow(/"extra", which is not a :segment/)
    expect(() => route('acme.insurance.c', { path: '/a/:id', params: t.object({ id: t.ref(c.Policy) }) as never })).toThrow(/":id" is ref/)
    expect(() => route('acme.insurance.d', { path: '/a/:id', params: t.object({ id: t.string().nullable() }) as never })).toThrow(/required and non-null/)
    expect(() => route('acme.insurance.e', { path: '/a/:id', params: t.object({ id: t.string().optional() }) as never })).toThrow(/required and non-null/)
    expect(() => route('acme.insurance.f', { path: '/a/x:id', params: t.object({}) as never })).toThrow(/must be a literal or exactly ":name"/)
    expect(() => route('acme.insurance.g', { path: '/a/:id/:id', params: t.object({ id: t.string() }) as never })).toThrow(/appears twice/)
    expect(() => route('acme.insurance.h', { path: 'a/:id', params: t.object({ id: t.string() }) as never })).toThrow(/must start with "\/"/)
    expect(() => route('acme.insurance.i', { path: '/a/:id', params: t.string() as never })).toThrow(/params must be t.object/)
    expect(() => route('acme.insurance.j', { path: '/a/:id', params: t.object({ id: t.enum(['a']) }) as never })).toThrow(/":id" is enum/)
    const ok = route('acme.insurance.ok', { path: '/a/:id/b/:n', params: t.object({ id: t.int64(), n: t.int() }) })
    expect(ok.resource).toMatchObject({ kind: 'route', path: '/a/:id/b/:n' })
  })

  it('builds hrefs: validated, percent-encoded, never traversing', () => {
    const r = route('acme.insurance.files', { path: '/files/:name/v/:n', params: t.object({ name: t.string(), n: t.int() }) })
    expect(href(r, { name: 'a b/c?d', n: 3 })).toBe('/files/a%20b%2Fc%3Fd/v/3')
    expect(href(c.PolicyRoute, { id: '123e4567-e89b-42d3-a456-426614174000' })).toBe('/insurance/policies/123e4567-e89b-42d3-a456-426614174000')
    expect(() => href(r, { name: '..', n: 1 })).toThrow(/cannot be "\.\."/)
    expect(() => href(r, { name: '', n: 1 })).toThrow(/cannot be ""/)
    expect(() => href(c.PolicyRoute, { id: 'nope' })).toThrow(/uuid/)
    expect(() => href(r, { name: 'x', n: 1.5 })).toThrow(/safe integer/)
    expect(() => href(r, {} as never)).toThrow(/name: value is required/)
    expect(href(route('acme.insurance.root', { path: '/', params: t.object({}) }), {})).toBe('/')
  })
})

describe('9. manifest builder holes', () => {
  it('lowers the first character of implicit client ids, keeping the export name', () => {
    const PolicyBadge = slot('crm.slots.clientToolbar', () => 'x')
    const TierCell = cell(c.ClientsTierColumn, () => 'x')
    const m = collectManifest({ ...ns, contracts: { ClientsTierColumn: c.ClientsTierColumn, ClientExtra: c.ClientExtra, Client: kit.Client }, client: { PolicyBadge, TierCell } })
    expect(m.resources.filter((r) => r.kind === 'slot' || r.kind === 'cell').map((r) => [r.id, r.export])).toEqual([
      ['acme.insurance.policyBadge', 'PolicyBadge'],
      ['acme.insurance.tierCell', 'TierCell'],
    ])
  })

  it('reports clashes between Foo and foo exports instead of emitting duplicates', () => {
    const Foo = slot('crm.slots.a', () => 'x')
    const foo = slot('crm.slots.a', () => 'y')
    expect(problems({ ...ns, client: { Foo, foo } })).toEqual([expect.stringMatching(/"acme.insurance.foo" is already declared/)])
  })

  it('rejects dangling in-package navigation references and allows foreign ones', () => {
    const lost = page('acme.insurance.lost', {
      path: '/lost',
      nav: { label: { en: 'x' }, group: 'acme.insurance.nogroup', parent: 'acme.insurance.noparent' },
      component: () => 'x',
    })
    const nav = navigation({
      groups: [{ id: 'acme.insurance.known', label: { en: 'Known' } }],
      palette: [{ id: 'acme.insurance.p', label: { en: 'P' }, target: 'acme.insurance.missing' }],
      patches: [{ ref: 'core.users', hide: true }, { ref: 'acme.insurance.ghost', hide: true }],
    })
    expect(problems({ ...ns, client: { lost, nav } })).toEqual([
      expect.stringMatching(/nav\.group "acme.insurance.nogroup" is not declared/),
      expect.stringMatching(/nav\.parent "acme.insurance.noparent" is not a page or group/),
      expect.stringMatching(/palette entry "acme.insurance.p" targets "acme.insurance.missing"/),
      expect.stringMatching(/nav patch references "acme.insurance.ghost"/),
    ])
    const fine = page('acme.insurance.fine', {
      path: '/fine',
      nav: { label: { en: 'x' }, group: 'core.settings', parent: 'crm.clients.list' },
      component: () => 'x',
    })
    const nav2 = navigation({ palette: [{ id: 'acme.insurance.q', label: { en: 'Q' }, target: 'crm.clients.view' }], patches: [{ ref: 'core.users', parent: 'crm.group' }] })
    expect(collectManifest({ ...ns, client: { fine, nav2 } }).resources).toHaveLength(2)
  })

  it('accepts a group that is the namespace itself and a parent that is an exported page', () => {
    const parent = page('acme.insurance.parent', { path: '/p', nav: { label: { en: 'P' }, group: 'acme.insurance' }, component: () => 'x' })
    const child = page('acme.insurance.child', { path: '/p/c', nav: { label: { en: 'C' }, parent: 'acme.insurance.parent' }, component: () => 'x' })
    const nav = navigation({ groups: [{ id: 'acme.insurance', label: { en: 'I' } }] })
    expect(collectManifest({ ...ns, client: { parent, child, nav } }).resources).toHaveLength(3)
  })

  it('rejects branded values nested in exported plain objects', () => {
    const Policy = entity('acme.insurance.n1', { a: field.text() })
    const Op = operation('acme.insurance.n2', { input: t.object({}) })
    const impl = implement(Op, async () => {})
    expect(
      problems({
        ...ns,
        contracts: { all: { Policy }, list: [Op], deep: { a: { b: [{ c: Policy }] } }, harmless: { a: 1, b: ['x'] } },
        server: { handlers: { impl } },
      }),
    ).toEqual([
      expect.stringMatching(/contracts export "all": contains "Policy"; export resources as top-level named exports/),
      expect.stringMatching(/contracts export "list": contains "0"; export resources as top-level named exports/),
      expect.stringMatching(/contracts export "deep": contains "a\.b\.0\.c"; export resources/),
      expect.stringMatching(/server export "handlers": contains "impl"; export resources as top-level named exports/),
    ])
  })

  it('rejects an empty read list as ambiguous and offers an explicit deny', () => {
    expect(() => entity('acme.insurance.amb', { a: field.text() }, { access: { read: [] } })).toThrow(/access\.read is empty, which is ambiguous/)
    const denied = entity('acme.insurance.deny', { a: field.text() }, { access: { read: [{ rows: 'none' }] } })
    expect(denied.resource.access).toEqual({ read: [{}] })
    const gated = entity('acme.insurance.gated', { a: field.text() }, { access: { read: [{ when: c.ReadPolicies, rows: 'none' }, { rows: 'all' }] } })
    expect(gated.resource.access?.read).toEqual([{ permission: 'acme.insurance.readPolicies' }, { all: true }])
    expect(entity('acme.insurance.open', { a: field.text() }, { access: {} }).resource.access).toEqual({})
  })
})

describe('10. errors', () => {
  it('only accepts issue instances produced by Issue.at', () => {
    const TooLong = issue('acme.insurance.tl', t.object({ max: t.int() }))
    const made = TooLong.at(['notes'], { max: 3 })
    const error = validation(made)
    expect(error).toBeInstanceOf(ContractError)
    expect(error.kind).toBe('validation')
    expect(error.issues).toEqual([made])
    expect(JSON.parse(JSON.stringify(error.issues))).toEqual([{ code: 'acme.insurance.tl', path: ['notes'], params: { max: 3 } }])
    const forged = { code: 'acme.insurance.tl', path: ['notes'], params: { max: 3 } } as unknown as ValidationIssue
    expect(() => validation(forged)).toThrow(TypeError)
    expect(() => validation(made, forged)).toThrow(/Issue\.at/)
  })

  it('creates domain errors with the contract code and params', () => {
    const NotActive = domainError('acme.insurance.na', t.object({ status: t.string() }))
    const e = NotActive.of({ status: 'x' })
    expect(e).toMatchObject({ name: 'ContractError', kind: 'domain', code: 'acme.insurance.na', params: { status: 'x' }, issues: [] })
  })

  it('narrows client errors by contract', () => {
    const wire = new RemoteError({ kind: 'domain', code: 'acme.insurance.notActive', params: { status: 'expired' } })
    expect(isError(wire, c.NotActive)).toBe(true)
    expect(isError(wire, c.TooLong as never)).toBe(false)
    expect(isError(new Error('x'), c.NotActive)).toBe(false)
    expect(isError(new RemoteError({ kind: 'domain', code: 'acme.insurance.notActive', params: { status: 5 } }), c.NotActive)).toBe(false)
    expect(isError(new RemoteError({ kind: 'internal', code: 'acme.insurance.notActive' }), c.NotActive)).toBe(false)

    const invalid = new RemoteError({
      kind: 'validation',
      code: 'validation',
      issues: [
        { code: 'acme.insurance.tooLong', phase: 'input', source: 'a', path: ['notes'], params: { max: 10 } },
        { code: 'acme.insurance.other', phase: 'input', source: 'a', path: ['x'], params: {} },
        { code: 'acme.insurance.tooLong', phase: 'write', source: 'b', path: ['y'], params: { max: 'bad' } },
      ],
    })
    expect(issuesOf(invalid, c.TooLong)).toEqual([
      { code: 'acme.insurance.tooLong', phase: 'input', source: 'a', path: ['notes'], params: { max: 10 } },
    ])
    expect(issuesOf(wire, c.TooLong)).toEqual([])
    expect(issuesOf('nope', c.TooLong)).toEqual([])
  })
})

describe('11. filters', () => {
  it('keeps falsy filter values and operators', () => {
    expect(compileFilter({ flag: false, name: '', n: 0, tags: { isNull: false }, k: { in: ['a'] } })).toEqual({
      fields: { flag: { eq: false }, name: { eq: '' }, n: { eq: 0 }, tags: { isNull: false }, k: { in: ['a'] } },
    })
  })

  it('compiles system columns and ref paths', () => {
    expect(compileFilter({ key: { in: ['a', 'b'] }, createdAt: { gte: '2026-01-01T00:00:00Z' }, 'holder.updatedAt': { lt: '2026-02-01T00:00:00Z' } })).toEqual({
      fields: {
        key: { in: ['a', 'b'] },
        createdAt: { gte: '2026-01-01T00:00:00Z' },
        'holder.updatedAt': { lt: '2026-02-01T00:00:00Z' },
      },
    })
    expect(compileFilter({ owner: $.user }).fields).toEqual({ owner: { eq: { $: 'user' } } })
  })
})

describe('12. screen replacement', () => {
  it('names a replacement page in this package and points at the replaced route', () => {
    const ClientView = page(kit.ClientView, { component: (p) => p.params.id })
    const own = page(c.PolicyRoute, { component: (p) => p.params.id })
    const m = collectManifest({ ...ns, contracts: { ...kit, PolicyRoute: c.PolicyRoute }, client: { ClientView, own } })
    expect(m.resources.find((r) => r.kind === 'page' && r.export === 'ClientView')).toEqual({
      id: 'acme.insurance.clientView',
      kind: 'page',
      path: '/crm/clients/:id',
      route: 'crm.routes.clientView',
      export: 'ClientView',
    })
    expect(m.resources.find((r) => r.export === 'own')).toMatchObject({ id: 'acme.insurance.policyRoute.page', route: 'acme.insurance.policyRoute' })
    expect(m.resources.map((r) => r.id)).not.toContain('crm.routes.clientView')
  })

  it('does not auto-bind: two replacements of one route are two distinct candidates', () => {
    const A = page(kit.ClientView, { component: () => 'a' })
    const B = page(kit.ClientView, { component: () => 'b' })
    const m = collectManifest({ ...ns, contracts: { ...kit }, client: { A, B } })
    expect(m.resources.filter((r) => r.route === 'crm.routes.clientView').map((r) => r.id)).toEqual(['acme.insurance.a', 'acme.insurance.b'])
  })
})

describe('kit exports stay usable for generated modules', () => {
  it('nativeEntity and nativeList produce kit-issued refs', () => {
    const E = nativeEntity('crm.thing', { a: field.text() }, { key: 'uuid' })
    expect(E.foreign).toBe(true)
    expect(nativeList('crm.lists.X')).toEqual({ listId: 'crm.lists.X' })
    expect(() => column(nativeList('crm.lists.X'), { id: 'acme.insurance.x', field: c.ClientExtra.fields.points, label: { en: 'x' } })).not.toThrow()
    void command
  })
})

describe('slots and cells', () => {
  const Widgets = defineSlot('acme.insurance.widgets', { props: t.object({ title: t.string(), count: t.int().optional() }), permission: c.ReadPolicies })

  it('declares a slot in the contracts graph with its typed props and permission', () => {
    const m = collectManifest({ ...ns, contracts: { Widgets, ReadPolicies: c.ReadPolicies } })
    const decl = m.resources.find((r) => r.id === 'acme.insurance.widgets')!
    expect(decl).toMatchObject({ kind: 'slot', permission: c.ReadPolicies.id })
    expect(decl.target).toBeUndefined()
    expect(decl.props).toEqual({
      kind: 'object',
      properties: [{ name: 'title', type: { kind: 'string' } }, { name: 'count', type: { kind: 'int', optional: true } }],
    })
    expect(Widgets.slotId).toBe('acme.insurance.widgets')
  })

  it('keeps a declaration out of the client graph and a contribution out of the contracts graph', () => {
    expect(problems({ ...ns, client: { Widgets } })).toEqual([expect.stringMatching(/slot contract must be declared and exported from the contracts graph/)])
    const contribution = slot(Widgets, () => 'x')
    expect(problems({ ...ns, contracts: { contribution } })).toEqual([expect.stringMatching(/slot belongs to the client graph/)])
  })

  it('records order, permission and the target of a contribution, and lets a typed ref or an ID name the slot', () => {
    const late = slot(Widgets, () => 'x', { order: 20, permission: c.ReadPolicies })
    const early = slot('crm.slots.clientSidebar', () => 'x', { order: -5 })
    const plain = slot(Widgets, () => 'x', { id: 'acme.insurance.explicit' })
    const m = collectManifest({ ...ns, contracts: { Widgets, ReadPolicies: c.ReadPolicies }, client: { late, early, plain } })
    const byId = (id: string) => m.resources.find((r) => r.id === id)!
    expect(byId('acme.insurance.late')).toMatchObject({ kind: 'slot', target: 'acme.insurance.widgets', order: 20, permission: c.ReadPolicies.id, export: 'late' })
    expect(byId('acme.insurance.early')).toMatchObject({ target: 'crm.slots.clientSidebar', order: -5 })
    expect(byId('acme.insurance.early').permission).toBeUndefined()
    expect(byId('acme.insurance.explicit').order).toBeUndefined()
  })

  it('refuses a contribution whose in-package slot or column was never declared', () => {
    const orphan = slot('acme.insurance.nowhere', () => 'x')
    expect(problems({ ...ns, client: { orphan } })).toEqual([expect.stringMatching(/references "acme.insurance.nowhere" which is not exported by the contracts graph/)])
    const detached = slotRef('acme.insurance.ghost')
    const viaRef = slot(detached, () => 'x')
    expect(problems({ ...ns, client: { viaRef } })).toEqual([expect.stringMatching(/"acme.insurance.ghost"/)])
  })

  it('types the props of a contribution by the slot', () => {
    // compile-time only: the props of `slot(Widgets, …)` are { title: string; count?: number }
    const typed = slot(Widgets, (p) => `${p.title}:${p.count ?? 0}`)
    // @ts-expect-error `title` is a string
    slot(Widgets, (p) => p.title.toFixed())
    expect(typed.component({ title: 'a' })).toBe('a:0')
  })
})
