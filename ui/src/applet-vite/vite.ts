/**
 * Applet Frontend Kit: Vite config helpers for applets running behind a base path
 * with dev proxy and optional local SDK aliasing.
 */
import type { UserConfig } from 'vite'
import fs from 'node:fs'
import path from 'node:path'

export type AppletViteOptions = {
  /** Applet base path (e.g. "/admin/ali/chat" or "/bi-chat"); overridden by applet-dev.json when present. */
  basePath: string
  /** Backend URL for proxy (e.g. "http://localhost:3200"); overridden by applet-dev.json when present. */
  backendUrl: string
  /** Directory containing applet-dev.json (default: process.cwd()). When set, base/port/proxy use manifest if present. */
  viteConfigDir?: string
  /** Enable Vite aliases to local SDK dist for HMR when iterating on SDK (default: from IOTA_SDK_DIST) */
  enableLocalSdkAliases?: boolean
  /** Override SDK dist directory when enableLocalSdkAliases is true (default: process.env.IOTA_SDK_DIST) */
  sdkDistDir?: string
  /** Merge additional Vite config */
  extend?: UserConfig
}

/** Shape of applet-dev.json written by the Go dev runner (single source of truth when using `just dev <name>`). */
export type AppletDevManifest = {
  basePath: string
  assetsBase: string
  vitePort: number
  backendUrl: string
}

const APPLET_DEV_MANIFEST = 'applet-dev.json'

/**
 * Reads applet-dev.json from viteDir (default process.cwd()). When using `just dev <name>`, the Go runner writes this file.
 * Returns null if file is missing or invalid (env vars remain the fallback).
 */
export function readAppletDevManifest(viteDir?: string): AppletDevManifest | null {
  const dir = viteDir ?? process.cwd()
  const filePath = path.join(dir, APPLET_DEV_MANIFEST)
  try {
    const raw = fs.readFileSync(filePath, 'utf-8')
    const data = JSON.parse(raw) as AppletDevManifest
    if (typeof data.basePath === 'string' && typeof data.assetsBase === 'string' && typeof data.vitePort === 'number' && typeof data.backendUrl === 'string') {
      return data
    }
  } catch {
    // file missing or invalid
  }
  return null
}

const DEFAULT_DEDUPE = ['react', 'react-dom', 'react-router-dom', 'react-is']

export type AppletDevManifestOrNull = AppletDevManifest | null

/**
 * Returns base URL for assets (with trailing slash). Prefers manifest when provided (avoids re-reading file), then applet-dev.json, then APPLET_ASSETS_BASE env.
 */
export function getAppletAssetsBase(viteDir?: string, manifest?: AppletDevManifestOrNull): string {
  const resolved = manifest ?? readAppletDevManifest(viteDir)
  if (resolved) {
    const base = resolved.assetsBase
    return base.endsWith('/') ? base : base + '/'
  }
  const base = process.env.APPLET_ASSETS_BASE ?? ''
  return base.endsWith('/') ? base : base ? base + '/' : '/'
}

/**
 * Returns dev server port. Prefers manifest when provided (avoids re-reading file), then applet-dev.json, then APPLET_VITE_PORT env.
 */
export function getAppletVitePort(defaultPort = 5173, viteDir?: string, manifest?: AppletDevManifestOrNull): number {
  const resolved = manifest ?? readAppletDevManifest(viteDir)
  if (resolved) return resolved.vitePort
  const p = process.env.APPLET_VITE_PORT
  if (p === undefined || p === '') return defaultPort
  const n = Number(p)
  return Number.isFinite(n) ? n : defaultPort
}

/**
 * Builds a full Vite config for an applet: base, port, dedupe, proxy, optional local SDK aliases.
 * When applet-dev.json is present (e.g. when using `just dev <name>`), base, port, basePath, and backendUrl come from it.
 *
 * **Merge semantics for `extend`:** When you pass `extend`, it is merged with the base config as follows:
 * - **resolve.alias**: arrays are concatenated (base aliases first, then extend aliases).
 * - **plugins**: arrays are concatenated (base plugins first, then extend plugins).
 * - **server**, **base**, **resolve.dedupe** and other scalar/object fields: extend overrides base (Object.assign-style).
 * To fully override the base config, spread first: `defineConfig({ ...createAppletViteConfig(opts), ...yourOverrides })`.
 */
export function createAppletViteConfig(opts: AppletViteOptions): UserConfig {
  const manifest = readAppletDevManifest(opts.viteConfigDir)
  const base = getAppletAssetsBase(opts.viteConfigDir, manifest)
  const port = getAppletVitePort(5173, opts.viteConfigDir, manifest)
  const basePath = manifest?.basePath ?? opts.basePath
  const backendUrl = manifest?.backendUrl ?? opts.backendUrl
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
        basePath,
        backendUrl,
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
