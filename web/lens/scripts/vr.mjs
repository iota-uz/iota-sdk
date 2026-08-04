import { spawnSync } from 'node:child_process'
import { existsSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const baselineDir = path.join(root, 'vr', 'baselines', process.platform)
const cliArgs = process.argv.slice(2)
const updateIndex = cliArgs.indexOf('--update')
const update = updateIndex >= 0
const forwarded = cliArgs.filter((_, index) => index !== updateIndex)
const hasBaseline = existsSync(baselineDir) && readdirSync(baselineDir).some((name) => name.endsWith('.png'))
const shouldUpdate = update || !hasBaseline

if (!hasBaseline && !update) {
  console.log(`No ${process.platform} baselines found; bootstrapping ${path.relative(root, baselineDir)}.`)
}

const pnpmDir = path.join(root, 'node_modules', '.pnpm')
const hasHermeticBrowser = existsSync(pnpmDir) && readdirSync(pnpmDir).some((entry) =>
  existsSync(path.join(pnpmDir, entry, 'node_modules', 'playwright-core', '.local-browsers')),
)
const env = { ...process.env }
if (hasHermeticBrowser && !env.PLAYWRIGHT_BROWSERS_PATH) env.PLAYWRIGHT_BROWSERS_PATH = '0'

const args = ['exec', 'playwright', 'test', ...forwarded]
if (shouldUpdate) args.push('--update-snapshots')
const result = spawnSync('pnpm', args, { cwd: root, env, stdio: 'inherit' })
process.exit(result.status ?? 1)
