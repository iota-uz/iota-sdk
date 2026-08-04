import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
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
  /** @param {unknown} value @returns {value is Record<string, unknown>} */
  const isRecord = (value) => typeof value === 'object' && value !== null && !Array.isArray(value)
  /** @param {unknown} value @param {string} key @returns {unknown[]} */
  const arrayField = (value, key) => isRecord(value) && Array.isArray(value[key]) ? value[key] : []
  /** @param {unknown} value @param {string} key @returns {string} */
  const stringField = (value, key) => isRecord(value) && typeof value[key] === 'string' ? value[key] : ''
  /** @param {unknown} error @returns {string} */
  const errorText = (error) => stringField(error, 'message') || stringField(error, 'value')
  /** @type {{ title: string, errors: unknown[] }[]} */
  const failures = []
  /** @param {unknown} suite */
  const visit = (suite) => {
    for (const spec of arrayField(suite, 'specs')) {
      for (const test of arrayField(spec, 'tests')) {
        for (const run of arrayField(test, 'results')) {
          if (stringField(run, 'status') === 'unexpected') {
            failures.push({ title: stringField(spec, 'title'), errors: arrayField(run, 'errors') })
          }
        }
      }
    }
    for (const child of arrayField(suite, 'suites')) visit(child)
  }
  const parsed = /** @type {unknown} */ (JSON.parse(readFileSync(report, 'utf8')))
  for (const suite of arrayField(parsed, 'suites')) visit(suite)
  const missingOnly = failures.length > 0 && failures.every((failure) =>
    failure.errors.some((error) => errorText(error).includes("A snapshot doesn't exist")),
  )
  if (missingOnly) {
    const names = failures.map((failure) => failure.title).sort()
    console.log(`Missing ${names.length} approved ${process.platform} baselines; generating candidates. Review and promote them before merge:`)
    console.log(names.map((name) => `  - ${name}`).join('\n'))
    const updateResult = spawnSync('pnpm', ['exec', 'playwright', 'test', ...forwarded, '--update-snapshots'], { cwd: root, env, stdio: 'inherit' })
    if ((updateResult.status ?? 1) === 0) {
      writeFileSync(missingMarker, `${names.join('\n')}\n`)
      process.exit(0)
    }
    process.exit(updateResult.status ?? 1)
  }
}
process.exit(result.status ?? 1)
