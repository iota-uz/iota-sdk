import type {
  AccessJSON, ConditionJSON, FieldJSON, FilterJSON, ISODateTime, Json, JsonNotNull, Label, ReadRuleJSON, TypeJSON, TypeKind,
} from './model.ts'
import { contract, omitEmpty, type Contract } from './resource.ts'
import {
  makeSchema, t, type DecimalOptions, type EntityLike, type Infer, type JsonConstraint, type Presence, type Ref, type SchemaLike, type StringOptions,
  type UserEntity, type UserRef,
} from './schema.ts'
import type { Permission } from './contracts.ts'
import { isInt64String, isUUID, validate } from './wire.ts'

export interface FieldSpec {
  readonly unique: boolean
  readonly index: boolean
  readonly default?: Json
  readonly renamedFrom: readonly string[]
  readonly label?: Label
}

/**
 * A typed column. Reading a row yields `T` (`T | null` for nullable columns).
 * - `.optional()` is a nullable column that may be omitted on insert: `T | null` on read, `T | null | undefined` on write.
 * - `.default(x)` may be omitted on insert; it reads `T` only while the column is not nullable.
 */
export interface Field<T = unknown, K extends Presence = 'req', N extends TypeKind = TypeKind> extends SchemaLike<T, K, N> {
  readonly spec: FieldSpec
  optional(): Field<T | null, 'opt', N>
  nullable(): Field<T | null, K, N>
  unique(): Field<T, K, N>
  index(): Field<T, K, N>
  /** Validated against the column type; a nullable column may default to `null`. */
  default(value: T): Field<T, 'def', N>
  /** Previous field keys; the physical column follows the field across renames. */
  renamedFrom(...keys: string[]): Field<T, K, N>
  label(label: Label): Field<T, K, N>
}

export type AnyField = Field<unknown, Presence, TypeKind>
export type FieldShape = Readonly<Record<string, AnyField>>

const emptySpec: FieldSpec = { unique: false, index: false, renamedFrom: [] }

function makeField<T, K extends Presence, N extends TypeKind>(type: TypeJSON, spec: FieldSpec = emptySpec): Field<T, K, N> {
  const base = makeSchema<T, K, N>(type)
  const next = <T2, K2 extends Presence>(patch: Partial<FieldSpec>, nextType: TypeJSON = type): Field<T2, K2, N> =>
    makeField<T2, K2, N>(nextType, { ...spec, ...patch })
  return {
    ...base,
    spec,
    optional: () => next<T | null, 'opt'>({}, { ...type, optional: true, nullable: true }),
    nullable: () => next<T | null, K>({}, { ...type, nullable: true }),
    unique: () => next<T, K>({ unique: true }),
    index: () => next<T, K>({ index: true }),
    default: (value: T) => {
      const json = value as Json
      try {
        validate(type, json)
      } catch (e) {
        throw new Error(`invalid field default: ${e instanceof Error ? e.message : String(e)}`)
      }
      return next<T, 'def'>({ default: json })
    },
    renamedFrom: (...keys: string[]) => next<T, K>({ renamedFrom: [...spec.renamedFrom, ...keys] }),
    label: (label: Label) => next<T, K>({ label }),
  }
}

function fieldOf<S extends SchemaLike<unknown, 'req', TypeKind>>(schema: S): Field<Infer<S>, 'req', S extends SchemaLike<unknown, 'req', infer N> ? N : TypeKind> {
  return makeField(schema.type)
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
  json: <T extends JsonConstraint<T> = JsonNotNull>() => fieldOf(t.json<T>()),
  ref: <E extends EntityLike>(entity: E) => fieldOf(t.ref(entity)),
} as const

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
/** The value of a field as read from a row: `T`, or `T | null` for optional/nullable columns. */
export type FieldValue<S> = S extends SchemaLike<infer V, Presence, TypeKind> ? V : never

export type Fields<E> = E extends { readonly fields: infer G extends FieldShape } ? G : never

export type FieldValues<E> = { readonly [K in keyof Fields<E>]: FieldValue<Fields<E>[K]> }

/**
 * An entity row as returned by the data layer: system columns plus every declared field (optional ones read `null`).
 * `ref` is the typed pointer to this record, so a handler never has to re-state the entity (`ref(Entity, row.key)`).
 * It is attached by the runtime context layer to every row `get`/`find`/`insert`/`update` returns.
 */
export type Row<E extends EntityLike> = Simplify<
  {
    readonly key: string
    /** Typed pointer to this record. */
    readonly ref: Ref<E>
    readonly createdAt: ISODateTime
    readonly updatedAt: ISODateTime
    /** Incremented on every write; pass it to `tx.update(..., { rowVersion })` for an optimistic check. */
    readonly rowVersion: number
  } & FieldValues<E>
>

/** An overlay row, keyed by the base record's key. */
export type OverlayRow<O> = Simplify<{ readonly baseKey: string } & FieldValues<O>>

export type RecordOf<E extends AnyEntity | AnyOverlay> = E extends AnyOverlay ? OverlayRow<E> : E extends AnyEntity ? Row<E> : never

/** Keys the data layer owns on every row; declaring a field with one of these keys is an error. */
export const RESERVED_KEYS: ReadonlySet<string> = new Set(['key', 'ref', 'createdAt', 'updatedAt', 'rowVersion', 'baseKey'])

type RequiredInsertKeys<F extends FieldShape> = { [K in keyof F]: F[K] extends SchemaLike<unknown, 'req'> ? K : never }[keyof F]
type OptionalInsertKeys<F extends FieldShape> = Exclude<keyof F, RequiredInsertKeys<F>>

/** Values accepted by `tx.insert`: fields that are optional or defaulted may be omitted. */
export type InsertOf<E> = Simplify<
  { [K in RequiredInsertKeys<Fields<E>>]: FieldValue<Fields<E>[K]> } & { [K in OptionalInsertKeys<Fields<E>>]?: FieldValue<Fields<E>[K]> }
>
export type PatchOf<E> = { [K in keyof Fields<E>]?: FieldValue<Fields<E>[K]> }

type IsUnion<T, U = T> = T extends unknown ? ([U] extends [T] ? false : true) : never

/** Compile-time message type: an argument that cannot satisfy it fails with the message in the error text. */
export type Hint<M extends string> = { readonly [K in M]: never }

/**
 * `Ref<E>` for a single entity. A union of entities (`cond ? A : B`) is rejected: the ref could belong to either
 * branch, so the call would type-check for the wrong record.
 */
export type RefTo<E extends EntityLike> = [IsUnion<E>] extends [false] ? Ref<E> : Hint<'entity is a union: narrow it to one entity before using a ref'>

// ---- filters ----

export interface UserPlaceholder {
  readonly $: 'user'
}
/** `$.user` is the current subject's user ref inside access rules and filters; it matches only `core.user` ref fields. */
export const $ = { user: { $: 'user' } as UserPlaceholder } as const

type Depth = readonly unknown[]
type Leaf<P extends string, V, N extends string> = readonly [path: P, value: V, kind: N]
/** System columns every row has. `key` supports equality only; the timestamps behave like datetime fields. */
type SystemLeaves = Leaf<'key', string, 'key'> | Leaf<'createdAt', ISODateTime, 'datetime'> | Leaf<'updatedAt', ISODateTime, 'datetime'>
type FieldLeaves<F extends FieldShape> = {
  [K in keyof F & string]: F[K] extends SchemaLike<infer V, Presence, infer N> ? Leaf<K, V, N> : never
}[keyof F & string]
type Prefixed<K extends string, L> = L extends readonly [infer P extends string, infer V, infer N extends string]
  ? Leaf<`${K}.${P}`, V, N>
  : never
type PathLeaves<F extends FieldShape, D extends Depth = readonly []> =
  | FieldLeaves<F>
  | SystemLeaves
  | {
      [K in keyof F & string]: NonNullable<Infer<F[K]>> extends Ref<infer E>
        ? D['length'] extends 2
          ? never
          : Fields<E> extends infer G extends FieldShape
            ? Prefixed<K, PathLeaves<G, readonly [...D, unknown]>>
            : never
        : never
    }[keyof F & string]

/** Kinds with a total order the database can sort and range-filter by. */
type Orderable = 'int' | 'int64' | 'decimal' | 'date' | 'datetime' | 'string'

type PathOfKind<L, Pred extends string> = L extends readonly [infer P extends string, unknown, infer N extends string]
  ? [N] extends [Pred] ? P : never
  : never

/** Field keys and system columns, plus dotted paths through ref fields ("holder.owner"); `json` fields are not filterable. */
export type PathOf<E> = PathOfKind<PathLeaves<Fields<E>>, Exclude<TypeKind, 'json'> | 'key'>
/** Paths a result can be ordered by: orderable field kinds and `createdAt` / `updatedAt` (not `key`, uuid, enum, bool, ref). */
export type OrderPathOf<E> = PathOfKind<PathLeaves<Fields<E>>, Orderable>

type Val<V> = NonNullable<V> extends UserRef ? V | UserPlaceholder : V

/** Name of a wire kind in the hints below (`key` and `ref` read better as they are). */
type KindName<N extends string> = N extends 'string' ? 'string' : N

/**
 * The conditions a field of wire kind `N` supports: ranges only on orderable kinds, `like` only on strings.
 * An operator the kind lacks is typed as a `Hint`, so using it names the reason in the error text.
 */
export type Ops<V, N extends string = string> = N extends 'key'
  ? { eq?: V; in?: readonly V[]; ne?: Hint<'key supports only eq and in'>; lt?: Hint<'key supports only eq and in'>; gt?: Hint<'key supports only eq and in'> }
  : { eq?: Val<V>; ne?: Val<V>; in?: readonly Val<V>[]; isNull?: boolean } & ([N] extends [Orderable]
      ? { lt?: NonNullable<V>; lte?: NonNullable<V>; gt?: NonNullable<V>; gte?: NonNullable<V> }
      : {
          lt?: Hint<`${KindName<N>} fields cannot be compared with lt/lte/gt/gte: ranges need int, int64, decimal, date, datetime or string`>
          lte?: Hint<`${KindName<N>} fields cannot be compared with lt/lte/gt/gte: ranges need int, int64, decimal, date, datetime or string`>
          gt?: Hint<`${KindName<N>} fields cannot be compared with lt/lte/gt/gte: ranges need int, int64, decimal, date, datetime or string`>
          gte?: Hint<`${KindName<N>} fields cannot be compared with lt/lte/gt/gte: ranges need int, int64, decimal, date, datetime or string`>
        }) &
      ([N] extends ['string'] ? { like?: string } : { like?: Hint<`like needs a string field, not ${KindName<N>}`> })

/** Conjunction of conditions on fields or ref paths; a bare value means equality. */
export type Where<E> = {
  [L in PathLeaves<Fields<E>> as L[0]]?: L[2] extends 'json' ? Hint<`json field '${L[0]}' cannot be filtered: filter on a typed field`> : Val<L[1]> | Ops<L[1], L[2]>
} & { or?: readonly Where<E>[] }

type OrderEntries<E> = {
  [L in PathLeaves<Fields<E>> as L[0]]: L[2] extends Orderable
    ? { field: L[0]; desc?: boolean }
    : { field: L[0]; desc?: boolean } & Hint<`cannot order by '${L[0]}': ${KindName<L[2]>} fields have no database order; order by an int, int64, decimal, date, datetime or string field, or createdAt/updatedAt`>
}

/** One `orderBy` entry: an orderable path; any other path is accepted by the types only to explain, in the error text, why not. */
export type OrderBy<E> = OrderEntries<E>[keyof OrderEntries<E>]

const opKeys = new Set(['eq', 'ne', 'in', 'lt', 'lte', 'gt', 'gte', 'isNull', 'like'])

function isPlaceholder(value: unknown): boolean {
  return typeof value === 'object' && value !== null && (value as { $?: unknown }).$ === 'user'
}

function toCondition(value: unknown): ConditionJSON {
  if (typeof value === 'object' && value !== null && !Array.isArray(value) && !isPlaceholder(value)) {
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

/** `$.user` is a `core.user` ref: reject it on a direct field of another type (dotted paths are checked by the types). */
function checkUserPlaceholders(owner: string, fields: FieldShape, where: object): void {
  for (const [key, value] of Object.entries(where)) {
    if (value === undefined) continue
    if (key === 'or') {
      for (const w of value as readonly object[]) checkUserPlaceholders(owner, fields, w)
      continue
    }
    if (key.includes('.')) continue
    const cond = toCondition(value)
    const used = isPlaceholder(cond.eq) || isPlaceholder(cond.ne) || (cond.in ?? []).some(isPlaceholder)
    if (!used) continue
    const type = fields[key]?.type
    if (type === undefined || type.kind !== 'ref' || type.entity !== 'core.user') {
      throw new Error(`${owner}: $.user can only be compared with a ref to core.user, but "${key}" is not one`)
    }
  }
}

// ---- access ----

export interface ReadRule<E> {
  /** Permission the subject must hold; omitted means any authenticated subject. */
  when?: Permission
  /** `'all'` every row, `'none'` no row (an explicit deny), or a filter. */
  rows: 'all' | 'none' | Where<E>
}

export interface EntityAccess<F extends FieldShape> {
  /**
   * Visibility rules (union of the rules whose permission the subject holds). Omit `read` for unrestricted rows;
   * an empty list is rejected as ambiguous: write `[{ rows: 'none' }]` to deny everyone explicitly.
   */
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
  const typed = omitEmpty({
    id,
    type: f.type,
    unique: f.spec.unique,
    index: f.spec.index,
    renamedFrom: [...f.spec.renamedFrom],
    label: f.spec.label,
  }) as FieldJSON
  // `default` is a JSON value: `false`, `0`, `""`, `[]` and `null` are real defaults and must be emitted.
  if (f.spec.default === undefined) return typed
  const { id: fid, type, unique, index, ...rest } = typed
  return { id: fid, type, ...(unique ? { unique } : {}), ...(index ? { index } : {}), default: f.spec.default, ...rest }
}

function compileAccess<F extends FieldShape>(owner: string, fields: F, access: EntityAccess<F> | undefined): AccessJSON | undefined {
  if (!access) return undefined
  if (access.read !== undefined && access.read.length === 0) {
    throw new Error(`${owner}: access.read is empty, which is ambiguous (unrestricted or denied?); omit it, or write [{ rows: 'none' }] to deny`)
  }
  const read: ReadRuleJSON[] = (access.read ?? []).map((r) => {
    if (typeof r.rows === 'object') checkUserPlaceholders(owner, fields, r.rows)
    return omitEmpty({
      permission: r.when?.id,
      all: r.rows === 'all',
      rows: typeof r.rows === 'object' ? compileFilter(r.rows) : undefined,
    }) as ReadRuleJSON
  })
  const permissions: Record<string, string[]> = {}
  for (const [k, perms] of Object.entries(access.fields ?? {})) {
    if (perms) permissions[k] = perms.map((p) => p.id)
  }
  return omitEmpty({ read, fields: Object.keys(permissions).length > 0 ? permissions : undefined }) as AccessJSON
}

function bind<F extends FieldShape>(owner: string, fields: F, rejectReserved: boolean): BoundFields<F> {
  const out: Record<string, AnyField & BoundMeta> = {}
  for (const [key, f] of Object.entries(fields)) {
    if (rejectReserved && RESERVED_KEYS.has(key)) throw new Error(`${owner}: field key "${key}" is reserved for the data layer`)
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
    access: compileAccess(id, fields, opts.access),
  })
  return { ...contract(resource), entityKind: 'own', keyType: 'uuid', fields: bind(id, fields, true) }
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
  return { ...contract(resource, true), entityKind: 'native', keyType: opts.key, fields: bind(id, fields, false) }
}

/**
 * A separate 1:1 typed table keyed by the base record key. A record without an overlay row
 * reads declared defaults or null, so every overlay field must be optional, nullable or defaulted.
 */
export function overlay<const Id extends string, B extends AnyEntity, F extends FieldShape>(
  id: Id,
  base: B,
  fields: F,
  opts: Pick<EntityOptions<F>, 'label' | 'description' | 'access'> = {},
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
    access: compileAccess(id, fields, opts.access),
  })
  return { ...contract(resource), base, fields: bind(id, fields, true) }
}

function validKey(keyType: KeyType, key: string): boolean {
  if (keyType === 'int64') return isInt64String(key)
  if (keyType === 'uuid') return isUUID(key)
  return key !== ''
}

/** Typed pointer to a record. The key is checked against the entity's key type (uuid for own entities). */
export function ref<E extends AnyEntity>(entity: E, key: string): Ref<E> {
  if (!validKey(entity.keyType, key)) throw new Error(`ref(${entity.id}): "${key}" is not a valid ${entity.keyType} key`)
  return { entity: entity.id, key }
}

export type { Ref, UserEntity, UserRef }
