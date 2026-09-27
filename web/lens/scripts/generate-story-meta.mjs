/**
 * Emits dist/meta.json — the story inventory the VR manifest test fetches.
 * Story ids mirror Ladle's: `<file>--<story>` in kebab-case, with explicit
 * `storyName` labels winning over export names, commas dropped, and " - "
 * folded to "--".
 */
import { readFileSync, readdirSync, writeFileSync, mkdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const src = join(dirname(fileURLToPath(import.meta.url)), '..', 'src')
const outDir = process.argv[2] ?? join(dirname(fileURLToPath(import.meta.url)), '..', 'dist')
const files = readdirSync(src).filter((name) => name.endsWith('.stories.tsx'))

const kebabExport = (name) => name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
const kebabName = (name) => name.replace(/,/g, '').trim().toLowerCase().replace(/\s+-\s+/g, '--').replace(/\s+/g, '-')

const stories = {}
for (const file of files) {
  const base = file.replace('.stories.tsx', '')
  const prefix = kebabExport(base)
  const source = readFileSync(join(src, file), 'utf8')
  const exports = [...source.matchAll(/export const ([A-Za-z0-9]+)[^=\n]*=/g)]
  const labels = [...source.matchAll(/([A-Za-z0-9]+)\.storyName = '([^']+)'/g)]
  for (const match of exports) {
    const exportName = match[1]
    // The explicit label belongs to the nearest export declared before it.
    const label = labels
      .filter(([_, owner]) => owner === exportName)
      .map(([, , value]) => value)[0]
    const name = label ? kebabName(label) : kebabExport(exportName)
    stories[`${prefix}--${name}`] = { name, importPath: `./${file}` }
  }
}

mkdirSync(outDir, { recursive: true })
writeFileSync(join(outDir, 'meta.json'), JSON.stringify({ stories }, null, 2))
console.log(`meta.json: ${Object.keys(stories).length} stories`)
