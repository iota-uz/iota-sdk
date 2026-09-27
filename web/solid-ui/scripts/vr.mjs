import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const baselineDir = path.join(root, 'vr', 'baselines', process.platform)
const cliArgs = process.argv.slice(2)
const update = cliArgs.includes('--update')
const forwarded = cliArgs.filter((arg) => arg !== '--update')
const hasBaseline = existsSync(baselineDir)
  && readdirSync(baselineDir).some((name) => name.endsWith('.png'))
const shouldUpdate = update || !hasBaseline

const packageBuild = spawnSync('pnpm', ['build:package'], { cwd: root, env: process.env, stdio: 'inherit' })
if ((packageBuild.status ?? 1) !== 0) process.exit(packageBuild.status ?? 1)

if (!hasBaseline && !update) {
  console.log(`No ${process.platform} baselines found; bootstrapping ${path.relative(root, baselineDir)}.`)
}

const env = { ...process.env }
const pnpmDir = path.join(root, 'node_modules', '.pnpm')
const hasHermeticBrowser = existsSync(pnpmDir) && readdirSync(pnpmDir).some((entry) =>
  existsSync(path.join(pnpmDir, entry, 'node_modules', 'playwright-core', '.local-browsers')),
)
if (hasHermeticBrowser && !env.PLAYWRIGHT_BROWSERS_PATH) env.PLAYWRIGHT_BROWSERS_PATH = '0'

const args = ['exec', 'playwright', 'test', ...forwarded]
if (shouldUpdate) args.push('--update-snapshots')
const report = path.join(root, 'vr', 'results', 'report.json')
const missingMarker = path.join(root, 'vr', 'results', 'missing-baselines.txt')
rmSync(report, { force: true })
rmSync(missingMarker, { force: true })
if (!shouldUpdate) {
  args.push('--reporter=json')
  env.PLAYWRIGHT_JSON_OUTPUT_NAME = report
}

const result = spawnSync('pnpm', args, { cwd: root, env, stdio: 'inherit' })
if ((result.status ?? 1) !== 0 && !shouldUpdate && existsSync(report)) {
  const parsed = JSON.parse(readFileSync(report, 'utf8'))
  const failures = []
  const visit = (suite) => {
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests ?? []) {
        for (const run of test.results ?? []) {
          if (run.status === 'unexpected') failures.push({ title: spec.title, errors: run.errors ?? [] })
        }
      }
    }
    for (const child of suite.suites ?? []) visit(child)
  }
  for (const suite of parsed.suites ?? []) visit(suite)
  const errorText = (error) => error.message || error.value || ''
  const missingOnly = failures.length > 0 && failures.every((failure) =>
    failure.errors.some((error) => errorText(error).includes("A snapshot doesn't exist")),
  )
  if (missingOnly) {
    const names = failures.map((failure) => failure.title).sort()
    console.log(`Missing ${names.length} approved ${process.platform} baselines; generating candidates.`)
    const updateResult = spawnSync(
      'pnpm',
      ['exec', 'playwright', 'test', ...forwarded, '--update-snapshots'],
      { cwd: root, env, stdio: 'inherit' },
    )
    if ((updateResult.status ?? 1) === 0) {
      writeFileSync(missingMarker, `${names.join('\n')}\n`)
      process.exit(0)
    }
    process.exit(updateResult.status ?? 1)
  }
}

process.exit(result.status ?? 1)
