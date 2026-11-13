// The package stylesheet compiler (src/build/css.ts) against the repository's own design-token source: the Tailwind
// config the host's pipeline uses is the one these tests compile with.
import { cpSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import * as esbuild from 'esbuild'
import { compile } from 'tailwindcss'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'

import { candidatesOf, stylesheetPaths, tailwindStylesheet, withoutPropertiesLayer, TAILWIND_OUTPUT } from '../src/build/css.ts'
import type { Layout } from '../src/build/layout.ts'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '..', '..', '..')
const require = createRequire(import.meta.url)

let work = ''
let layout: Layout

beforeAll(() => {
  work = mkdtempSync(join(tmpdir(), 'iota-styles-'))
  // The worker's theme directory as build-worker.mjs assembles it: the canonical config and Tailwind's two layers.
  const theme = join(work, 'theme')
  mkdirSync(theme, { recursive: true })
  cpSync(join(repoRoot, 'tailwind.config.js'), join(theme, 'tailwind.config.js'))
  const tailwind = dirname(require.resolve('tailwindcss/package.json'))
  for (const f of ['theme.css', 'utilities.css']) cpSync(join(tailwind, f), join(theme, f))
  layout = { theme } as Layout
})
afterAll(() => rmSync(work, { recursive: true, force: true }))

function source(name: string, text: string): string {
  const path = join(work, name)
  writeFileSync(path, text)
  return path
}

describe('candidatesOf', () => {
  it('finds class names in strings, attributes, template literals and object keys', () => {
    const found = candidatesOf(`
      const a = "p-4 flex";
      const b = 'hover:bg-gray-100 md:w-1/2';
      const c = \`gap-2 \${x} text-sm\`;
      <div class="rounded-lg border" classList={{ "font-bold": on() }} />
    `)
    for (const c of ['p-4', 'flex', 'hover:bg-gray-100', 'md:w-1/2', 'gap-2', 'text-sm', 'rounded-lg', 'border', 'font-bold']) expect(found, c).toContain(c)
  })

  it('keeps arbitrary values whole, quotes included', () => {
    const found = candidatesOf(`<i class="before:content-['*'] w-[calc(100%-2rem)] [&>*]:p-2 data-[state=open]:flex" />`)
    for (const c of ["before:content-['*']", 'w-[calc(100%-2rem)]', '[&>*]:p-2', 'data-[state=open]:flex']) expect(found, c).toContain(c)
  })

  it('drops tokens that cannot be classes', () => {
    expect(candidatesOf('x'.repeat(500))).toEqual(new Set())
  })
})

describe('tailwindStylesheet', () => {
  it('compiles the used classes with the SDK design tokens and nothing else', async () => {
    const file = source('a.tsx', `export const A = () => <div class="p-4 bg-primary-600 text-gray-700 border-subtle bg-surface-300" />`)
    const sheet = await tailwindStylesheet(esbuild, layout, [file])
    expect(sheet.diagnostics).toEqual([])
    const css = sheet.css!
    // The mapping of tailwind.config.js: colors are the host's token variables, not Tailwind's default palette.
    expect(css).toContain('oklch(var(--primary-600)')
    expect(css).toContain('oklch(var(--gray-700)')
    expect(css).toContain('oklch(var(--clr-border-subtle)')
    expect(css).toContain('oklch(var(--clr-surface-300))')
    expect(css).toContain('.p-4{padding:1rem}') // theme values are inlined: the sheet needs no theme variables of its own
    // Utilities only: no preflight, no base layer rules, and no class nobody uses.
    expect(css).not.toContain('box-sizing')
    expect(css).not.toContain('@layer base')
    expect(css).not.toContain('bg-fuchsia')
    expect(css).not.toContain('.flex{')
  })

  // Classes that, compiled the usual way, bring theme variables (:root), animation keyframes and @property registrations.
  const BROAD = 'p-4 gap-2 text-sm font-bold rounded-lg shadow-md ring-2 blur-sm transition translate-x-2 space-x-2 divide-x animate-spin animate-pulse font-sans font-mono bg-primary-600 text-gray-700 border-subtle ease-in-out duration-300 leading-tight tracking-wide'

  it('is utilities only: no theme variables, no layer-order statement, no properties fallback', async () => {
    const file = source('utilities-only.tsx', `export const U = "${BROAD}"`)
    const css = (await tailwindStylesheet(esbuild, layout, [file])).css!
    expect(css).toMatch(/^@layer utilities\{/)
    expect(css).not.toMatch(/:root|:host/)
    expect(css).not.toMatch(/@layer\s+(?:theme|base|components|properties)/)
    expect(css).not.toMatch(/@layer\s+[a-z, ]+;/) // no layer-order statement of any kind
    expect(css).not.toMatch(/@supports/)
    expect(css).not.toMatch(/var\(--(?:spacing|font-|blur-|default-|radius-|shadow-|text-|ease-|leading-|tracking-|animate-)/) // no reference to a theme variable the host may not define
    expect(css).toContain('@keyframes spin') // what an animation utility cannot work without stays
    expect(css).toContain('@property --tw-translate-y')
  })

  it('keeps only @property registrations that are identical to the ones the host stylesheet carries', async () => {
    const file = source('props.tsx', `export const U = "${BROAD}"`)
    const css = (await tailwindStylesheet(esbuild, layout, [file])).css!
    const registrations = css.match(/@property --tw-[\w-]+\{[^}]*\}/g) ?? []
    expect(registrations.length).toBeGreaterThan(10)
    // The host's own pipeline: `@import "tailwindcss"` (styles/tailwind/input.css) over the same classes.
    const tailwind = dirname(require.resolve('tailwindcss/package.json'))
    const compiler = await compile('@config "tailwind.config.js";\n@import "tailwindcss";', {
      base: tailwind,
      async loadStylesheet(id, base) {
        const name = id === 'tailwindcss' ? 'index.css' : id.replace('tailwindcss/', '').replace(/^\.\//, '')
        return { path: id, base, content: readFileSync(join(tailwind, name), 'utf8') }
      },
      async loadModule(id, base) {
        const mod: { exports: object } = { exports: {} }
        new Function('module', 'exports', readFileSync(join(layout.theme, id), 'utf8'))(mod, mod.exports)
        return { path: id, base, module: mod.exports as never }
      },
    })
    const host = (await esbuild.transform(compiler.build(BROAD.split(' ')), { loader: 'css', minify: true, legalComments: 'none' })).code
    for (const registration of registrations) expect(host, registration).toContain(registration)
  })

  it('strips only the properties layer, whatever surrounds it', () => {
    const css = '@layer properties;\n@layer utilities { .a { color: red } }\n@layer properties { @supports (x: y) { *, ::before { --tw-a: 0 } } }\n@keyframes k { to { opacity: 1 } }'
    expect(withoutPropertiesLayer(css)).toBe('\n@layer utilities { .a { color: red } }\n\n@keyframes k { to { opacity: 1 } }')
  })

  it('compiles variants and arbitrary values', async () => {
    const file = source('b.tsx', `export const B = "md:flex dark:bg-black hover:text-primary-700 w-[calc(100%-2rem)] -mt-2"`)
    const css = (await tailwindStylesheet(esbuild, layout, [file])).css!
    expect(css).toContain('width>=48rem')
    expect(css).toContain(':hover')
    expect(css).toContain('.dark')
    expect(css).toContain('calc(100% - 2rem)')
  })

  it('emits nothing when no class is a utility, and ignores files that are not scripts', async () => {
    const plain = source('c.ts', `export const text = "just some words, no utility here"`)
    const styles = source('d.css', `.p-4 { color: red }`)
    const sheet = await tailwindStylesheet(esbuild, layout, [plain, styles])
    expect(sheet.css).toBeUndefined()
    expect(sheet.diagnostics).toEqual([])
  })

  it('is deterministic', async () => {
    const file = source('e.tsx', `export const E = "p-4 flex gap-2 md:flex-col"`)
    expect((await tailwindStylesheet(esbuild, layout, [file])).css).toBe((await tailwindStylesheet(esbuild, layout, [file])).css)
  })

  it('reports an unreadable source as a diagnostic instead of throwing', async () => {
    const sheet = await tailwindStylesheet(esbuild, layout, [join(work, 'missing.tsx')])
    expect(sheet.css).toBeUndefined()
    expect(sheet.diagnostics[0]).toMatchObject({ code: 'css.tailwind', severity: 'error' })
  })
})

describe('stylesheetPaths', () => {
  it('lists the utilities first, then the scoped modules, only from the client subtree', () => {
    expect(stylesheetPaths(['server.mjs', 'client/index-ZZ.css', 'client/chunk-AA.css', TAILWIND_OUTPUT, 'client/index-1.js', 'other/x.css'])).toEqual([
      TAILWIND_OUTPUT, 'client/chunk-AA.css', 'client/index-ZZ.css',
    ])
  })
})
