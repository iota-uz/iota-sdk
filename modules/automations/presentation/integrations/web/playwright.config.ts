import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './browser-tests',
  timeout: 60_000,
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:4183', locale: 'en-US', timezoneId: 'UTC' },
  webServer: {
    command: './node_modules/.bin/vite fixtures/integrations --host 127.0.0.1 --port 4183 --strictPort',
    port: 4183,
    reuseExistingServer: true,
  },
})
