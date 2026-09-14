import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import solid from 'vite-plugin-solid'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [solid({ hot: false })],
  build: {
    outDir: path.resolve(rootDir, 'package-dist'),
    emptyOutDir: true,
    lib: {
      entry: path.resolve(rootDir, 'src/index.ts'),
      formats: ['es'],
      fileName: 'index',
    },
    rollupOptions: {
      external: ['solid-js', 'solid-js/web'],
    },
  },
})
