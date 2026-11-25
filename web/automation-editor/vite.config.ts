import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

// The fixture host: a fake EditorApi around the editor, for e2e, the benchmark and manual checks.
export default defineConfig({
  base: './',
  resolve: {
    alias: {
      '@iota-uz/sdk/solid-ui': path.resolve(rootDir, '../solid-ui/src/index.ts'),
      // As in the package (src/xy/d3-color.ts).
      'd3-color': path.resolve(rootDir, 'src/xy/d3-color.ts'),
    },
    dedupe: ['solid-js'],
  },
  plugins: [solid()],
  // The SDK's fonts at /assets/fonts, as the host serves them, so screenshots use the real typefaces.
  publicDir: path.resolve(rootDir, 'fixture/public'),
  server: { port: 5181, host: '0.0.0.0' },
  preview: { port: 5181, host: '0.0.0.0' },
  build: { outDir: path.resolve(rootDir, 'fixture-dist'), emptyOutDir: true },
})
