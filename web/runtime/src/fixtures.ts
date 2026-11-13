// Synthetic typed fixtures: records and overlay rows built from the real entity definitions.
import type { AnyEntity, AnyOverlay, InsertOf, OverlayRow, Row } from './entity.ts'
import type { ISODateTime, Json } from './model.ts'
import { t, type EntityLike, type Ref } from './schema.ts'
import { encode } from './wire.ts'

export const EPOCH = '2026-01-01T00:00:00Z' as ISODateTime

/** The system timestamps a native entity may declare as fields; a fixture defaults them to `EPOCH`. */
const TIMESTAMPS = ['createdAt', 'updatedAt'] as const
type Timestamp = (typeof TIMESTAMPS)[number]

/** `InsertOf<E>` where `createdAt` / `updatedAt` are always optional (they default to `EPOCH`), whether or not the entity declares them. */
export type FixtureValues<E> = {
  [K in keyof InsertOf<E> as K extends Timestamp ? never : K]: InsertOf<E>[K]
} & { [K in keyof InsertOf<E> as K extends Timestamp ? K : never]?: InsertOf<E>[K] }

export interface Fixture<E extends EntityLike> {
  /** The record's key: `ref.key`, for passing to `tx.get`-style calls and comparisons without going through `ref`. */
  readonly key: string
  readonly ref: Ref<E>
  /** The stored row as `get` returns it, including its typed `ref`. */
  readonly record: Row<E>
}

function hash(text: string): string {
  let a = 0x811c9dc5
  let b = 0x01000193
  for (let i = 0; i < text.length; i++) {
    a = Math.imul(a ^ text.charCodeAt(i), 0x01000193) >>> 0
    b = Math.imul(b + text.charCodeAt(i), 0x85ebca6b) >>> 0
  }
  return (a.toString(16).padStart(8, '0') + b.toString(16).padStart(8, '0')).repeat(2)
}

/** Deterministic UUID derived from the entity ID and the given values. */
function syntheticKey(entityId: string, values: object): string {
  const h = hash(entityId + JSON.stringify(values))
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-4${h.slice(13, 16)}-8${h.slice(17, 20)}-${h.slice(20, 32)}`
}

function materialize(fields: Readonly<Record<string, { spec: { default?: Json }; type: { optional?: boolean; nullable?: boolean } }>>, values: object): Record<string, unknown> {
  const given = values as Readonly<Record<string, unknown>>
  const out: Record<string, unknown> = {}
  for (const [key, f] of Object.entries(fields)) {
    out[key] = given[key] !== undefined ? given[key] : (TIMESTAMPS as readonly string[]).includes(key) ? EPOCH : (f.spec.default ?? null)
  }
  return out
}

function checked(entity: AnyEntity | AnyOverlay, record: Record<string, unknown>): void {
  const shape = Object.fromEntries(Object.entries(entity.fields).map(([k, f]) => [k, f]))
  encode(t.object(shape), record as never)
}

/**
 * A synthetic record of `entity` with its ref. Defaults apply, optional fields read `null`, values are
 * validated against the field types, and the key is deterministic unless given (native entities need one).
 * `createdAt` and `updatedAt` default to `EPOCH` on every entity, also when a native entity declares them as fields
 * (`core.User` requires them, `crm.Client` makes them optional: a fixture needs neither).
 */
export function fixture<E extends AnyEntity>(entity: E, values: FixtureValues<E>, opts: { key?: string } = {}): Fixture<E> {
  if (entity.entityKind === 'native' && opts.key === undefined) {
    throw new Error(`fixture(${entity.id}): native entities need an explicit key (${entity.keyType})`)
  }
  const key = opts.key ?? syntheticKey(entity.id, values)
  const fields = materialize(entity.fields, values)
  checked(entity, fields)
  return {
    key,
    ref: { entity: entity.id, key } as Fixture<E>['ref'],
    record: { key, ref: { entity: entity.id, key }, createdAt: EPOCH, updatedAt: EPOCH, rowVersion: 1, ...fields } as unknown as Row<E>,
  }
}

/** Non-enumerable marker naming the overlay an `overlayFixture` row belongs to (read by `createTestHost`). */
export const OVERLAY_OF: unique symbol = Symbol.for('iota.testing.overlayOf')

/** A synthetic overlay row for a base record. */
export function overlayFixture<O extends AnyOverlay>(overlay: O, base: Ref<O['base']>, values: Partial<InsertOf<O>> = {}): OverlayRow<O> {
  const fields = materialize(overlay.fields, values)
  checked(overlay, fields)
  const row = { baseKey: base.key, ...fields } as OverlayRow<O>
  Object.defineProperty(row, OVERLAY_OF, { value: overlay.id, enumerable: false })
  return row
}

