// The package stylesheet compiler (src/build/css.ts) against the repository's own design-token source: the Tailwind
// config the host's pipeline uses is the one these tests compile with.
import { cpSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import * as esbuild from 'esbuild'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'

import { candidatesOf, stylesheetPaths, tailwindStylesheet, TAILWIND_OUTPUT } from '../src/build/css.ts'
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
    expect(css).toContain('.p-4{padding:calc(var(--spacing) * 4)}')
    expect(css).toContain('--spacing: .25rem') // the theme variables the utilities use travel with them
    // Utilities only: no preflight, no base layer rules, and no class nobody uses.
    expect(css).not.toContain('box-sizing')
    expect(css).not.toContain('@layer base')
    expect(css).not.toContain('bg-fuchsia')
    expect(css).not.toContain('.flex{')
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
