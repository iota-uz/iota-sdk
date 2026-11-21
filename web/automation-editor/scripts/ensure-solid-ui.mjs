// The declarations import solid-ui's published types; build them when this package is built on its own.
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const solidUI = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../solid-ui')
if (!existsSync(path.join(solidUI, 'package-dist/index.d.ts'))) {
  execFileSync('pnpm', ['build:package'], { cwd: solidUI, stdio: 'inherit' })
}
