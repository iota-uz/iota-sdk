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

/**
 * Returns base URL for assets (with trailing slash). Prefers applet-dev.json when present, then APPLET_ASSETS_BASE env.
 */
export function getAppletAssetsBase(viteDir?: string): string {
  const manifest = readAppletDevManifest(viteDir)
  if (manifest) {
    const base = manifest.assetsBase
    return base.endsWith('/') ? base : base + '/'
  }
  const base = process.env.APPLET_ASSETS_BASE ?? ''
  return base.endsWith('/') ? base : base ? base + '/' : '/'
}

/**
 * Returns dev server port. Prefers applet-dev.json when present, then APPLET_VITE_PORT env.
 */
export function getAppletVitePort(defaultPort = 5173, viteDir?: string): number {
  const manifest = readAppletDevManifest(viteDir)
  if (manifest) return manifest.vitePort
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
  const base = manifest ? (manifest.assetsBase.endsWith('/') ? manifest.assetsBase : manifest.assetsBase + '/') : getAppletAssetsBase(opts.viteConfigDir)
  const port = manifest ? manifest.vitePort : getAppletVitePort(5173, opts.viteConfigDir)
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
 * Merges base config with extend: alias and plugins arrays are concatenated;
 * server, base, and other fields are overridden by b (spread).
 */
function mergeConfig(a: UserConfig, b: UserConfig): UserConfig {
  const merged = { ...a }
  if (b.base) merged.base = b.base
  if (b.resolve) {
    const aAlias = Array.isArray(merged.resolve?.alias) ? merged.resolve.alias : (merged.resolve?.alias ? [merged.resolve.alias] : [])
    const bAlias = Array.isArray(b.resolve.alias) ? b.resolve.alias : (b.resolve.alias ? [b.resolve.alias] : [])
    merged.resolve = {
      ...merged.resolve,
      ...b.resolve,
      alias: [...aAlias, ...bAlias],
      dedupe: b.resolve.dedupe ?? merged.resolve?.dedupe,
    }
  }
  if (b.server) {
    merged.server = { ...merged.server, ...b.server }
  }
  if (b.plugins) merged.plugins = [...(merged.plugins ?? []), ...b.plugins]
  return merged
}
