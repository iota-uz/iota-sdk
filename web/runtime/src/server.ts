// Server graph: implementations and the context interfaces the Node worker host provides.
// This file is pure: no Node, DOM or solid imports. Handlers never reach the client graph.
import type {
  AnyEntity, AnyOverlay, Hint, InsertOf, OrderBy, OwnEntity, PatchOf, RecordOf, RefTo, Row, Where,
} from './entity.ts'
import type {
  AtomicCommand, Callable, Command, ContactResolver, ContactTargets, DeliveryPolicy, Event, InputOf, Notice, Operation,
  OrdinaryCommand, Query, Workflow, SignalRef, ResultOf, OutOf,
} from './contracts.ts'
import type { Channel, Int64, ISODateTime, Json, JsonOf } from './model.ts'
import { RESOURCE } from './resource.ts'
import type { AnySchema, Infer, Ref, UserRef } from './schema.ts'

// ---------------------------------------------------------------- context interfaces

export type TriggerKind = 'ui' | 'mcp' | 'endpoint' | 'schedule' | 'subscription' | 'workflow'

/** A handler may return its result directly; `async` is the norm but never required. */
export type Awaitable<T> = T | Promise<T>

/** The identity an invocation acts as: the initiating user, or the trigger's explicit owner-user. */
export interface Principal {
  readonly tenantId: string
  readonly userId: Int64
  readonly kind: 'initiator' | 'owner'
  readonly locale?: string
  has(permission: string | { readonly id: string }): boolean
}

export interface InvocationInfo {
  readonly id: string
  readonly trigger: TriggerKind
  readonly parentId?: string
}

export interface Logger {
  debug(message: string, fields?: Readonly<Record<string, Json>>): void
  info(message: string, fields?: Readonly<Record<string, Json>>): void
  warn(message: string, fields?: Readonly<Record<string, Json>>): void
  error(message: string, fields?: Readonly<Record<string, Json>>): void
}

/** Secrets declared in the package capabilities; resolved by the host on demand. */
export interface Secrets {
  get(name: string): Promise<string>
}

export interface FindQuery<E> {
  /** Field keys, system columns or dotted ref paths such as "holder.owner". Access rules apply before filter, count and paging. */
  where?: Where<E>
  /** Orderable fields and `createdAt` / `updatedAt` (also through refs). */
  orderBy?: readonly OrderBy<E>[]
  /** Default 100, maximum 1000. */
  limit?: number
  offset?: number
}

/** Read-only typed data access. */
export interface Reader {
  get<E extends AnyEntity>(entity: E, ref: RefTo<E>): Promise<RecordOf<E> | null>
  /** Absence of an overlay row is explicit: the result is `null`, never a silent default object. */
  get<O extends AnyOverlay>(overlay: O, base: RefTo<O['base']>): Promise<RecordOf<O> | null>
  find<E extends AnyEntity>(entity: E, query?: FindQuery<E>): Promise<RecordOf<E>[]>
  count<E extends AnyEntity>(entity: E, where?: Where<E>): Promise<number>
}

/**
 * Everything a read-only handler (query, guard, write rule, precheck) may use: reads, other queries, identity and log.
 * There is no write API; a guard rejects by throwing `validation(Issue.at(...))` or a declared domain error.
 */
export interface ReadContext extends Reader {
  readonly user: Principal
  readonly invocation: InvocationInfo
  readonly log: Logger
  /** Runs another query in the same snapshot (or transaction) with the caller's rights. */
  query<Q extends Query>(query: Q, input: InputOf<Q>): Promise<Infer<Q['output']>>
}

export type QueryContext = ReadContext
/** Context of `precheck`, `guard` and `writeRule` handlers: reads only. */
export type GuardContext = ReadContext

type SubjectArg<S> = S extends AnyEntity ? { subject: Ref<S> } : { subject?: undefined }
export type EmitArgs<Ev extends AnyEvent> = { payload: Infer<Ev['payload']> } & SubjectArg<Ev['subject']>

export type Recipient =
  | { readonly kind: 'user'; readonly user: UserRef }
  | { readonly kind: 'record'; readonly record: Ref }
  | { readonly kind: 'address'; readonly address: Readonly<Partial<Record<Channel, string>>> }

/** Recipient ≠ subject ≠ resolved channel target. */
export const recipient = {
  /** An ERP user: only a ref to the platform's `core.user`. */
  user: (user: UserRef): Recipient => ({ kind: 'user', user }),
  record: (record: Ref): Recipient => ({ kind: 'record', record }),
  address: (address: Readonly<Partial<Record<Channel, string>>>): Recipient => ({ kind: 'address', address }),
} as const

export type NotifyArgs<N extends AnyNotice> = {
  payload: Infer<N['payload']>
  to: readonly [Recipient, ...Recipient[]]
  channels: readonly [Channel, ...Channel[]]
} & SubjectArg<N['subject']>

declare const instanceOf: unique symbol

/** Typed handle to a started workflow instance; `tx.signal` / `ctx.signal` check the signal name and payload against it. */
export interface WorkflowInstance<W extends AnyWorkflow = AnyWorkflow> {
  readonly workflow: W['id']
  /** The instance id. */
  readonly instance: string
  readonly [instanceOf]?: W
}

export interface StartOptions {
  /** Dedupes the start (tenant-scoped). */
  key: string
}

/** Payload type of the signal `N` of workflow `W`. */
export type SignalPayload<W extends AnyWorkflow, N extends keyof W['signals']> = Infer<W['signals'][N]['schema']>

/** `tx.call` accepts operations and atomic commands; anything else resolves to a readable message. */
type TxCallable<C> = C extends Callable
  ? C
  : C extends OrdinaryCommand
    ? Hint<'ordinary command: use ctx.call outside a transaction'>
    : C extends Query
      ? Hint<'query: use tx.query(Q, input)'>
      : Hint<'only operations and atomic commands can be called in a transaction'>

/** `ctx.call` starts a command invocation; operations belong in a transaction, queries use `ctx.query`. */
type CtxCallable<C> = C extends OrdinaryCommand | AtomicCommand
  ? C
  : C extends Operation
    ? Hint<'operation: call it inside ctx.transaction(tx => tx.call(Op, input))'>
    : C extends Query
      ? Hint<'query: use ctx.query(Q, input)'>
      : Hint<'only commands can be called from a command context'>

type WfCallable<C> = C extends OrdinaryCommand | AtomicCommand
  ? C
  : C extends Operation
    ? Hint<'operation: use wf.transaction(name, Op, input)'>
    : Hint<'only commands can be called with wf.call'>

type AnyCallable = Callable | OrdinaryCommand | Query

/** Generic writes take the package's own entities; anything else resolves to a readable message. */
type OwnOnly<E> = E extends OwnEntity
  ? E
  : E extends AnyEntity
    ? Hint<`'${E['id']}' belongs to the platform or another package: its data changes through a published operation (tx.call), not tx.insert/update/delete`>
    : Hint<'only entities declared by this package can be written'>

/**
 * The Go-owned transaction handle. `K` is "plain" where the caller must name a start key
 * and "derived" inside subscriptions and workflows, where the host derives it.
 */
export interface Tx<K extends 'plain' | 'derived' = 'plain'> extends ReadContext {
  /** Own entities only; native writes go through published operations. */
  insert<E extends AnyEntity>(entity: OwnOnly<E>, values: InsertOf<E>): Promise<RecordOf<E>>
  /** With `rowVersion`, fails with a version conflict unless the row is still at that version. */
  update<E extends AnyEntity>(entity: OwnOnly<E>, ref: RefTo<E>, patch: PatchOf<E>, options?: { rowVersion?: number }): Promise<RecordOf<E>>
  delete<E extends AnyEntity>(entity: OwnOnly<E>, ref: RefTo<E>): Promise<void>
  /** Upserts the 1:1 overlay row of `base`; omitted fields keep their value (or default). */
  put<O extends AnyOverlay>(overlay: O, base: RefTo<O['base']>, values: PatchOf<O>): Promise<RecordOf<O>>
  /** Operations and atomic commands join this transaction. Ordinary commands are rejected. */
  call<C extends AnyCallable>(callable: TxCallable<C>, input: InputOf<C>): Promise<ResultOf<C>>
  /** Enqueued atomically with the data. */
  emit<Ev extends AnyEvent>(event: Ev, args: EmitArgs<Ev>): Promise<void>
  notify<N extends AnyNotice>(notice: N, args: NotifyArgs<N>): Promise<void>
  /**
   * Property-style on purpose: parameters are compared strictly, so a `Tx<'plain'>` (key required) is never
   * assignable to a `Tx<'derived'>` (key derived) and a keyless start cannot be smuggled through assignment.
   */
  readonly start: <W extends AnyWorkflow>(
    workflow: W,
    input: InputOf<W>,
    ...options: K extends 'derived' ? [options?: Partial<StartOptions>] : [options: StartOptions]
  ) => Promise<WorkflowInstance<W>>
  /** Delivers a signal to a started instance; enqueued atomically with the data. */
  signal<W extends AnyWorkflow, N extends keyof W['signals'] & string>(
    instance: WorkflowInstance<W>,
    name: N,
    payload: SignalPayload<W, N>,
  ): Promise<void>
}

/** `ctx.query` is both the read-only snapshot context and the function that runs another query. */
export interface CommandQuery extends ReadContext {
  <Q extends Query>(query: Q, input: InputOf<Q>): Promise<Infer<Q['output']>>
}

export interface CommandContext {
  readonly user: Principal
  readonly invocation: InvocationInfo
  readonly log: Logger
  readonly secrets: Secrets
  /** Read-only snapshot access outside any transaction; call it to run a query: `ctx.query(Q, input)`. */
  readonly query: CommandQuery
  /** Opens one Go transaction. Not available inside a transaction. */
  transaction<T>(callback: (tx: Tx) => Promise<T>): Promise<T>
  /** Starts a new command invocation, which selects the currently active binding. */
  call<C extends AnyCallable>(command: CtxCallable<C>, input: InputOf<C>): Promise<ResultOf<C>>
  /** Delivers a signal to a started workflow instance. */
  signal<W extends AnyWorkflow, N extends keyof W['signals'] & string>(
    instance: WorkflowInstance<W>,
    name: N,
    payload: SignalPayload<W, N>,
  ): Promise<void>
}

/** An HTTP request as the endpoint sees it. */
export interface EndpointRequest {
  readonly method: string
  readonly path: string
  readonly headers: Readonly<Record<string, string>>
  readonly query: Readonly<Record<string, string>>
  /** The JSON body, parsed; `undefined` when the body is empty or not JSON. */
  readonly body: Json | undefined
  /** The exact request bytes (signature checks, non-JSON payloads). */
  readonly rawBody: Uint8Array
  /** The request bytes decoded as UTF-8. */
  text(): string
}

export interface EndpointResponse {
  status: number
  headers?: Readonly<Record<string, string>>
  body?: Json
}

export interface EndpointContext extends CommandContext {
  readonly request: EndpointRequest
}

export interface ScheduleContext extends CommandContext {
  /** The scheduled slot this run belongs to. */
  readonly slot: ISODateTime
}

export interface SubscriptionEvent<Ev extends AnyEvent = AnyEvent> {
  readonly id: string
  readonly event: Ev['id']
  readonly occurredAt: ISODateTime
  readonly subject: Ev['subject'] extends AnyEntity ? Ref<Ev['subject']> : undefined
  readonly payload: Infer<Ev['payload']>
  /** The user who caused the event, when there is one. */
  readonly initiator: Int64 | null
}

export type Duration = `${number}${'ms' | 's' | 'm' | 'h' | 'd'}`

export interface StepContext {
  /** Stable provider idempotency key "<instance>:<name>:<n>". */
  readonly idempotencyKey: string
}

export type TaskResult<D> =
  | { readonly outcome: 'decided'; readonly decision: D; readonly by: Int64 }
  | { readonly outcome: 'timeout' }

export type SignalResult<P> =
  | { readonly outcome: 'received'; readonly payload: P }
  | { readonly outcome: 'timeout' }

export interface TaskSpec<S extends AnySchema> {
  subject: Ref
  /** A ref to a `core.user`, or the acting principal. */
  assignee: UserRef | Principal
  decision: S
  timeout?: Duration
}

/** `unknown` when `T` is JSON (interfaces with JSON members included), otherwise a message in the error text. */
export type JsonResult<T> = [T] extends [JsonOf<T>] ? unknown : Hint<'result is not JSON: use plain data, string/number/boolean/null, arrays and objects of those'>

/** Output schema whose TS value is not JSON is rejected here; step and transaction results are journaled as JSON. */
type JsonOutput<S extends AnySchema> = Infer<S> extends JsonOf<Infer<S>> ? unknown : Hint<'output schema is not JSON'>

/** Durable primitives. Every call takes a stable `name`; completed occurrences replay their recorded result. */
export interface WorkflowContext {
  readonly user: Principal
  readonly instance: { readonly id: string }
  readonly log: Logger
  /** Secrets declared in the package capabilities, e.g. for the provider call of a `step`. */
  readonly secrets: Secrets
  /** DB effects and the occurrence receipt commit in one Go transaction. Results must be JSON (interfaces with JSON members are fine). */
  transaction<T>(name: string, callback: (tx: Tx<'derived'>) => Promise<T & JsonResult<T>>): Promise<T>
  transaction<S extends AnySchema>(
    name: string,
    options: { output: S & JsonOutput<S> },
    callback: (tx: Tx<'derived'>) => Promise<Infer<S>>,
  ): Promise<Infer<S>>
  transaction<C extends AnyCallable>(name: string, callable: TxCallable<C>, input: InputOf<C>): Promise<ResultOf<C>>
  /** Re-runnable external effect; at-least-once with a stable idempotency key. Results must be JSON. */
  step<T>(name: string, effect: (step: StepContext) => Promise<T & JsonResult<T>>): Promise<T>
  step<S extends AnySchema>(
    name: string,
    options: { output: S & JsonOutput<S> },
    effect: (step: StepContext) => Promise<Infer<S>>,
  ): Promise<Infer<S>>
  sleep(name: string, duration: Duration): Promise<void>
  waitFor<S extends AnySchema>(name: string, signal: SignalRef<S>): Promise<Infer<S>>
  waitFor<S extends AnySchema>(name: string, signal: SignalRef<S>, options: { timeout: Duration }): Promise<SignalResult<Infer<S>>>
  task<S extends AnySchema>(name: string, spec: TaskSpec<S>): Promise<TaskResult<Infer<S>>>
  /** New command invocation selecting the currently active binding; replay returns the receipt. */
  call<C extends AnyCallable>(name: string, command: WfCallable<C>, input: InputOf<C>): Promise<ResultOf<C>>
  start<W extends AnyWorkflow>(name: string, workflow: W, input: InputOf<W>): Promise<WorkflowInstance<W>>
  /** Journaled clock and id source: deterministic across replays. */
  now(name: string): Promise<ISODateTime>
  uuid(name: string): Promise<string>
}

export interface WriteChange<E extends AnyEntity> {
  readonly op: 'insert' | 'update' | 'delete'
  readonly existing: Row<E> | null
  /** The record as it will be written; null on delete. */
  readonly proposed: Row<E> | null
  readonly changed: readonly (keyof E['fields'] & string)[]
}

type AnyEvent = Event<string, AnyEntity | undefined, AnySchema>
type AnyNotice = Notice<string, AnyEntity | undefined, AnySchema>
type AnyWorkflow = Workflow<string, AnySchema, AnySchema | undefined, Readonly<Record<string, AnySchema>>>

// ---------------------------------------------------------------- handlers

export type { OutOf }

export type Implementable =
  | Query | Command | Operation | AnyWorkflow | ContactResolver | DeliveryPolicy

export type DeliveryRequest<E extends AnyEntity> = {
  readonly record: Ref<E>
  readonly notice: string
  readonly channel: Channel
  readonly target: string
}
export interface DeliveryDecision {
  readonly allow: boolean
  readonly reason?: string
}

/** Handler shape by contract kind. Atomic commands and operations get `Tx`; ordinary commands `CommandContext`. */
export type HandlerOf<C> =
  C extends Operation<string, infer I, infer O> ? (tx: Tx, input: Infer<I>) => Awaitable<OutOf<O>>
  : C extends Command<string, infer I, infer O, true> ? (tx: Tx, input: Infer<I>) => Awaitable<OutOf<O>>
  : C extends Command<string, infer I, infer O, false> ? (ctx: CommandContext, input: Infer<I>) => Awaitable<OutOf<O>>
  : C extends Query<string, infer I, infer O> ? (q: QueryContext, input: Infer<I>) => Awaitable<Infer<O>>
  : C extends Workflow<string, infer I, infer O, Readonly<Record<string, AnySchema>>> ? (wf: WorkflowContext, input: Infer<I>) => Awaitable<OutOf<O>>
  : C extends ContactResolver<string, infer E> ? (q: QueryContext, record: Ref<E>) => Awaitable<ContactTargets>
  : C extends DeliveryPolicy<string, infer E> ? (q: QueryContext, request: DeliveryRequest<E>) => Awaitable<DeliveryDecision>
  : never

export interface Implementation<C extends Implementable = Implementable> {
  readonly [RESOURCE]: true
  readonly serverKind: 'implementation'
  readonly contract: C
  readonly handler: HandlerOf<C>
}

/** Binds a server export to a contract by typed reference. The contract may belong to another package (replacement candidate). */
export function implement<C extends Implementable>(contract: C, handler: HandlerOf<C>): Implementation<C> {
  return { [RESOURCE]: true, serverKind: 'implementation', contract, handler }
}

export interface Subscription<Ev extends AnyEvent = AnyEvent> {
  readonly [RESOURCE]: true
  readonly serverKind: 'subscription'
  readonly id: string
  readonly event: Ev
  readonly handler: (tx: Tx<'derived'>, event: SubscriptionEvent<Ev>) => Awaitable<void>
}

export function subscribe<Ev extends AnyEvent>(
  id: string,
  event: Ev,
  handler: (tx: Tx<'derived'>, event: SubscriptionEvent<Ev>) => Awaitable<void>,
): Subscription<Ev> {
  return { [RESOURCE]: true, serverKind: 'subscription', id, event, handler }
}

export type GuardTarget = OrdinaryCommand | Callable | AnyEntity

export interface PrecheckGuard<C extends OrdinaryCommand = OrdinaryCommand> {
  readonly [RESOURCE]: true
  readonly serverKind: 'guard'
  readonly phase: 'input'
  readonly id: string
  readonly target: C
  readonly handler: (q: GuardContext, input: InputOf<C>) => Awaitable<void>
}

export interface OperationGuard<C extends Callable = Callable> {
  readonly [RESOURCE]: true
  readonly serverKind: 'guard'
  readonly phase: 'operation'
  readonly id: string
  readonly target: C
  readonly handler: (ctx: GuardContext, input: InputOf<C>) => Awaitable<void>
}

export interface WriteRule<E extends AnyEntity = AnyEntity> {
  readonly [RESOURCE]: true
  readonly serverKind: 'guard'
  readonly phase: 'write'
  readonly id: string
  readonly target: E
  readonly handler: (ctx: GuardContext, change: WriteChange<E>) => Awaitable<void>
}

/** Any guard regardless of its target type (handler parameters erased, so `WriteRule<E>` is assignable). */
type ErasedGuard<G extends { readonly handler: unknown }> = Omit<G, 'handler'> & {
  readonly handler: (ctx: GuardContext, arg: never) => Awaitable<void>
}
export type AnyGuard = ErasedGuard<PrecheckGuard> | ErasedGuard<OperationGuard> | ErasedGuard<WriteRule>

/** Preliminary input check of an ordinary command on a read snapshot; not a race-free guarantee. Read-only context. */
export function precheck<C extends OrdinaryCommand>(
  id: string,
  command: C,
  handler: (q: GuardContext, input: InputOf<C>) => Awaitable<void>,
): PrecheckGuard<C> {
  return { [RESOURCE]: true, serverKind: 'guard', phase: 'input', id, target: command, handler }
}

/**
 * Additive check in the transaction of an operation or atomic command, independent of the active implementation.
 * It reads that transaction's state through a read-only context and rejects by throwing; it cannot write.
 */
export function guard<C extends Callable>(
  id: string,
  target: C,
  handler: (ctx: GuardContext, input: InputOf<C>) => Awaitable<void>,
): OperationGuard<C> {
  return { [RESOURCE]: true, serverKind: 'guard', phase: 'operation', id, target, handler }
}

/** Entity write rule, run in the writing transaction before the row is written. Read-only context: reject by throwing. */
export function writeRule<E extends AnyEntity>(
  id: string,
  entity: E,
  handler: (ctx: GuardContext, change: WriteChange<E>) => Awaitable<void>,
): WriteRule<E> {
  return { [RESOURCE]: true, serverKind: 'guard', phase: 'write', id, target: entity, handler }
}

// ---------------------------------------------------------------- notifications

export interface TemplateArgs<N extends AnyNotice> {
  readonly payload: Infer<N['payload']>
  readonly subject: N['subject'] extends AnyEntity ? Ref<N['subject']> : undefined
  readonly locale: string
}

type MaybePromise<T> = T | Promise<T>
export interface EmailContent {
  subject: string
  html: string
  text?: string
}
export interface InAppContent {
  title: string
  body: string
  url?: string
}
export interface PushContent {
  title: string
  body: string
  data?: Readonly<Record<string, string>>
}

export interface TemplateSet<N extends AnyNotice> {
  sms?: (args: TemplateArgs<N>) => MaybePromise<string>
  email?: (args: TemplateArgs<N>) => MaybePromise<EmailContent>
  inApp?: (args: TemplateArgs<N>) => MaybePromise<InAppContent>
  push?: (args: TemplateArgs<N>) => MaybePromise<PushContent>
}

export interface Templates<N extends AnyNotice = AnyNotice> {
  readonly [RESOURCE]: true
  readonly serverKind: 'templates'
  readonly notice: N
  readonly renderers: TemplateSet<N>
}

/** Any templates value regardless of its notice (renderer arguments erased, so `Templates<N>` is assignable). */
export interface AnyTemplates {
  readonly [RESOURCE]: true
  readonly serverKind: 'templates'
  readonly notice: AnyNotice
  readonly renderers: { readonly [K in keyof TemplateSet<AnyNotice>]?: (args: never) => unknown }
}

export function templates<N extends AnyNotice>(notice: N, renderers: TemplateSet<N>): Templates<N> {
  return { [RESOURCE]: true, serverKind: 'templates', notice, renderers }
}

// ---------------------------------------------------------------- schedules and endpoints

export interface Schedule {
  readonly [RESOURCE]: true
  readonly serverKind: 'schedule'
  readonly id: string
  readonly cron: string
  readonly timezone?: string
  readonly handler: (ctx: ScheduleContext) => Awaitable<void>
}

/** Runs as the trigger's owner-user. `cron` is a 5-field expression, optionally with an IANA timezone. */
export function schedule(
  id: string,
  cron: string | { cron: string; timezone?: string },
  handler: (ctx: ScheduleContext) => Awaitable<void>,
): Schedule {
  const spec = typeof cron === 'string' ? { cron } : cron
  return { [RESOURCE]: true, serverKind: 'schedule', id, ...spec, handler }
}

export interface EndpointSpec {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  path: string
  /** Public endpoints run as the trigger owner; authenticated ones as the calling user. */
  public?: boolean
}

export interface Endpoint {
  readonly [RESOURCE]: true
  readonly serverKind: 'endpoint'
  readonly id: string
  readonly spec: EndpointSpec
  readonly handler: (ctx: EndpointContext) => Awaitable<EndpointResponse>
}

export function endpoint(id: string, spec: EndpointSpec, handler: (ctx: EndpointContext) => Awaitable<EndpointResponse>): Endpoint {
  return { [RESOURCE]: true, serverKind: 'endpoint', id, spec, handler }
}

export type ServerValue = Implementation | Subscription | AnyGuard | Templates | Schedule | Endpoint
