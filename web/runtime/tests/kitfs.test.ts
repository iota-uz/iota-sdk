// The kit installer writes what a server (or the build worker) sends, so it must refuse anything that could land
// outside `.iota/kit` and must not destroy the installed kit before it knows the new one is safe.
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { InvalidKitError, installKit, kitNamespace } from '../src/cli/kitfs.ts'

let sandbox = ''
let pkg = ''
const source = { kind: 'server', url: 'https://example.test' } as const
const good = { '@iota-kit/crm': { 'index.js': 'export const A = 1\n', 'index.d.ts': 'export const A: 1\n', 'nested/dir/x.json': '{}' } }

beforeEach(() => {
  sandbox = mkdtempSync(join(tmpdir(), 'iota-kitfs-'))
  pkg = join(sandbox, 'pkg')
  mkdirSync(pkg)
})
afterEach(() => rmSync(sandbox, { recursive: true, force: true }))

/** Everything under the sandbox except the package's kit: nothing may have been written there. */
function outside(): string[] {
  return readdirSync(sandbox, { recursive: true }).map(String).filter((p) => !p.startsWith('pkg')).sort()
}

describe('installKit', () => {
  it('installs modules, records every file hash and leaves no staging directory', () => {
    const record = installKit(pkg, 'h1', good, source)
    expect(readFileSync(join(pkg, '.iota/kit/crm/nested/dir/x.json'), 'utf8')).toBe('{}')
    expect(record.modules).toEqual(['@iota-kit/crm'])
    expect(Object.keys(record.files?.['@iota-kit/crm'] ?? {}).sort()).toEqual(['index.d.ts', 'index.js', 'nested/dir/x.json'])
    expect(readdirSync(join(pkg, '.iota'))).toEqual(['kit'])
  })

  it.each([
    ['a traversing module name', { '@iota-kit/../../evil': { 'index.js': 'x' } }],
    ['a module name with a slash', { '@iota-kit/a/b': { 'index.js': 'x' } }],
    ['a module name with an uppercase namespace', { '@iota-kit/Crm': { 'index.js': 'x' } }],
    ['a foreign scope', { '@evil/crm': { 'index.js': 'x' } }],
    ['an empty namespace', { '@iota-kit/': { 'index.js': 'x' } }],
    ['a parent segment', { '@iota-kit/crm': { '../../escape.js': 'x' } }],
    ['a parent segment in the middle', { '@iota-kit/crm': { 'a/../../escape.js': 'x' } }],
    ['an absolute path', { '@iota-kit/crm': { '/tmp/escape.js': 'x' } }],
    ['a drive path', { '@iota-kit/crm': { 'C:/escape.js': 'x' } }],
    ['a backslash', { '@iota-kit/crm': { 'a\\..\\escape.js': 'x' } }],
    ['a dot segment', { '@iota-kit/crm': { './index.js': 'x' } }],
    ['an empty segment', { '@iota-kit/crm': { 'a//b.js': 'x' } }],
    ['an empty path', { '@iota-kit/crm': { '': 'x' } }],
  ])('refuses %s before touching the installed kit', (_, modules) => {
    installKit(pkg, 'h1', good, source)
    expect(() => installKit(pkg, 'h2', modules, source)).toThrow(InvalidKitError)
    expect(readFileSync(join(pkg, '.iota/kit/crm/index.js'), 'utf8')).toBe('export const A = 1\n')
    expect(JSON.parse(readFileSync(join(pkg, '.iota/kit/kit.json'), 'utf8')).hash).toBe('h1')
    expect(outside()).toEqual([])
    expect(existsSync(join(sandbox, 'escape.js'))).toBe(false)
    expect(readdirSync(join(pkg, '.iota'))).toEqual(['kit'])
  })

  it('does not delete the installed kit when the new one cannot be written', () => {
    installKit(pkg, 'h1', good, source)
    // A regular file where the module directory must be: the write fails midway.
    const blocked = { '@iota-kit/crm': { 'index.js': 'x', 'index.js/child': 'y' } }
    expect(() => installKit(pkg, 'h2', blocked, source)).toThrow()
    expect(readFileSync(join(pkg, '.iota/kit/crm/index.js'), 'utf8')).toBe('export const A = 1\n')
    expect(readdirSync(join(pkg, '.iota'))).toEqual(['kit'])
  })

  it('replaces an old kit and a kit left in node_modules', () => {
    mkdirSync(join(pkg, 'node_modules/@iota-kit/old'), { recursive: true })
    writeFileSync(join(pkg, 'node_modules/@iota-kit/old/index.js'), 'x')
    installKit(pkg, 'h1', good, source)
    installKit(pkg, 'h2', { '@iota-kit/core': { 'index.js': 'y' } }, source)
    expect(existsSync(join(pkg, 'node_modules/@iota-kit'))).toBe(false)
    expect(existsSync(join(pkg, '.iota/kit/crm'))).toBe(false)
    expect(readFileSync(join(pkg, '.iota/kit/core/index.js'), 'utf8')).toBe('y')
  })

  it('does not follow a symlinked kit directory out of the package', () => {
    const elsewhere = join(sandbox, 'elsewhere')
    mkdirSync(elsewhere)
    writeFileSync(join(elsewhere, 'keep.txt'), 'keep')
    mkdirSync(join(pkg, '.iota'))
    symlinkSync(elsewhere, join(pkg, '.iota/kit'), 'dir')
    installKit(pkg, 'h1', good, source)
    expect(readFileSync(join(elsewhere, 'keep.txt'), 'utf8')).toBe('keep')
    expect(readdirSync(elsewhere)).toEqual(['keep.txt'])
    expect(readFileSync(join(pkg, '.iota/kit/crm/index.js'), 'utf8')).toBe('export const A = 1\n')
  })
})

describe('kitNamespace', () => {
  it('accepts manifest namespaces only', () => {
    expect(kitNamespace('@iota-kit/crm')).toBe('crm')
    expect(kitNamespace('@iota-kit/acme.loyalty')).toBe('acme.loyalty')
    for (const bad of ['@iota-kit/..', '@iota-kit/a..b', '@iota-kit/.a', '@iota-kit/a.', '@iota-kit/a b', 'crm', '@iota-kit/a/../b']) {
      expect(() => kitNamespace(bad), bad).toThrow(InvalidKitError)
    }
  })
})
