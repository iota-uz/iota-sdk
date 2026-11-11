// Negative type tests: every case must fail to compile (`@ts-expect-error` fails if it compiles).
// Not enforceable by types: a client-graph file importing a server value (`@iota-uz/sdk/runtime/server`
// or the server entry). The build slice rejects that edge with a source-located diagnostic.
import {
  ref, t, href, entity, field, overlay, column, nativeList,
  type Ref,
} from '../src/index.ts'
import { page } from '../src/client.ts'
import {
  guard, implement, precheck, subscribe, writeRule,
  type CommandContext, type QueryContext, type Tx, type WorkflowContext,
} from '../src/server.ts'
import * as kit from './kit.ts'
import * as c from './acme/contracts.ts'

declare const tx: Tx
declare const derived: Tx<'derived'>
declare const ctx: CommandContext
declare const q: QueryContext
declare const wf: WorkflowContext
declare const policyRef: Ref<typeof c.Policy>
declare const customerRef: Ref<typeof c.Customer>
declare const clientRef: Ref<typeof kit.Client>

// --- tx.call accepts only operations and atomic commands
// @ts-expect-error ordinary command inside tx.call
void tx.call(c.RenewPolicy, { policy: policyRef })
void tx.call(c.ExtendExpiry, { policy: policyRef, months: 1 })
void tx.call(c.ActivatePolicy, { policy: policyRef })
// @ts-expect-error ordinary native command inside tx.call
void tx.call(kit.SendInvoice, { client: clientRef })
// @ts-expect-error queries are not callable inside a transaction
void tx.call(c.Expiring, { before: '2026-01-01' })

// --- generic writes only for own entities
// @ts-expect-error native entity insert
void tx.insert(kit.Client, { name: 'x' })
// @ts-expect-error native entity update
void tx.update(kit.Client, clientRef, { name: 'x' })
// @ts-expect-error native entity delete
void tx.delete(kit.Client, clientRef)
// @ts-expect-error overlays are written with tx.put
void tx.insert(c.ClientExtra, { points: 1 })

// --- refs of different entities are not assignable
// @ts-expect-error Ref<Customer> where Ref<Policy> expected
void tx.get(c.Policy, customerRef)
// @ts-expect-error Ref<Customer> in an operation input
void tx.call(c.ExtendExpiry, { policy: customerRef, months: 1 })
// @ts-expect-error overlay base must be a ref of its base entity
void tx.get(c.ClientExtra, policyRef)
// @ts-expect-error update with a ref of another entity
void tx.update(c.Policy, customerRef, { notes: 'x' })

// --- overlay absence is explicit
async function overlayWithoutNullCheck(): Promise<number> {
  const extra = await tx.get(c.ClientExtra, clientRef)
  // @ts-expect-error extra may be null
  return extra.points
}
async function overlayWithNullCheck(): Promise<number> {
  const extra = await tx.get(c.ClientExtra, clientRef)
  return extra?.points ?? 0
}
void overlayWithoutNullCheck
void overlayWithNullCheck

// --- atomicity is part of the public contract
// @ts-expect-error atomic replacement typed as an ordinary handler
void implement(kit.RejectIncoming, async (ctx: CommandContext, input) => ({ rejected: input.reason === ctx.invocation.id }))
// @ts-expect-error ordinary replacement typed with Tx
void implement(kit.SendInvoice, async (tx: Tx) => ({ number: tx.invocation.id }))
// @ts-expect-error operation implemented with a CommandContext
void implement(c.ExtendExpiry, async (ctx: CommandContext) => ({ expiresOn: ctx.invocation.id }))
// @ts-expect-error query implemented with a Tx
void implement(c.Expiring, async (tx: Tx) => [])

// --- context shape
// @ts-expect-error Tx has no nested transaction
void tx.transaction(async () => 1)
// @ts-expect-error CommandContext cannot call operations
void ctx.call(c.ExtendExpiry, { policy: policyRef, months: 1 })
// @ts-expect-error QueryContext is read-only
void q.insert(c.Policy, {})
// @ts-expect-error workflow transaction callback tx is derived-key; plain operation needs explicit input
void wf.transaction('x', c.ExtendExpiry)

// --- guards bind to the right target kind
// @ts-expect-error precheck is for ordinary commands
void precheck('acme.insurance.bad1', c.ActivatePolicy, async () => {})
// @ts-expect-error operation guard is not for ordinary commands
void guard('acme.insurance.bad2', c.RenewPolicy, async () => {})
// @ts-expect-error write rules target entities, not overlays
void writeRule('acme.insurance.bad3', c.ClientExtra, async () => {})
// @ts-expect-error unknown field in `changed`
void writeRule('acme.insurance.bad4', c.Policy, async (_tx, { changed }) => { void changed.includes('nope') })

// --- start keys: required outside subscriptions and workflows
// @ts-expect-error key required in command transactions
void tx.start(c.RenewalFollowUp, { policy: policyRef })
void tx.start(c.RenewalFollowUp, { policy: policyRef }, { key: 'k' })
void derived.start(c.RenewalFollowUp, { policy: policyRef })
void subscribe('acme.insurance.okSub', c.PolicyRenewed, async (tx2, e) => { await tx2.start(c.RenewalFollowUp, { policy: e.subject }) })

// --- workflow results are JSON
// @ts-expect-error Date is not JSON
void wf.step('d', async () => new Date())
// @ts-expect-error bigint is not JSON
void wf.transaction('b', async () => 1n)
// @ts-expect-error class instances are not JSON
void wf.step('c', async () => new Map<string, string>())
void wf.step('ok', async () => ({ a: 1, b: ['x'], c: null }))

// --- payloads, params and issue params keep their types
// @ts-expect-error payload field with the wrong type
void tx.emit(c.PolicyRenewed, { subject: policyRef, payload: { expiresOn: 'soon', premium: 1, version: 1 } })
// @ts-expect-error subject of the wrong entity
void tx.emit(c.PolicyRenewed, { subject: customerRef, payload: { expiresOn: kit as never, premium: kit as never, version: kit as never } })
// @ts-expect-error missing recipients
void tx.notify(c.RenewalReminder, { subject: policyRef, payload: { number: 'n', expiresOn: kit as never }, channels: ['sms'] })
// @ts-expect-error unknown channel
void tx.notify(c.RenewalReminder, { subject: policyRef, payload: { number: 'n', expiresOn: kit as never }, to: [], channels: ['fax'] })
// @ts-expect-error route param with the wrong type
void href(c.PolicyRoute, { id: 5 })
// @ts-expect-error route has no such param
void page(c.PolicyRoute, { component: (p) => p.params.nope })
// @ts-expect-error issue params
void c.TooLong.at(['notes'], { max: 'ten' })
// @ts-expect-error domain error params
void c.NotActive.of({ status: 1 })
// @ts-expect-error int64 is a string on the wire, not a number
void t.int64().nullable() satisfies never

// --- writes are checked against field types
// @ts-expect-error missing required field
void tx.insert(c.Policy, { number: 'n' })
// @ts-expect-error unknown field
void tx.insert(c.Customer, { name: 'n', nope: 1 })
// @ts-expect-error enum literal
void tx.update(c.Policy, policyRef, { status: 'cancelled' })
// @ts-expect-error overlay put with an unknown value
void tx.put(c.ClientExtra, clientRef, { loyaltyTier: 'platinum' })

// --- filters are typed paths
// @ts-expect-error unknown path
void q.find(c.Policy, { where: { 'holder.nope': 'x' } })
// @ts-expect-error value type of a path
void q.find(c.Policy, { where: { 'holder.name': 5 } })
void q.find(c.Policy, { where: { 'holder.name': 'x', 'holder.owner': { isNull: true } } })

// --- declaration-time rules
// @ts-expect-error reserved field key is a runtime error but a path to a missing field is a type error
void c.ClientExtra.fields.nope
// @ts-expect-error columns take overlay fields, not a string
void column(nativeList('crm.lists.X'), { id: 'a.b', field: 'loyaltyTier', label: { en: 'x' } })

export { entity, field, overlay, ref }
