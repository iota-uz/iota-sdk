import type { ClientRouteContext } from './bootstrap'
import type { SessionService } from './transport'

export interface StandardAppletContext {
  config: {
    basePath: string
    rpcUIEndpoint: string
    streamEndpoint?: string
    uploadEndpoint?: string
    assetsBasePath?: string
    shellMode?: string
  }
  session?: { csrfToken?: string }
  user?: unknown
  tenant?: unknown
  locale?: unknown
  extensions?: unknown
}

export interface StandardAppletInitial<TContext extends StandardAppletContext = StandardAppletContext> {
  appletContext: TContext
}

export interface BiChatHTTPConfig {
  baseUrl: string
  rpcEndpoint: string
  streamEndpoint?: string
  uploadEndpoint?: string
  csrfToken?: string | (() => string)
  headers?: Record<string, string>
  rpcTimeoutMs?: number
  streamConnectTimeoutMs?: number
}

/** Replaces useHttpDataSourceConfigFromApplet/window globals for client routes. */
export function biChatHTTPConfigFromRoute<TContext extends StandardAppletContext>(
  route: ClientRouteContext<StandardAppletInitial<TContext>>,
  session?: SessionService,
  options: { rpcTimeoutMs?: number; streamConnectTimeoutMs?: number } = {},
): BiChatHTTPConfig {
  const config = route.initial.appletContext.config
  return {
    // Applet context endpoints are route-ready paths (and may also be absolute
    // URLs). HttpDataSource concatenates baseUrl with each endpoint, so using
    // the applet's mount basePath here would incorrectly turn `/rpc` into
    // `/admin/ali/chat/rpc` and prefix the stream endpoint a second time.
    baseUrl: '',
    rpcEndpoint: config.rpcUIEndpoint,
    ...(config.streamEndpoint ? { streamEndpoint: config.streamEndpoint } : {}),
    ...(config.uploadEndpoint ? { uploadEndpoint: config.uploadEndpoint } : {}),
    ...(session ? { csrfToken: () => session.snapshot().csrf ?? '' } : route.csrf ? { csrfToken: route.csrf } : {}),
    ...options,
  }
}

