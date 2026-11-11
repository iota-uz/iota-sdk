import type { ResourceJSON, ResourceKind } from './model.ts'

/** Every value the registry collects carries this brand. */
export const RESOURCE: unique symbol = Symbol.for('iota.resource')

/** A pure, declared resource: carries the manifest JSON that describes it. */
export interface Contract<Id extends string = string, K extends ResourceKind = ResourceKind> {
  readonly [RESOURCE]: true
  readonly id: Id
  readonly kind: K
  readonly resource: ResourceJSON
  /** Declared by another package or by the platform; referenced, never re-declared by this package. */
  readonly foreign: boolean
}

export function isBranded(value: unknown): value is { readonly [RESOURCE]: true } {
  return typeof value === 'object' && value !== null && (value as { [RESOURCE]?: unknown })[RESOURCE] === true
}

export function isContract(value: unknown): value is Contract {
  if (!isBranded(value)) return false
  const v = value as Partial<Contract>
  return typeof v.id === 'string' && typeof v.kind === 'string' && typeof v.resource === 'object'
}

export function contract<Id extends string, K extends ResourceKind>(
  resource: ResourceJSON & { id: Id; kind: K },
  foreign = false,
): Contract<Id, K> {
  return { [RESOURCE]: true, id: resource.id, kind: resource.kind, resource, foreign }
}

/** Marks a contract as declared elsewhere (kit-generated refs): it is referenced, never emitted into this manifest. */
export function foreign<C extends Contract>(c: C): C {
  return { ...c, foreign: true }
}

export function omitEmpty<T extends object>(value: T): T {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(value)) {
    if (v === undefined || v === false || v === '' || (Array.isArray(v) && v.length === 0)) continue
    out[k] = v
  }
  return out as T
}
