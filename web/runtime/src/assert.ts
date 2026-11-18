// Small typed assertion helper for package tests (`import { assert } from "@iota-uz/sdk/runtime/testing"`).
// It needs no Node typings, so a package tsconfig with `"types": []` can use it, and its failures read
// like a diff: what was expected, what was received, and where the two first differ.
import type { Json } from './model.ts'

/** Thrown by every failed assertion; the test runner prints `message` and the source location of the call. */
export class AssertionError extends Error {
  readonly actual: unknown
  readonly expected: unknown
  readonly operator: string

  constructor(message: string, details: { actual?: unknown; expected?: unknown; operator: string }) {
    super(message)
    this.name = 'AssertionError'
    this.actual = details.actual
    this.expected = details.expected
    this.operator = details.operator
  }
}

/** What a rejection or a throw must look like. A string or RegExp matches the message. */
export type ErrorMatcher =
  | string
  | RegExp
  | (abstract new (...args: never[]) => Error)
  | { readonly id: string; readonly of: (...args: never[]) => unknown }
  | { readonly code?: string; readonly kind?: string; readonly message?: string | RegExp; readonly issue?: { readonly id: string } }

type Fn = () => unknown

function show(value: unknown): string {
  const seen = new WeakSet<object>()
  const text = JSON.stringify(
    value,
    (_key, v: unknown) => {
      if (typeof v === 'bigint') return `${v}n`
      if (v === undefined) return '[undefined]'
      if (typeof v === 'function') return `[function ${v.name}]`
      if (typeof v === 'object' && v !== null) {
        if (seen.has(v)) return '[circular]'
        seen.add(v)
        if (v instanceof Error) return { name: v.name, message: v.message }
      }
      return v
    },
    2,
  )
  return text === undefined ? String(value) : text
}

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null

/** Path of the first difference between two values, or undefined when they are structurally equal. */
function firstDifference(a: unknown, b: unknown, path: string): string | undefined {
  if (Object.is(a, b)) return undefined
  if (!isObject(a) || !isObject(b)) return path === '' ? '(value)' : path
  if (Array.isArray(a) !== Array.isArray(b)) return path === '' ? '(value)' : path
  if (a instanceof Date && b instanceof Date) return a.getTime() === b.getTime() ? undefined : path
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const key of [...keys].sort()) {
    const next = Array.isArray(a) ? `${path}[${key}]` : path === '' ? key : `${path}.${key}`
    if (!(key in a) || !(key in b)) return next
    const d = firstDifference(a[key], b[key], next)
    if (d !== undefined) return d
  }
  return undefined
}

function fail(operator: string, message: string | undefined, fallback: string, actual?: unknown, expected?: unknown): never {
  throw new AssertionError(message ?? fallback, { actual, expected, operator })
}

function describeError(err: unknown): string {
  if (err instanceof Error) {
    const code = (err as { code?: unknown }).code
    return `${err.name}${typeof code === 'string' ? ` [${code}]` : ''}: ${err.message}`
  }
  return String(err)
}

function matches(err: unknown, expected: ErrorMatcher): boolean {
  if (typeof expected === 'string') return err instanceof Error && err.message.includes(expected)
  if (expected instanceof RegExp) return err instanceof Error && expected.test(err.message)
  if (typeof expected === 'function') return err instanceof expected
  if ('of' in expected && 'id' in expected) return isObject(err) && err.code === expected.id
  const o = expected as { code?: string; kind?: string; message?: string | RegExp; issue?: { id: string } }
  const e = err as { code?: unknown; kind?: unknown; message?: unknown; issues?: readonly { code: string }[] }
  if (!isObject(err)) return false
  if (o.code !== undefined && e.code !== o.code) return false
  if (o.kind !== undefined && e.kind !== o.kind) return false
  if (o.issue !== undefined && !(e.issues ?? []).some((i) => i.code === o.issue?.id)) return false
  if (o.message !== undefined) {
    const text = String(e.message ?? '')
    if (typeof o.message === 'string' ? !text.includes(o.message) : !o.message.test(text)) return false
  }
  return true
}

function describeMatcher(expected: ErrorMatcher): string {
  if (typeof expected === 'string' || expected instanceof RegExp) return `message ${String(expected)}`
  if (typeof expected === 'function') return `instance of ${expected.name}`
  if ('of' in expected && 'id' in expected) return `domain error ${expected.id}`
  return show(expected)
}

export interface Assert {
  /** Fails unless `value` is truthy; narrows its type. */
  ok(value: unknown, message?: string): asserts value
  /** Strict equality (`Object.is`): primitives and identical references. */
  equal(actual: unknown, expected: unknown, message?: string): void
  notEqual(actual: unknown, expected: unknown, message?: string): void
  /** Structural equality of JSON-like values (objects, arrays, dates, primitives); the message names the first differing path. */
  deepEqual(actual: unknown, expected: unknown, message?: string): void
  /** `actual` contains every property of `expected` (recursively for objects; arrays must match element for element). */
  partial(actual: unknown, expected: Json | object, message?: string): void
  /** The text matches the pattern (RegExp) or contains the substring. */
  match(text: string, pattern: RegExp | string, message?: string): void
  /** `fn` throws synchronously. */
  throws(fn: Fn, expected?: ErrorMatcher, message?: string): void
  /**
   * The promise rejects. Pass the promise itself, or a function that returns it (a handler that may be synchronous
   * included: a synchronous throw counts as a rejection). `expected` may be a message substring or RegExp, an error
   * class, a domain error contract (`NotActive`), or `{ code, kind, message, issue }`:
   * `await assert.rejects(call(), { kind: "validation", issue: TooLong })`.
   */
  rejects(promiseOrFn: unknown, expected?: ErrorMatcher, message?: string): Promise<void>
  /** Always fails. */
  fail(message: string): never
}

export const assert: Assert = {
  ok(value, message) {
    if (!value) fail('ok', message, `expected a truthy value, got ${show(value)}`, value, true)
  },
  equal(actual, expected, message) {
    if (!Object.is(actual, expected)) {
      fail('equal', message, `expected ${show(expected)}\n     got ${show(actual)}`, actual, expected)
    }
  },
  notEqual(actual, expected, message) {
    if (Object.is(actual, expected)) fail('notEqual', message, `expected a value other than ${show(expected)}`, actual, expected)
  },
  deepEqual(actual, expected, message) {
    const at = firstDifference(actual, expected, '')
    if (at !== undefined) {
      fail('deepEqual', message, `values differ at ${at}\nexpected ${show(expected)}\n     got ${show(actual)}`, actual, expected)
    }
  },
  partial(actual, expected, message) {
    const visit = (a: unknown, e: unknown, path: string): string | undefined => {
      if (isObject(e) && !Array.isArray(e)) {
        if (!isObject(a)) return path === '' ? '(value)' : path
        for (const [k, v] of Object.entries(e)) {
          const d = visit(a[k], v, path === '' ? k : `${path}.${k}`)
          if (d !== undefined) return d
        }
        return undefined
      }
      return firstDifference(a, e, path)
    }
    const at = visit(actual, expected, '')
    if (at !== undefined) {
      fail('partial', message, `value does not contain the expected properties (first difference at ${at})\nexpected ${show(expected)}\n     got ${show(actual)}`, actual, expected)
    }
  },
  match(text, pattern, message) {
    const ok = typeof pattern === 'string' ? text.includes(pattern) : pattern.test(text)
    if (!ok) fail('match', message, `expected ${show(text)} to match ${String(pattern)}`, text, pattern)
  },
  throws(fn, expected, message) {
    try {
      fn()
    } catch (err) {
      if (expected !== undefined && !matches(err, expected)) {
        fail('throws', message, `threw ${describeError(err)}, expected ${describeMatcher(expected)}`, err, expected)
      }
      return
    }
    fail('throws', message, 'expected the function to throw')
  },
  async rejects(promiseOrFn, expected, message) {
    try {
      await (typeof promiseOrFn === 'function' ? (promiseOrFn as () => unknown)() : promiseOrFn)
    } catch (err) {
      if (expected !== undefined && !matches(err, expected)) {
        fail('rejects', message, `rejected with ${describeError(err)}, expected ${describeMatcher(expected)}`, err, expected)
      }
      return
    }
    fail('rejects', message, 'expected the promise to reject')
  },
  fail(message) {
    throw new AssertionError(message, { operator: 'fail' })
  },
}
