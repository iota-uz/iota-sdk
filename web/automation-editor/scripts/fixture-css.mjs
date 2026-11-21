// Builds the solid-ui stylesheet (Tailwind CLI, as solid-ui's own package build does) for the fixture host.
import { execFileSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const solidUI = path.resolve(root, '../solid-ui')
execFileSync('pnpm', ['exec', 'tailwindcss', '--input', 'src/standalone.css', '--output', path.join(root, 'fixture/solid-ui.generated.css')], { cwd: solidUI, stdio: 'inherit' })
