// Browser end-to-end suite of the extension runtime UI: real Chromium against the composed application of
// pkg/extruntime/e2e served by a real Go server (see global-setup.ts). Run with `just extruntime-browser`
// or, by hand from the repository root:
//
//   web/client-host/node_modules/.bin/playwright test --config pkg/extruntime/e2e/browser/playwright.config.ts
//
// @playwright/test is installed with web/client-host; the config, the runner and the specs share that copy.
import playwright from '../../../../web/client-host/node_modules/@playwright/test/index.js'

const { defineConfig, devices } = playwright

export const PORT = Number(process.env.IOTA_EXT_BROWSER_PORT ?? 4290)

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  globalSetup: './global-setup.ts',
  // One application, shared state (bindings, the shell): the specs run one after another.
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: process.env.CI ? [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]] : 'list',
  outputDir: 'test-results',
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    ...devices['Desktop Chrome'],
  },
})
