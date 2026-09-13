import { assertSDKIdentity } from './identity'
import { CLIENT_HOST_PROTOCOL_VERSION } from './protocol'

export const CLIENT_BOOTSTRAP_VERSION = '1.0.0'

export class ClientHostProtocolError extends Error {
  constructor(readonly expected: string, readonly actual: string, contract = 'client host protocol') {
    super(`${contract} ${actual} is incompatible with ${expected}`)
    this.name = 'ClientHostProtocolError'
  }
}

export interface ClientRouteContext<TInitial = unknown> {
  bootstrapVersion?: string
  protocolVersion: string
  sdkReleaseVersion: string
  sdkCommit: string
  initial: TInitial
  theme: 'light' | 'dark'
  csrf?: string
  route?: { id: string; path: string; featureId: string }
  session?: { csrf?: string; expiresAt?: string; refreshEndpoint?: string; reauthUrl?: string }
  locale?: { language: string; messages: Readonly<Record<string, string>> }
  user?: unknown
  tenant?: unknown
  permissions?: readonly string[]
  services?: { rpc?: string; telemetry?: string }
}

function major(version: string): string { return version.split('.', 1)[0] ?? '' }

function invalidBootstrapField(field: string, value: unknown): never {
  const actual = value === null ? 'null' : typeof value
  throw new ClientHostProtocolError('string', actual, `client bootstrap ${field}`)
}

function requiredString(context: Record<string, unknown>, field: string): string {
  const value = context[field]
  if (typeof value !== 'string') invalidBootstrapField(field, value)
  return value
}

export function assertClientHostProtocol(actual: string): void {
  if (major(actual) !== major(CLIENT_HOST_PROTOCOL_VERSION)) {
    throw new ClientHostProtocolError(CLIENT_HOST_PROTOCOL_VERSION, actual)
  }
}

export function assertClientBootstrap(context: unknown, requireVersion = true): asserts context is ClientRouteContext {
  if (!context || typeof context !== 'object' || Array.isArray(context)) {
    throw new ClientHostProtocolError(CLIENT_BOOTSTRAP_VERSION, context === null ? 'null' : typeof context, 'client bootstrap')
  }
  const payload = context as Record<string, unknown>
  const protocolVersion = requiredString(payload, 'protocolVersion')
  const sdkReleaseVersion = requiredString(payload, 'sdkReleaseVersion')
  const sdkCommit = requiredString(payload, 'sdkCommit')
  if (!Object.hasOwn(payload, 'initial')) invalidBootstrapField('initial', undefined)
  if (payload.theme !== 'light' && payload.theme !== 'dark') invalidBootstrapField('theme', payload.theme)
  if (payload.bootstrapVersion !== undefined && typeof payload.bootstrapVersion !== 'string') {
    invalidBootstrapField('bootstrapVersion', payload.bootstrapVersion)
  }

  assertClientHostProtocol(protocolVersion)
  assertSDKIdentity({
    releaseVersion: sdkReleaseVersion,
    sourceCommit: sdkCommit,
    protocolVersion,
  })
  const bootstrapVersion = payload.bootstrapVersion as string | undefined
  if ((requireVersion || bootstrapVersion !== undefined) && major(bootstrapVersion ?? '') !== major(CLIENT_BOOTSTRAP_VERSION)) {
    throw new ClientHostProtocolError(CLIENT_BOOTSTRAP_VERSION, bootstrapVersion ?? 'missing', 'client bootstrap')
  }
}

export function readClientBootstrap<TInitial = unknown>(document: Document, elementID = 'iota-client-context'): ClientRouteContext<TInitial> {
  const element = document.getElementById(elementID)
  if (!element?.textContent) throw new ClientHostProtocolError(CLIENT_BOOTSTRAP_VERSION, 'missing', 'client bootstrap')
  const context: unknown = JSON.parse(element.textContent)
  assertClientBootstrap(context)
  return context as ClientRouteContext<TInitial>
}
