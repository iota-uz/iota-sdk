import { writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const args = process.argv.slice(2)
const url = args.find((arg) => !arg.startsWith('--'))
const cookieFlag = args.indexOf('--cookie')
const cookie = cookieFlag >= 0 ? args[cookieFlag + 1] : process.env.LENS_SESSION_COOKIE
const outputFlag = args.indexOf('--output')
const outputName = outputFlag >= 0 ? args[outputFlag + 1] : 'small.json'

if (!url || !outputName) {
  console.error('Usage: just lens fixture <url> [--cookie "sid=..."] [--output <name.json>]')
  process.exit(2)
}
if (path.basename(outputName) !== outputName || path.extname(outputName) !== '.json') {
  throw new Error('--output must be a .json filename without directory components')
}

const response = await fetch(url, {
  headers: cookie ? { Cookie: cookie } : undefined,
})

if (!response.ok) {
  throw new Error(`fixture request failed with ${response.status} ${response.statusText}`)
}

const document = await response.text()
JSON.parse(document)
const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const output = path.resolve(scriptDir, '../fixtures', outputName)
await writeFile(output, document.endsWith('\n') ? document : `${document}\n`)
console.log(`Wrote ${output}`)
