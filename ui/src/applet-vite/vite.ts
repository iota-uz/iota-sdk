/**
 * Applet Frontend Kit: Vite config helpers for applets running behind a base path
 * with dev proxy and optional local SDK aliasing.
 */
import type { UserConfig } from 'vite'
import path from 'node:path'

export type AppletViteOptions = {
  /** Applet base path (e.g. "/admin/ali/chat" or "/bi-chat"). */
  basePath: string
  /** Backend URL for proxy (e.g. "http://localhost:3200"). */
  backendUrl: string
  /** Enable Vite aliases to local SDK dist for HMR when iterating on SDK (default: from IOTA_SDK_DIST) */
  enableLocalSdkAliases?: boolean
  /** Override SDK dist directory when enableLocalSdkAliases is true (default: process.env.IOTA_SDK_DIST) */
  sdkDistDir?: string
  /** Merge additional Vite config */
  extend?: UserConfig
}

const DEFAULT_DEDUPE = ['react', 'react-dom', 'react-router-dom', 'react-is']

/**
 * Returns base URL for assets (with trailing slash). Uses APPLET_ASSETS_BASE env if set, otherwise derives from basePath.
 */
export function getAppletAssetsBase(basePath: string): string {
  const normalized = basePath.replace(/\/+$/, '')
  const base = process.env.APPLET_ASSETS_BASE ?? normalized + '/assets/'
  return base.endsWith('/') ? base : base + '/'
}

/**
 * Returns dev server port from APPLET_VITE_PORT env, or the given default.
 */
export function getAppletVitePort(defaultPort = 5173): number {
  const p = process.env.APPLET_VITE_PORT
  if (p === undefined || p === '') return defaultPort
  const n = Number(p)
  return Number.isFinite(n) ? n : defaultPort
}

/**
 * Builds a full Vite config for an applet: base, port, dedupe, proxy, optional local SDK aliases.
 *
 * **Merge semantics for `extend`:** When you pass `extend`, it is merged with the base config as follows:
 * - **resolve.alias**: arrays are concatenated (base aliases first, then extend aliases).
 * - **plugins**: arrays are concatenated (base plugins first, then extend plugins).
 * - **server**, **base**, **resolve.dedupe** and other scalar/object fields: extend overrides base (Object.assign-style).
 * To fully override the base config, spread first: `defineConfig({ ...createAppletViteConfig(opts), ...yourOverrides })`.
 */
export function createAppletViteConfig(opts: AppletViteOptions): UserConfig {
  const base = getAppletAssetsBase(opts.basePath)
  const port = getAppletVitePort(5173)
  const config: UserConfig = {
    base,
    resolve: {
      dedupe: DEFAULT_DEDUPE,
      alias: createLocalSdkAliases({
        enabled: opts.enableLocalSdkAliases ?? Boolean(process.env.IOTA_SDK_DIST),
        sdkDistDir: opts.sdkDistDir ?? process.env.IOTA_SDK_DIST,
      }),
    },
    server: {
      port,
      strictPort: true,
      proxy: createAppletBackendProxy({
        basePath: opts.basePath,
        backendUrl: opts.backendUrl,
      }),
    },
  }
  if (opts.extend) {
    return mergeConfig(config, opts.extend)
  }
  return config
}

/**
 * Returns proxy entries for applet RPC and stream under basePath.
 * Use as server.proxy in Vite config.
 * Note: /stream is SSE; plain string targets do not set ws or changeOrigin. If WebSocket upgrade or SSE proxying issues arise, configure proxy with ws: true or a custom configure.
 */
export function createAppletBackendProxy(opts: {
  basePath: string
  backendUrl: string
}): Record<string, string> {
  const base = opts.basePath.replace(/\/+$/, '')
  const target = opts.backendUrl.replace(/\/+$/, '')
  return {
    [base + '/rpc']: target,
    [base + '/stream']: target,
  }
}

/**
 * Returns resolve.alias entries to point @iota-uz/sdk and @iota-uz/sdk/bichat to a local dist.
 * Use when IOTA_SDK_DIST is set or sdkDistDir is passed, so the app uses the local SDK build with HMR.
 */
export function createLocalSdkAliases(opts?: {
  enabled?: boolean
  sdkDistDir?: string
}): Array<{ find: string | RegExp; replacement: string }> {
  const enabled = opts?.enabled ?? Boolean(opts?.sdkDistDir ?? process.env.IOTA_SDK_DIST)
  const dir = opts?.sdkDistDir ?? process.env.IOTA_SDK_DIST
  if (!enabled || !dir) return []
  const sdkDist = path.resolve(dir)
  return [
    { find: /^@iota-uz\/sdk\/bichat$/, replacement: path.join(sdkDist, 'bichat/index.mjs') },
    { find: /^@iota-uz\/sdk$/, replacement: path.join(sdkDist, 'index.mjs') },
  ]
}

/**
 * Merges base config with extend. Start with merged = { ...a, ...b } so no Vite fields from b are dropped.
 * We capture a.resolve, b.resolve, a.server, b.server, a.plugins, b.plugins before the spread so we never
 * read b's values as "original" a when merging. resolve.alias: only coerce to array when both sides are
 * actually arrays (concat); otherwise leave Record/object as-is and prefer b's value then a's.
 */
function mergeConfig(a: UserConfig, b: UserConfig): UserConfig {
  const aResolve = a.resolve
  const bResolve = b.resolve
  const aServer = a.server
  const bServer = b.server
  const aPlugins = a.plugins
  const bPlugins = b.plugins

  const merged: UserConfig = { ...a, ...b }

  if (bResolve) {
    const aAlias = aResolve?.alias
    const bAlias = bResolve.alias
    const aIsArray = Array.isArray(aAlias)
    const bIsArray = Array.isArray(bAlias)
    const alias =
      aIsArray && bIsArray
        ? [...(aAlias as Array<{ find: string | RegExp; replacement: string }>), ...(bAlias as Array<{ find: string | RegExp; replacement: string }>)]
        : (bAlias !== undefined ? bAlias : aResolve?.alias)
    merged.resolve = {
      ...aResolve,
      ...bResolve,
      alias,
      dedupe: bResolve.dedupe ?? merged.resolve?.dedupe ?? aResolve?.dedupe,
    }
  }
  if (bServer) {
    merged.server = { ...aServer, ...bServer }
  }
  if (bPlugins) {
    merged.plugins = [...(aPlugins ?? []), ...bPlugins]
  }
  return merged
}
