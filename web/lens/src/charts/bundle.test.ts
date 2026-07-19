import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { describe, expect, it } from 'vitest'

interface ManifestEntry {
  file: string
  imports?: string[]
  dynamicImports?: string[]
}

describe('chart bundle boundary', () => {
  it('keeps ECharts out of the core entry bundle', async () => {
    const distPath = path.resolve(import.meta.dirname, '../../../../pkg/lens/render/react/dist')
    const manifestPath = path.join(distPath, '.vite/manifest.json')
    const manifest = JSON.parse(await readFile(manifestPath, 'utf8')) as Record<string, ManifestEntry>
    const entry = manifest['index.html']

    expect(entry).toBeDefined()

    const staticGraph = new Set<string>()
    const visit = (key: string) => {
      if (staticGraph.has(key)) return
      staticGraph.add(key)
      for (const imported of manifest[key]?.imports ?? []) visit(imported)
    }
    visit('index.html')

    expect([...staticGraph].some((key) => key.includes('echarts'))).toBe(false)

    const dynamicGraph = new Set<string>()
    const visitDynamic = (key: string) => {
      if (dynamicGraph.has(key) || staticGraph.has(key)) return
      dynamicGraph.add(key)
      for (const imported of manifest[key]?.imports ?? []) visitDynamic(imported)
      for (const imported of manifest[key]?.dynamicImports ?? []) visitDynamic(imported)
    }
    for (const key of staticGraph) {
      for (const imported of manifest[key]?.dynamicImports ?? []) visitDynamic(imported)
    }
    const echartsEntries = Object.keys(manifest).filter((key) => key.includes('echarts'))

    expect(echartsEntries.every((key) => dynamicGraph.has(key))).toBe(true)

    const staticChunks = await Promise.all(
      [...staticGraph].map((key) => readFile(path.join(distPath, manifest[key]!.file), 'utf8')),
    )
    const echartsRuntimeMarker = 'zrender'

    expect(staticChunks.join('\n')).not.toContain(echartsRuntimeMarker)
    expect(staticChunks.reduce((size, chunk) => size + Buffer.byteLength(chunk), 0)).toBeLessThan(350_000)
  })
})
