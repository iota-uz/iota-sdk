// Manifest extraction: turns the exported values of the three entry modules into manifest JSON.
// No global registry: only what the entries export (and the brand marks) is collected.
import type { ClientValue } from './client.ts'
import type { Channel, GuardJSON, ImplKind, ManifestJSON, ResourceJSON, TypeJSON, CapabilitiesJSON } from './model.ts'
import { MANIFEST_SCHEMA } from './model.ts'
import { isBranded, isContract, isKitIssued, type Contract } from './resource.ts'
import type { AnyGuard, Implementation, ServerValue } from './server.ts'

export interface CollectInput {
  namespace: string
  /** `import * as contracts from "./contracts"` */
  contracts: Readonly<Record<string, unknown>>
  server: Readonly<Record<string, unknown>>
  client: Readonly<Record<string, unknown>>
  capabilities?: CapabilitiesJSON
  /** Foreign namespace → catalog snapshot hash this build resolved against. */
  dependencies?: Readonly<Record<string, string>>
}

export class ManifestError extends Error {
  readonly problems: readonly string[]

  constructor(problems: readonly string[]) {
    super(`invalid extension package:\n${problems.map((p) => `  - ${p}`).join('\n')}`)
    this.name = 'ManifestError'
    this.problems = problems
  }
}

const NAMESPACE_PATTERN = /^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$/
const ID_PATTERN = /^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$/
const CRON_PATTERN = /^(@[a-z]+|\S+(\s+\S+){4})$/
const CLIENT_KINDS = new Set(['page', 'navigation', 'slot', 'cell', 'shell'])
const CHANNELS: readonly Channel[] = ['sms', 'email', 'inApp', 'push']
const IMPLEMENTABLE: Readonly<Record<string, (c: Contract) => ImplKind | undefined>> = {
  operation: () => 'operation',
  command: (c) => ((c as { atomic?: boolean }).atomic === true ? 'atomic' : 'command'),
  query: () => 'query',
  workflow: () => 'workflow',
  contactResolver: () => 'contactResolver',
  deliveryPolicy: () => 'deliveryPolicy',
}

type Graph = 'contracts' | 'server' | 'client'

function typeRefs(type: TypeJSON | undefined, out: string[]): void {
  if (!type) return
  if (type.kind === 'ref' && type.entity) out.push(type.entity)
  type.properties?.forEach((p) => typeRefs(p.type, out))
  typeRefs(type.items, out)
}

function referencedIds(r: ResourceJSON): string[] {
  const out: string[] = []
  r.fields?.forEach((f) => typeRefs(f.type, out))
  for (const t of [r.input, r.output, r.params, r.payload, r.props]) typeRefs(t, out)
  Object.values(r.signals ?? {}).forEach((t) => typeRefs(t, out))
  out.push(...(r.errors ?? []))
  for (const id of [r.base, r.subject, r.permission, r.route, r.overlay]) if (id) out.push(id)
  if ((r.kind === 'slot' || r.kind === 'cell') && r.target) out.push(r.target)
  return out
}

/** Inside-package navigation references must resolve; ones pointing at core or other packages are checked at activation. */
function checkNavigation(
  resources: readonly ResourceJSON[],
  namespace: string,
  inPackage: (id: string) => boolean,
  problems: string[],
): void {
  const ids = new Set(resources.map((r) => r.id))
  const groups = new Set(resources.flatMap((r) => (r.kind === 'navigation' ? (r.groups ?? []).map((g) => g.id) : [])))
  const known = (id: string): boolean => ids.has(id) || groups.has(id)
  for (const r of resources) {
    const where = `resource "${r.id}"`
    if (r.nav?.group && inPackage(r.nav.group) && !groups.has(r.nav.group)) {
      problems.push(`${where}: nav.group "${r.nav.group}" is not declared by a navigation(...) groups entry of "${namespace}"`)
    }
    if (r.nav?.parent && inPackage(r.nav.parent) && !known(r.nav.parent)) {
      problems.push(`${where}: nav.parent "${r.nav.parent}" is not a page or group of "${namespace}"`)
    }
    for (const entry of r.palette ?? []) {
      if (inPackage(entry.target) && !ids.has(entry.target)) {
        problems.push(`${where}: palette entry "${entry.id}" targets "${entry.target}", which is not exported by "${namespace}"`)
      }
    }
    for (const patch of r.patches ?? []) {
      for (const id of [patch.ref, patch.parent]) {
        if (id && inPackage(id) && !known(id)) problems.push(`${where}: nav patch references "${id}", which is not declared by "${namespace}"`)
      }
    }
  }
}

const lowerFirst = (name: string): string => name.charAt(0).toLowerCase() + name.slice(1)

const MAX_SCAN_DEPTH = 6

/** Finds a branded value hidden inside a plain object or array, e.g. `export const all = { Policy }`. */
function nestedBranded(value: unknown, seen: Set<unknown> = new Set(), depth = 0): string | undefined {
  if (typeof value !== 'object' || value === null || depth > MAX_SCAN_DEPTH || seen.has(value)) return undefined
  seen.add(value)
  const proto: unknown = Object.getPrototypeOf(value)
  if (!Array.isArray(value) && proto !== Object.prototype && proto !== null) return undefined
  for (const [key, inner] of Object.entries(value)) {
    if (isBranded(inner)) return key
    const found = nestedBranded(inner, seen, depth + 1)
    if (found !== undefined) return `${key}.${found}`
  }
  return undefined
}

/** Collects the manifest of a package, or throws a `ManifestError` listing every problem found. */
export function collectManifest(input: CollectInput): ManifestJSON {
  const problems: string[] = []
  const { namespace } = input
  const inPackage = (id: string): boolean => id === namespace || id.startsWith(`${namespace}.`)
  if (!NAMESPACE_PATTERN.test(namespace)) problems.push(`invalid namespace "${namespace}"`)

  const owned = new Map<string, string>() // id → where it is declared
  const claim = (id: string, where: string): boolean => {
    if (!ID_PATTERN.test(id)) {
      problems.push(`${where}: invalid id "${id}"`)
      return false
    }
    if (!id.startsWith(`${namespace}.`)) {
      problems.push(`${where}: id "${id}" is outside namespace "${namespace}"`)
      return false
    }
    const prior = owned.get(id)
    if (prior !== undefined) {
      problems.push(`${where}: id "${id}" is already declared by ${prior}`)
      return false
    }
    owned.set(id, where)
    return true
  }

  const resources: ResourceJSON[] = []
  const seen = new Set<unknown>()
  const declared = (id: string, where: string): void => {
    if (!resources.some((r) => r.id === id)) problems.push(`${where}: references "${id}" which is not exported by the contracts graph`)
  }

  /** A `foreign` contract belongs to the platform or another package and must come from the developer kit. */
  const external = (c: Contract, where: string): void => {
    if (!isKitIssued(c)) problems.push(`${where}: "${c.id}" is marked foreign but was not issued by the developer kit`)
    else if (inPackage(c.id)) problems.push(`${where}: kit contract "${c.id}" is inside this package's own namespace "${namespace}"`)
  }
  /** Referencing a contract: foreign ones must be kit-issued; own ones must be exported by the contracts graph. */
  const reference = (c: Contract, where: string): void => {
    if (c.foreign) external(c, where)
    else if (!inPackage(c.id)) problems.push(`${where}: "${c.id}" is outside namespace "${namespace}"; contracts of other packages come from the developer kit`)
    else declared(c.id, where)
  }

  const collectResources = (graph: 'contracts' | 'client', ns: Readonly<Record<string, unknown>>): void => {
    for (const [name, value] of Object.entries(ns)) {
      const where = `${graph} export "${name}"`
      if (!isBranded(value)) {
        const inner = nestedBranded(value)
        if (inner !== undefined) problems.push(`${where}: contains "${inner}"; export resources as top-level named exports`)
        continue
      }
      if (!isContract(value)) {
        problems.push(`${where}: server value exported from the ${graph} graph`)
        continue
      }
      if (seen.has(value)) continue
      seen.add(value)
      if (value.foreign) {
        external(value, where)
        continue
      }
      // A slot declaration is a contract (other packages reference it through the kit); only contributions are client values.
      const isClient = CLIENT_KINDS.has(value.kind) && !(value.kind === 'slot' && value.resource.target === undefined)
      if (graph === 'contracts' && isClient) {
        problems.push(`${where}: ${value.kind} belongs to the client graph`)
        continue
      }
      if (graph === 'client' && !isClient) {
        problems.push(`${where}: ${value.kind} contract must be declared and exported from the contracts graph`)
        continue
      }
      const resource = { ...value.resource }
      if (isClient) {
        const implicit = (value as ClientValue).implicitId
        if (implicit === 'export') resource.id = `${namespace}.${lowerFirst(name)}`
        else if (implicit === 'navigation') resource.id = `${namespace}.navigation`
        else if (implicit === 'shell') resource.id = `${namespace}.shell`
        if (value.kind !== 'navigation') resource.export = name
      }
      if (claim(resource.id, where)) resources.push(resource)
    }
  }
  collectResources('contracts', input.contracts)
  collectResources('client', input.client)

  for (const r of resources) for (const id of referencedIds(r)) if (inPackage(id)) declared(id, `resource "${r.id}"`)
  checkNavigation(resources, namespace, inPackage, problems)

  const implementations: NonNullable<ManifestJSON['implementations']> = []
  const subscriptions: NonNullable<ManifestJSON['subscriptions']> = []
  const guards: GuardJSON[] = []
  const templates: NonNullable<ManifestJSON['templates']> = []
  const schedules: NonNullable<ManifestJSON['schedules']> = []
  const endpoints: NonNullable<ManifestJSON['endpoints']> = []
  const implemented = new Map<string, string>()
  const templated = new Map<string, string>()

  for (const [name, value] of Object.entries(input.server)) {
    const where = `server export "${name}"`
    if (!isBranded(value)) {
      const inner = nestedBranded(value)
      if (inner !== undefined) problems.push(`${where}: contains "${inner}"; export resources as top-level named exports`)
      continue
    }
    if (isContract(value)) {
      problems.push(`${where}: contract exported from the server graph; declare it in the contracts graph`)
      continue
    }
    if (seen.has(value)) continue
    seen.add(value)
    const sv = value as ServerValue
    switch (sv.serverKind) {
      case 'implementation': {
        const c = (sv as Implementation).contract
        const kind = IMPLEMENTABLE[c.kind]?.(c)
        if (kind === undefined) {
          problems.push(`${where}: cannot implement contract "${c.id}" of kind "${c.kind}"`)
          break
        }
        reference(c, where)
        const prior = implemented.get(c.id)
        if (prior !== undefined) {
          problems.push(`${where}: "${c.id}" is already implemented by export "${prior}"`)
          break
        }
        implemented.set(c.id, name)
        implementations.push({ export: name, implements: c.id, kind })
        break
      }
      case 'subscription': {
        reference(sv.event, where)
        if (claim(sv.id, where)) subscriptions.push({ id: sv.id, export: name, event: sv.event.id })
        break
      }
      case 'guard': {
        const g: AnyGuard = sv
        const target = g.target
        const ok =
          (g.phase === 'input' && target.kind === 'command' && !target.atomic) ||
          (g.phase === 'operation' && (target.kind === 'operation' || (target.kind === 'command' && target.atomic))) ||
          (g.phase === 'write' && target.kind === 'entity')
        if (!ok) {
          problems.push(`${where}: ${g.phase} guard cannot target ${target.kind} "${target.id}"`)
          break
        }
        reference(target, where)
        if (claim(g.id, where)) guards.push({ id: g.id, export: name, target: target.id, phase: g.phase })
        break
      }
      case 'templates': {
        const channels = CHANNELS.filter((ch) => typeof sv.renderers[ch] === 'function')
        const prior = templated.get(sv.notice.id)
        if (prior !== undefined) {
          problems.push(`${where}: notice "${sv.notice.id}" already has templates in export "${prior}"`)
          break
        }
        if (channels.length === 0) {
          problems.push(`${where}: templates for "${sv.notice.id}" define no channel`)
          break
        }
        reference(sv.notice, where)
        templated.set(sv.notice.id, name)
        templates.push({ export: name, notice: sv.notice.id, channels })
        break
      }
      case 'schedule': {
        if (!CRON_PATTERN.test(sv.cron.trim())) problems.push(`${where}: invalid cron "${sv.cron}"`)
        else if (claim(sv.id, where)) {
          schedules.push({ id: sv.id, export: name, cron: sv.cron.trim(), ...(sv.timezone ? { timezone: sv.timezone } : {}) })
        }
        break
      }
      case 'endpoint': {
        if (!sv.spec.path.startsWith('/')) problems.push(`${where}: endpoint path must start with "/"`)
        else if (claim(sv.id, where)) {
          endpoints.push({ id: sv.id, export: name, method: sv.spec.method, path: sv.spec.path, ...(sv.spec.public ? { public: true as const } : {}) })
        }
        break
      }
      default:
        problems.push(`${where}: unknown server value`)
    }
  }

  if (problems.length > 0) throw new ManifestError(problems)

  const byId = <T extends { id: string }>(a: T, b: T) => a.id.localeCompare(b.id)
  const byExport = <T extends { export: string }>(a: T, b: T) => a.export.localeCompare(b.export)
  const manifest: ManifestJSON = {
    schema: MANIFEST_SCHEMA,
    namespace,
    resources: resources.sort(byId),
  }
  if (implementations.length > 0) manifest.implementations = implementations.sort(byExport)
  if (subscriptions.length > 0) manifest.subscriptions = subscriptions.sort(byId)
  if (guards.length > 0) manifest.guards = guards.sort(byId)
  if (templates.length > 0) manifest.templates = templates.sort(byExport)
  if (schedules.length > 0) manifest.schedules = schedules.sort(byId)
  if (endpoints.length > 0) manifest.endpoints = endpoints.sort(byId)
  manifest.capabilities = capabilitiesOf(input.capabilities)
  if (input.dependencies && Object.keys(input.dependencies).length > 0) manifest.dependencies = { ...input.dependencies }
  return manifest
}

function capabilitiesOf(c: CapabilitiesJSON | undefined): CapabilitiesJSON {
  const out: CapabilitiesJSON = {}
  if (c?.network?.length) out.network = [...c.network]
  if (c?.secrets?.length) out.secrets = [...c.secrets]
  return out
}
