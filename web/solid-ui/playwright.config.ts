import path from 'node:path'
import { defineConfig } from '@playwright/test'

const platform = process.platform
const vrPort = Number(process.env.SOLID_UI_VR_PORT ?? '61010')
const templPort = Number(process.env.SOLID_UI_TEMPL_VR_PORT ?? '61011')
if (!Number.isInteger(vrPort) || vrPort < 1 || vrPort > 65_535) {
  throw new Error('SOLID_UI_VR_PORT must be a valid TCP port')
}

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
    baseURL: `http://127.0.0.1:${vrPort}`,
    colorScheme: 'light',
    deviceScaleFactor: 1,
    launchOptions: {
      args: ['--disable-gpu', '--force-color-profile=srgb', '--disable-lcd-text'],
    },
    locale: 'en-US',
    timezoneId: 'UTC',
    trace: 'retain-on-failure',
    viewport: { width: 1440, height: 900 },
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
  webServer: [
    {
      command: `pnpm exec vite build --outDir gallery-dist && mkdir -p gallery-dist/package-dist && cp package-dist/standalone.css gallery-dist/package-dist/standalone.css && cp -R package-dist/fonts gallery-dist/package-dist/fonts && pnpm exec vite preview --outDir gallery-dist --host 127.0.0.1 --port ${vrPort}`,
      url: `http://127.0.0.1:${vrPort}`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: `go run ./vr/templ-fixture -port ${templPort}`,
      url: `http://127.0.0.1:${templPort}/health`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
})
