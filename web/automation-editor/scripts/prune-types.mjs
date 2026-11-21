// Keeps only the declarations the public entry reaches, and proves no @xyflow/system type leaks out.
import { readdir, readFile, rm } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const dist = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../package-dist')
const reached = new Set()
const visit = async (file) => {
  if (reached.has(file)) return
  reached.add(file)
  const text = await readFile(file, 'utf8')
  if (/@xyflow|d3-/.test(text)) throw new Error(`${path.relative(dist, file)} exposes an @xyflow/system or d3 type`)
  for (const match of text.matchAll(/(?:from|import\()\s*['"](\.{1,2}\/[^'"]+)['"]/g)) {
    const target = path.resolve(path.dirname(file), match[1].replace(/\.js$/, ''))
    await visit(`${target}.d.ts`).catch(async () => visit(path.join(target, 'index.d.ts')))
  }
}
await visit(path.join(dist, 'index.d.ts'))
const walk = async (dir) => {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) await walk(file)
    else if (file.endsWith('.d.ts') && !reached.has(file)) await rm(file)
  }
}
await walk(dist)
console.log(`types: ${reached.size} declaration files reachable from index.d.ts`)
