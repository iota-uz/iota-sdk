// Prepares the fixture host: the solid-ui stylesheet (Tailwind CLI, as solid-ui's own package build does)
// and the SDK fonts at /assets/fonts.
import { execFileSync } from 'node:child_process'
import { cp, mkdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const solidUI = path.resolve(root, '../solid-ui')
execFileSync('pnpm', ['exec', 'tailwindcss', '--input', 'src/standalone.css', '--output', path.join(root, 'fixture/solid-ui.generated.css')], { cwd: solidUI, stdio: 'inherit' })
await mkdir(path.join(root, 'fixture/public/assets'), { recursive: true })
await cp(path.resolve(root, '../../modules/core/presentation/assets/fonts'), path.join(root, 'fixture/public/assets/fonts'), { recursive: true })
