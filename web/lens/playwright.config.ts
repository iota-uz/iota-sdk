import path from 'node:path'
import { defineConfig } from '@playwright/test'

const platform = process.platform
const vrPort = Number(process.env.LENS_VR_PORT ?? '61000')
if (!Number.isInteger(vrPort) || vrPort < 1 || vrPort > 65_535) throw new Error('LENS_VR_PORT must be a valid TCP port')
const vrBaseURL = `http://127.0.0.1:${vrPort}`

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
    baseURL: vrBaseURL,
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
    command: `pnpm ladle:build && pnpm exec ladle preview --viteConfig .ladle/vite.config.ts --host 127.0.0.1 --port ${vrPort}`,
    url: vrBaseURL,
    reuseExistingServer: false,
    timeout: 120_000,
  },
})
