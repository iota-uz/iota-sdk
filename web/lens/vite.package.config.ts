import path from 'node:path'
import { fileURLToPath } from 'node:url'
import solid from 'vite-plugin-solid'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  resolve: {
    alias: {
      '@iota-uz/sdk/client-host': path.resolve(rootDir, '../client-host/src/index.ts'),
    },
    dedupe: ['solid-js'],
  },
  plugins: [solid()],
  build: {
    outDir: path.resolve(rootDir, 'package-dist'),
    emptyOutDir: true,
    cssCodeSplit: false,
    lib: { entry: path.resolve(rootDir, 'src/index.ts'), formats: ['es'], fileName: 'index' },
    rollupOptions: {
      external: ['solid-js', 'solid-js/web', 'solid-js/store', '@iota-uz/sdk/client-host'],
      output: { assetFileNames: (asset) => asset.name?.endsWith('.css') ? 'style.css' : 'assets/[name]-[hash][extname]' },
    },
  },
})
