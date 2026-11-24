// The package's bounds, measured as a consumer ships them (minified, gzip, solid-js external):
// - the entry stays small and holds neither @xyflow/system nor d3;
// - the lazy canvas chunk with @xyflow/system and d3 is at most 52 KB gzip.
import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'
import { transformWithEsbuild } from 'vite'

const dist = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../package-dist')
// 50 KB before Tidy v2 (#1391): the orthogonal router (src/route.ts) runs in the canvas chunk.
// 51 KB before #1346: validation warnings on the cards, warnings of a passed save and issue fixes that
// follow their item. 51 KB as well before the runs view (#1347): the editor holds the Runs tab, the Dry
// run / Test node docks and the loaders of their own lazy chunk (the list, the run view, the drawer and
// both panels live there).
export const CHUNK_GZIP_LIMIT = 52 * 1024
// 24 KB before Tidy v2 (#1391), whose layout and its public helpers (tidyLayout, ensureLayout, placeNew) live in the entry.
const ENTRY_GZIP_LIMIT = 26 * 1024
const CANVAS = /XYPanZoom|zoomIdentity|d3-zoom|__zoom/

const files = (await readdir(dist)).filter((name) => name.endsWith('.js') && !name.includes('.server'))
const sizes = new Map()
for (const name of files) {
  const text = await readFile(path.join(dist, name), 'utf8')
  const minified = (await transformWithEsbuild(text, name, { minify: true, format: 'esm', loader: 'js' })).code
  if (/\bfunction createRoot\(|\bfunction createSignal\(/.test(minified)) throw new Error(`${name} bundles solid-js: it must stay external`)
  sizes.set(name, { text, gz: gzipSync(minified).length })
  console.log(`${name}: gzip ${(gzipSync(minified).length / 1024).toFixed(1)} KB (minified)`)
}

// Everything the entry imports statically is paid by every importer of the subpath.
const staticClosure = (name, seen = new Set()) => {
  if (seen.has(name)) return seen
  seen.add(name)
  const text = sizes.get(name)?.text ?? ''
  for (const match of text.matchAll(/(?:^|\n)\s*(?:import|export)[^'"\n]*from\s*["']\.\/([^"']+)["']/g)) staticClosure(match[1], seen)
  return seen
}
const entry = staticClosure('index.js')
let entryGz = 0
for (const name of entry) {
  const file = sizes.get(name)
  if (!file) continue
  if (CANVAS.test(file.text)) throw new Error(`the entry statically imports ${name}, which holds the canvas implementation: it must stay in the lazy chunk`)
  entryGz += file.gz
}
if (entryGz > ENTRY_GZIP_LIMIT) throw new Error(`the entry is ${(entryGz / 1024).toFixed(1)} KB gzip, over ${ENTRY_GZIP_LIMIT / 1024} KB`)

const canvas = [...sizes].filter(([name, file]) => !entry.has(name) && CANVAS.test(file.text))
if (canvas.length !== 1) throw new Error(`expected one lazy canvas chunk, found ${canvas.length}`)
const [name, file] = canvas[0]
if (file.gz > CHUNK_GZIP_LIMIT) throw new Error(`canvas chunk ${name} is ${(file.gz / 1024).toFixed(1)} KB gzip, over the ${CHUNK_GZIP_LIMIT / 1024} KB budget`)
console.log(`entry ${(entryGz / 1024).toFixed(1)} KB gzip; canvas chunk ${(file.gz / 1024).toFixed(1)} KB gzip (budget ${CHUNK_GZIP_LIMIT / 1024} KB)`)
