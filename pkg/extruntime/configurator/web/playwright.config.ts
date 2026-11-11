import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './browser-tests',
  timeout: 60_000,
  use: { baseURL: 'http://127.0.0.1:4179' },
  webServer: {
    command: 'pnpm vite fixtures/configurator --host 127.0.0.1 --port 4179',
    port: 4179,
    reuseExistingServer: true,
  },
})
