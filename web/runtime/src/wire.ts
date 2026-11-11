// Exact wire codec shared by the Node host and the client transport.
// Rules mirror Go manifest.ValidateValue: safe-integer `int`, string `int64`/`decimal`,
// strict ISO date/datetime, typed refs, no unknown object keys.
import type { Decimal, Int64, ISODate, ISODateTime, Json, TypeJSON } from './model.ts'
import type { AnySchema, Infer } from './schema.ts'

export class WireError extends Error {
  readonly path: readonly string[]

  constructor(path: readonly string[], message: string) {
    super(path.length === 0 ? message : `${path.join('.')}: ${message}`)
    this.name = 'WireError'
    this.path = path
  }
}

const INT64_PATTERN = /^-?(0|[1-9][0-9]{0,18})$/
const DECIMAL_PATTERN = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?$/
const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/
const DATETIME_PATTERN = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-](\d{2}):(\d{2}))$/
const UUID_PATTERN = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/
const INT64_MIN = -(2n ** 63n)
const INT64_MAX = 2n ** 63n - 1n

function daysIn(year: number, month: number): number {
  if (month === 2) return year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0) ? 29 : 28
  return [4, 6, 9, 11].includes(month) ? 30 : 31
}

function validDate(y: string, m: string, d: string): boolean {
  const year = Number(y)
  const month = Number(m)
  return month >= 1 && month <= 12 && Number(d) >= 1 && Number(d) <= daysIn(year, month)
}

export function isInt64String(s: string): boolean {
  if (!INT64_PATTERN.test(s)) return false
  const n = BigInt(s)
  return n >= INT64_MIN && n <= INT64_MAX
}

export function isISODate(s: string): boolean {
  const m = DATE_PATTERN.exec(s)
  return m !== null && validDate(m[1]!, m[2]!, m[3]!)
}

export function isISODateTime(s: string): boolean {
  const m = DATETIME_PATTERN.exec(s)
  if (m === null || !validDate(m[1]!, m[2]!, m[3]!)) return false
  if (Number(m[4]) > 23 || Number(m[5]) > 59 || Number(m[6]) > 59) return false
  return m[8] === undefined || (Number(m[8]) <= 23 && Number(m[9]) <= 59)
}

export function isDecimalString(s: string, precision = 0, scale = 0): boolean {
  if (!DECIMAL_PATTERN.test(s)) return false
  const dot = s.indexOf('.')
  if (scale > 0 && dot >= 0 && s.length - dot - 1 > scale) return false
  if (precision > 0) {
    const digits = s.replace('-', '').replace('.', '').replace(/^0+/, '')
    if (digits.length > precision) return false
  }
  return true
}

/** Checked constructors for the branded wire scalars. */
export function int64(value: bigint | string): Int64 {
  const s = typeof value === 'bigint' ? value.toString() : value
  if (!isInt64String(s)) throw new WireError([], `expected int64 decimal string, got ${s}`)
  return s as Int64
}

export function decimal(value: string, o: { precision?: number; scale?: number } = {}): Decimal {
  if (!isDecimalString(value, o.precision, o.scale)) throw new WireError([], `expected decimal string, got ${value}`)
  return value as Decimal
}

export function isoDate(value: string | Date): ISODate {
  const s = typeof value === 'string' ? value : value.toISOString().slice(0, 10)
  if (!isISODate(s)) throw new WireError([], `expected date YYYY-MM-DD, got ${s}`)
  return s as ISODate
}

export function isoDateTime(value: string | Date): ISODateTime {
  const s = typeof value === 'string' ? value : value.toISOString()
  if (!isISODateTime(s)) throw new WireError([], `expected RFC3339 datetime with offset, got ${s}`)
  return s as ISODateTime
}

export function toBigInt(value: Int64): bigint {
  return BigInt(value)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function fail(path: readonly string[], message: string): never {
  throw new WireError(path, message)
}

function asType(schema: AnySchema | TypeJSON): TypeJSON {
  return 'kind' in schema ? schema : schema.type
}

function normalizeJson(value: unknown, path: readonly string[]): Json {
  if (value === null || typeof value === 'boolean' || typeof value === 'string') return value
  if (typeof value === 'number') {
    if (!Number.isFinite(value)) fail(path, 'invalid json: non-finite number')
    return value
  }
  if (Array.isArray(value)) return value.map((item, i) => normalizeJson(item, [...path, String(i)]))
  if (isRecord(value) && Object.getPrototypeOf(value) === Object.prototype) {
    const out: Record<string, Json> = {}
    for (const [k, v] of Object.entries(value)) {
      if (v !== undefined) out[k] = normalizeJson(v, [...path, k])
    }
    return out
  }
  return fail(path, 'invalid json')
}

function walk(type: TypeJSON, value: unknown, path: readonly string[]): Json {
  if (value === null || value === undefined) {
    if (value === null && type.nullable) return null
    return fail(path, 'value is required')
  }
  switch (type.kind) {
    case 'string': {
      if (typeof value !== 'string') return fail(path, 'expected string for string')
      if (type.maxLength && [...value].length > type.maxLength) fail(path, `longer than ${type.maxLength}`)
      return value
    }
    case 'date':
      if (typeof value !== 'string') return fail(path, 'expected string for date')
      if (!isISODate(value)) fail(path, 'expected date YYYY-MM-DD')
      return value
    case 'datetime':
      if (typeof value !== 'string') return fail(path, 'expected string for datetime')
      if (!isISODateTime(value)) fail(path, 'expected RFC3339 datetime with offset')
      return value
    case 'uuid':
      if (typeof value !== 'string') return fail(path, 'expected string for uuid')
      if (!UUID_PATTERN.test(value)) fail(path, 'expected uuid')
      return value
    case 'enum':
      if (typeof value !== 'string') return fail(path, 'expected string for enum')
      if (!(type.values ?? []).includes(value)) fail(path, `expected one of ${(type.values ?? []).join(', ')}`)
      return value
    case 'int64':
      if (typeof value !== 'string') return fail(path, 'expected string for int64')
      if (!INT64_PATTERN.test(value)) fail(path, 'expected int64 decimal string')
      if (!isInt64String(value)) fail(path, 'int64 out of range')
      return value
    case 'decimal': {
      if (typeof value !== 'string') return fail(path, 'expected string for decimal')
      if (!DECIMAL_PATTERN.test(value)) fail(path, 'expected decimal string')
      const dot = value.indexOf('.')
      if (type.scale && dot >= 0 && value.length - dot - 1 > type.scale) fail(path, `more than ${type.scale} fractional digits`)
      if (type.precision && !isDecimalString(value, type.precision, 0)) fail(path, `more than ${type.precision} digits`)
      return value
    }
    case 'int':
      if (typeof value !== 'number' || !Number.isSafeInteger(value)) return fail(path, 'expected safe integer')
      return value
    case 'bool':
      if (typeof value !== 'boolean') return fail(path, 'expected boolean')
      return value
    case 'json':
      return normalizeJson(value, path)
    case 'ref': {
      if (!isRecord(value) || typeof value.entity !== 'string' || typeof value.key !== 'string' || value.key === '') {
        return fail(path, 'expected ref {entity,key}')
      }
      if (type.entity && value.entity !== type.entity) fail(path, `expected ref to ${type.entity}, got ${value.entity}`)
      return { entity: value.entity, key: value.key }
    }
    case 'array': {
      if (!Array.isArray(value)) return fail(path, 'expected array')
      const items = type.items
      if (!items) return fail(path, 'array without item type')
      return value.map((item, i) => walk(items, item, [...path, String(i)]))
    }
    case 'object': {
      if (!isRecord(value)) return fail(path, 'expected object')
      const props = type.properties ?? []
      const out: Record<string, Json> = {}
      for (const p of props) {
        const v = value[p.name]
        if (v === undefined) {
          if (p.type.optional) continue
          fail([...path, p.name], 'value is required')
        }
        out[p.name] = walk(p.type, v, [...path, p.name])
      }
      const known = new Set(props.map((p) => p.name))
      for (const k of Object.keys(value)) {
        if (!known.has(k) && value[k] !== undefined) fail([...path, k], 'unknown property')
      }
      return out
    }
    default:
      return fail(path, `unsupported type ${String(type.kind)}`)
  }
}

/** Throws `WireError` (with a path) when `value` does not satisfy the schema. */
export function validate(schema: AnySchema | TypeJSON, value: unknown): void {
  walk(asType(schema), value, [])
}

/** TS value → JSON-safe wire value (validated; `undefined` properties dropped). */
export function encode<S extends AnySchema>(schema: S, value: Infer<S>): Json {
  return walk(asType(schema), value, [])
}

/** JSON wire value → typed TS value (validated; int64/decimal stay strings, refs plain `{entity,key}`). */
export function decode<S extends AnySchema>(schema: S, json: unknown): Infer<S> {
  return walk(asType(schema), json, []) as Infer<S>
}
