import { defineConfig } from '@playwright/test'
export default defineConfig({ testDir: './tests', retries: 0, use: { browserName: 'chromium' } })
