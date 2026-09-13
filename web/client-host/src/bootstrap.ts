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

export function assertClientHostProtocol(actual: string): void {
  if (major(actual) !== major(CLIENT_HOST_PROTOCOL_VERSION)) {
    throw new ClientHostProtocolError(CLIENT_HOST_PROTOCOL_VERSION, actual)
  }
}

export function assertClientBootstrap(context: ClientRouteContext, requireVersion = true): void {
  assertClientHostProtocol(context.protocolVersion)
  assertSDKIdentity({
    releaseVersion: context.sdkReleaseVersion,
    sourceCommit: context.sdkCommit,
    protocolVersion: context.protocolVersion,
  })
  if (requireVersion && major(context.bootstrapVersion ?? '') !== major(CLIENT_BOOTSTRAP_VERSION)) {
    throw new ClientHostProtocolError(CLIENT_BOOTSTRAP_VERSION, context.bootstrapVersion ?? 'missing', 'client bootstrap')
  }
}

export function readClientBootstrap<TInitial = unknown>(document: Document, elementID = 'iota-client-context'): ClientRouteContext<TInitial> {
  const element = document.getElementById(elementID)
  if (!element?.textContent) throw new ClientHostProtocolError(CLIENT_BOOTSTRAP_VERSION, 'missing', 'client bootstrap')
  const context = JSON.parse(element.textContent) as ClientRouteContext<TInitial>
  assertClientBootstrap(context)
  return context
}
