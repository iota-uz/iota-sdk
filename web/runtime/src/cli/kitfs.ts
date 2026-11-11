// The developer kit on disk: `.iota/kit/<namespace>/{index.js,index.d.ts,package.json}` plus `.iota/kit/kit.json`
// recording which snapshot those files came from and the hash of every file.
//
// The kit lives outside `node_modules` on purpose: a package manager owns that directory and deletes what it did
// not install (`npm install` removes a generated `node_modules/@iota-kit`). The build and the CLI read the kit from
// here and resolve `@iota-kit/*` themselves; the editor finds it through the `paths` entry of tsconfig.json
// (`@iota-kit/*` -> `./.iota/kit/*`, see `ensureEditorPaths`). Packages that still have the old
// `node_modules/@iota-kit` layout keep working until the kit is regenerated, which moves it.
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

import type { KitModules, KitPayload } from '../build/protocol.ts'

const SCOPE = '@iota-kit'

/** Where the kit is installed, relative to the package directory. */
export const KIT_DIR = '.iota/kit'
/** The tsconfig `paths` entry that points the editor at the kit. */
export const KIT_PATHS_KEY = `${SCOPE}/*`
export const KIT_PATHS_VALUE = `./${KIT_DIR}/*`

export interface KitRecord {
  /** Snapshot hash: what `build` pins in the manifest's `dependencies`. */
  hash: string
  source: { kind: 'catalog'; files: string[] } | { kind: 'server'; url: string }
  modules: string[]
  /** SHA-256 of every installed file: module name -> relative path -> hex digest. Lets `kit status` see edits and gaps. */
  files?: Record<string, Record<string, string>>
}

export type KitLocation = 'kit' | 'legacy' | 'none'

export interface LoadedKit extends KitPayload {
  installed: boolean
  /** `legacy`: the kit sits in `node_modules/@iota-kit`, where a package manager may delete it. */
  location: KitLocation
}

export type ModuleState = 'ok' | 'missing' | 'modified' | 'unrecorded'

export interface ModuleReport {
  name: string
  state: ModuleState
  /** What is wrong, one line each (empty when ok). */
  problems: string[]
}

export interface KitReport {
  location: KitLocation
  record: KitRecord | undefined
  modules: ModuleReport[]
  /** True when a kit is installed and every module matches its record. */
  ok: boolean
}

const sha256 = (data: string | Uint8Array): string => createHash('sha256').update(data).digest('hex')

const kitRoot = (dir: string): string => join(dir, KIT_DIR)
const recordFile = (dir: string): string => join(kitRoot(dir), 'kit.json')
const legacyScope = (dir: string): string => join(dir, 'node_modules', SCOPE)
const legacyRecord = (dir: string): string => join(dir, 'node_modules', '.iota-kit.json')

/** Module directory of `<SCOPE>/<ns>` inside the layout at `location`. */
function moduleDir(dir: string, location: KitLocation, name: string): string {
  return join(location === 'legacy' ? legacyScope(dir) : kitRoot(dir), name.slice(SCOPE.length + 1))
}

function readTree(root: string, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const entry of readdirSync(join(root, prefix), { withFileTypes: true })) {
    const rel = prefix === '' ? entry.name : `${prefix}/${entry.name}`
    if (entry.isDirectory()) Object.assign(out, readTree(root, rel))
    else if (entry.isFile()) out[rel] = readFileSync(join(root, rel), 'utf8')
  }
  return out
}

/** Module directories (holding an index.js) below `root`, as `@iota-kit/<name>`. */
function moduleNames(root: string): string[] {
  if (!existsSync(root)) return []
  return readdirSync(root)
    .sort()
    .filter((name) => {
      const module = join(root, name)
      return statSync(module).isDirectory() && existsSync(join(module, 'index.js'))
    })
    .map((name) => `${SCOPE}/${name}`)
}

function locate(dir: string): KitLocation {
  if (moduleNames(kitRoot(dir)).length > 0 || existsSync(recordFile(dir))) return 'kit'
  if (moduleNames(legacyScope(dir)).length > 0) return 'legacy'
  return 'none'
}

function parseRecord(file: string): KitRecord | undefined {
  try {
    return JSON.parse(readFileSync(file, 'utf8')) as KitRecord
  } catch {
    return undefined
  }
}

export function readRecord(dir: string): KitRecord | undefined {
  const location = locate(dir)
  if (location === 'none') return undefined
  return parseRecord(location === 'legacy' ? legacyRecord(dir) : recordFile(dir))
}

/** The kit the package builds against; an empty kit when none is installed. */
export function loadKit(dir: string): LoadedKit {
  const location = locate(dir)
  const modules: Record<string, Record<string, string>> = {}
  if (location !== 'none') {
    for (const name of moduleNames(location === 'legacy' ? legacyScope(dir) : kitRoot(dir))) modules[name] = readTree(moduleDir(dir, location, name))
  }
  const names = Object.keys(modules)
  const report = inspectKit(dir)
  // The recorded hash identifies the server's snapshot; edited or foreign files no longer match it.
  const intact = !report.modules.some((m) => m.state === 'modified' || m.state === 'unrecorded')
  const hash =
    (intact ? report.record?.hash : undefined) ?? sha256(JSON.stringify(names.map((n) => [n, modules[n]])))
  return { hash, modules, installed: names.length > 0, location }
}

/** Compares the installed files with the record: missing modules or files, edited files, modules nobody recorded. */
export function inspectKit(dir: string): KitReport {
  const location = locate(dir)
  if (location === 'none') return { location, record: undefined, modules: [], ok: false }
  const record = readRecord(dir)
  const onDisk = moduleNames(location === 'legacy' ? legacyScope(dir) : kitRoot(dir))
  const names = [...new Set([...(record?.modules ?? []), ...onDisk])].sort()
  const modules: ModuleReport[] = names.map((name) => {
    const root = moduleDir(dir, location, name)
    const problems: string[] = []
    if (record === undefined || !record.modules.includes(name)) {
      return { name, state: 'unrecorded', problems: [`not part of the recorded snapshot${record === undefined ? ' (kit.json is missing or unreadable)' : ''}`] }
    }
    if (!existsSync(join(root, 'index.js'))) {
      return { name, state: 'missing', problems: [`${join(KIT_DIR, name.slice(SCOPE.length + 1), 'index.js')} is missing`] }
    }
    let state: ModuleState = 'ok'
    const expected = record.files?.[name]
    if (expected === undefined) {
      if (!existsSync(join(root, 'index.d.ts'))) {
        problems.push('index.d.ts is missing')
        state = 'missing'
      }
    } else {
      for (const [path, digest] of Object.entries(expected).sort(([a], [b]) => (a < b ? -1 : 1))) {
        const file = join(root, path)
        if (!existsSync(file)) {
          problems.push(`${path} is missing`)
          state = 'missing'
        } else if (sha256(readFileSync(file)) !== digest) {
          problems.push(`${path} differs from the recorded snapshot`)
          if (state === 'ok') state = 'modified'
        }
      }
    }
    return { name, state, problems }
  })
  return { location, record, modules, ok: modules.length > 0 && modules.every((m) => m.state === 'ok') }
}

/**
 * Replaces the installed kit with `modules` (each: relative path -> text or bytes) and records its provenance.
 * A kit left in the pre-`.iota/kit` location (`node_modules/@iota-kit`) is removed: it is superseded.
 */
export function installKit(dir: string, hash: string, modules: Readonly<Record<string, Readonly<Record<string, string | Uint8Array>>>>, source: KitRecord['source']): KitRecord {
  rmSync(kitRoot(dir), { recursive: true, force: true })
  rmSync(legacyScope(dir), { recursive: true, force: true })
  rmSync(legacyRecord(dir), { force: true })
  const files: Record<string, Record<string, string>> = {}
  for (const [name, content] of Object.entries(modules)) {
    if (!name.startsWith(`${SCOPE}/`)) throw new Error(`unexpected kit module "${name}"`)
    files[name] = {}
    for (const [path, data] of Object.entries(content)) {
      const target = join(moduleDir(dir, 'kit', name), path)
      mkdirSync(dirname(target), { recursive: true })
      writeFileSync(target, data)
      files[name][path] = sha256(data)
    }
  }
  const record: KitRecord = { hash, source, modules: Object.keys(modules).sort(), files }
  mkdirSync(kitRoot(dir), { recursive: true })
  writeFileSync(recordFile(dir), JSON.stringify(record, null, 2) + '\n')
  return record
}

export type EditorPaths = 'present' | 'added' | 'absent' | 'manual'

function mapped(text: string): boolean {
  return text.includes(KIT_PATHS_KEY) && text.includes(KIT_PATHS_VALUE)
}

/** Whether the package's tsconfig.json lets the editor resolve `@iota-kit/*` (without changing it). */
export function editorPaths(dir: string): Exclude<EditorPaths, 'added'> {
  const file = join(dir, 'tsconfig.json')
  if (!existsSync(file)) return 'absent'
  return mapped(readFileSync(file, 'utf8')) ? 'present' : 'manual'
}

/**
 * Adds the `@iota-kit/*` paths entry to an existing plain-JSON tsconfig.json that lacks it (the migration of
 * packages created before the kit moved out of node_modules). A tsconfig with comments is left alone: `manual`.
 */
export function ensureEditorPaths(dir: string): EditorPaths {
  const state = editorPaths(dir)
  if (state !== 'manual') return state
  const file = join(dir, 'tsconfig.json')
  try {
    const doc = JSON.parse(readFileSync(file, 'utf8')) as { compilerOptions?: { paths?: Record<string, string[]> } }
    if (typeof doc !== 'object' || doc === null || Array.isArray(doc)) return 'manual'
    doc.compilerOptions = { ...(doc.compilerOptions ?? {}), paths: { ...(doc.compilerOptions?.paths ?? {}), [KIT_PATHS_KEY]: [KIT_PATHS_VALUE] } }
    writeFileSync(file, JSON.stringify(doc, null, 2) + '\n')
    return 'added'
  } catch {
    return 'manual'
  }
}

export type { KitModules }
