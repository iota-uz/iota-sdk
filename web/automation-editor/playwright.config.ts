import { defineConfig, devices } from '@playwright/test'

const port = Number(process.env.AE_FIXTURE_PORT ?? '5181')
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './e2e',
  outputDir: './e2e/results',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 1,
  reporter: process.env.CI ? [['list']] : [['line']],
  use: {
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    trace: 'retain-on-failure',
    viewport: { width: 1440, height: 900 },
  },
  projects: [
    { name: 'e2e', testMatch: /editor\.spec\.ts/, use: { browserName: 'chromium' } },
    { name: 'touch', testMatch: /touch\.spec\.ts/, use: { browserName: 'chromium', hasTouch: true, viewport: { width: 1180, height: 820 } } },
    { name: 'ipad', testMatch: /touch\.spec\.ts/, use: { ...devices['iPad (gen 7) landscape'], browserName: 'webkit' } },
    { name: 'bench', testMatch: /bench\.spec\.ts/, use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: 'node scripts/fixture-css.mjs && pnpm exec vite build && pnpm exec vite preview --port ' + port + ' --strictPort',
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
})
