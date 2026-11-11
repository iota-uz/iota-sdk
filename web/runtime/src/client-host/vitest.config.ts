import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'

const here = path.dirname(fileURLToPath(import.meta.url))
const clientHost = path.resolve(here, '../../../client-host')
const solidRoot = path.join(clientHost, 'node_modules/solid-js')

// Browser-graph tests of the runtime host. Run with
//   web/client-host/node_modules/.bin/vitest run --config web/runtime/src/client-host/vitest.config.ts
// (client-host owns jsdom and solid-js in the workspace).
export default defineConfig({
  root: here,
  test: {
    include: ['*.test.ts'],
    environment: 'jsdom',
    // One solid-js instance for the runtime code and the client host.
    server: { deps: { inline: [/solid-js/] } },
  },
  resolve: {
    alias: {
      '@iota-uz/sdk/client-host': path.join(clientHost, 'src/solid-index.ts'),
      'solid-js/web': path.join(solidRoot, 'web/dist/web.js'),
      'solid-js/store': path.join(solidRoot, 'store/dist/store.js'),
      'solid-js': path.join(solidRoot, 'dist/solid.js'),
    },
  },
  server: { fs: { allow: [path.resolve(here, '../../../..')] } },
})
