// Type tests for the authoring-API audit. Every `@ts-expect-error` must fail to compile for the stated reason;
// positive lines must compile. Equality helpers make the inferred types exact, not just assignable.
import * as pub from '../src/index.ts'
import {
  $, ContractError, entity, field, href, issue, overlay, ref, route, t, validation, workflow, domainError,
  type Infer, type Json, type Ref, type Row, type UserRef, type ValidationIssue,
} from '../src/index.ts'
import { isError, issuesOf, page, RemoteError, type CallError } from '../src/client.ts'
import {
  endpoint, guard, implement, precheck, recipient, writeRule,
  type CommandContext, type EndpointRequest, type GuardContext, type QueryContext, type Tx, type WorkflowContext,
  type WorkflowInstance,
} from '../src/server.ts'
import * as kit from './kit.ts'
import * as c from './acme/contracts.ts'
import * as acmeServer from './acme/server.ts'
import type { AnyGuard } from '../src/server.ts'

type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false
function assertType<_T extends true>(): void {}

declare const tx: Tx
declare const plainTx: Tx<'plain'>
declare const ctx: CommandContext
declare const q: QueryContext
declare const wf: WorkflowContext
declare const policyRef: Ref<typeof c.Policy>
declare const customerRef: Ref<typeof c.Customer>
declare const userRef: UserRef
declare const clientRef: Ref<typeof kit.Client>

// ---- 1. contracts cannot be forged by ID from the public surface
// @ts-expect-error `foreign` is only available from the kit entry
void pub.foreign
// @ts-expect-error nativeEntity is kit-only
void pub.nativeEntity
// @ts-expect-error nativeList is kit-only
void pub.nativeList
// @ts-expect-error the raw contract constructor is internal
void pub.contract

// ---- 3. Tx<'plain'> is not assignable to Tx<'derived'>
// @ts-expect-error a keyless start cannot be smuggled through assignment
const derivedFromPlain: Tx<'derived'> = plainTx
void derivedFromPlain
declare const derivedTx: Tx<'derived'>
const plainFromDerived: Tx<'plain'> = derivedTx
void plainFromDerived

// ---- 4. refs
async function refs(): Promise<void> {
  const row = await tx.insert(c.Policy, { number: 'n', holder: customerRef, premium: kit as never, expiresOn: kit as never })
  assertType<Equal<typeof row.ref, Ref<typeof c.Policy>>>()
  await tx.update(c.Policy, row.ref, { notes: 'x' })
  const found = await q.find(c.Customer)
  await tx.get(c.Customer, found[0]!.ref)
  // @ts-expect-error a Policy row's ref is not a Customer ref
  await tx.get(c.Customer, row.ref)
  const overlayRow = await tx.put(c.ClientExtra, clientRef, { points: 1 })
  // @ts-expect-error overlay rows have no ref (they are keyed by baseKey)
  void overlayRow.ref
}
void refs
// @ts-expect-error only core.user refs are user recipients
void recipient.user(customerRef)
void recipient.user(userRef)
void recipient.user(ref(kit.User, '42'))
// @ts-expect-error only core.user refs are task assignees
void wf.task('t', { subject: policyRef, assignee: customerRef, decision: t.object({}) })
void wf.task('t', { subject: policyRef, assignee: userRef, decision: t.object({}) })
void wf.task('t', { subject: policyRef, assignee: wf.user, decision: t.object({}) })
// $.user only matches refs to core.user
void q.find(c.Customer, { where: { owner: $.user } })
// @ts-expect-error holder is a ref to Customer, not to core.user
void q.find(c.Policy, { where: { holder: $.user } })
// @ts-expect-error a plain string field cannot be compared with $.user
void q.find(c.Policy, { where: { number: $.user } })
// @ts-expect-error commands are not entities: no t.ref to a command
void t.ref(c.RenewPolicy)
// @ts-expect-error overlays are not referenceable entities
void field.ref(c.ClientExtra)
void t.ref(kit.User)
// @ts-expect-error a union of entities: the ref could belong to either branch
void tx.get(Math.random() > 0.5 ? c.Policy : c.Customer, policyRef)
// @ts-expect-error same for writes
void tx.update(Math.random() > 0.5 ? c.Policy : c.Customer, customerRef, {})
void tx.get(c.Policy, policyRef)

// ---- 5. JSON results
// @ts-expect-error t.json<T> requires JSON
void t.json<Date>()
// @ts-expect-error nor a bigint
void field.json<bigint>()
void t.json<{ a: number[] }>()
interface Receipt {
  id: string
  lines: { sku: string; qty: number }[]
  note?: string | null
}
void t.json<Receipt>
void wf.step('iface', async (): Promise<Receipt> => ({ id: 'x', lines: [], note: null }))
void wf.transaction('iface', async (): Promise<Receipt> => ({ id: 'x', lines: [] }))
// @ts-expect-error Date is not JSON (the message names the problem)
void wf.step('d', async () => new Date())
// @ts-expect-error a function is not JSON
void wf.transaction('f', async () => () => 1)
// @ts-expect-error output schemas must be JSON-valued too
void wf.step('o', { output: t.object({ d: t.json<Date>() }) }, async () => ({ d: new Date() }))
void wf.step('o', { output: t.object({ n: t.int(), r: t.ref(c.Policy) }) }, async () => ({ n: 1, r: policyRef }))
void wf.transaction('rows', async (tx2) => tx2.find(c.Policy))
const jsonValue: Json = { a: [1, null] }
void jsonValue

// ---- 6. optional / nullable / default semantics match the wire
const Probe = entity('acme.insurance.probe', {
  req: field.text(),
  nul: field.text().nullable(),
  opt: field.text().optional(),
  def: field.int().default(0),
  optDef: field.int().optional().default(5),
  nulDef: field.int().nullable().default(null),
  flag: field.bool().default(false),
})
type ProbeRow = Row<typeof Probe>
assertType<Equal<ProbeRow['req'], string>>()
assertType<Equal<ProbeRow['nul'], string | null>>()
assertType<Equal<ProbeRow['opt'], string | null>>()
assertType<Equal<ProbeRow['def'], number>>()
assertType<Equal<ProbeRow['optDef'], number | null>>()
assertType<Equal<ProbeRow['nulDef'], number | null>>()
assertType<Equal<ProbeRow['flag'], boolean>>()
assertType<Equal<Infer<ReturnType<typeof field.text>>, string>>()
// the optional field accepts null, a value or nothing on input; the required one must be given
void tx.insert(Probe, { req: 'a', nul: null })
void tx.insert(Probe, { req: 'a', nul: 'x', opt: null, def: 1, optDef: null, nulDef: 2, flag: true })
// @ts-expect-error nullable (not optional) columns must be written explicitly
void tx.insert(Probe, { req: 'a' })
// @ts-expect-error a defaulted, non-nullable column does not take null
void tx.insert(Probe, { req: 'a', nul: null, def: null })
// @ts-expect-error default values are typed against the column
void field.int().default('0')
// the input type of an optional property inside an object schema
const Opt = t.object({ a: t.string().optional(), b: t.string().nullable() })
assertType<Equal<Infer<typeof Opt>, { a?: string; b: string | null }>>()

// ---- 7. t.row / t.fields
const PolicyRow = t.row(c.Policy)
assertType<Equal<Infer<typeof PolicyRow>, Row<typeof c.Policy>>>()
const CustomerName = t.fields(c.Customer, ['name'])
assertType<Equal<Infer<typeof CustomerName>, { readonly name: string }>>()
// @ts-expect-error picking a field the entity does not have
void t.fields(c.Customer, ['nope'])
void implement(c.GetPolicy, async (tx2, input) => tx2.get(c.Policy, input.policy))
// @ts-expect-error the row has to be complete
void implement(c.GetPolicy, async () => ({ key: 'k' }))

// ---- 8. routes: path params and params schema agree
void route('acme.insurance.okRoute', { path: '/a/:id/b/:n', params: t.object({ id: t.uuid(), n: t.int() }) })
void route('acme.insurance.noParams', { path: '/a/b', params: t.object({}) })
// @ts-expect-error `n` is a :segment but has no schema
void route('acme.insurance.r1', { path: '/a/:id/:n', params: t.object({ id: t.uuid() }) })
// @ts-expect-error `extra` is not a :segment of the path
void route('acme.insurance.r2', { path: '/a/:id', params: t.object({ id: t.uuid(), extra: t.string() }) })
// @ts-expect-error refs cannot travel in a URL segment
void route('acme.insurance.r3', { path: '/a/:id', params: t.object({ id: t.ref(c.Policy) }) })
// @ts-expect-error nor objects
void route('acme.insurance.r4', { path: '/a/:id', params: t.object({ id: t.object({}) }) })
// @ts-expect-error nor optional params
void route('acme.insurance.r5', { path: '/a/:id', params: t.object({ id: t.string().optional() }) })
// @ts-expect-error nor nullable ones
void route('acme.insurance.r6', { path: '/a/:id', params: t.object({ id: t.string().nullable() }) })
void href(c.PolicyRoute, { id: '123e4567-e89b-42d3-a456-426614174000' })
void page('acme.insurance.byPath', { path: '/x/:slug', component: (p) => p.params.slug })
// @ts-expect-error pages declared by path know their :params
void page('acme.insurance.byPath2', { path: '/x/:slug', component: (p) => p.params.nope })

// ---- 10. errors
// @ts-expect-error validation takes issues made by Issue.at, not literals
void validation({ code: 'acme.insurance.tooLong', path: ['x'], params: { max: 1 } })
// @ts-expect-error a literal has no issue brand
const literal: ValidationIssue = { code: 'x', path: [], params: null }
void literal
void validation(c.TooLong.at(['notes'], { max: 1 }), c.TooLong.at(['other'], { max: 2 }))
// @ts-expect-error ContractError cannot be constructed by authors
void new ContractError('domain', 'x', null, [])
// the hooks' error type narrows the same way
function hookErrors(error: CallError | undefined): void {
  if (isError(error, c.NotActive)) {
    const status: string = error.body.params.status
    void status
  }
  for (const i of issuesOf(error, c.TooLong)) void (i.params.max satisfies number)
}
void hookErrors
async function clientGuards(err: unknown): Promise<void> {
  if (isError(err, c.NotActive)) {
    const status: string = err.body.params.status
    // @ts-expect-error params keep their declared type
    const wrong: number = err.body.params.status
    void [status, wrong, err.body.code satisfies 'acme.insurance.notActive']
  }
  for (const i of issuesOf(err, c.TooLong)) {
    const max: number = i.params.max
    // @ts-expect-error the issue is TooLong, whose params have no `min`
    void i.params.min
    void [max, i.code satisfies 'acme.insurance.tooLong']
  }
  void RemoteError
}
void clientGuards
void issue
void domainError

// ---- 11. filters
void q.find(c.Customer, { where: { name: { like: 'A%' } } })
// @ts-expect-error like is for string fields only
void q.find(c.Policy, { where: { expiresOn: { like: '2026%' } } })
// @ts-expect-error like on an enum
void q.find(c.Policy, { where: { status: { like: 'a%' } } })
// @ts-expect-error uuid / ref fields have no range
void q.find(c.Policy, { where: { holder: { lt: customerRef } } })
// @ts-expect-error enums are not orderable
void q.find(c.Policy, { where: { status: { gt: 'draft' } } })
void q.find(c.Policy, { where: { expiresOn: { gte: '2026-01-01' as never, lt: '2027-01-01' as never }, premium: { gt: '1.00' as never } } })
// @ts-expect-error json fields cannot be filtered
void q.find(c.Policy, { where: { extra: { isNull: true } } })
// @ts-expect-error json fields cannot be ordered
void q.find(c.Policy, { orderBy: [{ field: 'extra' }] })
// system columns
void q.find(c.Policy, { where: { key: 'k' } })
void q.find(c.Policy, { where: { key: { in: ['a', 'b'] } } })
// @ts-expect-error key supports equality and in only
void q.find(c.Policy, { where: { key: { like: 'a%' } } })
// @ts-expect-error nor ranges
void q.find(c.Policy, { where: { key: { gt: 'a' } } })
void q.find(c.Policy, { where: { createdAt: { gte: '2026-01-01T00:00:00Z' as never }, 'holder.updatedAt': { lt: '2026-01-01T00:00:00Z' as never } } })
void q.find(c.Policy, { orderBy: [{ field: 'createdAt', desc: true }, { field: 'updatedAt' }, { field: 'holder.name' }, { field: 'premium' }] })
// @ts-expect-error key is not an order column
void q.find(c.Policy, { orderBy: [{ field: 'key' }] })
// @ts-expect-error nor are enums
void q.find(c.Policy, { orderBy: [{ field: 'status' }] })

// ---- 12. screen replacement from a kit route
const replacement = page(kit.ClientView, { component: (p) => p.params.id })
void replacement
// @ts-expect-error the kit route knows its params
void page(kit.ClientView, { component: (p) => p.params.nope })

// ---- 13. queries from server code
async function queries(): Promise<void> {
  const viaTx = await tx.query(c.Expiring, { before: '2026-01-01' as never })
  const viaQ = await q.query(c.Expiring, { before: '2026-01-01' as never })
  const viaCtx = await ctx.query(c.Expiring, { before: '2026-01-01' as never })
  const snapshot = await ctx.query.get(c.Policy, policyRef)
  assertType<Equal<typeof viaTx, typeof viaQ>>()
  assertType<Equal<typeof viaQ, typeof viaCtx>>()
  assertType<Equal<(typeof viaTx)[number]['policy'], Ref<typeof c.Policy>>>()
  void snapshot
  // @ts-expect-error the input is typed by the query contract
  await tx.query(c.Expiring, { before: 1 })
  // @ts-expect-error only queries
  await q.query(c.ExtendExpiry, { policy: policyRef, months: 1 })
}
void queries

// ---- 14. signals and instances
async function signals(): Promise<void> {
  const instance = await tx.start(c.RenewalFollowUp, { policy: policyRef }, { key: 'k' })
  assertType<Equal<typeof instance, WorkflowInstance<typeof c.RenewalFollowUp>>>()
  const id: string = instance.instance
  await tx.signal(instance, 'decision', { approved: true })
  await ctx.signal(instance, 'decision', { approved: false })
  // @ts-expect-error unknown signal name
  await tx.signal(instance, 'nope', { approved: true })
  // @ts-expect-error payload is typed by the workflow contract
  await tx.signal(instance, 'decision', { approved: 'yes' })
  const other = workflow('acme.insurance.other', { input: t.object({}), signals: { ping: t.object({ n: t.int() }) } })
  const otherInstance = await tx.start(other, {}, { key: 'o' })
  // @ts-expect-error `decision` belongs to another workflow
  await ctx.signal(otherInstance, 'decision', { approved: true })
  await ctx.signal(otherInstance, 'ping', { n: 1 })
  void id
}
void signals

// ---- 15. secrets and log
async function secretsAndLog(): Promise<void> {
  const token: string = await wf.secrets.get('X')
  tx.log.info('x', { a: 1 })
  ctx.log.warn('x')
  q.log.error('x')
  wf.log.debug('x')
  void token
}
void secretsAndLog

// ---- 16. endpoint request
declare const request: EndpointRequest
const bytes: Uint8Array = request.rawBody
const textBody: string = request.text()
void [bytes, textBody, request.body]
void endpoint

// ---- 17. guards are read-only
void guard('acme.insurance.ro1', c.ExtendExpiry, async (g) => {
  await g.get(c.Policy, policyRef)
  await g.find(c.Policy)
  await g.count(c.Policy)
  await g.query(c.Expiring, { before: '2026-01-01' as never })
  // @ts-expect-error no insert in a guard
  await g.insert(c.Policy, {})
  // @ts-expect-error no update
  await g.update(c.Policy, policyRef, {})
  // @ts-expect-error no delete
  await g.delete(c.Policy, policyRef)
  // @ts-expect-error no overlay writes
  await g.put(c.ClientExtra, clientRef, {})
  // @ts-expect-error no emit
  await g.emit(c.PolicyRenewed, { subject: policyRef, payload: kit as never })
  // @ts-expect-error no calls
  await g.call(c.ExtendExpiry, { policy: policyRef, months: 1 })
  // @ts-expect-error no start
  await g.start(c.RenewalFollowUp, { policy: policyRef }, { key: 'k' })
})
void writeRule('acme.insurance.ro2', c.Policy, async (g, { proposed }) => {
  void proposed?.ref
  // @ts-expect-error no writes in a write rule
  await g.insert(c.Policy, {})
})
void precheck('acme.insurance.ro3', c.RenewPolicy, async (g) => {
  // @ts-expect-error no writes in a precheck
  await g.update(c.Policy, policyRef, {})
})
const guardContext: GuardContext = q
void guardContext

// ---- 18. readable mistakes
// @ts-expect-error "ordinary command: use ctx.call outside a transaction"
void tx.call(c.RenewPolicy, { policy: policyRef })
// @ts-expect-error "query: use tx.query(Q, input)"
void tx.call(c.Expiring, { before: '2026-01-01' as never })
// @ts-expect-error "operation: call it inside ctx.transaction(tx => tx.call(Op, input))"
void ctx.call(c.ExtendExpiry, { policy: policyRef, months: 1 })
// @ts-expect-error "operation: use wf.transaction(name, Op, input)"
void wf.call('x', c.ExtendExpiry, { policy: policyRef, months: 1 })
// @ts-expect-error ordinary commands cannot run in a workflow transaction
void wf.transaction('x', c.RenewPolicy, { policy: policyRef })
void wf.transaction('x', c.ExtendExpiry, { policy: policyRef, months: 1 })
// handlers need not be async
void implement(c.ExtendExpiry, () => ({ expiresOn: '2026-01-01' as never }))
void implement(c.Expiring, () => [])
void writeRule('acme.insurance.sync', c.Policy, () => {})
// other wrong shapes stay normal type errors
// @ts-expect-error the returned object must match the output
void implement(c.ExtendExpiry, () => ({ expires: 1 }))

export { overlay, ContractError }

// any guard, whatever its target, fits a list of guards
{
  const guards: readonly AnyGuard[] = [acmeServer.notesRule, acmeServer.extendGuard, acmeServer.rejectGuard]
  void guards
}
