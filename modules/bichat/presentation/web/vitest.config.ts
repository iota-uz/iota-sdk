import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import solid from 'vite-plugin-solid'

const packageRoot = path.dirname(fileURLToPath(import.meta.url))
const solidRoot = path.join(packageRoot, 'node_modules/solid-js')

export default defineConfig({
  plugins: [solid({ hot: false })],
  test: {
    root: packageRoot,
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
    // Keep solid-js a single module instance across render boundaries, as in
    // web/client-host: mixed node/browser condition resolution would split
    // owner/cleanup registries and break onCleanup.
    server: { deps: { inline: [/solid-js/] } },
  },
  resolve: {
    alias: [{ find: '@', replacement: path.join(packageRoot, 'src') }],
  },
})
