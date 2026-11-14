import { describe, expect, it } from 'vitest'
import { collectManifest, ManifestError } from '../src/manifest.ts'
import { command, entity, field, notice, operation, t } from '../src/index.ts'
import { foreign } from '../src/kit.ts'
import { endpoint, guard, implement, precheck, schedule, templates } from '../src/server.ts'
import { page, navigation, shellRenderer } from '../src/client.ts'
import * as contracts from '../typetests/acme/contracts.ts'
import * as server from '../typetests/acme/server.ts'
import * as client from '../typetests/acme/client.ts'

const base = { namespace: 'acme.insurance', contracts, server, client }

const problems = (input: Parameters<typeof collectManifest>[0]): readonly string[] => {
  try {
    collectManifest(input)
  } catch (e) {
    expect(e).toBeInstanceOf(ManifestError)
    return (e as ManifestError).problems
  }
  throw new Error('expected collectManifest to throw')
}

describe('collectManifest', () => {
  const manifest = collectManifest({ ...base, capabilities: { network: ['sms.example.uz'], secrets: ['SMS_TOKEN'] } })

  it('emits the manifest envelope', () => {
    expect(manifest.schema).toBe('iota.manifest/1')
    expect(manifest.namespace).toBe('acme.insurance')
    expect(manifest.capabilities).toEqual({ network: ['sms.example.uz'], secrets: ['SMS_TOKEN'] })
    const ids = manifest.resources.map((r) => r.id)
    expect(ids).toEqual([...ids].sort((a, b) => a.localeCompare(b)))
    expect(ids).not.toContain('core.user')
    expect(ids).not.toContain('core.commands.rejectIncoming')
  })

  it('serializes entities with Go omitempty semantics', () => {
    const policy = manifest.resources.find((r) => r.id === 'acme.insurance.policy')!
    expect(policy.kind).toBe('entity')
    expect(policy.fields?.[0]).toEqual({ id: 'number', type: { kind: 'string' }, unique: true })
    expect(policy.fields?.find((f) => f.id === 'status')).toEqual({
      id: 'status',
      type: { kind: 'enum', values: ['draft', 'active', 'expired'] },
      default: 'draft',
    })
    expect(policy.fields?.find((f) => f.id === 'notes')?.type).toEqual({ kind: 'string', optional: true, nullable: true })
    expect(policy.access?.read).toEqual([
      { permission: 'acme.insurance.readPolicies', all: true },
      { rows: { fields: { 'holder.owner': { eq: { $: 'user' } } } } },
    ])
    expect(JSON.stringify(policy)).not.toMatch(/"(unique|index)":false/)
  })

  it('serializes commands, operations, overlays, columns and events', () => {
    const find = (id: string) => manifest.resources.find((r) => r.id === id)!
    expect(find('acme.insurance.renewPolicy')).toMatchObject({ kind: 'command', errors: ['acme.insurance.notActive'], permission: 'acme.insurance.readPolicies' })
    expect(find('acme.insurance.renewPolicy').atomic).toBeUndefined()
    expect(find('acme.insurance.activatePolicy').atomic).toBe(true)
    expect(find('acme.insurance.extendExpiry').kind).toBe('operation')
    expect(find('acme.insurance.clientExtra')).toMatchObject({ kind: 'overlay', base: 'crm.client' })
    expect(find('acme.insurance.clientsTier')).toEqual({
      id: 'acme.insurance.clientsTier', kind: 'column', label: { en: 'Tier', ru: 'Уровень', uz: 'Daraja' },
      list: 'crm.lists.Clients', overlay: 'acme.insurance.clientExtra', field: 'loyaltyTier',
      filterable: true, sortable: true, exportable: true,
    })
    expect(find('acme.insurance.policyRenewed.v1')).toMatchObject({ kind: 'event', subject: 'acme.insurance.policy' })
    expect(find('acme.insurance.renewalFollowUp').signals).toEqual({
      decision: { kind: 'object', properties: [{ name: 'approved', type: { kind: 'bool' } }] },
    })
    expect(find('acme.insurance.customerContacts')).toMatchObject({ kind: 'contactResolver', subject: 'acme.insurance.customer' })
    expect(find('acme.insurance.customerConsent').kind).toBe('deliveryPolicy')
  })

  it('serializes client resources with export names and implicit ids', () => {
    const find = (id: string) => manifest.resources.find((r) => r.id === id)!
    expect(find('acme.insurance.policies')).toMatchObject({ kind: 'page', path: '/insurance/policies', export: 'policies', permission: 'acme.insurance.readPolicies' })
    expect(find('acme.insurance.policyRoute.page')).toMatchObject({ kind: 'page', route: 'acme.insurance.policyRoute', path: '/insurance/policies/:id' })
    expect(manifest.resources.filter((r) => r.kind === 'page').map((r) => r.id)).toContain('acme.insurance.policyRoute.page')
    expect(find('acme.insurance.navigation')).toMatchObject({ kind: 'navigation', patches: [{ ref: 'core.users', hide: true }] })
    expect(find('acme.insurance.navigation').export).toBeUndefined()
    expect(find('acme.insurance.shell')).toMatchObject({ kind: 'shell', export: 'shell' })
    expect(find('acme.insurance.tierCell')).toMatchObject({ kind: 'cell', target: 'acme.insurance.clientsTier', export: 'tierCell' })
    expect(find('acme.insurance.clientBadge')).toMatchObject({ kind: 'slot', target: 'crm.slots.clientToolbar' })
  })

  it('records implementations, including replacements of foreign contracts', () => {
    expect(manifest.implementations).toEqual(
      expect.arrayContaining([
        { export: 'extendExpiry', implements: 'acme.insurance.extendExpiry', kind: 'operation' },
        { export: 'renew', implements: 'acme.insurance.renewPolicy', kind: 'command' },
        { export: 'activate', implements: 'acme.insurance.activatePolicy', kind: 'atomic' },
        { export: 'expiring', implements: 'acme.insurance.expiring', kind: 'query' },
        { export: 'renewalFollowUp', implements: 'acme.insurance.renewalFollowUp', kind: 'workflow' },
        { export: 'rejectIncoming', implements: 'core.commands.rejectIncoming', kind: 'atomic' },
        { export: 'sendInvoice', implements: 'core.commands.sendInvoice', kind: 'command' },
        { export: 'customerContacts', implements: 'acme.insurance.customerContacts', kind: 'contactResolver' },
      ]),
    )
  })

  it('records subscriptions, guards with phases, templates, schedules and endpoints', () => {
    expect(manifest.subscriptions).toEqual([{ id: 'acme.insurance.onRenewed', export: 'onRenewed', event: 'acme.insurance.policyRenewed.v1' }])
    expect(manifest.guards).toEqual(
      expect.arrayContaining([
        { id: 'acme.insurance.renewPrecheck', export: 'renewPrecheck', target: 'acme.insurance.renewPolicy', phase: 'input' },
        { id: 'acme.insurance.extendGuard', export: 'extendGuard', target: 'acme.insurance.extendExpiry', phase: 'operation' },
        { id: 'acme.insurance.rejectGuard', export: 'rejectGuard', target: 'core.commands.rejectIncoming', phase: 'operation' },
        { id: 'acme.insurance.notesRule', export: 'notesRule', target: 'acme.insurance.policy', phase: 'write' },
      ]),
    )
    expect(manifest.templates).toEqual([
      { export: 'reminderTemplates', notice: 'acme.insurance.renewalReminder.v1', channels: ['sms', 'email', 'inApp', 'push'] },
    ])
    expect(manifest.schedules).toEqual([{ id: 'acme.insurance.nightly', export: 'nightly', cron: '0 3 * * *', timezone: 'Asia/Tashkent' }])
    expect(manifest.endpoints).toEqual([
      { id: 'acme.insurance.paymentWebhook', export: 'paymentWebhook', method: 'POST', path: '/insurance/webhook', public: true },
    ])
    expect(JSON.stringify(manifest)).not.toContain('contractHash')
  })

  it('is deterministic', () => {
    expect(JSON.stringify(collectManifest(base))).toBe(JSON.stringify(collectManifest(base)))
  })
})

describe('collectManifest errors', () => {
  const ns = { namespace: 'acme.insurance', server: {}, client: {} }

  it('rejects ids outside the namespace', () => {
    const Other = entity('other.thing', { a: field.text() })
    expect(problems({ ...ns, contracts: { Other } })).toEqual([expect.stringMatching(/outside namespace "acme.insurance"/)])
  })

  it('rejects duplicate ids', () => {
    const A = entity('acme.insurance.same', { a: field.text() })
    const B = operation('acme.insurance.same', { input: t.object({}) })
    expect(problems({ ...ns, contracts: { A, B } })).toEqual([expect.stringMatching(/"acme.insurance.same" is already declared by contracts export "A"/)])
  })

  it('rejects ids shared between server values and resources', () => {
    const A = entity('acme.insurance.same', { a: field.text() })
    const s = schedule('acme.insurance.same', '* * * * *', async () => {})
    expect(problems({ ...ns, contracts: { A }, server: { s } })).toEqual([expect.stringMatching(/already declared/)])
  })

  it('rejects implementing a contract kind that is not implementable', () => {
    const E = entity('acme.insurance.e', { a: field.text() })
    const bad = implement(E as never, (async () => {}) as never)
    expect(problems({ ...ns, contracts: { E }, server: { bad } })).toEqual([expect.stringMatching(/cannot implement contract "acme.insurance.e" of kind "entity"/)])
  })

  it('rejects two exports implementing one contract', () => {
    const Op = operation('acme.insurance.op', { input: t.object({}) })
    const one = implement(Op, async () => {})
    const two = implement(Op, async () => {})
    expect(problems({ ...ns, contracts: { Op }, server: { one, two } })).toEqual([expect.stringMatching(/already implemented by export "one"/)])
  })

  it('rejects a local contract that the contracts graph does not export', () => {
    const Hidden = operation('acme.insurance.hidden', { input: t.object({}) })
    const impl = implement(Hidden, async () => {})
    expect(problems({ ...ns, contracts: {}, server: { impl } })).toEqual([expect.stringMatching(/not exported by the contracts graph/)])
  })

  it('allows foreign contracts to be implemented and guarded', () => {
    const Native = foreign(command('core.commands.x', { atomic: true, input: t.object({}) }))
    const impl = implement(Native, async () => {})
    const g = guard('acme.insurance.g', Native, async () => {})
    const manifest = collectManifest({ ...ns, contracts: {}, server: { impl, g } })
    expect(manifest.implementations).toEqual([{ export: 'impl', implements: 'core.commands.x', kind: 'atomic' }])
    expect(manifest.resources).toEqual([])
  })

  it('rejects guards on the wrong target kind and unknown cron', () => {
    const Atomic = command('acme.insurance.atomic', { atomic: true, input: t.object({}) })
    const bad = precheck('acme.insurance.bad', Atomic as never, async () => {})
    const cron = schedule('acme.insurance.cron', 'not a cron', async () => {})
    expect(problems({ ...ns, contracts: { Atomic }, server: { bad, cron } })).toEqual([
      expect.stringMatching(/input guard cannot target command/),
      expect.stringMatching(/invalid cron/),
    ])
  })

  it('rejects values exported from the wrong graph', () => {
    const A = entity('acme.insurance.a', { a: field.text() })
    const impl = implement(operation('acme.insurance.o', { input: t.object({}) }), async () => {})
    const p = page('acme.insurance.p', { path: '/p', component: () => 'p' })
    const found = problems({ ...ns, contracts: { impl, p }, server: { A }, client: { A } })
    expect(found).toEqual(
      expect.arrayContaining([
        expect.stringMatching(/contracts export "impl": server value exported from the contracts graph/),
        expect.stringMatching(/contracts export "p": page belongs to the client graph/),
        expect.stringMatching(/server export "A": contract exported from the server graph/),
        expect.stringMatching(/client export "A": entity contract must be declared/),
      ]),
    )
  })

  it('rejects references to undeclared entities of the own namespace', () => {
    const Missing = entity('acme.insurance.missing', { a: field.text() })
    const Op = operation('acme.insurance.refs', { input: t.object({ m: t.ref(Missing) }) })
    expect(problems({ ...ns, contracts: { Op } })).toEqual([expect.stringMatching(/references "acme.insurance.missing"/)])
  })

  it('rejects duplicate templates, templates without channels and bad endpoint paths', () => {
    const N = notice('acme.insurance.n', { payload: t.object({}) })
    const one = templates(N, { sms: () => 'x' })
    const two = templates(N, { email: () => ({ subject: 's', html: 'h' }) })
    const none = templates(notice('acme.insurance.n2', { payload: t.object({}) }), {})
    const ep = endpoint('acme.insurance.ep', { method: 'GET', path: 'no-slash' }, async () => ({ status: 200 }))
    expect(problems({ ...ns, contracts: { N }, server: { one, two, ep, none } })).toEqual(
      expect.arrayContaining([
        expect.stringMatching(/already has templates in export "one"/),
        expect.stringMatching(/endpoint path must start with "\/"/),
        expect.stringMatching(/define no channel/),
      ]),
    )
  })

  it('names implicit client ids and rejects clashes', () => {
    const nav1 = navigation({})
    const nav2 = navigation({})
    expect(problems({ ...ns, contracts: {}, client: { nav1, nav2 } })).toEqual([expect.stringMatching(/"acme.insurance.navigation" is already declared/)])
    const shell = shellRenderer(() => 'x')
    expect(collectManifest({ ...ns, contracts: {}, client: { shell } }).resources).toEqual([
      { id: 'acme.insurance.shell', kind: 'shell', export: 'shell' },
    ])
  })

  it('rejects an invalid namespace and a reserved-key entity', () => {
    expect(problems({ ...ns, namespace: 'Acme', contracts: {} })).toEqual([expect.stringMatching(/invalid namespace/)])
    expect(() => entity('acme.insurance.r', { createdAt: field.text() })).toThrow(/reserved/)
    expect(() => entity('acme.insurance.r', { key: field.text() })).toThrow(/reserved/)
  })
})
