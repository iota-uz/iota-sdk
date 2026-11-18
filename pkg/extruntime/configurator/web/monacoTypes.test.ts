import { describe, expect, it, vi } from 'vitest'
import { TypeLibraries, compilerOptions, configureTypeScript, modelUri, pathOf, typeUri, type TsDefaults } from './monacoTypes'
import { languageOf, themeName } from './monaco'

function fakeDefaults(): TsDefaults & { libs: Map<string, string>; disposed: string[] } {
  const libs = new Map<string, string>()
  const disposed: string[] = []
  return {
    libs, disposed,
    setCompilerOptions: vi.fn(), setDiagnosticsOptions: vi.fn(), setEagerModelSync: vi.fn(),
    addExtraLib(content, filePath) {
      libs.set(filePath!, content)
      return { dispose: () => { libs.delete(filePath!); disposed.push(filePath!) } }
    },
  }
}

const enums = {
  ScriptTarget: { ES2022: 9 }, ModuleKind: { ESNext: 99 }, ModuleResolutionKind: { NodeJs: 2 }, JsxEmit: { Preserve: 1 },
}

describe('uris', () => {
  it('puts draft files at the virtual root and typings below node_modules', () => {
    expect(modelUri('src/contracts.ts')).toBe('file:///src/contracts.ts')
    expect(modelUri('/src/a.ts')).toBe('file:///src/a.ts')
    expect(typeUri('@iota-kit/core/index.d.ts')).toBe('file:///node_modules/@iota-kit/core/index.d.ts')
    expect(pathOf('file:///src/contracts.ts')).toBe('src/contracts.ts')
    expect(pathOf('file:///node_modules/@iota-kit/core/index.d.ts')).toBeUndefined()
    expect(pathOf('inmemory://model/1')).toBeUndefined()
  })
})

describe('TypeScript configuration', () => {
  it('resolves the kit and the SDK subpaths through paths, with Solid JSX', () => {
    const o = compilerOptions(enums)
    expect(o).toMatchObject({ jsx: 1, jsxImportSource: 'solid-js', moduleResolution: 2, baseUrl: 'file:///' })
    const paths = o.paths as Record<string, string[]>
    expect(paths['@iota-kit/*']).toEqual(['node_modules/@iota-kit/*/index.d.ts'])
    expect(paths['@iota-uz/sdk/*']![0]).toBe('node_modules/@iota-uz/sdk/dist/*/index.d.ts')
  })

  it('resolves the SDK entries a package may import: the runtime API and the Solid UI kit', () => {
    // The paths mapping as TypeScript applies it: the first candidate that exists in the virtual node_modules wins.
    const o = compilerOptions(enums)
    const [pattern, candidates] = Object.entries(o.paths as Record<string, string[]>).find(([k]) => k === '@iota-uz/sdk/*')!
    const files = new Set(['node_modules/@iota-uz/sdk/dist/solid-ui/index.d.ts', 'node_modules/@iota-uz/sdk/dist/runtime/client.d.ts'])
    const resolve = (specifier: string) => {
      const star = specifier.slice(pattern.indexOf('*'))
      return candidates.map((c) => c.replace('*', star)).find((c) => files.has(c))
    }
    expect(resolve('@iota-uz/sdk/solid-ui')).toBe('node_modules/@iota-uz/sdk/dist/solid-ui/index.d.ts')
    expect(resolve('@iota-uz/sdk/runtime/client')).toBe('node_modules/@iota-uz/sdk/dist/runtime/client.d.ts')
  })

  it('configures both defaults', () => {
    const a = fakeDefaults()
    const b = fakeDefaults()
    configureTypeScript([a, b], enums)
    for (const d of [a, b]) {
      expect(d.setCompilerOptions).toHaveBeenCalledTimes(1)
      expect(d.setEagerModelSync).toHaveBeenCalledWith(true)
    }
  })
})

describe('TypeLibraries', () => {
  const kit = (hash: string, extra: Record<string, string> = {}) => ({ hash, files: { '@iota-kit/core/index.d.ts': 'export declare const a: 1', ...extra } })

  it('registers each file with every defaults object', () => {
    const ts = fakeDefaults()
    const js = fakeDefaults()
    const libs = new TypeLibraries([ts, js])
    expect(libs.set(kit('h1'))).toBe(true)
    expect([...ts.libs.keys()]).toEqual(['file:///node_modules/@iota-kit/core/index.d.ts'])
    expect(js.libs.size).toBe(1)
    expect(libs.hash).toBe('h1')
  })

  it('does nothing for the same hash and replaces the files for a new one', () => {
    const d = fakeDefaults()
    const libs = new TypeLibraries([d])
    libs.set(kit('h1'))
    expect(libs.set(kit('h1'))).toBe(false)
    expect(d.disposed).toEqual([])
    expect(libs.set(kit('h2', { '@iota-kit/acme/index.d.ts': 'x' }))).toBe(true)
    expect(d.disposed).toEqual(['file:///node_modules/@iota-kit/core/index.d.ts'])
    expect([...d.libs.keys()].sort()).toEqual(['file:///node_modules/@iota-kit/acme/index.d.ts', 'file:///node_modules/@iota-kit/core/index.d.ts'])
  })

  it('removes everything on dispose and refuses later loads', () => {
    const d = fakeDefaults()
    const libs = new TypeLibraries([d])
    libs.set(kit('h1'))
    libs.dispose()
    expect(d.libs.size).toBe(0)
    expect(libs.set(kit('h2'))).toBe(false)
    expect(d.libs.size).toBe(0)
  })
})

describe('languages and theme', () => {
  it('chooses the language of a draft file and Monaco theme of the host theme', () => {
    expect(languageOf('src/Page.tsx')).toBe('typescript')
    expect(languageOf('src/a.mjs')).toBe('javascript')
    expect(languageOf('package.json')).toBe('json')
    expect(languageOf('README.md')).toBe('markdown')
    expect(languageOf('logo.bin')).toBe('plaintext')
    expect(themeName('dark')).toBe('vs-dark')
    expect(themeName('light')).toBe('vs')
  })
})
