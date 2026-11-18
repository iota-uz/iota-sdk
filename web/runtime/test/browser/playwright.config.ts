
// Browser test of the runtime host: a package page and a whole-shell renderer
// around a legacy Templ page. Run (browsers via `playwright install chromium`):
//   cd web/client-host && node_modules/.bin/playwright test --config ../runtime/test/browser/playwright.config.ts
export default {
  testDir: '.',
  testMatch: '*.spec.ts',
  use: { baseURL: 'http://127.0.0.1:4179' },
  webServer: {
    command: 'node ../../../client-host/node_modules/vite/bin/vite.js --config vite.config.ts --host 127.0.0.1 --port 4179',
    cwd: import.meta.dirname,
    port: 4179,
    reuseExistingServer: true,
  },
}
