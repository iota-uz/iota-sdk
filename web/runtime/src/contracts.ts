import type { Json, Label, ResourceJSON } from './model.ts'
import { contract, omitEmpty, type Contract } from './resource.ts'
import type { AnyField, AnyEntity, BoundMeta } from './entity.ts'
import type { AnySchema, Infer } from './schema.ts'

export type OutOf<O> = O extends AnySchema ? Infer<O> : void

// ---- permissions, errors, issues ----

export interface Permission<Id extends string = string> extends Contract<Id, 'permission'> {
  readonly label: Label
}

export function permission<const Id extends string>(id: Id, spec: { label: Label }): Permission<Id> {
  return { ...contract({ id, kind: 'permission', label: spec.label }), label: spec.label }
}

export interface ValidationIssue<P = unknown> {
  readonly code: string
  readonly path: readonly string[]
  readonly params: P
}

/** Error thrown by handlers; the host maps it to the wire error `{kind, code, params, issues}`. */
export class ContractError<P = unknown> extends Error {
  readonly kind: 'domain' | 'validation'
  readonly code: string
  readonly params: P
  readonly issues: readonly ValidationIssue[]

  constructor(kind: 'domain' | 'validation', code: string, params: P, issues: readonly ValidationIssue[] = []) {
    super(`${code}${issues.length > 0 ? ` (${issues.map((i) => `${i.path.join('.')}: ${i.code}`).join(', ')})` : ''}`)
    this.name = 'ContractError'
    this.kind = kind
    this.code = code
    this.params = params
    this.issues = issues
  }
}

export interface DomainError<Id extends string = string, P extends AnySchema = AnySchema> extends Contract<Id, 'domainError'> {
  readonly params: P
  /** `throw NotActive.of({ status })` */
  of(params: Infer<P>): ContractError<Infer<P>>
}

export function domainError<const Id extends string, P extends AnySchema>(id: Id, params: P): DomainError<Id, P> {
  return {
    ...contract({ id, kind: 'domainError', params: params.type }),
    params,
    of: (value) => new ContractError('domain', id, value),
  }
}

export interface Issue<Id extends string = string, P extends AnySchema = AnySchema> extends Contract<Id, 'issue'> {
  readonly params: P
  /** `validation(TooLong.at(["notes"], { max: 10 }))` */
  at(path: readonly string[], params: Infer<P>): ValidationIssue<Infer<P>>
}

export function issue<const Id extends string, P extends AnySchema>(id: Id, params: P): Issue<Id, P> {
  return {
    ...contract({ id, kind: 'issue', params: params.type }),
    params,
    at: (path, value) => ({ code: id, path, params: value }),
  }
}

export function validation(...issues: readonly [ValidationIssue, ...ValidationIssue[]]): ContractError<null> {
  return new ContractError('validation', 'validation', null, issues)
}

// ---- callable contracts ----

interface CallableSpec<I extends AnySchema, O extends AnySchema | undefined> {
  input: I
  output?: O
  errors?: readonly DomainError[]
  permission?: Permission
  label?: Label
  description?: string
}

function callable(
  id: string,
  kind: 'query' | 'command' | 'operation',
  spec: CallableSpec<AnySchema, AnySchema | undefined>,
  atomic: boolean,
): ResourceJSON {
  return omitEmpty({
    id,
    kind,
    label: spec.label,
    description: spec.description,
    atomic: atomic || undefined,
    input: spec.input.type,
    output: spec.output?.type,
    errors: spec.errors?.map((e) => e.id),
    permission: spec.permission?.id,
  }) as ResourceJSON
}

export interface Query<Id extends string = string, I extends AnySchema = AnySchema, O extends AnySchema = AnySchema>
  extends Contract<Id, 'query'> {
  readonly input: I
  readonly output: O
}

export function query<const Id extends string, I extends AnySchema, O extends AnySchema>(
  id: Id,
  spec: CallableSpec<I, O> & { output: O },
): Query<Id, I, O> {
  return { ...contract({ ...callable(id, 'query', spec, false), id, kind: 'query' }), input: spec.input, output: spec.output }
}

export interface Command<
  Id extends string = string,
  I extends AnySchema = AnySchema,
  O extends AnySchema | undefined = AnySchema | undefined,
  A extends boolean = boolean,
> extends Contract<Id, 'command'> {
  readonly input: I
  readonly output: O
  /** Part of the public contract: every implementation, including replacements, receives `Tx`. */
  readonly atomic: A
}

export type OrdinaryCommand = Command<string, AnySchema, AnySchema | undefined, false>
export type AtomicCommand = Command<string, AnySchema, AnySchema | undefined, true>

/** An ordinary command is an intent with an async `CommandContext`; `atomic: true` runs wholly in one transaction. */
export function command<const Id extends string, I extends AnySchema, O extends AnySchema | undefined = undefined>(
  id: Id,
  spec: CallableSpec<I, O> & { atomic: true },
): Command<Id, I, O, true>
export function command<const Id extends string, I extends AnySchema, O extends AnySchema | undefined = undefined>(
  id: Id,
  spec: CallableSpec<I, O> & { atomic?: false },
): Command<Id, I, O, false>
export function command(id: string, spec: CallableSpec<AnySchema, AnySchema | undefined> & { atomic?: boolean }): Command {
  const atomic = spec.atomic === true
  return { ...contract({ ...callable(id, 'command', spec, atomic), id, kind: 'command' }), input: spec.input, output: spec.output, atomic }
}

/** A named atomic domain primitive; `tx.call` joins the caller's transaction. */
export interface Operation<Id extends string = string, I extends AnySchema = AnySchema, O extends AnySchema | undefined = AnySchema | undefined>
  extends Contract<Id, 'operation'> {
  readonly input: I
  readonly output: O
  readonly atomic: true
}

export function operation<const Id extends string, I extends AnySchema, O extends AnySchema | undefined = undefined>(
  id: Id,
  spec: CallableSpec<I, O>,
): Operation<Id, I, O> {
  return { ...contract({ ...callable(id, 'operation', spec, false), id, kind: 'operation' }), input: spec.input, output: spec.output as O, atomic: true }
}

export type Callable = Operation | AtomicCommand
export type InputOf<C extends { readonly input: AnySchema }> = Infer<C['input']>
export type ResultOf<C extends { readonly output: AnySchema | undefined }> = OutOf<C['output']>

// ---- events, notices, workflows ----

export interface Event<Id extends string = string, S extends AnyEntity | undefined = AnyEntity | undefined, P extends AnySchema = AnySchema>
  extends Contract<Id, 'event'> {
  readonly subject: S
  readonly payload: P
}

export function event<const Id extends string, P extends AnySchema, S extends AnyEntity | undefined = undefined>(
  id: Id,
  spec: { subject?: S; payload: P; description?: string },
): Event<Id, S, P> {
  const resource = omitEmpty({ id, kind: 'event' as const, description: spec.description, subject: spec.subject?.id, payload: spec.payload.type })
  return { ...contract(resource), subject: spec.subject as S, payload: spec.payload }
}

export interface Notice<Id extends string = string, S extends AnyEntity | undefined = AnyEntity | undefined, P extends AnySchema = AnySchema>
  extends Contract<Id, 'notice'> {
  readonly subject: S
  readonly payload: P
}

export function notice<const Id extends string, P extends AnySchema, S extends AnyEntity | undefined = undefined>(
  id: Id,
  spec: { subject?: S; payload: P; description?: string },
): Notice<Id, S, P> {
  const resource = omitEmpty({ id, kind: 'notice' as const, description: spec.description, subject: spec.subject?.id, payload: spec.payload.type })
  return { ...contract(resource), subject: spec.subject as S, payload: spec.payload }
}

/** Handle to a signal of a workflow, passed to `wf.waitFor`. */
export interface SignalRef<S extends AnySchema = AnySchema> {
  readonly workflow: string
  readonly name: string
  readonly schema: S
}

export interface Workflow<
  Id extends string = string,
  I extends AnySchema = AnySchema,
  O extends AnySchema | undefined = AnySchema | undefined,
  G extends Readonly<Record<string, AnySchema>> = Readonly<Record<string, AnySchema>>,
> extends Contract<Id, 'workflow'> {
  readonly input: I
  readonly output: O
  readonly signals: { readonly [K in keyof G]: SignalRef<G[K]> }
}

export function workflow<
  const Id extends string,
  I extends AnySchema,
  O extends AnySchema | undefined = undefined,
  G extends Readonly<Record<string, AnySchema>> = Record<never, AnySchema>,
>(id: Id, spec: { input: I; output?: O; signals?: G; description?: string }): Workflow<Id, I, O, G> {
  const signalTypes = Object.fromEntries(Object.entries(spec.signals ?? {}).map(([k, s]) => [k, s.type]))
  const resource = omitEmpty({
    id,
    kind: 'workflow' as const,
    description: spec.description,
    input: spec.input.type,
    output: spec.output?.type,
    signals: Object.keys(signalTypes).length > 0 ? signalTypes : undefined,
  }) as ResourceJSON
  const signals = Object.fromEntries(
    Object.entries(spec.signals ?? {}).map(([name, schema]) => [name, { workflow: id, name, schema }]),
  )
  return { ...contract({ ...resource, id, kind: 'workflow' }), input: spec.input, output: spec.output as O, signals: signals as unknown as Workflow<Id, I, O, G>['signals'] }
}

// ---- routes ----

export interface Route<Id extends string = string, P extends AnySchema = AnySchema> extends Contract<Id, 'route'> {
  readonly path: string
  readonly params: P
}

export function route<const Id extends string, P extends AnySchema>(id: Id, spec: { path: string; params: P }): Route<Id, P> {
  return { ...contract({ id, kind: 'route', path: spec.path, params: spec.params.type }), path: spec.path, params: spec.params }
}

/** Concrete URL of a route: `/insurance/policies/:id` + `{ id }`. */
export function href<R extends Route>(r: R, params: Infer<R['params']>): string {
  const values = params as Readonly<Record<string, Json | undefined>>
  return r.path.replace(/:([A-Za-z0-9_]+)/g, (_, name: string) => encodeURIComponent(String(values[name] ?? '')))
}

// ---- notification contracts (#926) ----

export type ContactTargets = Readonly<Partial<Record<'sms' | 'email' | 'push', readonly string[]>>>

/** Resolves a record recipient of `E` to channel targets. Implemented with `implement(...)` on the server. */
export interface ContactResolver<Id extends string = string, E extends AnyEntity = AnyEntity> extends Contract<Id, 'contactResolver'> {
  readonly entity: E
}

export function contactResolver<const Id extends string, E extends AnyEntity>(id: Id, entity: E): ContactResolver<Id, E> {
  return { ...contract({ id, kind: 'contactResolver', subject: entity.id }), entity }
}

/** Business consent/opt-out rule for records of `E`; the decision applies per notice, channel and target. */
export interface DeliveryPolicy<Id extends string = string, E extends AnyEntity = AnyEntity> extends Contract<Id, 'deliveryPolicy'> {
  readonly entity: E
}

export function deliveryPolicy<const Id extends string, E extends AnyEntity>(id: Id, entity: E): DeliveryPolicy<Id, E> {
  return { ...contract({ id, kind: 'deliveryPolicy', subject: entity.id }), entity }
}

// ---- native list columns ----

/** A native list owned by the platform (from the kit), e.g. `crm.lists.Clients`. */
export interface ListRef<Id extends string = string> {
  readonly listId: Id
}

export function nativeList<const Id extends string>(id: Id): ListRef<Id> {
  return { listId: id }
}

export interface Column<Id extends string = string> extends Contract<Id, 'column'> {
  readonly list: string
}

export interface ColumnSpec {
  id: string
  /** An overlay field, e.g. `ClientExtra.fields.loyaltyTier`. */
  field: AnyField & BoundMeta
  label: Label
  filterable?: boolean
  sortable?: boolean
  exportable?: boolean
}

/** Adds an overlay field as a column of a native list, filter, sort and export. */
export function column<const S extends ColumnSpec>(list: ListRef, spec: S & { id: S['id'] }): Column<S['id']> {
  const resource = omitEmpty({
    id: spec.id,
    kind: 'column' as const,
    label: spec.label,
    list: list.listId,
    overlay: spec.field.owner,
    field: spec.field.key,
    filterable: spec.filterable,
    sortable: spec.sortable,
    exportable: spec.exportable,
  }) as ResourceJSON
  return { ...contract({ ...resource, id: spec.id, kind: 'column' }), list: list.listId }
}
