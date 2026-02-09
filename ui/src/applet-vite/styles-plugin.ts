/**
 * Vite plugin that provides a virtual module exporting the compiled applet CSS string
 * for Shadow DOM injection (defineReactAppletElement({ styles })).
 * Can compile CSS via Tailwind CLI in the plugin or fall back to reading a prebuilt file.
 */
import type { Plugin } from 'vite'
import { spawnSync } from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import { createRequire } from 'node:module'
import os from 'node:os'
import path from 'node:path'

export const VIRTUAL_APPLET_STYLES_ID = 'virtual:applet-styles'
const RESOLVED_ID = '\0' + VIRTUAL_APPLET_STYLES_ID

export type AppletStylesVirtualModuleOptions = {
  /**
   * Input CSS for Tailwind (e.g. src/index.css). When set, the plugin tries to run Tailwind CLI to compile.
   * Default: "src/index.css"
   */
  inputCss?: string
  /**
   * Path to Tailwind config (optional). If not set, CLI uses its default lookup.
   */
  tailwindConfigPath?: string
  /**
   * Path to the compiled CSS file when not using Tailwind CLI (fallback), or when Tailwind is not available.
   * Default: "dist/style.css"
   */
  outputCssPath?: string
  /**
   * CSS to prepend: package specifiers (e.g. "@iota-uz/sdk/bichat/styles.css") or paths relative to project root.
   * Specifiers are resolved via Node resolution from the project root.
   * Default: []
   */
  prependCss?: string[]
}

/**
 * Resolves a prepend CSS entry: absolute paths returned as-is; non-relative, non-absolute
 * specifiers (e.g. @iota-uz/sdk/bichat/styles.css or some-lib/styles.css) are resolved via
 * Node module resolution first; relative paths are joined to root.
 */
function resolvePrependPath(specifierOrPath: string, root: string): string | null {
  if (path.isAbsolute(specifierOrPath)) {
    return specifierOrPath
  }
  if (!specifierOrPath.startsWith('.') && !specifierOrPath.startsWith('/')) {
    try {
      const req = createRequire(path.join(root, 'package.json'))
      return req.resolve(specifierOrPath)
    } catch {
      return null
    }
  }
  return path.join(root, specifierOrPath)
}

/**
 * Creates a Vite plugin that registers virtual:applet-styles and exports the compiled CSS string.
 * When inputCss is set, tries to run Tailwind CLI to compile; if Tailwind is not available, falls back to reading outputCssPath.
 */
export function createAppletStylesVirtualModulePlugin(
  options: AppletStylesVirtualModuleOptions = {}
): Plugin {
  const inputCss = options.inputCss ?? 'src/index.css'
  const outputCssPath = options.outputCssPath ?? 'dist/style.css'
  const tailwindConfigPath = options.tailwindConfigPath
  const prependCss = options.prependCss ?? []
  let root = process.cwd()

  return {
    name: 'applet-styles-virtual-module',
    configResolved(config) {
      root = config.root
    },
    resolveId(id: string) {
      if (id === VIRTUAL_APPLET_STYLES_ID) return RESOLVED_ID
      return null
    },
    load(id: string) {
      if (id !== RESOLVED_ID) return null

      const parts: string[] = []

      for (const p of prependCss) {
        const full = resolvePrependPath(p, root)
        if (full) {
          try {
            parts.push(fs.readFileSync(full, 'utf-8'))
          } catch {
            // Prepend file missing (e.g. optional SDK styles); skip
          }
        }
      }

      const inputPath = path.join(root, inputCss)
      let mainCss: string | null = null

      // Try Tailwind CLI if input file exists. spawnSync blocks the event loop; for parallel builds consider a random suffix (e.g. crypto.randomUUID()).
      if (fs.existsSync(inputPath)) {
        const tmpFile = path.join(os.tmpdir(), `applet-styles-${Date.now()}-${crypto.randomUUID()}.css`)
        const args = ['-i', inputPath, '-o', tmpFile]
        if (tailwindConfigPath) {
          const configPath = path.join(root, tailwindConfigPath)
          if (fs.existsSync(configPath)) {
            args.push('-c', configPath)
          }
        }
        const result = spawnSync('pnpm', ['exec', 'tailwindcss', ...args], {
          cwd: root,
          shell: true,
          stdio: 'pipe',
        })
        if (result.status === 0 && fs.existsSync(tmpFile)) {
          mainCss = fs.readFileSync(tmpFile, 'utf-8')
          try {
            fs.unlinkSync(tmpFile)
          } catch {
            /* ignore */
          }
        }
      }

      if (mainCss === null) {
        const fallbackPath = path.isAbsolute(outputCssPath) ? outputCssPath : path.join(root, outputCssPath)
        try {
          mainCss = fs.readFileSync(fallbackPath, 'utf-8')
        } catch {
          if (process.env.NODE_ENV !== 'production') {
            console.warn(
              `[applet-styles] Could not compile via Tailwind CLI and ${fallbackPath} not found; export empty string. Install tailwindcss in the project or build CSS first.`
            )
          }
          mainCss = ''
        }
      }

      parts.push(mainCss)
      const css = parts.join('\n')
      return `export default ${JSON.stringify(css)}`
    },
  }
}

/**
 * Convenience plugin for BiChat applets: compiles app CSS and prepends SDK BiChat styles.
 * Uses default input src/index.css and package specifier @iota-uz/sdk/bichat/styles.css.
 */
export function createBichatStylesPlugin(
  options: Omit<AppletStylesVirtualModuleOptions, 'prependCss'> = {}
): Plugin {
  return createAppletStylesVirtualModulePlugin({
    ...options,
    inputCss: options.inputCss ?? 'src/index.css',
    outputCssPath: options.outputCssPath ?? 'dist/style.css',
    prependCss: ['@iota-uz/sdk/bichat/styles.css'],
  })
}
