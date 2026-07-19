import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { describe, expect, it } from 'vitest'

interface ManifestEntry {
  file: string
  imports?: string[]
  dynamicImports?: string[]
}

describe('chart bundle boundary', () => {
  it('keeps ECharts behind the adapter dynamic import', async () => {
    const manifestPath = path.resolve(import.meta.dirname, '../../../../pkg/lens/render/react/dist/.vite/manifest.json')
    const manifest = JSON.parse(await readFile(manifestPath, 'utf8')) as Record<string, ManifestEntry>
    const entry = manifest['index.html']
    const adapterKey = 'src/charts/echarts/adapter.ts'

    expect(entry).toBeDefined()
    expect(entry?.dynamicImports).toContain(adapterKey)
    expect(manifest[adapterKey]?.file).not.toBe(entry?.file)

    const staticGraph = new Set<string>()
    const visit = (key: string) => {
      if (staticGraph.has(key)) return
      staticGraph.add(key)
      for (const imported of manifest[key]?.imports ?? []) visit(imported)
    }
    visit('index.html')

    expect(staticGraph).not.toContain(adapterKey)
    expect([...staticGraph].some((key) => key.includes('echarts'))).toBe(false)
  })
})
