import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './browser-tests',
  timeout: 60_000,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:4184', locale: 'en-US', timezoneId: 'UTC', deviceScaleFactor: 1,
    launchOptions: { args: ['--disable-gpu', '--force-color-profile=srgb', '--disable-lcd-text'] },
  },
  // Q1: screenshots of every state against the Linux baseline.
  snapshotPathTemplate: 'browser-tests/__screenshots__/{platform}/{arg}{ext}',
  expect: { toHaveScreenshot: { animations: 'disabled', caret: 'hide', scale: 'css', threshold: 0.1, maxDiffPixels: 0 } },
  webServer: {
    command: './node_modules/.bin/vite fixtures/automations --host 127.0.0.1 --port 4184 --strictPort',
    port: 4184,
    reuseExistingServer: true,
  },
})
