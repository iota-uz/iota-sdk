import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { glob } from 'glob'
import { JSDOM } from 'jsdom'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const docsRoot = path.resolve(__dirname, '..')

const { window } = new JSDOM('<!doctype html><html><body></body></html>')
globalThis.window = window
globalThis.document = window.document
globalThis.Node = window.Node
globalThis.HTMLElement = window.HTMLElement
globalThis.SVGElement = window.SVGElement
globalThis.DOMParser = window.DOMParser
globalThis.location = window.location
globalThis.getComputedStyle = window.getComputedStyle
globalThis.CSS = window.CSS

if (!('navigator' in globalThis)) {
  Object.defineProperty(globalThis, 'navigator', {
    value: window.navigator,
    configurable: true,
  })
}

const { default: mermaid } = await import('mermaid')
mermaid.initialize({ startOnLoad: false, securityLevel: 'loose' })

function extractMermaidBlocks(contents) {
  const blocks = []
  const re = /```mermaid\s*\n([\s\S]*?)```/g
  let match
  while ((match = re.exec(contents)) !== null) {
    blocks.push(match[1].trim())
  }
  return blocks
}

async function validateBlock(block, { file, index }) {
  try {
    const result = mermaid.parse(block)
    if (result && typeof result.then === 'function') {
      await result
    }
    return null
  } catch (err) {
    const firstLine = block.split('\n')[0] || '(empty)'
    const message = err instanceof Error ? err.message : String(err)
    return { file, index, firstLine, message }
  }
}

async function main() {
  const files = await glob('content/**/*.{md,mdx}', { cwd: docsRoot, nodir: true })
  const failures = []

  for (const rel of files) {
    const abs = path.join(docsRoot, rel)
    const contents = await fs.readFile(abs, 'utf8')
    const blocks = extractMermaidBlocks(contents)
    for (let i = 0; i < blocks.length; i++) {
      const failure = await validateBlock(blocks[i], { file: rel, index: i + 1 })
      if (failure) failures.push(failure)
    }
  }

  if (failures.length) {
    for (const f of failures) {
      console.error(`[mermaid] ${f.file} block #${f.index}: ${f.firstLine}`)
      console.error(`  ${f.message}`)
    }
    console.error(`\nMermaid validation failed: ${failures.length} invalid block(s).`)
    process.exitCode = 1
    return
  }

  console.log('Mermaid validation passed.')
}

await main()
