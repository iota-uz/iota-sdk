import type { AnyEntity, AnyOverlay, FieldValues, Fields, RecordOf } from './entity.ts'
import type {
  Decimal, Int64, ISODate, ISODateTime, Json, JsonNotNull, PropertyJSON, TypeJSON, TypeKind,
} from './model.ts'

declare const phantom: unique symbol
declare const refTarget: unique symbol

/**
 * What `t.json<T>()` / `field.json<T>()` accept: any JSON value but `null`, or an `interface`/object type whose
 * members are JSON (interfaces have no implicit index signature, so `T extends Json` alone would reject them).
 */
export type JsonConstraint<T> = JsonNotNull | ({ readonly [K in keyof T]: Json | undefined } & object)

export type Presence = 'req' | 'opt' | 'def'

/** Anything that names an entity (own or native). Commands, overlays and other contracts cannot be referenced by `ref`. */
export interface EntityLike {
  readonly id: string
  readonly kind: 'entity'
}

/** Typed pointer to a record. Refs of different entities are not assignable to each other. */
export type Ref<E extends EntityLike = EntityLike> = {
  readonly entity: E['id']
  readonly key: string
  readonly [refTarget]?: E
}

/** The platform's user entity: the only valid target of user recipients, task assignees and `$.user`. */
export interface UserEntity extends EntityLike {
  readonly id: 'core.user'
}

/** Ref to a `core.user` record. Obtain one from the kit (`ref(kit.User, id)`) or from a row's `ref`. */
export type UserRef = Ref<UserEntity>

/** What a schema statically carries: the TS value type, whether an object key may be absent, the wire kind. */
export interface SchemaLike<T = unknown, K extends Presence = 'req', N extends TypeKind = TypeKind> {
  readonly [phantom]?: { readonly value: T; readonly presence: K; readonly kind: N }
  readonly type: TypeJSON
}

/** A wire-exact value schema. `T` is the TS value type, `K` whether an object key may be absent, `N` the wire kind. */
export interface Schema<T = unknown, K extends Presence = 'req', N extends TypeKind = TypeKind> extends SchemaLike<T, K, N> {
  /** The object key may be absent (the value is not nullable: use `.nullable()` as well to allow `null`). */
  optional(): Schema<T, 'opt', N>
  /** The value may be JSON null. */
  nullable(): Schema<T | null, K, N>
}

export type AnySchema = Schema<unknown, Presence, TypeKind>

export type Infer<S> = S extends SchemaLike<infer T, infer _K, infer _N> ? T : never

type Simplify<T> = { [K in keyof T]: T[K] }
export type AnySchemaLike = SchemaLike<unknown, Presence, TypeKind>
export type Shape = Readonly<Record<string, AnySchemaLike>>
type OptionalKeys<S extends Shape> = { [K in keyof S]: S[K] extends SchemaLike<unknown, 'opt'> ? K : never }[keyof S]
export type InferObject<S extends Shape> = Simplify<
  { [K in Exclude<keyof S, OptionalKeys<S>>]: Infer<S[K]> } & { [K in OptionalKeys<S>]?: Infer<S[K]> }
>

/** Object schema; keeps its shape so route params and other structural checks can inspect the properties. */
export interface ObjectSchema<S extends Shape = Shape> extends Schema<InferObject<S>, 'req', 'object'> {
  readonly shape: S
}

export function makeSchema<T, K extends Presence = 'req', N extends TypeKind = TypeKind>(type: TypeJSON): Schema<T, K, N> {
  return {
    type,
    optional: () => makeSchema<T, 'opt', N>({ ...type, optional: true }),
    nullable: () => makeSchema<T | null, K, N>({ ...type, nullable: true }),
  }
}

export interface StringOptions {
  maxLength?: number
}

export interface DecimalOptions {
  precision: number
  scale: number
}

function scalar<T, N extends TypeKind>(kind: N, extra: Partial<TypeJSON> = {}): Schema<T, 'req', N> {
  return makeSchema<T, 'req', N>({ kind, ...extra })
}

function entityId(e: EntityLike): string {
  return e.id
}

/** Row property types as read: every field is present (optional fields read `null`). */
function rowType(type: TypeJSON): TypeJSON {
  const { optional: _optional, ...rest } = type
  return rest
}

function objectOf(properties: PropertyJSON[]): Schema<never, 'req', 'object'> {
  return makeSchema<never, 'req', 'object'>(properties.length > 0 ? { kind: 'object', properties } : { kind: 'object' })
}

function fieldProperties(source: AnyEntity | AnyOverlay, pick?: readonly string[]): PropertyJSON[] {
  const keys = pick ?? Object.keys(source.fields)
  return keys.map((name) => {
    const f = source.fields[name]
    if (f === undefined) throw new Error(`${source.id}: no field "${name}"`)
    return { name, type: rowType(f.type) }
  })
}

const KEY_KIND: Readonly<Record<string, TypeKind>> = { int64: 'int64', uuid: 'uuid', string: 'string' }

export const t = {
  string: (o: StringOptions = {}) => scalar<string, 'string'>('string', o.maxLength ? { maxLength: o.maxLength } : {}),
  int: () => scalar<number, 'int'>('int'),
  int64: () => scalar<Int64, 'int64'>('int64'),
  decimal: (o: DecimalOptions) => scalar<Decimal, 'decimal'>('decimal', { precision: o.precision, scale: o.scale }),
  bool: () => scalar<boolean, 'bool'>('bool'),
  date: () => scalar<ISODate, 'date'>('date'),
  datetime: () => scalar<ISODateTime, 'datetime'>('datetime'),
  uuid: () => scalar<string, 'uuid'>('uuid'),
  enum: <const V extends readonly [string, ...string[]]>(values: V) => scalar<V[number], 'enum'>('enum', { values: [...values] }),
  /** Any JSON value except `null`; add `.nullable()` for null. `T` must be JSON (interfaces with JSON members are fine). */
  json: <T extends JsonConstraint<T> = JsonNotNull>() => scalar<T, 'json'>('json'),
  ref: <E extends EntityLike>(entity: E) => scalar<Ref<E>, 'ref'>('ref', { entity: entityId(entity) }),
  object: <S extends Shape>(shape: S): ObjectSchema<S> => {
    const properties: PropertyJSON[] = Object.entries(shape).map(([name, s]) => ({ name, type: s.type }))
    const base = makeSchema<InferObject<S>, 'req', 'object'>(properties.length > 0 ? { kind: 'object', properties } : { kind: 'object' })
    return { ...base, shape }
  },
  array: <S extends AnySchema>(items: S) => scalar<Infer<S>[], 'array'>('array', { items: items.type }),
  /**
   * The record `tx.get` / `tx.find` / `tx.insert` / `tx.update` return for `source`: `key`, `createdAt`, `updatedAt`,
   * `rowVersion`, `ref` and every declared field (optional fields read `null`). Overlays: `baseKey` and the fields.
   * Lets a command output return rows without re-declaring them.
   */
  row: <E extends AnyEntity | AnyOverlay>(source: E): Schema<RecordOf<E>> => {
    if ('base' in source) {
      const keyKind = KEY_KIND[source.base.keyType] ?? 'string'
      return objectOf([{ name: 'baseKey', type: { kind: keyKind } }, ...fieldProperties(source)]) as unknown as Schema<RecordOf<E>>
    }
    const keyKind = source.entityKind === 'native' ? (KEY_KIND[source.keyType] ?? 'string') : 'uuid'
    return objectOf([
      { name: 'key', type: { kind: keyKind } },
      { name: 'createdAt', type: { kind: 'datetime' } },
      { name: 'updatedAt', type: { kind: 'datetime' } },
      { name: 'rowVersion', type: { kind: 'int' } },
      { name: 'ref', type: { kind: 'ref', entity: source.id } },
      ...fieldProperties(source),
    ]) as unknown as Schema<RecordOf<E>>
  },
  /** Declared fields of `source` as they read from a row (no system columns), optionally only `pick`ed keys. */
  fields: <E extends AnyEntity | AnyOverlay, const K extends keyof Fields<E> & string = keyof Fields<E> & string>(
    source: E,
    pick?: readonly K[],
  ): Schema<Simplify<Pick<FieldValues<E>, K>>> =>
    objectOf(fieldProperties(source, pick)) as unknown as Schema<Simplify<Pick<FieldValues<E>, K>>>,
} as const
