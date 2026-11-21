import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitest/config'
import solid from 'vite-plugin-solid'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [solid({ hot: false })],
  resolve: {
    alias: { '@iota-uz/sdk/solid-ui': path.resolve(rootDir, '../solid-ui/src/index.ts') },
    conditions: ['development', 'browser'],
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: './src/test/setup.ts',
    restoreMocks: true,
  },
})
