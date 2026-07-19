import path from 'node:path'
import { defineConfig } from '@playwright/test'

const platform = process.platform

export default defineConfig({
  testDir: './vr',
  outputDir: './vr/results',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 1,
  reporter: process.env.CI ? [['list']] : [['line']],
  snapshotPathTemplate: path.join('vr', 'baselines', platform, '{arg}{ext}'),
  expect: {
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      scale: 'css',
      threshold: 0,
      maxDiffPixels: 0,
    },
  },
  use: {
    baseURL: 'http://127.0.0.1:61000',
    colorScheme: 'light',
    deviceScaleFactor: 1,
    // Canvas output must not depend on the host GPU: hardware raster is
    // nondeterministic run-to-run at maxDiffPixels 0.
    launchOptions: {
      args: ['--disable-gpu', '--force-color-profile=srgb', '--disable-lcd-text'],
    },
    locale: 'en-US',
    timezoneId: 'UTC',
    trace: 'retain-on-failure',
    viewport: { width: 1600, height: 1000 },
  },
  projects: [{
    name: 'chromium',
    use: { browserName: 'chromium' },
  }],
  webServer: {
    command: 'pnpm ladle:build && pnpm ladle:preview',
    url: 'http://127.0.0.1:61000',
    reuseExistingServer: false,
    timeout: 120_000,
  },
})
