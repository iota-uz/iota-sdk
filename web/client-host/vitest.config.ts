import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

const packageRoot = path.dirname(fileURLToPath(import.meta.url))
const solidRoot = path.join(packageRoot, 'node_modules/solid-js')

export default defineConfig({
  test: { include: ['src/**/*.test.ts', 'src/**/*.test.tsx'] },
  resolve: {
    alias: {
      'solid-js/web': path.join(solidRoot, 'web/dist/web.js'),
      'solid-js/store': path.join(solidRoot, 'store/dist/store.js'),
      'solid-js': path.join(solidRoot, 'dist/solid.js'),
    },
  },
})
