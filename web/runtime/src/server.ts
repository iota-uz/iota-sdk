// Server graph: implementations and the context interfaces the Node worker host provides.
// This file is pure: no Node, DOM or solid imports. Handlers never reach the client graph.
import type {
  AnyEntity, AnyOverlay, InsertOf, OwnEntity, PatchOf, PathOf, RecordOf, Row, Where,
} from './entity.ts'
import type {
  AtomicCommand, Callable, Command, ContactResolver, ContactTargets, DeliveryPolicy, Event, InputOf, Notice, Operation,
  OrdinaryCommand, Query, Workflow, SignalRef, ResultOf, OutOf,
} from './contracts.ts'
import type { Channel, Int64, ISODateTime, Json } from './model.ts'
import { RESOURCE } from './resource.ts'
import type { AnySchema, Infer, Ref } from './schema.ts'

// ---------------------------------------------------------------- context interfaces

export type TriggerKind = 'ui' | 'mcp' | 'endpoint' | 'schedule' | 'subscription' | 'workflow'

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
  /** Field keys or dotted ref paths such as "holder.owner". Access rules apply before filter, count and paging. */
  where?: Where<E>
  orderBy?: readonly { field: PathOf<E>; desc?: boolean }[]
  /** Default 100, maximum 1000. */
  limit?: number
  offset?: number
}

/** Read-only typed data access. */
export interface Reader {
  get<E extends AnyEntity>(entity: E, ref: Ref<E>): Promise<RecordOf<E> | null>
  /** Absence of an overlay row is explicit: the result is `null`, never a silent default object. */
  get<O extends AnyOverlay>(overlay: O, base: Ref<O['base']>): Promise<RecordOf<O> | null>
  find<E extends AnyEntity>(entity: E, query?: FindQuery<E>): Promise<RecordOf<E>[]>
  count<E extends AnyEntity>(entity: E, where?: Where<E>): Promise<number>
}

export interface QueryContext extends Reader {
  readonly user: Principal
  readonly invocation: InvocationInfo
  readonly log: Logger
}

type SubjectArg<S> = S extends AnyEntity ? { subject: Ref<S> } : { subject?: undefined }
export type EmitArgs<Ev extends AnyEvent> = { payload: Infer<Ev['payload']> } & SubjectArg<Ev['subject']>

export type Recipient =
  | { readonly kind: 'user'; readonly user: Ref }
  | { readonly kind: 'record'; readonly record: Ref }
  | { readonly kind: 'address'; readonly address: Readonly<Partial<Record<Channel, string>>> }

/** Recipient ≠ subject ≠ resolved channel target. */
export const recipient = {
  user: (user: Ref): Recipient => ({ kind: 'user', user }),
  record: (record: Ref): Recipient => ({ kind: 'record', record }),
  address: (address: Readonly<Partial<Record<Channel, string>>>): Recipient => ({ kind: 'address', address }),
} as const

export type NotifyArgs<N extends AnyNotice> = {
  payload: Infer<N['payload']>
  to: readonly [Recipient, ...Recipient[]]
  channels: readonly [Channel, ...Channel[]]
} & SubjectArg<N['subject']>

export interface WorkflowHandle<W extends AnyWorkflow = AnyWorkflow> {
  readonly workflow: W['id']
  readonly instance: string
}

export interface StartOptions {
  /** Dedupes the start (tenant-scoped). */
  key: string
}

/**
 * The Go-owned transaction handle. `K` is "plain" where the caller must name a start key
 * and "derived" inside subscriptions and workflows, where the host derives it.
 */
export interface Tx<K extends 'plain' | 'derived' = 'plain'> extends Reader {
  readonly user: Principal
  readonly invocation: InvocationInfo
  /** Own entities only; native writes go through published operations. */
  insert<E extends OwnEntity>(entity: E, values: InsertOf<E>): Promise<RecordOf<E>>
  /** With `rowVersion`, fails with a version conflict unless the row is still at that version. */
  update<E extends OwnEntity>(entity: E, ref: Ref<E>, patch: PatchOf<E>, options?: { rowVersion?: number }): Promise<RecordOf<E>>
  delete<E extends OwnEntity>(entity: E, ref: Ref<E>): Promise<void>
  /** Upserts the 1:1 overlay row of `base`; omitted fields keep their value (or default). */
  put<O extends AnyOverlay>(overlay: O, base: Ref<O['base']>, values: PatchOf<O>): Promise<RecordOf<O>>
  /** Operations and atomic commands join this transaction. Ordinary commands are rejected. */
  call<C extends Callable>(callable: C, input: InputOf<C>): Promise<ResultOf<C>>
  /** Enqueued atomically with the data. */
  emit<Ev extends AnyEvent>(event: Ev, args: EmitArgs<Ev>): Promise<void>
  notify<N extends AnyNotice>(notice: N, args: NotifyArgs<N>): Promise<void>
  start<W extends AnyWorkflow>(
    workflow: W,
    input: InputOf<W>,
    ...options: K extends 'derived' ? [options?: Partial<StartOptions>] : [options: StartOptions]
  ): Promise<WorkflowHandle<W>>
}

export interface CommandContext {
  readonly user: Principal
  readonly invocation: InvocationInfo
  readonly log: Logger
  readonly secrets: Secrets
  /** Read-only snapshot access outside any transaction. */
  readonly query: QueryContext
  /** Opens one Go transaction. Not available inside a transaction. */
  transaction<T>(callback: (tx: Tx) => Promise<T>): Promise<T>
  /** Starts a new command invocation, which selects the currently active binding. */
  call<C extends OrdinaryCommand | AtomicCommand>(command: C, input: InputOf<C>): Promise<ResultOf<C>>
}

export interface EndpointRequest {
  readonly method: string
  readonly path: string
  readonly headers: Readonly<Record<string, string>>
  readonly query: Readonly<Record<string, string>>
  readonly body: Json | undefined
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
  /** A user ref from the kit, or a principal. */
  assignee: Ref | Principal
  decision: S
  timeout?: Duration
}

/** Durable primitives. Every call takes a stable `name`; completed occurrences replay their recorded result. */
export interface WorkflowContext {
  readonly user: Principal
  readonly instance: { readonly id: string }
  readonly log: Logger
  /** DB effects and the occurrence receipt commit in one Go transaction. Results must be JSON. */
  transaction<T extends Json | void>(name: string, callback: (tx: Tx<'derived'>) => Promise<T>): Promise<T>
  transaction<C extends Callable>(name: string, callable: C, input: InputOf<C>): Promise<ResultOf<C>>
  /** Re-runnable external effect; at-least-once with a stable idempotency key. Results must be JSON. */
  step<T extends Json | void>(name: string, effect: (step: StepContext) => Promise<T>): Promise<T>
  step<S extends AnySchema>(name: string, options: { output: S }, effect: (step: StepContext) => Promise<Infer<S>>): Promise<Infer<S>>
  sleep(name: string, duration: Duration): Promise<void>
  waitFor<S extends AnySchema>(name: string, signal: SignalRef<S>): Promise<Infer<S>>
  waitFor<S extends AnySchema>(name: string, signal: SignalRef<S>, options: { timeout: Duration }): Promise<SignalResult<Infer<S>>>
  task<S extends AnySchema>(name: string, spec: TaskSpec<S>): Promise<TaskResult<Infer<S>>>
  /** New command invocation selecting the currently active binding; replay returns the receipt. */
  call<C extends OrdinaryCommand | AtomicCommand>(name: string, command: C, input: InputOf<C>): Promise<ResultOf<C>>
  start<W extends AnyWorkflow>(name: string, workflow: W, input: InputOf<W>): Promise<WorkflowHandle<W>>
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
  C extends Operation<string, infer I, infer O> ? (tx: Tx, input: Infer<I>) => Promise<OutOf<O>>
  : C extends Command<string, infer I, infer O, true> ? (tx: Tx, input: Infer<I>) => Promise<OutOf<O>>
  : C extends Command<string, infer I, infer O, false> ? (ctx: CommandContext, input: Infer<I>) => Promise<OutOf<O>>
  : C extends Query<string, infer I, infer O> ? (q: QueryContext, input: Infer<I>) => Promise<Infer<O>>
  : C extends Workflow<string, infer I, infer O, Readonly<Record<string, AnySchema>>> ? (wf: WorkflowContext, input: Infer<I>) => Promise<OutOf<O>>
  : C extends ContactResolver<string, infer E> ? (q: QueryContext, record: Ref<E>) => Promise<ContactTargets>
  : C extends DeliveryPolicy<string, infer E> ? (q: QueryContext, request: DeliveryRequest<E>) => Promise<DeliveryDecision>
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
  readonly handler: (tx: Tx<'derived'>, event: SubscriptionEvent<Ev>) => Promise<void>
}

export function subscribe<Ev extends AnyEvent>(
  id: string,
  event: Ev,
  handler: (tx: Tx<'derived'>, event: SubscriptionEvent<Ev>) => Promise<void>,
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
  readonly handler: (q: QueryContext, input: InputOf<C>) => Promise<void>
}

export interface OperationGuard<C extends Callable = Callable> {
  readonly [RESOURCE]: true
  readonly serverKind: 'guard'
  readonly phase: 'operation'
  readonly id: string
  readonly target: C
  readonly handler: (tx: Tx, input: InputOf<C>) => Promise<void>
}

export interface WriteRule<E extends AnyEntity = AnyEntity> {
  readonly [RESOURCE]: true
  readonly serverKind: 'guard'
  readonly phase: 'write'
  readonly id: string
  readonly target: E
  readonly handler: (tx: Tx, change: WriteChange<E>) => Promise<void>
}

export type AnyGuard = PrecheckGuard | OperationGuard | WriteRule

/** Preliminary input check of an ordinary command on a read snapshot; not a race-free guarantee. */
export function precheck<C extends OrdinaryCommand>(
  id: string,
  command: C,
  handler: (q: QueryContext, input: InputOf<C>) => Promise<void>,
): PrecheckGuard<C> {
  return { [RESOURCE]: true, serverKind: 'guard', phase: 'input', id, target: command, handler }
}

/** Additive check in the transaction of an operation or atomic command, independent of the active implementation. */
export function guard<C extends Callable>(
  id: string,
  target: C,
  handler: (tx: Tx, input: InputOf<C>) => Promise<void>,
): OperationGuard<C> {
  return { [RESOURCE]: true, serverKind: 'guard', phase: 'operation', id, target, handler }
}

/** Entity write rule, run in the writing transaction before the row is written. */
export function writeRule<E extends AnyEntity>(
  id: string,
  entity: E,
  handler: (tx: Tx, change: WriteChange<E>) => Promise<void>,
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
  readonly handler: (ctx: ScheduleContext) => Promise<void>
}

/** Runs as the trigger's owner-user. `cron` is a 5-field expression, optionally with an IANA timezone. */
export function schedule(
  id: string,
  cron: string | { cron: string; timezone?: string },
  handler: (ctx: ScheduleContext) => Promise<void>,
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
  readonly handler: (ctx: EndpointContext) => Promise<EndpointResponse>
}

export function endpoint(id: string, spec: EndpointSpec, handler: (ctx: EndpointContext) => Promise<EndpointResponse>): Endpoint {
  return { [RESOURCE]: true, serverKind: 'endpoint', id, spec, handler }
}

export type ServerValue = Implementation | Subscription | AnyGuard | Templates | Schedule | Endpoint
