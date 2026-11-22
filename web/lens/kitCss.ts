import { readFileSync } from 'node:fs'
import postcss from 'postcss'
import tailwindcss from '@tailwindcss/postcss'
import type { Plugin } from 'vite'

const suffix = '?kit-css'
const prefix = '\0kit-css:'

/**
 * Compiles a Tailwind entry sheet to a string module (`import css from
 * './sheet.css?kit-css'`). Development, tests and stories use it for the
 * shared filter-chip kit's utility sheet, which the published package gets
 * prebuilt from `@iota-uz/sdk/solid-ui/embedded-css`. Vite's own `?inline`
 * cannot stand in: it hands Tailwind a path it scans no sources from, so the
 * sheet comes out empty.
 */
export function kitCss(): Plugin {
  return {
    name: 'lens-kit-css',
    enforce: 'pre',
    async resolveId(source, importer) {
      if (!source.endsWith(suffix)) return null
      const resolved = await this.resolve(source.slice(0, -suffix.length), importer, { skipSelf: true })
      // The .mjs ending keeps Vite's CSS pipeline off the virtual module.
      return resolved ? `${prefix}${resolved.id}.mjs` : null
    },
    async load(id) {
      if (!id.startsWith(prefix)) return null
      const file = id.slice(prefix.length, -'.mjs'.length)
      this.addWatchFile(file)
      const result = await postcss([tailwindcss()]).process(readFileSync(file, 'utf8'), { from: file })
      for (const message of result.messages) {
        if (message.type === 'dependency' && typeof message.file === 'string') this.addWatchFile(message.file)
      }
      return `export default ${JSON.stringify(result.css)}`
    },
  }
}
