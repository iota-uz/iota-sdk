import type { Decimal, Int64, ISODate, ISODateTime, Json, PropertyJSON, TypeJSON } from './model.ts'

declare const phantom: unique symbol
declare const refTarget: unique symbol

export type Presence = 'req' | 'opt' | 'def'

/** Anything that names an entity; used to keep `Ref` independent of the entity module. */
export interface EntityLike {
  readonly id: string
}

/** Typed pointer to a record. Refs of different entities are not assignable to each other. */
export interface Ref<E extends EntityLike = EntityLike> {
  readonly entity: E['id']
  readonly key: string
  readonly [refTarget]?: E
}

/** A wire-exact value schema. `T` is the TS value type, `K` whether an object key may be absent. */
export interface Schema<T = unknown, K extends Presence = 'req'> {
  readonly [phantom]?: { readonly value: T; readonly presence: K }
  readonly type: TypeJSON
  /** The object key may be absent. */
  optional(): Schema<T, 'opt'>
  /** The value may be JSON null. */
  nullable(): Schema<T | null, K>
}

export type AnySchema = Schema<unknown, Presence>

export type Infer<S> = S extends Schema<infer T, infer _K> ? T : never

type Simplify<T> = { [K in keyof T]: T[K] }
export type Shape = Readonly<Record<string, AnySchema>>
type OptionalKeys<S extends Shape> = { [K in keyof S]: S[K] extends Schema<unknown, 'opt'> ? K : never }[keyof S]
export type InferObject<S extends Shape> = Simplify<
  { [K in Exclude<keyof S, OptionalKeys<S>>]: Infer<S[K]> } & { [K in OptionalKeys<S>]?: Infer<S[K]> }
>

export function makeSchema<T, K extends Presence = 'req'>(type: TypeJSON): Schema<T, K> {
  return {
    type,
    optional: () => makeSchema<T, 'opt'>({ ...type, optional: true }),
    nullable: () => makeSchema<T | null, K>({ ...type, nullable: true }),
  }
}

export interface StringOptions {
  maxLength?: number
}

export interface DecimalOptions {
  precision: number
  scale: number
}

function scalar<T>(kind: TypeJSON['kind'], extra: Partial<TypeJSON> = {}): Schema<T> {
  return makeSchema<T>({ kind, ...extra })
}

function entityId(e: EntityLike): string {
  return e.id
}

export const t = {
  string: (o: StringOptions = {}) => scalar<string>('string', o.maxLength ? { maxLength: o.maxLength } : {}),
  int: () => scalar<number>('int'),
  int64: () => scalar<Int64>('int64'),
  decimal: (o: DecimalOptions) => scalar<Decimal>('decimal', { precision: o.precision, scale: o.scale }),
  bool: () => scalar<boolean>('bool'),
  date: () => scalar<ISODate>('date'),
  datetime: () => scalar<ISODateTime>('datetime'),
  uuid: () => scalar<string>('uuid'),
  enum: <const V extends readonly [string, ...string[]]>(values: V) => scalar<V[number]>('enum', { values: [...values] }),
  json: <T = Json>() => scalar<T>('json'),
  ref: <E extends EntityLike>(entity: E) => scalar<Ref<E>>('ref', { entity: entityId(entity) }),
  object: <S extends Shape>(shape: S): Schema<InferObject<S>> => {
    const properties: PropertyJSON[] = Object.entries(shape).map(([name, s]) => ({ name, type: s.type }))
    return makeSchema<InferObject<S>>(properties.length > 0 ? { kind: 'object', properties } : { kind: 'object' })
  },
  array: <S extends AnySchema>(items: S) => scalar<Infer<S>[]>('array', { items: items.type }),
} as const
