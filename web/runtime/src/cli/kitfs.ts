// The developer kit on disk: `node_modules/@iota-kit/<namespace>/{index.js,index.d.ts,package.json}` plus
// `node_modules/.iota-kit.json` recording which snapshot those files came from.
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

import type { KitModules, KitPayload } from '../build/protocol.ts'

const SCOPE = '@iota-kit'

export interface KitRecord {
  /** Snapshot hash: what `build` pins in the manifest's `dependencies`. */
  hash: string
  source: { kind: 'catalog'; files: string[] } | { kind: 'server'; url: string }
  modules: string[]
}

const scopeDir = (dir: string): string => join(dir, 'node_modules', SCOPE)
const recordFile = (dir: string): string => join(dir, 'node_modules', '.iota-kit.json')

function readTree(root: string, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const entry of readdirSync(join(root, prefix), { withFileTypes: true })) {
    const rel = prefix === '' ? entry.name : `${prefix}/${entry.name}`
    if (entry.isDirectory()) Object.assign(out, readTree(root, rel))
    else if (entry.isFile()) out[rel] = readFileSync(join(root, rel), 'utf8')
  }
  return out
}

/** The kit the package builds against; an empty kit when none is installed. */
export function loadKit(dir: string): KitPayload & { installed: boolean } {
  const scope = scopeDir(dir)
  const modules: Record<string, Record<string, string>> = {}
  if (existsSync(scope)) {
    for (const name of readdirSync(scope).sort()) {
      const module = join(scope, name)
      if (statSync(module).isDirectory() && existsSync(join(module, 'index.js'))) modules[`${SCOPE}/${name}`] = readTree(module)
    }
  }
  let hash: string | undefined
  try {
    hash = (JSON.parse(readFileSync(recordFile(dir), 'utf8')) as Partial<KitRecord>).hash
  } catch {
    hash = undefined
  }
  const names = Object.keys(modules)
  hash ??= createHash('sha256').update(JSON.stringify(names.map((n) => [n, modules[n]]))).digest('hex')
  return { hash, modules, installed: names.length > 0 }
}

export function readRecord(dir: string): KitRecord | undefined {
  try {
    return JSON.parse(readFileSync(recordFile(dir), 'utf8')) as KitRecord
  } catch {
    return undefined
  }
}

/** Replaces the installed kit with `modules` (each: relative path -> text or bytes) and records its provenance. */
export function installKit(dir: string, hash: string, modules: Readonly<Record<string, Readonly<Record<string, string | Uint8Array>>>>, source: KitRecord['source']): KitRecord {
  const scope = scopeDir(dir)
  rmSync(scope, { recursive: true, force: true })
  for (const [name, files] of Object.entries(modules)) {
    if (!name.startsWith(`${SCOPE}/`)) throw new Error(`unexpected kit module "${name}"`)
    for (const [path, content] of Object.entries(files)) {
      const target = join(dir, 'node_modules', name, path)
      mkdirSync(dirname(target), { recursive: true })
      writeFileSync(target, content)
    }
  }
  const record: KitRecord = { hash, source, modules: Object.keys(modules).sort() }
  mkdirSync(join(dir, 'node_modules'), { recursive: true })
  writeFileSync(recordFile(dir), JSON.stringify(record, null, 2) + '\n')
  return record
}

export type { KitModules }
