import type { TypeJSON } from '../model.ts'

/**
 * Decodes the string path parameters of a route with the types the server
 * validated them against: int becomes a number, bool a boolean, everything
 * else (uuid, date, decimal, int64, enum, string) stays its exact string.
 * Without types the strings are returned unchanged.
 */
export function decodeRouteParams(types: TypeJSON | undefined, raw: Readonly<Record<string, string>>): Record<string, unknown> {
  if (!types || types.kind !== 'object') return { ...raw }
  const out: Record<string, unknown> = {}
  for (const prop of types.properties ?? []) {
    const value = raw[prop.name]
    if (value === undefined) {
      if (!prop.type.optional) throw new Error(`route parameter ${prop.name} is missing`)
      continue
    }
    switch (prop.type.kind) {
      case 'int': {
        const n = Number(value)
        if (!Number.isSafeInteger(n)) throw new Error(`route parameter ${prop.name} is not a safe integer`)
        out[prop.name] = n
        break
      }
      case 'bool':
        if (value !== 'true' && value !== 'false') throw new Error(`route parameter ${prop.name} is not a boolean`)
        out[prop.name] = value === 'true'
        break
      default:
        out[prop.name] = value
    }
  }
  return out
}
