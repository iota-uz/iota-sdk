// Building blocks for tests without a Granite checkout: synthetic typed fixtures and a registry of
// fake external effects. The runtime harness that runs handlers against them is a separate slice.
import type { AnyEntity, AnyOverlay, InsertOf, OverlayRow, Row } from './entity.ts'
import type { ISODateTime, Json } from './model.ts'
import { t, type EntityLike, type Ref } from './schema.ts'
import { encode } from './wire.ts'

export { collectManifest, ManifestError } from './manifest.ts'
export type { CollectInput } from './manifest.ts'

// ---------------------------------------------------------------- fixtures

const EPOCH = '2026-01-01T00:00:00Z' as ISODateTime

export interface Fixture<E extends EntityLike> {
  readonly ref: Ref<E>
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
    out[key] = given[key] !== undefined ? given[key] : (f.spec.default ?? null)
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
 */
export function fixture<E extends AnyEntity>(entity: E, values: InsertOf<E>, opts: { key?: string } = {}): Fixture<E> {
  if (entity.entityKind === 'native' && opts.key === undefined) {
    throw new Error(`fixture(${entity.id}): native entities need an explicit key (${entity.keyType})`)
  }
  const key = opts.key ?? syntheticKey(entity.id, values)
  const fields = materialize(entity.fields, values)
  checked(entity, fields)
  return {
    ref: { entity: entity.id, key } as Fixture<E>['ref'],
    record: { key, ref: { entity: entity.id, key }, createdAt: EPOCH, updatedAt: EPOCH, rowVersion: 1, ...fields } as Row<E>,
  }
}

/** A synthetic overlay row for a base record. */
export function overlayFixture<O extends AnyOverlay>(overlay: O, base: Ref<O['base']>, values: Partial<InsertOf<O>> = {}): OverlayRow<O> {
  const fields = materialize(overlay.fields, values)
  checked(overlay, fields)
  return { baseKey: base.key, ...fields } as OverlayRow<O>
}

// ---------------------------------------------------------------- fake external effects

/** A fake provider channel: records every attempt, dedupes deliveries by idempotency key. */
export interface FakeChannel<M extends { readonly idempotencyKey?: string }> {
  /** Every attempt, including repeats and failures. */
  readonly calls: readonly M[]
  /** Unique deliveries: repeated idempotency keys are delivered once. */
  readonly delivered: readonly M[]
  send(message: M): Promise<{ readonly id: string }>
  /** The next `count` sends reject with `error`. */
  failNext(error: Error, count?: number): void
  reset(): void
}

export interface SmsMessage {
  to: string
  text: string
  idempotencyKey?: string
}
export interface EmailMessage {
  to: string
  subject: string
  html?: string
  text?: string
  idempotencyKey?: string
}
export interface PushMessage {
  to: string
  title: string
  body: string
  idempotencyKey?: string
}

function fakeChannel<M extends { readonly idempotencyKey?: string }>(name: string): FakeChannel<M> {
  let calls: M[] = []
  let delivered: M[] = []
  let ids = new Map<string, string>()
  let failures: Error[] = []
  return {
    get calls() {
      return calls
    },
    get delivered() {
      return delivered
    },
    async send(message) {
      calls = [...calls, message]
      const failure = failures.shift()
      if (failure) throw failure
      const key = message.idempotencyKey
      const known = key === undefined ? undefined : ids.get(key)
      if (known !== undefined) return { id: known }
      const id = `${name}-${delivered.length + 1}`
      if (key !== undefined) ids.set(key, id)
      delivered = [...delivered, message]
      return { id }
    },
    failNext(error, count = 1) {
      failures = [...failures, ...Array.from({ length: count }, () => error)]
    },
    reset() {
      calls = []
      delivered = []
      ids = new Map()
      failures = []
    },
  }
}

export interface FakeHttpRequest {
  method: string
  url: string
  headers: Readonly<Record<string, string>>
  body?: Json
}
export interface FakeHttpResponse {
  status: number
  headers?: Readonly<Record<string, string>>
  body?: Json
}

export class UnmatchedRequestError extends Error {
  constructor(request: FakeHttpRequest) {
    super(`no fake http route for ${request.method} ${request.url}`)
    this.name = 'UnmatchedRequestError'
  }
}

export interface FakeHttp {
  readonly calls: readonly FakeHttpRequest[]
  /** Registers a canned response (or a function of the request) for `method` + `url`. */
  route(method: string, url: string, response: FakeHttpResponse | ((request: FakeHttpRequest) => FakeHttpResponse)): void
  request(request: FakeHttpRequest): Promise<FakeHttpResponse>
  reset(): void
}

function fakeHttp(): FakeHttp {
  let calls: FakeHttpRequest[] = []
  let routes = new Map<string, FakeHttpResponse | ((request: FakeHttpRequest) => FakeHttpResponse)>()
  return {
    get calls() {
      return calls
    },
    route(method, url, response) {
      routes.set(`${method.toUpperCase()} ${url}`, response)
    },
    async request(request) {
      calls = [...calls, request]
      const handler = routes.get(`${request.method.toUpperCase()} ${request.url}`)
      if (handler === undefined) throw new UnmatchedRequestError(request)
      return typeof handler === 'function' ? handler(request) : handler
    },
    reset() {
      calls = []
      routes = new Map()
    },
  }
}

/** Registry of fake external effects a test wires into step/transaction code under test. */
export interface FakeEffects {
  readonly sms: FakeChannel<SmsMessage>
  readonly email: FakeChannel<EmailMessage>
  readonly push: FakeChannel<PushMessage>
  readonly http: FakeHttp
  reset(): void
}

export function fakeEffects(): FakeEffects {
  const sms = fakeChannel<SmsMessage>('sms')
  const email = fakeChannel<EmailMessage>('email')
  const push = fakeChannel<PushMessage>('push')
  const http = fakeHttp()
  return {
    sms,
    email,
    push,
    http,
    reset() {
      sms.reset()
      email.reset()
      push.reset()
      http.reset()
    },
  }
}
