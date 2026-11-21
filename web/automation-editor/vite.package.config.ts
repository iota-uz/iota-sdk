import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

// The published chunk: only solid-js (and the SDK's own solid-ui subpath) stay external.
// @xyflow/system and d3 are bundled into the chunk, as Lens bundles ECharts.
export default defineConfig({
  plugins: [solid({ hot: false })],
  resolve: { dedupe: ['solid-js'] },
  build: {
    outDir: path.resolve(rootDir, 'package-dist'),
    emptyOutDir: true,
    cssCodeSplit: false,
    lib: { entry: path.resolve(rootDir, 'src/index.ts'), formats: ['es'], fileName: 'index' },
    rollupOptions: {
      external: ['solid-js', 'solid-js/web', 'solid-js/store', '@iota-uz/sdk/solid-ui'],
    },
  },
})
