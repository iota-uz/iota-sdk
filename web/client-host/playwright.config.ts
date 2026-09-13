import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './browser-tests',
  use: { baseURL: 'http://127.0.0.1:4178' },
  webServer: {
    command: 'pnpm vite fixtures/product-editor --host 127.0.0.1 --port 4178',
    port: 4178,
    reuseExistingServer: true,
  },
})
