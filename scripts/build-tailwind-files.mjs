import { cp, mkdir, writeFile } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import path from 'node:path'

const repoRoot = path.resolve(process.cwd())
const outDir = path.join(repoRoot, 'tailwind')

await mkdir(outDir, { recursive: true })

const iotaCssPath = path.join(outDir, 'iota.css')
await cp(path.join(repoRoot, 'styles/tailwind/iota.css'), iotaCssPath)

const mainCssPath = path.join(outDir, 'main.css')
await writeFile(
  mainCssPath,
  `@import "tailwindcss";\n@import "./iota.css";\n`,
  'utf8',
)

const createConfigPath = path.join(outDir, 'create-config.cjs')
await cp(
  path.join(repoRoot, 'ui/tailwind/create-config.cjs'),
  createConfigPath,
)

const compiledInputPath = path.join(repoRoot, 'styles/tailwind/input.css')
const compiledCssPath = path.join(outDir, 'compiled.css')

const compiled = spawnSync(
  'pnpm',
  [
    'exec',
    'tailwindcss',
    '--input',
    compiledInputPath,
    '--output',
    compiledCssPath,
    '--minify',
  ],
  { cwd: repoRoot, stdio: 'inherit' },
)

if (compiled.status !== 0) {
  throw new Error(`tailwindcss compilation failed with exit code ${compiled.status}`)
}
