import type { ResourceJSON, ResourceKind } from './model.ts'

/** Every value the registry collects carries this brand. */
export const RESOURCE: unique symbol = Symbol.for('iota.resource')

/** Set only on contracts issued through the developer-kit entry (`@iota-uz/sdk/runtime/kit`). */
export const KIT: unique symbol = Symbol.for('iota.kit')

/** A pure, declared resource: carries the manifest JSON that describes it. */
export interface Contract<Id extends string = string, K extends ResourceKind = ResourceKind> {
  readonly [RESOURCE]: true
  readonly id: Id
  readonly kind: K
  readonly resource: ResourceJSON
  /** Declared by another package or by the platform; referenced, never re-declared by this package. */
  readonly foreign: boolean
  readonly [KIT]?: true
}

export function isBranded(value: unknown): value is { readonly [RESOURCE]: true } {
  return typeof value === 'object' && value !== null && (value as { [RESOURCE]?: unknown })[RESOURCE] === true
}

export function isContract(value: unknown): value is Contract {
  if (!isBranded(value)) return false
  const v = value as Partial<Contract>
  return typeof v.id === 'string' && typeof v.kind === 'string' && typeof v.resource === 'object'
}

/** True for contracts issued by the developer kit; the only contracts allowed to be `foreign`. */
export function isKitIssued(value: Contract): boolean {
  return value.foreign && (value as { [KIT]?: unknown })[KIT] === true
}

/** Internal constructor of every contract. `foreign` is reserved for the kit entry. */
export function contract<Id extends string, K extends ResourceKind>(
  resource: ResourceJSON & { id: Id; kind: K },
  foreign = false,
): Contract<Id, K> {
  const base = { [RESOURCE]: true, id: resource.id, kind: resource.kind, resource, foreign } as const
  return foreign ? { ...base, [KIT]: true } : base
}

/**
 * Omits the zero values Go's `omitempty` drops for typed fields (`false`, `""`, `[]`, `undefined`).
 * Never use it for JSON-valued members (`default`, filter values): there `false`, `""`, `0` and `[]` are real values.
 */
export function omitEmpty<T extends object>(value: T): T {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(value)) {
    if (v === undefined || v === false || v === '' || (Array.isArray(v) && v.length === 0)) continue
    out[k] = v
  }
  return out as T
}
