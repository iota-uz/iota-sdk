// Fake external effects a test wires into step/transaction code under test.
import type { Json } from './model.ts'

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
  data?: Readonly<Record<string, string>>
  idempotencyKey?: string
}
/** An in-app notification for an ERP user (`to` is the user key). */
export interface InAppMessage {
  to: string
  title: string
  body: string
  url?: string
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

/**
 * Registry of fake external effects a test wires into step/transaction code under test.
 *
 * `createTestHost` delivers committed `tx.notify` calls to `sms`, `email`, `push` and `inApp` (rendering the
 * package's `templates` first); `http` backs `fetch`: while a test host exists, `fetch(url)` in the code under
 * test is answered by `http.route(method, url, response)` and an unrouted request throws `UnmatchedRequestError`.
 */
export interface FakeEffects {
  readonly sms: FakeChannel<SmsMessage>
  readonly email: FakeChannel<EmailMessage>
  readonly push: FakeChannel<PushMessage>
  readonly inApp: FakeChannel<InAppMessage>
  readonly http: FakeHttp
  reset(): void
}

export function fakeEffects(): FakeEffects {
  const sms = fakeChannel<SmsMessage>('sms')
  const email = fakeChannel<EmailMessage>('email')
  const push = fakeChannel<PushMessage>('push')
  const inApp = fakeChannel<InAppMessage>('inApp')
  const http = fakeHttp()
  return {
    sms,
    email,
    push,
    inApp,
    http,
    reset() {
      sms.reset()
      email.reset()
      push.reset()
      inApp.reset()
      http.reset()
    },
  }
}
