// In-memory host for package tests: runs handlers against the real contract definitions without Granite,
// a database or a Node pool. It mirrors the Go host's rules where tests can observe them:
//   * own entities are tables; filter/order/paging follow pkg/extruntime/data (SQL null semantics, ref
//     paths as left joins, read access before filter and paging, nulls last ascending, key tiebreak);
//   * overlays are separate 1:1 rows; a missing row reads `null`;
//   * ordinary commands get a CommandContext (ctx.transaction opens one transaction, never nested),
//     operations/atomic commands/subscriptions get a Tx; `tx.call` accepts only operations and atomic
//     commands; native entities reject generic writes;
//   * emit/notify/start/signal are recorded on commit and discarded on rollback; signals reach the in-memory
//     workflow run of the instance (or wait in its inbox until the test creates the run);
//   * `q.query` / `tx.query` / `ctx.query` run the bound query in the same snapshot (transaction state inside a
//     transaction, the committed state the call started from otherwise) with the caller's rights;
//   * precheck / guard / write-rule handlers get a read-only context (no write members), like the worker;
//   * rows carry their typed `ref`; handlers may be synchronous;
//   * workflow primitives are journaled by (name, n) and driven by the test: a task/signal/sleep that is
//     not satisfied suspends the run, exactly like the replay engine.
import type { Command, Callable, Event, OrdinaryCommand, Query, Workflow } from './contracts.ts'
import { compileFilter } from './entity.ts'
import type { AnyEntity, AnyOverlay } from './entity.ts'
import { type FakeEffects, fakeEffects } from './fakes.ts'
import { EPOCH, OVERLAY_OF, type Fixture } from './fixtures.ts'
import type { ConditionJSON, FilterJSON, ISODateTime, Json, ReadRuleJSON, TypeJSON } from './model.ts'
import type { AnySchema, Infer, Ref } from './schema.ts'
import type {
  Awaitable, CommandContext, Duration, FindQuery, GuardContext, Implementation, Implementable, InvocationInfo, Logger, OperationGuard,
  Principal, PrecheckGuard, QueryContext, ReadContext, Reader, SignalPayload, SignalResult, StepContext, Subscription, SubscriptionEvent,
  TaskResult, TaskSpec, TriggerKind, Tx, WorkflowContext, WorkflowInstance, WriteRule,
} from './server.ts'
import { decode, encode, validate } from './wire.ts'

// ---------------------------------------------------------------- errors and registry

/** Rejections the real host produces (`code` mirrors pkg/extruntime/data and the host worker). */
export class TestHostError extends Error {
  readonly kind: 'internal'
  readonly code: string

  constructor(code: string, message: string) {
    super(message)
    this.name = 'TestHostError'
    this.kind = 'internal'
    this.code = code
  }
}

const fail = (code: string, message: string): never => {
  throw new TestHostError(code, message)
}

interface TestingState {
  cases: { name: string; fn: () => unknown; timeoutMs?: number }[]
  http: { request(request: never): Promise<unknown> } | null
}

/** Shared with the build worker's test runner (test-child.ts) through the global symbol registry. */
function state(): TestingState {
  const g = globalThis as Record<symbol, unknown>
  return (g[Symbol.for('iota.testing.state')] ??= { cases: [], http: null }) as TestingState
}

let prefix = ''

/** Registers a test case for the package test runner (`*.test.ts`). */
export function test(name: string, fn: () => unknown, options: { timeoutMs?: number } = {}): void {
  state().cases.push({ name: prefix + name, fn, ...(options.timeoutMs === undefined ? {} : { timeoutMs: options.timeoutMs }) })
}

/** Groups cases: their names are prefixed with the group name. */
export function describe(name: string, body: () => void): void {
  const before = prefix
  prefix = `${before}${name} > `
  try {
    body()
  } finally {
    prefix = before
  }
}

// ---------------------------------------------------------------- options and results

export interface EmittedEvent {
  readonly event: string
  readonly subject: Ref | undefined
  readonly payload: Json
  readonly initiator: string
}
export interface SentNotice {
  readonly notice: string
  readonly subject: Ref | undefined
  readonly payload: Json
  readonly to: readonly object[]
  readonly channels: readonly string[]
}
/** A signal delivered by `tx.signal` (on commit) or `ctx.signal`, in delivery order. */
export interface DeliveredSignal {
  readonly workflow: string
  readonly instance: string
  readonly name: string
  readonly payload: Json
}
export interface StartedWorkflow {
  readonly workflow: string
  readonly instance: string
  readonly key: string | undefined
  readonly input: Json
}

type AnyWorkflow = Workflow<string, AnySchema, AnySchema | undefined, Readonly<Record<string, AnySchema>>>

/**
 * A write rule of any entity. `AnyGuard`'s `WriteRule<AnyEntity>` rejects `WriteRule<SomeEntity>` (the handler's
 * `change` parameter is invariant), so the host accepts the rule with its change type erased.
 */
export type AnyWriteRule = Omit<WriteRule, 'handler'> & { readonly handler: (ctx: GuardContext, change: never) => Awaitable<void> }

/** Operation/atomic-command guards, ordinary-command prechecks and entity write rules. */
export type TestGuard = PrecheckGuard | OperationGuard | AnyWriteRule

export interface TestHostOptions {
  /** Entity records (`fixture(...)`) and overlay rows (`overlayFixture(...)`) the tables start with. */
  fixtures?: readonly (Fixture<never> | Fixture<AnyEntity> | object)[]
  /** Definitions the host cannot learn from calls (targets of ref paths such as "holder.owner"). */
  entities?: readonly (AnyEntity | AnyOverlay)[]
  fakes?: FakeEffects
  /** Handlers `tx.call`, `ctx.call` and `wf.call` dispatch to, by contract ID. */
  implementations?: readonly Implementation[]
  guards?: readonly TestGuard[]
  user?: { id?: string; permissions?: readonly string[]; locale?: string; kind?: 'initiator' | 'owner' }
  secrets?: Readonly<Record<string, string>>
  /** Start of the host clock. */
  now?: ISODateTime
  trigger?: TriggerKind
}

export type PendingWork =
  | { readonly kind: 'sleep'; readonly name: string; readonly n: number; readonly wakeAt: ISODateTime }
  | { readonly kind: 'task'; readonly name: string; readonly n: number }
  | { readonly kind: 'signal'; readonly name: string; readonly n: number }

export type WorkflowOutcome<T> =
  | { readonly status: 'completed'; readonly value: T }
  | { readonly status: 'suspended'; readonly on: PendingWork }
  | { readonly status: 'failed'; readonly error: unknown }

export interface OpenTask {
  readonly name: string
  readonly n: number
  readonly subject: Ref
  readonly assignee: unknown
  readonly status: 'open' | 'decided' | 'expired'
}

/** One workflow instance: repeated `run()` calls replay the journal, like the real engine. */
export interface WorkflowRun<W extends AnyWorkflow = AnyWorkflow> {
  readonly instance: string
  run(): Promise<WorkflowOutcome<W['output'] extends AnySchema ? Infer<W['output']> : void>>
  /** Decides the open task `name` (validated against its decision schema). */
  decide(name: string, decision: unknown, options?: { by?: string; n?: number }): void
  /** Delivers a signal; `signal` is `Workflow.signals.<name>` or the signal name. The payload is validated against its schema. */
  signal<N extends keyof W['signals'] & string>(signal: W['signals'][N] | N, payload: SignalPayload<W, N>): void
  readonly tasks: readonly OpenTask[]
  /** Completed occurrences, in order, as `name#n`. */
  readonly journal: readonly string[]
}

export interface TestHost {
  readonly fakes: FakeEffects
  readonly user: Principal
  readonly clock: { now(): ISODateTime; advance(duration: Duration): void }
  /** Committed outbox. */
  readonly emitted: readonly EmittedEvent[]
  readonly notices: readonly SentNotice[]
  readonly started: readonly StartedWorkflow[]
  /** Committed signals, in delivery order. */
  readonly signals: readonly DeliveredSignal[]
  readonly logs: readonly { readonly level: string; readonly message: string }[]
  /** Read-only snapshot context (a query); `query.query(Q, input)` runs a bound query. */
  query(): QueryContext
  /** Ordinary-command context: `ctx.transaction` opens one transaction. */
  command(): CommandContext
  /**
   * Runs `callback` in one transaction (operation / atomic command context): commit, or rollback on throw.
   * The callback may return directly, so `host.transaction((tx) => impl.handler(tx, input))` works for sync handlers.
   */
  transaction<T>(callback: (tx: Tx) => Awaitable<T>): Promise<T>
  /** Delivers an event to a subscription in its own transaction. */
  deliver<Ev extends Event>(subscription: Subscription<Ev>, event: { subject?: Ref; payload: Json; initiator?: string }): Promise<void>
  /**
   * Starts a workflow instance driven by the test. Pass `instance` (e.g. of `host.started`) to drive an instance
   * started with `tx.start`: signals sent to it so far are in its inbox.
   */
  workflow<C extends AnyWorkflow & Implementable>(
    implementation: Implementation<C>,
    input: Infer<C['input']>,
    options?: { instance?: string },
  ): WorkflowRun<C>
  /** Committed rows of an entity, in insertion order. */
  table<E extends AnyEntity>(entity: E): readonly Readonly<Record<string, unknown>>[]
  /** The committed overlay row of a base record, or null. */
  overlayRow<O extends AnyOverlay>(overlay: O, base: Ref<O['base']>): Readonly<Record<string, unknown>> | null
}

// ---------------------------------------------------------------- value semantics

type Row = Record<string, unknown>

const DURATION = /^(\d+)(ms|s|m|h|d)$/
const UNIT_MS = { ms: 1, s: 1000, m: 60_000, h: 3_600_000, d: 86_400_000 } as const

function durationMs(d: string): number {
  const m = DURATION.exec(d)
  if (m === null) return fail('runtime.invalidDuration', `invalid duration "${d}"`)
  return Number(m[1]) * UNIT_MS[m[2] as keyof typeof UNIT_MS]
}

const json = <T>(value: T): T => (value === undefined ? value : (JSON.parse(JSON.stringify(value)) as T))
const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v)

function cmpDecimal(a: string, b: string): number {
  const parse = (s: string): [boolean, string, string] => {
    const neg = s.startsWith('-')
    const [i = '0', f = ''] = (neg ? s.slice(1) : s).split('.')
    return [neg, i.replace(/^0+(?=\d)/, ''), f]
  }
  const [an, ai, af] = parse(a)
  const [bn, bi, bf] = parse(b)
  const scale = Math.max(af.length, bf.length)
  const x = BigInt(ai + af.padEnd(scale, '0')) * (an ? -1n : 1n)
  const y = BigInt(bi + bf.padEnd(scale, '0')) * (bn ? -1n : 1n)
  return x < y ? -1 : x > y ? 1 : 0
}

const order = (x: number | bigint | string | boolean, y: number | bigint | string | boolean): number => (x < y ? -1 : x > y ? 1 : 0)

/** Orders two non-null values of a wire type; refs compare by key. */
function compare(type: TypeJSON | undefined, a: unknown, b: unknown): number {
  const key = (v: unknown): string => (isRecord(v) ? String(v.key) : String(v))
  switch (type?.kind) {
    case 'int':
      return order(Number(a), Number(b))
    case 'int64':
      return order(BigInt(String(a)), BigInt(String(b)))
    case 'decimal':
      return cmpDecimal(String(a), String(b))
    case 'datetime':
      return order(Date.parse(String(a)), Date.parse(String(b)))
    case 'bool':
      return order(Boolean(a), Boolean(b))
    case 'ref':
      return order(key(a), key(b))
    case 'json':
    case 'object':
    case 'array':
      return JSON.stringify(a) === JSON.stringify(b) ? 0 : 1
    case undefined:
      return typeof a === 'number' && typeof b === 'number' ? order(a, b) : order(key(a), key(b))
    default:
      return order(String(a), String(b))
  }
}

const ORDERABLE = new Set(['string', 'int', 'int64', 'decimal', 'date', 'datetime', 'uuid', 'enum'])
const MAX_PATH = 4
const MAX_IN = 1000
const DEFAULT_LIMIT = 100
const MAX_LIMIT = 1000

/** SQL LIKE (`%`, `_`, backslash escape) as a regular expression. */
function likeToRegExp(pattern: string): RegExp {
  let out = ''
  for (let i = 0; i < pattern.length; i++) {
    const ch = pattern[i] ?? ''
    if (ch === '\\' && i + 1 < pattern.length) out += (pattern[++i] ?? '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    else if (ch === '%') out += '[\\s\\S]*'
    else if (ch === '_') out += '[\\s\\S]'
    else out += ch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  }
  return new RegExp(`^${out}$`)
}

// ---------------------------------------------------------------- storage

class Store {
  tables = new Map<string, Map<string, Row>>()
  overlays = new Map<string, Map<string, Row>>()

  clone(): Store {
    const c = new Store()
    for (const [id, rows] of this.tables) c.tables.set(id, new Map([...rows].map(([k, r]) => [k, structuredClone(r)])))
    for (const [id, rows] of this.overlays) c.overlays.set(id, new Map([...rows].map(([k, r]) => [k, structuredClone(r)])))
    return c
  }

  table(id: string): Map<string, Row> {
    let t = this.tables.get(id)
    if (t === undefined) this.tables.set(id, (t = new Map()))
    return t
  }

  overlay(id: string): Map<string, Row> {
    let t = this.overlays.get(id)
    if (t === undefined) this.overlays.set(id, (t = new Map()))
    return t
  }
}

type Def = AnyEntity | AnyOverlay
const isOverlay = (d: Def): d is AnyOverlay => d.kind === 'overlay'

interface Pending {
  emitted: EmittedEvent[]
  notices: SentNotice[]
  started: StartedWorkflow[]
  signals: DeliveredSignal[]
}

// ---------------------------------------------------------------- the host

export function createTestHost(options: TestHostOptions = {}): TestHost {
  const fakes = options.fakes ?? fakeEffects()
  state().http = fakes.http as never

  const permissions = new Set(options.user?.permissions ?? [])
  const user: Principal = {
    tenantId: 'tenant-test',
    userId: (options.user?.id ?? '1') as Principal['userId'],
    kind: options.user?.kind ?? 'initiator',
    ...(options.user?.locale === undefined ? {} : { locale: options.user.locale }),
    has: (p) => {
      const id = typeof p === 'string' ? p : p.id
      return id === '' || permissions.has(id)
    },
  }

  let store = new Store()
  const defs = new Map<string, Def>()
  const committed: Pending = { emitted: [], notices: [], started: [], signals: [] }
  const logs: { level: string; message: string }[] = []
  const impls = new Map<string, Implementation>()
  const writeRules = new Map<string, AnyWriteRule[]>()
  const opGuards = new Map<string, OperationGuard[]>()
  const prechecks = new Map<string, PrecheckGuard[]>()
  const workflowDefs = new Map<string, AnyWorkflow>()
  /** Runs the test drives, by instance, and signals that arrived before a run exists for the instance. */
  const runs = new Map<string, { readonly workflow: AnyWorkflow; push(name: string, payload: unknown): void }>()
  const inboxes = new Map<string, { name: string; payload: unknown }[]>()
  let clockMs = Date.parse(options.now ?? EPOCH)
  let counter = 0
  const nextId = (p: string): string => `${p}-${++counter}`
  const nextKey = (): string => `00000000-0000-4000-8000-${(++counter).toString(16).padStart(12, '0')}`
  const nowIso = (): ISODateTime => new Date(clockMs).toISOString() as ISODateTime

  const register = (def: Def): void => {
    if (!defs.has(def.id)) defs.set(def.id, def)
  }
  for (const d of options.entities ?? []) register(d)
  for (const impl of options.implementations ?? []) impls.set(impl.contract.id, impl)
  const push = <T>(map: Map<string, T[]>, id: string, value: T): void => void map.set(id, [...(map.get(id) ?? []), value])
  for (const g of options.guards ?? []) {
    if (g.phase === 'write') push(writeRules, g.target.id, g)
    else if (g.phase === 'operation') push(opGuards, g.target.id, g)
    else push(prechecks, g.target.id, g)
    if (g.phase === 'write') register(g.target)
  }

  for (const f of options.fixtures ?? []) {
    const overlayId = (f as Record<symbol, unknown>)[OVERLAY_OF]
    if (typeof overlayId === 'string') {
      const row = f as Row
      store.overlay(overlayId).set(String(row.baseKey), structuredClone({ ...row }))
    } else {
      const fx = f as Fixture<AnyEntity>
      // `ref` is derived on read (like the worker); the table holds the stored columns only.
      const record: Row = structuredClone({ ...fx.record })
      delete record.ref
      store.table(fx.ref.entity).set(fx.ref.key, record)
    }
  }

  // ---- read model

  const typeAt = (def: Def | undefined, key: string): TypeJSON | undefined => def?.fields[key]?.type

  const hiddenField = (def: Def | undefined, key: string): boolean => {
    const required = (def?.resource.access?.fields ?? {})[key] ?? []
    return required.some((p) => !user.has(p))
  }

  const subjectRef = (): Ref => ({ entity: 'core.user', key: user.userId })

  /** SQL-style evaluation of a filter against a row of `def`. */
  const matches = (s: Store, def: Def, row: Row, filter: FilterJSON, trusted: boolean): boolean => {
    for (const [path, cond] of Object.entries(filter.fields ?? {}).sort(([a], [b]) => (a < b ? -1 : 1))) {
      const [value, type] = resolvePath(s, def, row, path, trusted)
      if (!condition(cond, value, type, path)) return false
    }
    const alternatives = filter.or ?? []
    return alternatives.length === 0 || alternatives.some((alt) => matches(s, def, row, alt, trusted))
  }

  const resolvePath = (s: Store, def: Def, row: Row, path: string, trusted: boolean): [unknown, TypeJSON | undefined] => {
    const segments = path.split('.')
    if (segments.length > MAX_PATH) fail('data.invalidQuery', `path "${path}" is deeper than ${MAX_PATH}`)
    let current: Row | undefined = row
    let currentDef: Def | undefined = def
    for (const seg of segments.slice(0, -1)) {
      const type = typeAt(currentDef, seg)
      if (currentDef !== undefined && type === undefined) fail('data.unknownField', `"${seg}" on ${currentDef.id}`)
      if (!trusted && hiddenField(currentDef, seg)) fail('data.fieldNotAccessible', `"${seg}" on ${currentDef?.id ?? '?'}`)
      if (type !== undefined && type.kind !== 'ref') fail('data.invalidQuery', `"${seg}" is not a ref, cannot continue path "${path}"`)
      const target: { entity?: string; key?: string } | null | undefined = current === undefined ? undefined : (current[seg] as { entity?: string; key?: string } | null | undefined)
      const targetDef: Def | undefined = type?.entity === undefined ? undefined : defs.get(type.entity)
      const next: Row | undefined = target?.entity === undefined || target.key === undefined ? undefined : s.table(target.entity).get(target.key)
      // A joined entity is visible only through its own read access.
      current = next !== undefined && targetDef !== undefined && !trusted && !visible(s, targetDef, next) ? undefined : next
      currentDef = targetDef
    }
    const last = segments[segments.length - 1] ?? ''
    const type = typeAt(currentDef, last)
    if (currentDef !== undefined && type === undefined) fail('data.unknownField', `"${last}" on ${currentDef.id}`)
    if (!trusted && hiddenField(currentDef, last)) fail('data.fieldNotAccessible', `"${last}" on ${currentDef?.id ?? '?'}`)
    return [current === undefined ? null : (current[last] ?? null), type]
  }

  const resolveValue = (raw: unknown): unknown => {
    if (isRecord(raw) && Object.keys(raw).length === 1 && '$' in raw) {
      if (raw.$ !== 'user') fail('data.invalidQuery', `unknown placeholder ${JSON.stringify(raw)}`)
      return subjectRef()
    }
    return raw
  }

  const condition = (c: ConditionJSON, value: unknown, type: TypeJSON | undefined, field: string): boolean => {
    const present = value !== null && value !== undefined
    const checked = (raw: unknown): unknown => {
      const v = resolveValue(raw)
      if (v !== null && type !== undefined) {
        try {
          validate({ ...type, optional: undefined, nullable: undefined } as TypeJSON, v)
        } catch (e) {
          fail('data.invalidQuery', `value for "${field}": ${(e as Error).message}`)
        }
      }
      return v
    }
    const eq = (raw: unknown): boolean => {
      const v = checked(raw)
      return present && v !== null && compare(type, value, v) === 0
    }
    const parts: boolean[] = []
    if (c.eq !== undefined) parts.push(c.eq === null ? !present : eq(c.eq))
    if (c.ne !== undefined) parts.push(c.ne === null ? present : !present || !eq(c.ne))
    if (c.in !== undefined) {
      if (c.in.length > MAX_IN) fail('data.invalidQuery', `"${field}": in-list longer than ${MAX_IN}`)
      parts.push(c.in.length === 0 ? false : c.in.map(checked).some((v) => present && v !== null && compare(type, value, v) === 0))
    }
    for (const [op, raw] of [['lt', c.lt], ['lte', c.lte], ['gt', c.gt], ['gte', c.gte]] as const) {
      if (raw === undefined) continue
      if (type !== undefined && !ORDERABLE.has(type.kind)) fail('data.invalidQuery', `"${field}" (${type.kind}) cannot be compared with ${op}`)
      const v = checked(raw)
      const d = present && v !== null ? compare(type, value, v) : undefined
      parts.push(d !== undefined && (op === 'lt' ? d < 0 : op === 'lte' ? d <= 0 : op === 'gt' ? d > 0 : d >= 0))
    }
    if (c.isNull !== undefined) parts.push(c.isNull ? !present : present)
    if (c.like !== undefined) {
      if (type !== undefined && type.kind !== 'string' && type.kind !== 'enum') fail('data.invalidQuery', `"${field}" (${type.kind}) does not support like`)
      parts.push(present && likeToRegExp(c.like).test(String(value)))
    }
    if (parts.length === 0) fail('data.invalidQuery', `empty condition for "${field}"`)
    return parts.every(Boolean)
  }

  /** Row visibility of an entity for the subject: the OR of the applicable read rules. */
  const visible = (s: Store, def: Def, row: Row): boolean => {
    const rules: readonly ReadRuleJSON[] = def.resource.access?.read ?? []
    if (rules.length === 0) return true
    for (const rule of rules) {
      if (!user.has(rule.permission ?? '')) continue
      if (rule.all === true) return true
      if (rule.rows !== undefined && matches(s, def, row, rule.rows, true)) return true
    }
    return false
  }

  /** Entity rows carry a typed `ref` to themselves (`Row<E>.ref`); overlay rows (keyed by `baseKey`) have none. */
  const withRef = (def: Def, row: Row): Row => (isOverlay(def) ? row : { ...row, ref: { entity: def.id, key: String(row.key) } })

  const project = (def: Def, row: Row): Row => {
    const out: Row = {}
    for (const [k, v] of Object.entries(row)) if (!(k in def.fields) || !hiddenField(def, k)) out[k] = structuredClone(v)
    return withRef(def, out)
  }

  const makeReader = (s: () => Store): Reader => {
    /** Visible rows matching the filter, before ordering and paging. */
    const matching = (entity: AnyEntity, where: object | undefined): Row[] => {
      register(entity)
      const st = s()
      const filter = where === undefined ? {} : compileFilter(where)
      return [...st.table(entity.id).values()].filter((r) => visible(st, entity, r)).filter((r) => matches(st, entity, r, filter, false))
    }
    const find = (entity: AnyEntity, q: FindQuery<AnyEntity> = {}): Row[] => {
      const st = s()
      const limit = q.limit ?? 0
      const offset = q.offset ?? 0
      if (limit < 0 || offset < 0) fail('data.invalidQuery', 'limit and offset must not be negative')
      let rows = matching(entity, q.where)
      const orders = (q.orderBy ?? []).map((o) => ({ field: o.field as string, desc: o.desc === true }))
      for (const o of orders) {
        const sample = rows[0]
        const [, type] = sample === undefined ? [null, typeAt(entity, o.field.split('.')[0] ?? '')] : resolvePath(st, entity, sample, o.field, false)
        if (type !== undefined && !ORDERABLE.has(type.kind) && type.kind !== 'ref' && type.kind !== 'bool') fail('data.invalidQuery', `"${o.field}" (${type.kind}) cannot be ordered`)
      }
      const keyed = rows.map((r) => ({ r, vals: orders.map((o) => resolvePath(st, entity, r, o.field, false)) }))
      keyed.sort((a, b) => {
        for (const [i, o] of orders.entries()) {
          const [av, type] = a.vals[i] ?? [null, undefined]
          const [bv] = b.vals[i] ?? [null, undefined]
          // PostgreSQL: NULLS LAST ascending, NULLS FIRST descending.
          const an = av === null || av === undefined
          const bn = bv === null || bv === undefined
          const d = an && bn ? 0 : an ? 1 : bn ? -1 : compare(type, av, bv)
          if (d !== 0) return o.desc ? -d : d
        }
        return order(String(a.r.key), String(b.r.key))
      })
      rows = keyed.map((k) => k.r)
      return rows.slice(offset, offset + Math.min(limit === 0 ? DEFAULT_LIMIT : limit, MAX_LIMIT)).map((r) => project(entity, r))
    }
    const get = (def: Def, ref: Ref): Row | null => {
      register(def)
      const st = s()
      if (isOverlay(def)) {
        const row = st.overlay(def.id).get(ref.key)
        return row === undefined ? null : project(def, row)
      }
      if (ref.entity !== def.id) fail('data.invalidRef', `ref to "${ref.entity}" used with entity "${def.id}"`)
      const row = st.table(def.id).get(ref.key)
      return row === undefined || !visible(st, def, row) ? null : project(def, row)
    }
    const count = (entity: AnyEntity, where?: object): number => matching(entity, where).length
    return {
      get: (async (def: Def, ref: Ref) => get(def, ref)) as Reader['get'],
      find: (async (entity: AnyEntity, q?: FindQuery<AnyEntity>) => {
        return find(entity, q)
      }) as unknown as Reader['find'],
      count: (async (entity: AnyEntity, where?: object) => count(entity, where)) as Reader['count'],
    }
  }

  // ---- contexts

  const logger = (): Logger => {
    const at = (level: string) => (message: string, fields?: Readonly<Record<string, Json>>) =>
      void logs.push({ level, message: fields === undefined ? message : `${message} ${JSON.stringify(fields)}` })
    return { debug: at('debug'), info: at('info'), warn: at('warn'), error: at('error') }
  }

  const invocation = (parentId?: string): InvocationInfo => ({
    id: nextId('inv'),
    trigger: options.trigger ?? 'ui',
    ...(parentId === undefined ? {} : { parentId }),
  })

  const secrets = {
    get: async (name: string): Promise<string> => options.secrets?.[name] ?? fail('runtime.secretUndeclared', `secret "${name}" is not provided to this test host`),
  }

  /**
   * Read-only data access over `s()` plus `query`, which runs a bound query on the store the call sees (the
   * transaction's working state inside a transaction, the committed state it started from otherwise).
   * `open` rejects use after the owning transaction ended.
   */
  const readContext = (s: () => Store, info: InvocationInfo, open: () => void = () => undefined): ReadContext => {
    const reader = makeReader(s)
    return {
      user,
      invocation: info,
      log: logger(),
      get: (async (...a: Parameters<Reader['get']>) => (open(), reader.get(...a))) as Reader['get'],
      find: (async (...a: Parameters<Reader['find']>) => (open(), reader.find(...a))) as Reader['find'],
      count: (async (...a: Parameters<Reader['count']>) => (open(), reader.count(...a))) as Reader['count'],
      query: (async (q: Query, input: unknown) => {
        open()
        return runQuery(s(), q, input, info.id)
      }) as ReadContext['query'],
    }
  }

  /** Context of precheck / guard / write-rule handlers: reads, identity and log. It has no write members. */
  const guardContext = (ctx: ReadContext): GuardContext =>
    Object.freeze({ user: ctx.user, invocation: ctx.invocation, log: ctx.log, get: ctx.get, find: ctx.find, count: ctx.count, query: ctx.query })

  /** Runs a bound query against `snapshot` with the caller's rights. */
  const runQuery = async (snapshot: Store, q: Query, input: unknown, parent: string): Promise<unknown> => {
    if (q.kind !== 'query') return fail('runtime.invalidInput', `"${q.id}" is a ${q.kind}, not a query`)
    const valid = validateInput(q, input)
    const handler = implementationOf(q.id).handler as unknown as (ctx: QueryContext, input: unknown) => Awaitable<unknown>
    const out = await handler(readContext(() => snapshot, invocation(parent)), valid)
    return decode(q.output, encode(q.output, out as never))
  }

  // ---- signals

  /** Validates a signal to a started instance (or a run) and returns what is delivered. */
  const checkSignal = (target: { readonly workflow: string; readonly instance: string }, name: string, payload: unknown, starting: readonly StartedWorkflow[]): DeliveredSignal => {
    const run = runs.get(target.instance)
    const workflowId = run?.workflow.id ?? [...committed.started, ...starting].find((x) => x.instance === target.instance)?.workflow
    if (workflowId === undefined || workflowId !== target.workflow) {
      return fail('workflow.unknownInstance', `no started instance "${target.instance}" of workflow "${target.workflow}"`)
    }
    const schema = (run?.workflow ?? workflowDefs.get(workflowId))?.signals[name]?.schema
    if (schema === undefined) return fail('workflow.unknownSignal', `workflow "${workflowId}" declares no signal "${name}"`)
    try {
      return { workflow: workflowId, instance: target.instance, name, payload: json(encode(schema, payload as never)) }
    } catch (e) {
      return fail('workflow.invalidSignal', `signal "${name}" of "${workflowId}": ${(e as Error).message}`)
    }
  }

  const deliverSignal = (signal: DeliveredSignal): void => {
    committed.signals.push(signal)
    const run = runs.get(signal.instance)
    if (run !== undefined) run.push(signal.name, signal.payload)
    else inboxes.set(signal.instance, [...(inboxes.get(signal.instance) ?? []), { name: signal.name, payload: signal.payload }])
  }

  interface Scope {
    store: Store
    pending: Pending
    open: boolean
    derived: boolean
    info: InvocationInfo
  }

  const validateInput = (c: { id: string; input: AnySchema }, input: unknown): unknown => {
    try {
      return decode(c.input, encode(c.input, input as never))
    } catch (e) {
      return fail('runtime.invalidInput', `${c.id}: ${(e as Error).message}`)
    }
  }

  const implementationOf = (id: string): Implementation => {
    const impl = impls.get(id)
    if (impl === undefined) return fail('test.noImplementation', `no implementation of "${id}" is bound in this test host; pass it in createTestHost({ implementations })`)
    return impl
  }

  /** Entity write rules run in the writing transaction, before the row is written, with a read-only context. */
  const writeRuleHooks = async (tx: Tx, def: AnyEntity, op: 'insert' | 'update' | 'delete', existing: Row | null, proposed: Row | null): Promise<void> => {
    const rules = writeRules.get(def.id) ?? []
    if (rules.length === 0) return
    const changed = Object.keys(def.fields).filter((k) => JSON.stringify(existing?.[k] ?? null) !== JSON.stringify(proposed?.[k] ?? null))
    const change = {
      op,
      existing: existing === null ? null : withRef(def, structuredClone(existing)),
      proposed: proposed === null ? null : withRef(def, structuredClone(proposed)),
      changed,
    }
    for (const rule of rules) await rule.handler(guardContext(tx), change as never)
  }

  /** Runs an operation / atomic command in the caller's transaction, guards first (read-only context). */
  const callCallable = async (tx: Tx, callable: Callable, input: unknown): Promise<unknown> => {
    const valid = validateInput(callable, input)
    for (const g of opGuards.get(callable.id) ?? []) await g.handler(guardContext(tx), valid as never)
    const out = await (implementationOf(callable.id).handler as unknown as (tx: Tx, input: unknown) => Awaitable<unknown>)(tx, valid)
    return callable.output === undefined ? undefined : decode(callable.output as AnySchema, encode(callable.output as AnySchema, out as never))
  }

  const makeTx = (scope: Scope): Tx<'plain'> & Tx<'derived'> => {
    const guard = (): void => {
      if (!scope.open) fail('runtime.txClosed', 'transaction is closed')
    }
    const reads = readContext(() => scope.store, scope.info, guard)
    const own = (def: AnyEntity, op: string): void => {
      register(def)
      if (def.entityKind === 'native') fail('data.nativeWrite', `${op} on native entity "${def.id}": native entities are read-only through generic data calls`)
    }
    const checkUnique = (def: AnyEntity, row: Row, exceptKey: string | undefined): void => {
      for (const [k, f] of Object.entries(def.fields)) {
        if (f.spec.unique !== true || row[k] === null || row[k] === undefined) continue
        for (const other of scope.store.table(def.id).values()) {
          if (other.key !== exceptKey && JSON.stringify(other[k]) === JSON.stringify(row[k])) fail('data.uniqueViolation', `field "${k}" already holds this value`)
        }
      }
    }
    const shapeOf = (def: Def, keys: readonly string[]): AnySchema => {
      const properties = keys.map((k) => {
        const f = def.fields[k]
        if (f === undefined) return fail('data.unknownField', `"${k}" on ${def.id}`)
        return { name: k, type: f.type }
      })
      return { type: { kind: 'object', properties } } as unknown as AnySchema
    }
    const tx = {
      ...reads,
      insert: async (def: AnyEntity, values: Row) => {
        guard()
        own(def, 'insert')
        const row: Row = {}
        for (const [k, f] of Object.entries(def.fields)) row[k] = values[k] !== undefined ? values[k] : (f.spec.default ?? null)
        for (const k of Object.keys(values)) if (!(k in def.fields)) fail('data.unknownField', `"${k}" on ${def.id}`)
        try {
          encode(shapeOf(def, Object.keys(def.fields)), row as never)
        } catch (e) {
          fail('data.invalidValue', (e as Error).message)
        }
        checkUnique(def, row, undefined)
        const full: Row = { key: nextKey(), createdAt: nowIso(), updatedAt: nowIso(), rowVersion: 1, ...structuredClone(row) }
        await writeRuleHooks(tx as unknown as Tx, def, 'insert', null, full)
        scope.store.table(def.id).set(String(full.key), full)
        return project(def, full)
      },
      update: async (def: AnyEntity, ref: Ref, patch: Row, opts?: { rowVersion?: number }) => {
        guard()
        own(def, 'update')
        const existing = scope.store.table(def.id).get(ref.key)
        if (existing === undefined || !visible(scope.store, def, existing)) return fail('data.notFound', `${def.id} ${ref.key}`)
        if (opts?.rowVersion !== undefined && opts.rowVersion !== existing.rowVersion) {
          fail('data.versionConflict', `${def.id} ${ref.key} is at version ${String(existing.rowVersion)}, expected ${opts.rowVersion}`)
        }
        const keys = Object.keys(patch).filter((k) => patch[k] !== undefined)
        try {
          encode(shapeOf(def, keys), Object.fromEntries(keys.map((k) => [k, patch[k]])) as never)
        } catch (e) {
          fail('data.invalidValue', (e as Error).message)
        }
        const next: Row = { ...structuredClone(existing), ...structuredClone(Object.fromEntries(keys.map((k) => [k, patch[k]]))) }
        checkUnique(def, next, ref.key)
        next.updatedAt = nowIso()
        next.rowVersion = Number(existing.rowVersion) + 1
        await writeRuleHooks(tx as unknown as Tx, def, 'update', existing, next)
        scope.store.table(def.id).set(ref.key, next)
        return project(def, next)
      },
      delete: async (def: AnyEntity, ref: Ref) => {
        guard()
        own(def, 'delete')
        const existing = scope.store.table(def.id).get(ref.key)
        if (existing === undefined || !visible(scope.store, def, existing)) return fail('data.notFound', `${def.id} ${ref.key}`)
        await writeRuleHooks(tx as unknown as Tx, def, 'delete', existing, null)
        scope.store.table(def.id).delete(ref.key)
      },
      put: async (overlay: AnyOverlay, base: Ref, values: Row) => {
        guard()
        register(overlay)
        const baseTable = scope.store.tables.get(overlay.base.id)
        if (baseTable !== undefined && !baseTable.has(base.key)) fail('data.notFound', `${overlay.base.id} ${base.key}`)
        const keys = Object.keys(values).filter((k) => values[k] !== undefined)
        try {
          encode(shapeOf(overlay, keys), Object.fromEntries(keys.map((k) => [k, values[k]])) as never)
        } catch (e) {
          fail('data.invalidValue', (e as Error).message)
        }
        const existing = scope.store.overlay(overlay.id).get(base.key)
        const row: Row = existing === undefined ? { baseKey: base.key } : structuredClone(existing)
        for (const [k, f] of Object.entries(overlay.fields)) {
          if (values[k] !== undefined) row[k] = structuredClone(values[k])
          else if (!(k in row)) row[k] = f.spec.default ?? null
        }
        scope.store.overlay(overlay.id).set(base.key, row)
        return project(overlay, row)
      },
      call: async (callable: Callable, input: unknown) => {
        guard()
        const c = callable as { id: string; kind: string; atomic?: boolean }
        if (!(c.kind === 'operation' || (c.kind === 'command' && c.atomic === true))) {
          fail('runtime.callNotAtomic', `tx.call accepts only operations and atomic commands, not ${c.kind === 'command' ? 'ordinary command ' : c.kind + ' '}"${c.id}"`)
        }
        return callCallable(tx as unknown as Tx, callable, input)
      },
      emit: async (event: { id: string; subject?: AnyEntity; payload: AnySchema }, args: { subject?: Ref; payload: unknown }) => {
        guard()
        if (event.subject !== undefined && args.subject?.entity !== event.subject.id) fail('runtime.invalidInput', `event "${event.id}" needs a subject of "${event.subject.id}"`)
        scope.pending.emitted.push({ event: event.id, subject: args.subject, payload: encode(event.payload, args.payload as never), initiator: user.userId })
      },
      notify: async (notice: { id: string; subject?: AnyEntity; payload: AnySchema }, args: { subject?: Ref; payload: unknown; to: readonly object[]; channels: readonly string[] }) => {
        guard()
        if (notice.subject !== undefined && args.subject?.entity !== notice.subject.id) fail('runtime.invalidInput', `notice "${notice.id}" needs a subject of "${notice.subject.id}"`)
        if (args.to.length === 0 || args.channels.length === 0) fail('runtime.invalidInput', `notice "${notice.id}" needs recipients and channels`)
        scope.pending.notices.push({ notice: notice.id, subject: args.subject, payload: encode(notice.payload, args.payload as never), to: json(args.to), channels: [...args.channels] })
      },
      start: async (wf: Workflow, input: unknown, opts?: { key?: string }) => {
        guard()
        if (!scope.derived && opts?.key === undefined) fail('runtime.startKeyRequired', `tx.start("${wf.id}") needs { key } outside subscriptions and workflows`)
        const known = opts?.key === undefined ? undefined : [...committed.started, ...scope.pending.started].find((x) => x.workflow === wf.id && x.key === opts.key)
        if (known !== undefined) return { workflow: wf.id, instance: known.instance }
        const instance = nextId('wf')
        scope.pending.started.push({ workflow: wf.id, instance, key: opts?.key, input: encode(wf.input, validateInput(wf, input) as never) })
        workflowDefs.set(wf.id, wf as AnyWorkflow)
        return { workflow: wf.id, instance }
      },
      signal: async (target: WorkflowInstance, name: string, payload: unknown) => {
        guard()
        scope.pending.signals.push(checkSignal(target, name, payload, scope.pending.started))
      },
    }
    return tx as unknown as Tx<'plain'> & Tx<'derived'>
  }

  const flush = (pending: Pending): void => {
    committed.emitted.push(...pending.emitted)
    committed.notices.push(...pending.notices)
    committed.started.push(...pending.started)
    for (const signal of pending.signals) deliverSignal(signal)
  }

  /** Runs `body` in a transaction on a working copy: adopted on success, discarded on throw. */
  const inTransaction = async <T>(body: (tx: Tx<'plain'> & Tx<'derived'>) => Promise<T>, derived: boolean, info = invocation()): Promise<T> => {
    const scope: Scope = { store: store.clone(), pending: { emitted: [], notices: [], started: [], signals: [] }, open: true, derived, info }
    const tx = makeTx(scope)
    try {
      const result = await body(tx)
      scope.open = false
      store = scope.store
      flush(scope.pending)
      return result
    } catch (e) {
      scope.open = false
      throw e
    }
  }

  const queryContext = (info = invocation()): QueryContext => readContext(() => store, info)

  const commandContext = (info = invocation()): CommandContext => {
    let inTx = false
    const snapshot = queryContext(info)
    const ctx: CommandContext = {
      user,
      invocation: info,
      log: logger(),
      secrets,
      // Callable like the worker's `ctx.query`: it is the read-only snapshot context and runs a query.
      query: Object.assign((q: Query, input: unknown) => snapshot.query(q, input), snapshot) as unknown as CommandContext['query'],
      async transaction<T>(callback: (tx: Tx) => Promise<T>): Promise<T> {
        if (inTx) fail('runtime.nestedTransaction', 'ctx.transaction cannot be nested')
        inTx = true
        try {
          return await inTransaction((tx) => callback(tx), false, info)
        } finally {
          inTx = false
        }
      },
      call: (async (command: OrdinaryCommand | Command<string, AnySchema, AnySchema | undefined, true>, input: unknown) => invokeCommand(command, input, info.id)) as CommandContext['call'],
      signal: (async (target: WorkflowInstance, name: string, payload: unknown) => deliverSignal(checkSignal(target, name, payload, []))) as CommandContext['signal'],
    }
    return ctx
  }

  /** A new command invocation: prechecks and the bound implementation, in the right context kind. */
  const invokeCommand = async (command: { id: string; atomic: boolean; input: AnySchema; output: AnySchema | undefined }, input: unknown, parent?: string): Promise<unknown> => {
    const valid = validateInput(command, input)
    const info = invocation(parent)
    const finish = (out: unknown): unknown => (command.output === undefined ? undefined : decode(command.output, encode(command.output, out as never)))
    if (command.atomic) {
      return finish(await inTransaction((tx) => callCallable(tx as unknown as Tx, command as never, valid), false, info))
    }
    for (const g of prechecks.get(command.id) ?? []) await g.handler(guardContext(queryContext(info)), valid as never)
    return finish(await (implementationOf(command.id).handler as unknown as (ctx: CommandContext, input: unknown) => Awaitable<unknown>)(commandContext(info), valid))
  }

  // ---- workflows

  class Suspend extends Error {
    readonly on: PendingWork
    constructor(on: PendingWork) {
      super(`workflow suspended on ${on.kind} "${on.name}"`)
      this.name = 'Suspend'
      this.on = on
    }
  }

  interface Occurrence {
    kind: string
    status: 'completed' | 'waiting'
    result?: unknown
    wakeAt?: number
    attempts: number
  }
  interface TaskState extends OpenTask {
    schema: AnySchema
    due?: number
    decision?: unknown
    by?: string
    mutableStatus: 'open' | 'decided' | 'expired'
  }

  const createWorkflow = (impl: Implementation<AnyWorkflow & Implementable>, input: unknown, instanceId?: string): WorkflowRun => {
    const instance = instanceId ?? nextId('wf')
    const workflow = impl.contract as AnyWorkflow
    const validInput = validateInput(workflow, input)
    const startedAs = committed.started.find((x) => x.instance === instance)
    if (startedAs !== undefined && startedAs.workflow !== workflow.id) {
      fail('workflow.unknownInstance', `instance "${instance}" was started as "${startedAs.workflow}", not "${workflow.id}"`)
    }
    if (runs.has(instance)) fail('test.duplicateRun', `instance "${instance}" already has a run in this test host`)
    const journal = new Map<string, Occurrence>()
    const tasks: TaskState[] = []
    // Signals that reached the instance before the test created its run are consumed in arrival order.
    const signals: { name: string; payload: unknown; consumed: boolean }[] = (inboxes.get(instance) ?? []).map((x) => ({ ...x, consumed: false }))
    inboxes.delete(instance)
    workflowDefs.set(workflow.id, workflow)
    runs.set(instance, { workflow, push: (name, payload) => void signals.push({ name, payload, consumed: false }) })

    const run = async (): Promise<WorkflowOutcome<unknown>> => {
      const counters = new Map<string, number>()
      const occurrence = (name: string, kind: string): { key: string; n: number; occ: Occurrence | undefined } => {
        const n = counters.get(name) ?? 0
        counters.set(name, n + 1)
        const key = `${name}#${n}`
        const occ = journal.get(key)
        if (occ !== undefined && occ.kind !== kind) fail('workflow.nondeterminism', `occurrence ${key} was ${occ.kind}, now ${kind}`)
        return { key, n, occ }
      }
      const done = (key: string, kind: string, result: unknown, prior?: Occurrence): void =>
        void journal.set(key, { kind, status: 'completed', result: result === undefined ? null : json(result), attempts: (prior?.attempts ?? 0) + 1 })

      const wf: WorkflowContext = {
        user,
        instance: { id: instance },
        log: logger(),
        secrets,
        transaction: (async (name: string, body: unknown, txInput?: unknown) => {
          const { key, occ } = occurrence(name, 'transaction')
          if (occ?.status === 'completed') return occ.result
          // Overloads: (name, callback) | (name, { output }, callback) | (name, callable, input).
          const callback = typeof body === 'function' ? body : typeof txInput === 'function' ? txInput : undefined
          const schema = callback !== undefined && isRecord(body) ? (body.output as AnySchema | undefined) : undefined
          const result =
            callback !== undefined
              ? await inTransaction(async (tx) => (callback as (tx: Tx<'derived'>) => Awaitable<unknown>)(tx), true)
              : await inTransaction((tx) => callCallable(tx as unknown as Tx, body as Callable, txInput), true)
          if (schema !== undefined) encode(schema, result as never)
          done(key, 'transaction', result)
          return result
        }) as WorkflowContext['transaction'],
        step: (async (name: string, a: unknown, b?: unknown) => {
          const effect = (typeof a === 'function' ? a : b) as (step: StepContext) => Promise<unknown>
          const schema = isRecord(a) ? (a.output as AnySchema | undefined) : undefined
          const { key, n, occ } = occurrence(name, 'step')
          if (occ?.status === 'completed') return occ.result
          const attempt: Occurrence = occ ?? { kind: 'step', status: 'waiting', attempts: 0 }
          journal.set(key, { ...attempt, attempts: attempt.attempts + 1 })
          const value = await effect({ idempotencyKey: `${instance}:${name}:${n}` })
          if (schema !== undefined) encode(schema, value as never)
          done(key, 'step', value, journal.get(key))
          return value
        }) as WorkflowContext['step'],
        sleep: async (name: string, duration: Duration) => {
          const { key, n, occ } = occurrence(name, 'sleep')
          if (occ?.status === 'completed') return
          const wakeAt = occ?.wakeAt ?? clockMs + durationMs(duration)
          if (clockMs >= wakeAt) return done(key, 'sleep', null)
          journal.set(key, { kind: 'sleep', status: 'waiting', wakeAt, attempts: 1 })
          throw new Suspend({ kind: 'sleep', name, n, wakeAt: new Date(wakeAt).toISOString() as ISODateTime })
        },
        waitFor: (async (name: string, signal: { name: string }, opts?: { timeout: Duration }) => {
          const { key, n, occ } = occurrence(name, 'waitFor')
          const reply = (result: SignalResult<unknown>): unknown => (opts === undefined ? (result as { payload?: unknown }).payload : result)
          if (occ?.status === 'completed') return reply(occ.result as SignalResult<unknown>)
          const queued = signals.find((s) => !s.consumed && s.name === signal.name)
          if (queued !== undefined) {
            queued.consumed = true
            const result: SignalResult<unknown> = { outcome: 'received', payload: queued.payload }
            done(key, 'waitFor', result)
            return reply(result)
          }
          const wakeAt = opts === undefined ? undefined : (occ?.wakeAt ?? clockMs + durationMs(opts.timeout))
          if (wakeAt !== undefined && clockMs >= wakeAt) {
            done(key, 'waitFor', { outcome: 'timeout' })
            return reply({ outcome: 'timeout' })
          }
          journal.set(key, { kind: 'waitFor', status: 'waiting', ...(wakeAt === undefined ? {} : { wakeAt }), attempts: 1 })
          throw new Suspend({ kind: 'signal', name, n })
        }) as WorkflowContext['waitFor'],
        task: (async (name: string, spec: TaskSpec<AnySchema>) => {
          const { key, n, occ } = occurrence(name, 'task')
          if (occ?.status === 'completed') return occ.result
          let task = tasks.find((t) => t.name === name && t.n === n)
          if (task === undefined) {
            task = {
              name, n, subject: spec.subject, assignee: spec.assignee, status: 'open', mutableStatus: 'open', schema: spec.decision,
              ...(spec.timeout === undefined ? {} : { due: clockMs + durationMs(spec.timeout) }),
            }
            tasks.push(task)
          }
          if (task.mutableStatus === 'decided') {
            const result: TaskResult<unknown> = { outcome: 'decided', decision: task.decision, by: (task.by ?? user.userId) as never }
            done(key, 'task', result)
            return result
          }
          if (task.due !== undefined && clockMs >= task.due) {
            task.mutableStatus = 'expired'
            done(key, 'task', { outcome: 'timeout' })
            return { outcome: 'timeout' }
          }
          journal.set(key, { kind: 'task', status: 'waiting', attempts: 1 })
          throw new Suspend({ kind: 'task', name, n })
        }) as WorkflowContext['task'],
        call: (async (name: string, command: { id: string; atomic: boolean; input: AnySchema; output: AnySchema | undefined }, callInput: unknown) => {
          const { key, occ } = occurrence(name, 'call')
          if (occ?.status === 'completed') return occ.result
          const result = await invokeCommand(command, callInput, instance)
          done(key, 'call', result)
          return result
        }) as WorkflowContext['call'],
        start: (async (name: string, child: Workflow, childInput: unknown) => {
          const { key, n, occ } = occurrence(name, 'start')
          if (occ?.status === 'completed') return occ.result as WorkflowInstance
          const childInstance = nextId('wf')
          workflowDefs.set(child.id, child as AnyWorkflow)
          committed.started.push({ workflow: child.id, instance: childInstance, key: `${instance}:${name}:${n}`, input: encode(child.input, validateInput(child, childInput) as never) })
          const handle = { workflow: child.id, instance: childInstance }
          done(key, 'start', handle)
          return handle
        }) as WorkflowContext['start'],
        now: async (name: string) => {
          const { key, occ } = occurrence(name, 'now')
          if (occ?.status === 'completed') return occ.result as ISODateTime
          const at = nowIso()
          done(key, 'now', at)
          return at
        },
        uuid: async (name: string) => {
          const { key, occ } = occurrence(name, 'uuid')
          if (occ?.status === 'completed') return occ.result as string
          const id = nextKey()
          done(key, 'uuid', id)
          return id
        },
      }
      try {
        const value = await (impl.handler as (wf: WorkflowContext, input: unknown) => Awaitable<unknown>)(wf, validInput)
        return { status: 'completed', value }
      } catch (e) {
        if (e instanceof Suspend) return { status: 'suspended', on: e.on }
        return { status: 'failed', error: e }
      }
    }

    const driver = {
      instance,
      run,
      decide(name: string, decision: unknown, opts: { by?: string; n?: number } = {}) {
        const task = tasks.find((t) => t.name === name && t.mutableStatus === 'open' && (opts.n === undefined || t.n === opts.n))
        if (task === undefined) return fail('workflow.noOpenTask', `no open task "${name}" in ${instance}`)
        try {
          encode(task.schema, decision as never)
        } catch (e) {
          return fail('workflow.invalidDecision', (e as Error).message)
        }
        task.decision = json(decision)
        task.by = opts.by ?? user.userId
        task.mutableStatus = 'decided'
      },
      signal(signal: { readonly name: string } | string, payload: unknown) {
        const name = typeof signal === 'string' ? signal : signal.name
        deliverSignal(checkSignal({ workflow: workflow.id, instance }, name, payload, []))
      },
      get tasks() {
        return tasks.map((t) => ({ name: t.name, n: t.n, subject: t.subject, assignee: t.assignee, status: t.mutableStatus }))
      },
      get journal() {
        return [...journal].filter(([, o]) => o.status === 'completed').map(([k]) => k)
      },
    }
    return driver as unknown as WorkflowRun
  }

  // ---- public surface

  const host: TestHost = {
    fakes,
    user,
    clock: {
      now: nowIso,
      advance: (d) => void (clockMs += durationMs(d)),
    },
    get emitted() {
      return committed.emitted
    },
    get notices() {
      return committed.notices
    },
    get started() {
      return committed.started
    },
    get signals() {
      return committed.signals
    },
    get logs() {
      return logs
    },
    query: () => queryContext(),
    command: () => commandContext(),
    transaction: (callback) => inTransaction(async (tx) => callback(tx), false),
    deliver: (async (subscription: Subscription, event: { subject?: Ref; payload: Json; initiator?: string }) => {
      const payload = decode(subscription.event.payload, event.payload)
      const delivered: SubscriptionEvent = {
        id: nextId('evt'),
        event: subscription.event.id,
        occurredAt: nowIso(),
        subject: event.subject as never,
        payload,
        initiator: (event.initiator ?? user.userId) as never,
      }
      await inTransaction(async (tx) => {
        await subscription.handler(tx, delivered as never)
      }, true)
    }) as TestHost['deliver'],
    workflow: ((impl: Implementation<AnyWorkflow & Implementable>, input: unknown, opts?: { instance?: string }) =>
      createWorkflow(impl, input, opts?.instance)) as TestHost['workflow'],
    table: ((entity: AnyEntity) => {
      register(entity)
      return [...store.table(entity.id).values()].map((r) => project(entity, r))
    }) as TestHost['table'],
    overlayRow: ((overlay: AnyOverlay, base: Ref) => {
      register(overlay)
      const row = store.overlay(overlay.id).get(base.key)
      return row === undefined ? null : project(overlay, row)
    }) as TestHost['overlayRow'],
  }
  return host
}
