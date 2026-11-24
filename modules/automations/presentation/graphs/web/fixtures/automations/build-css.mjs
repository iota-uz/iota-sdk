// Builds the fixture's stylesheet with the SDK's Tailwind pipeline (tokens, iota.css, the screen and solid-ui sources).
import { spawnSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = path.dirname(fileURLToPath(import.meta.url))
const result = spawnSync('pnpm', ['exec', 'tailwindcss', '--input', path.join(dir, 'fixture.css'), '--output', path.join(dir, 'fixture.generated.css')], { stdio: 'inherit', cwd: path.resolve(dir, '../..') })
process.exit(result.status ?? 1)
