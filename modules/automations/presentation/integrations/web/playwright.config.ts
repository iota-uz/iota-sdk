import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './browser-tests',
  timeout: 60_000,
  workers: 1,
  use: {
    baseURL: 'http://127.0.0.1:4183', locale: 'en-US', timezoneId: 'UTC', deviceScaleFactor: 1,
    launchOptions: { args: ['--disable-gpu', '--force-color-profile=srgb', '--disable-lcd-text'] },
  },
  // Q1: screenshots of every state against the Linux baseline, like the solid-ui gallery.
  snapshotPathTemplate: 'browser-tests/__screenshots__/{platform}/{arg}{ext}',
  // threshold: per-pixel colour distance that absorbs antialiasing of rounded borders, which varies by run;
  // any pixel beyond it fails (maxDiffPixels 0).
  expect: { toHaveScreenshot: { animations: 'disabled', caret: 'hide', scale: 'css', threshold: 0.1, maxDiffPixels: 0 } },
  webServer: {
    command: './node_modules/.bin/vite fixtures/integrations --host 127.0.0.1 --port 4183 --strictPort',
    port: 4183,
    reuseExistingServer: true,
  },
})
