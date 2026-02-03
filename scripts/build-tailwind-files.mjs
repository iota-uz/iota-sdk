import { cp, mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'

const repoRoot = path.resolve(process.cwd())
const outDir = path.join(repoRoot, 'tailwind')

const sdkMainCssPath = path.join(
  repoRoot,
  'modules/core/presentation/assets/css/main.css',
)

await mkdir(outDir, { recursive: true })

const sdkMainCss = await readFile(sdkMainCssPath, 'utf8')

const sdkMainCssBody = (() => {
  const lines = sdkMainCss.split('\n')
  while (
    lines.length > 0 &&
    (/^\s*@tailwind\b/.test(lines[0]) ||
      /^\s*@config\b/.test(lines[0]) ||
      /^\s*@import\s+["']tailwindcss["']\s*;?\s*$/.test(lines[0]))
  ) {
    lines.shift()
  }
  if (lines.length > 0 && lines[0].trim() === '') {
    lines.shift()
  }
  return lines.join('\n')
})()

const iotaCssPath = path.join(outDir, 'iota.css')
await writeFile(iotaCssPath, `${sdkMainCssBody}\n`, 'utf8')

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
