// The dependency spec `iota-ext init` writes for @iota-uz/sdk: the one the SDK was installed with, so that the next
// `npm install` of the new package resolves the same build (a tarball, a linked checkout, a git ref or a registry version).
import { existsSync, readFileSync } from 'node:fs'
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path'

const SDK = '@iota-uz/sdk'
const DEP_FIELDS = ['dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies'] as const
const PATH_PROTOCOLS = ['file:', 'link:', 'portal:'] as const

/** The directory whose node_modules holds the running SDK, when the CLI runs from an installed copy. */
function installingProject(sdkDir: string): string | undefined {
  const marker = `${sep}node_modules${sep}@iota-uz${sep}sdk`
  const at = sdkDir.lastIndexOf(marker)
  return at > 0 ? sdkDir.slice(0, at) : undefined
}

function declaredSpec(projectDir: string): string | undefined {
  const file = join(projectDir, 'package.json')
  if (!existsSync(file)) return undefined
  try {
    const pkg = JSON.parse(readFileSync(file, 'utf8')) as Record<string, Record<string, string> | undefined>
    for (const field of DEP_FIELDS) {
      const spec = pkg[field]?.[SDK]
      if (typeof spec === 'string' && spec.trim() !== '') return spec.trim()
    }
  } catch {
    // an unreadable package.json is not the installing one
  }
  return undefined
}

function findDeclaration(sdkDir: string, cwd: string): { projectDir: string; spec: string } | undefined {
  const candidates: string[] = []
  const installed = installingProject(sdkDir)
  if (installed !== undefined) candidates.push(installed)
  for (let dir = resolve(cwd); ; dir = dirname(dir)) {
    candidates.push(dir)
    if (dirname(dir) === dir) break
  }
  for (const projectDir of candidates) {
    const spec = declaredSpec(projectDir)
    if (spec !== undefined) return { projectDir, spec }
  }
  return undefined
}

function pathSpec(spec: string): { protocol: string; path: string } | undefined {
  for (const protocol of PATH_PROTOCOLS) {
    if (spec.startsWith(protocol)) return { protocol, path: spec.slice(protocol.length) }
  }
  // npm also takes a bare path (`../sdk.tgz`, `./sdk`) as a file spec.
  if (/^\.{1,2}[\\/]/.test(spec)) return { protocol: 'file:', path: spec }
  return undefined
}

const posix = (p: string): string => p.split(sep).join('/')

/**
 * The spec for the package being created in `newDir`. A path spec (file:, link:, portal:, bare ./ ../) is rewritten to resolve
 * from `newDir`; git and URL specs are kept; a registry version, range, tag or workspace spec becomes the exact installed
 * version (the extension runtime ships from the CLI's own version, and earlier builds have no dist/runtime).
 */
export function resolveSdkSpec(o: { sdkDir: string; sdkVersion: string; cwd: string; newDir: string }): string {
  const exact = o.sdkVersion === '0.0.0' ? '*' : o.sdkVersion
  const found = findDeclaration(o.sdkDir, o.cwd)
  if (found === undefined) return exact
  const spec = found.spec
  const asPath = pathSpec(spec)
  if (asPath !== undefined) {
    if (asPath.path.startsWith('~') || isAbsolute(asPath.path)) return spec
    const target = resolve(found.projectDir, asPath.path)
    const rel = posix(relative(o.newDir, target)) || '.'
    return `${asPath.protocol}${rel.startsWith('.') ? rel : `./${rel}`}`
  }
  if (/^(git(\+[a-z]+)?|github|gitlab|bitbucket|https?):/.test(spec) || /^[\w.-]+\/[\w.-]+(#.*)?$/.test(spec)) return spec
  return exact
}
