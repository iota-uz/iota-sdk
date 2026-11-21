import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

// The server build of the same entry: importing the subpath during SSR must not touch the DOM.
export default defineConfig({
  plugins: [solid({ hot: false, ssr: true })],
  build: {
    ssr: path.resolve(rootDir, 'src/index.ts'),
    outDir: path.resolve(rootDir, 'package-dist'),
    emptyOutDir: false,
    rollupOptions: {
      external: ['solid-js', 'solid-js/web', 'solid-js/store', '@iota-uz/sdk/solid-ui'],
      output: { entryFileNames: 'index.server.js' },
    },
  },
  ssr: { noExternal: ['@xyflow/system', /^d3-/] },
})
