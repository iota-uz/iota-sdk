import type { AccessJSON, ConditionJSON, FieldJSON, FilterJSON, Json, Label, ReadRuleJSON, TypeJSON } from './model.ts'
import { contract, omitEmpty, type Contract } from './resource.ts'
import { makeSchema, t, type DecimalOptions, type Infer, type Presence, type Ref, type Schema, type StringOptions } from './schema.ts'
import type { Permission } from './contracts.ts'

export interface FieldSpec {
  readonly unique: boolean
  readonly index: boolean
  readonly default?: Json
  readonly renamedFrom: readonly string[]
  readonly label?: Label
}

/** A typed column. `.optional()` is a nullable column that may be omitted on insert. */
export interface Field<T = unknown, K extends Presence = 'req'> extends Schema<T, K> {
  readonly spec: FieldSpec
  optional(): Field<T, 'opt'>
  nullable(): Field<T | null, K>
  unique(): Field<T, K>
  index(): Field<T, K>
  default(value: T): Field<T, 'def'>
  /** Previous field keys; the physical column follows the field across renames. */
  renamedFrom(...keys: string[]): Field<T, K>
  label(label: Label): Field<T, K>
}

export type AnyField = Field<unknown, Presence>
export type FieldShape = Readonly<Record<string, AnyField>>

const emptySpec: FieldSpec = { unique: false, index: false, renamedFrom: [] }

function makeField<T, K extends Presence>(type: TypeJSON, spec: FieldSpec = emptySpec): Field<T, K> {
  const base = makeSchema<T, K>(type)
  const next = <T2, K2 extends Presence>(patch: Partial<FieldSpec>, nextType: TypeJSON = type): Field<T2, K2> =>
    makeField<T2, K2>(nextType, { ...spec, ...patch })
  return {
    ...base,
    spec,
    optional: () => next<T, 'opt'>({}, { ...type, optional: true, nullable: true }),
    nullable: () => next<T | null, K>({}, { ...type, nullable: true }),
    unique: () => next<T, K>({ unique: true }),
    index: () => next<T, K>({ index: true }),
    default: (value: T) => next<T, 'def'>({ default: value as Json }),
    renamedFrom: (...keys: string[]) => next<T, K>({ renamedFrom: [...spec.renamedFrom, ...keys] }),
    label: (label: Label) => next<T, K>({ label }),
  }
}

function fieldOf<S extends Schema>(schema: S): Field<Infer<S>> {
  return makeField<Infer<S>, 'req'>(schema.type)
}

const text = (o?: StringOptions) => fieldOf(t.string(o))

export const field = {
  text,
  string: text,
  int: () => fieldOf(t.int()),
  int64: () => fieldOf(t.int64()),
  decimal: (o: DecimalOptions) => fieldOf(t.decimal(o)),
  bool: () => fieldOf(t.bool()),
  date: () => fieldOf(t.date()),
  datetime: () => fieldOf(t.datetime()),
  uuid: () => fieldOf(t.uuid()),
  enum: <const V extends readonly [string, ...string[]]>(values: V) => fieldOf(t.enum(values)),
  json: <T = Json>() => fieldOf(t.json<T>()),
  ref: <E extends EntityLike>(entity: E) => fieldOf(t.ref(entity)),
} as const

type EntityLike = { readonly id: string }

export interface BoundMeta {
  readonly owner: string
  readonly key: string
}
export type BoundFields<F extends FieldShape> = { readonly [K in keyof F]: F[K] & BoundMeta }

export type EntityOrigin = 'own' | 'native'
export type KeyType = 'int64' | 'uuid' | 'string'

export interface Entity<Id extends string = string, F extends FieldShape = FieldShape, O extends EntityOrigin = EntityOrigin>
  extends Contract<Id, 'entity'> {
  readonly entityKind: O
  readonly keyType: KeyType
  readonly fields: BoundFields<F>
}

export type OwnEntity = Entity<string, FieldShape, 'own'>
export type AnyEntity = Entity<string, FieldShape, EntityOrigin>

export interface Overlay<Id extends string = string, B extends AnyEntity = AnyEntity, F extends FieldShape = FieldShape>
  extends Contract<Id, 'overlay'> {
  readonly base: B
  readonly fields: BoundFields<F>
}
export type AnyOverlay = Overlay<string, AnyEntity, FieldShape>

// ---- values read from / written to records ----

type Simplify<T> = { [K in keyof T]: T[K] }
export type FieldValue<S> = S extends Schema<infer V, infer K> ? (K extends 'opt' ? V | null : V) : never

type Fields<E> = E extends { readonly fields: infer G extends FieldShape } ? G : never

/** A record as read: `key` plus every declared field (optional ones read `null`). */
export type RecordOf<E> = Simplify<{ readonly key: string } & { readonly [K in keyof Fields<E>]: FieldValue<Fields<E>[K]> }>

type RequiredInsertKeys<F extends FieldShape> = { [K in keyof F]: F[K] extends Schema<unknown, 'req'> ? K : never }[keyof F]
type OptionalInsertKeys<F extends FieldShape> = Exclude<keyof F, RequiredInsertKeys<F>>

/** Values accepted by `tx.insert`: fields that are optional or defaulted may be omitted. */
export type InsertOf<E> = Simplify<
  { [K in RequiredInsertKeys<Fields<E>>]: Infer<Fields<E>[K]> } & { [K in OptionalInsertKeys<Fields<E>>]?: Infer<Fields<E>[K]> }
>
export type PatchOf<E> = { [K in keyof Fields<E>]?: Infer<Fields<E>[K]> }

// ---- filters ----

export interface UserPlaceholder {
  readonly $: 'user'
}
/** `$.user` is the current subject's user ref inside access rules and filters. */
export const $ = { user: { $: 'user' } as UserPlaceholder } as const

type Depth = readonly unknown[]
type Prefixed<K extends string, E> = E extends readonly [infer P extends string, infer V] ? readonly [`${K}.${P}`, V] : never
type PathEntry<F extends FieldShape, D extends Depth = readonly []> = {
  [K in keyof F & string]:
    | readonly [K, FieldValue<F[K]>]
    | (NonNullable<Infer<F[K]>> extends Ref<infer E>
        ? D['length'] extends 2
          ? never
          : Fields<E> extends infer G extends FieldShape
            ? Prefixed<K, PathEntry<G, readonly [...D, unknown]>>
            : never
        : never)
}[keyof F & string]

/** Field keys, plus dotted paths through ref fields ("holder.owner"). */
export type PathOf<E> = PathEntry<Fields<E>>[0]

type Val<V> = NonNullable<V> extends Ref ? V | UserPlaceholder : V
export interface Ops<V> {
  eq?: Val<V>
  ne?: Val<V>
  in?: readonly Val<V>[]
  lt?: V
  lte?: V
  gt?: V
  gte?: V
  isNull?: boolean
  like?: string
}

/** Conjunction of conditions on fields or ref paths; a bare value means equality. */
export type Where<E> = {
  [P in PathEntry<Fields<E>> as P[0]]?: Val<P[1]> | Ops<P[1]>
} & { or?: readonly Where<E>[] }

const opKeys = new Set(['eq', 'ne', 'in', 'lt', 'lte', 'gt', 'gte', 'isNull', 'like'])

function toCondition(value: unknown): ConditionJSON {
  if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
    const keys = Object.keys(value)
    if (keys.length > 0 && keys.every((k) => opKeys.has(k))) return { ...(value as ConditionJSON) }
  }
  return { eq: value as Json }
}

export function compileFilter(where: object): FilterJSON {
  const fields: Record<string, ConditionJSON> = {}
  let or: FilterJSON[] | undefined
  for (const [key, value] of Object.entries(where)) {
    if (value === undefined) continue
    if (key === 'or') or = (value as readonly object[]).map(compileFilter)
    else fields[key] = toCondition(value)
  }
  return omitEmpty({ fields: Object.keys(fields).length > 0 ? fields : undefined, or })
}

// ---- access ----

export interface ReadRule<E> {
  /** Permission the subject must hold; omitted means any authenticated subject. */
  when?: Permission
  rows: 'all' | Where<E>
}

export interface EntityAccess<F extends FieldShape> {
  read?: readonly ReadRule<{ readonly fields: F }>[]
  /** Permissions required to read a field; applies to every surface. */
  fields?: { readonly [K in keyof F]?: readonly Permission[] }
}

export interface EntityOptions<F extends FieldShape> {
  label?: Label
  description?: string
  access?: EntityAccess<F>
}

function compileField(id: string, f: AnyField): FieldJSON {
  return omitEmpty({
    id,
    type: f.type,
    unique: f.spec.unique,
    index: f.spec.index,
    default: f.spec.default,
    renamedFrom: [...f.spec.renamedFrom],
    label: f.spec.label,
  }) as FieldJSON
}

function compileAccess<F extends FieldShape>(access: EntityAccess<F> | undefined): AccessJSON | undefined {
  if (!access) return undefined
  const read: ReadRuleJSON[] = (access.read ?? []).map((r) =>
    omitEmpty({
      permission: r.when?.id,
      all: r.rows === 'all',
      rows: r.rows === 'all' ? undefined : compileFilter(r.rows),
    }) as ReadRuleJSON,
  )
  const fields: Record<string, string[]> = {}
  for (const [k, perms] of Object.entries(access.fields ?? {})) {
    if (perms) fields[k] = perms.map((p) => p.id)
  }
  return omitEmpty({ read, fields: Object.keys(fields).length > 0 ? fields : undefined }) as AccessJSON
}

function bind<F extends FieldShape>(owner: string, fields: F): BoundFields<F> {
  const out: Record<string, AnyField & BoundMeta> = {}
  for (const [key, f] of Object.entries(fields)) {
    if (key === 'key') throw new Error(`${owner}: field key "key" is reserved for the record key`)
    out[key] = { ...f, owner, key }
  }
  return out as BoundFields<F>
}

/** An entity owned by this package: one typed table, generic CRUD allowed. */
export function entity<const Id extends string, F extends FieldShape>(
  id: Id,
  fields: F,
  opts: EntityOptions<F> = {},
): Entity<Id, F, 'own'> {
  const resource = omitEmpty({
    id,
    kind: 'entity' as const,
    label: opts.label,
    description: opts.description,
    fields: Object.entries(fields).map(([k, f]) => compileField(k, f)),
    access: compileAccess(opts.access),
  })
  return { ...contract(resource), entityKind: 'own', keyType: 'uuid', fields: bind(id, fields) }
}

/**
 * Descriptor of a platform/Granite-owned entity, produced by the developer kit.
 * Readable through queries and refs; generic writes are rejected (types and runtime).
 */
export function nativeEntity<const Id extends string, F extends FieldShape>(
  id: Id,
  fields: F,
  opts: { key: KeyType },
): Entity<Id, F, 'native'> {
  const resource = {
    id,
    kind: 'entity' as const,
    fields: Object.entries(fields).map(([k, f]) => compileField(k, f)),
  }
  return { ...contract(resource, true), entityKind: 'native', keyType: opts.key, fields: bind(id, fields) }
}

/**
 * A separate 1:1 typed table keyed by the base record key. A record without an overlay row
 * reads declared defaults or null, so every overlay field must be optional, nullable or defaulted.
 */
export function overlay<const Id extends string, B extends AnyEntity, F extends FieldShape>(
  id: Id,
  base: B,
  fields: F,
  opts: Pick<EntityOptions<F>, 'label' | 'description'> = {},
): Overlay<Id, B, F> {
  for (const [key, f] of Object.entries(fields)) {
    if (!f.type.optional && !f.type.nullable && f.spec.default === undefined) {
      throw new Error(`${id}: overlay field "${key}" needs .optional(), .nullable() or .default(...): a base record may have no overlay row`)
    }
  }
  const resource = omitEmpty({
    id,
    kind: 'overlay' as const,
    label: opts.label,
    description: opts.description,
    base: base.id,
    fields: Object.entries(fields).map(([k, f]) => compileField(k, f)),
  })
  return { ...contract(resource), base, fields: bind(id, fields) }
}

export function ref<E extends EntityLike>(entity: E, key: string): Ref<E> {
  return { entity: entity.id, key }
}

export type { Ref }
