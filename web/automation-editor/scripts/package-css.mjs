// Inlines the @imports of src/styles.css into package-dist/styles.css (plain CSS, SDK tokens only).
import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const entry = path.join(root, 'src/styles.css')

async function inline(file, seen = new Set()) {
  if (seen.has(file)) return ''
  seen.add(file)
  const text = await readFile(file, 'utf8')
  const parts = []
  for (const line of text.split('\n')) {
    const match = /^@import\s+"([^"]+)";\s*$/.exec(line.trim())
    if (match) parts.push(await inline(path.resolve(path.dirname(file), match[1]), seen))
    else parts.push(line)
  }
  return parts.join('\n')
}

const css = await inline(entry)
if (/#[0-9a-fA-F]{3,8}\b/.test(css.replace(/\/\*[\s\S]*?\*\//g, ''))) throw new Error('automation-editor CSS must use SDK tokens, not hex colours (V1)')
await writeFile(path.join(root, 'package-dist/styles.css'), css)
