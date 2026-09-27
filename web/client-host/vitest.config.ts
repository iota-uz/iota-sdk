import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

const packageRoot = path.dirname(fileURLToPath(import.meta.url))
const solidRoot = path.join(packageRoot, 'node_modules/solid-js')

export default defineConfig({
  test: {
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    // Keep solid-js a single module instance: solid-js/web would otherwise
    // resolve its core import through the node condition (server build) and
    // split owner/cleanup registries across rendered components.
    server: { deps: { inline: [/solid-js/] } },
  },
  resolve: {
    alias: {
      'solid-js/web': path.join(solidRoot, 'web/dist/web.js'),
      'solid-js/store': path.join(solidRoot, 'store/dist/store.js'),
      'solid-js': path.join(solidRoot, 'dist/solid.js'),
    },
  },
})
