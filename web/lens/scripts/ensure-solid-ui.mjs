// The package declarations import solid-ui's published types (PeriodChips uses the shared
// filter-chip kit); build them when this package is built on its own.
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const solidUI = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../solid-ui')
// Both declarations Lens maps (tsconfig.package.json): a build that stopped
// after tsc leaves index.d.ts without embedded-css.d.ts, and a retry must
// rebuild rather than skip.
const required = ['package-dist/index.d.ts', 'package-dist/embedded-css.d.ts']
if (!required.every((file) => existsSync(path.join(solidUI, file)))) {
  execFileSync('pnpm', ['build:package'], { cwd: solidUI, stdio: 'inherit' })
}
