import { writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const args = process.argv.slice(2)
const url = args.find((arg) => !arg.startsWith('--'))
const cookieFlag = args.indexOf('--cookie')
const cookie = cookieFlag >= 0 ? args[cookieFlag + 1] : process.env.LENS_SESSION_COOKIE

if (!url) {
  console.error('Usage: just lens fixture <url> [--cookie "sid=..."]')
  process.exit(2)
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
const output = path.resolve(scriptDir, '../fixtures/live.json')
await writeFile(output, document.endsWith('\n') ? document : `${document}\n`)
console.log(`Wrote ${output}`)
