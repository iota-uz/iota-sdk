export type ShellRegion = 'header' | 'footer'

export interface ShellNavigationItem {
  key: string
  title: string
  href?: string
  icon?: string
  parent?: string
  workspace?: string
  order?: number
  permissions?: readonly string[]
}

export interface ShellNavigationPatch {
  title?: string
  hidden?: boolean
  parent?: string | null
  workspace?: string
  order?: number
}

export interface ShellNavigationNode extends Omit<ShellNavigationItem, 'parent'> {
  children: ShellNavigationNode[]
}

export interface ShellContribution {
  key: string
  region: ShellRegion
  order?: number
  mount(container: HTMLElement): void | (() => void)
}

export type ShellDiagnosticCode =
  | 'duplicate-navigation-key'
  | 'duplicate-contribution-key'
  | 'unknown-navigation-target'
  | 'unknown-navigation-parent'
  | 'navigation-cycle'
  | 'navigation-patch-conflict'

export interface ShellDiagnostic {
  code: ShellDiagnosticCode
  owner: string
  key: string
  message: string
}

interface Owned<T> { owner: string; seq: number; value: T }
interface OwnedPatch extends Owned<ShellNavigationPatch> { key: string }

const patchFields = ['title', 'hidden', 'parent', 'workspace', 'order'] as const

function byOrder<T extends { order?: number }>(a: Owned<T>, b: Owned<T>): number {
  return (a.value.order ?? 0) - (b.value.order ?? 0) || a.seq - b.seq
}

/**
 * ShellRegistry is the renderer-neutral navigation and region contribution
 * model of the authenticated shell. Contributions are addressed by owner so a
 * failing extension is removed in one call, and conflicts surface as
 * diagnostics instead of silently overriding another owner.
 */
export class ShellRegistry {
  static readonly BASE_OWNER = 'sdk'

  private seq = 0
  private readonly items: Owned<ShellNavigationItem>[] = []
  private readonly patches: OwnedPatch[] = []
  private readonly contributionList: Owned<ShellContribution>[] = []
  private readonly listeners = new Set<() => void>()

  constructor(base: readonly ShellNavigationItem[] = []) {
    for (const item of base) this.items.push({ owner: ShellRegistry.BASE_OWNER, seq: this.seq++, value: item })
  }

  addNavigation(owner: string, items: readonly ShellNavigationItem[]): () => void {
    const added = items.map((value) => ({ owner, seq: this.seq++, value }))
    this.items.push(...added)
    this.changed()
    return () => this.remove(this.items, added)
  }

  patchNavigation(owner: string, key: string, patch: ShellNavigationPatch): () => void {
    const added: OwnedPatch = { owner, key, seq: this.seq++, value: patch }
    this.patches.push(added)
    this.changed()
    return () => this.remove(this.patches, [added])
  }

  contribute(owner: string, contribution: ShellContribution): () => void {
    const added = { owner, seq: this.seq++, value: contribution }
    this.contributionList.push(added)
    this.changed()
    return () => this.remove(this.contributionList, [added])
  }

  disposeOwner(owner: string): void {
    const before = this.items.length + this.patches.length + this.contributionList.length
    for (const list of [this.items, this.patches, this.contributionList] as Owned<unknown>[][]) {
      for (let index = list.length - 1; index >= 0; index -= 1) if (list[index]!.owner === owner) list.splice(index, 1)
    }
    if (before !== this.items.length + this.patches.length + this.contributionList.length) this.changed()
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  navigation(): ShellNavigationNode[] {
    return this.resolve().tree
  }

  diagnostics(): ShellDiagnostic[] {
    return [...this.resolve().diagnostics, ...this.contributionDiagnostics()]
  }

  contributions(region: ShellRegion): ShellContribution[] {
    return this.activeContributions().filter(({ value }) => value.region === region).sort(byOrder).map(({ value }) => value)
  }

  /** Mounts the region's contributions and keeps them in sync until disposed. */
  mountRegion(region: ShellRegion, element: HTMLElement): () => void {
    const mounted = new Map<ShellContribution, { host: HTMLElement; cleanup: void | (() => void) }>()
    const sync = () => {
      const wanted = this.contributions(region)
      for (const [contribution, entry] of mounted) {
        if (wanted.includes(contribution)) continue
        this.cleanup(entry)
        mounted.delete(contribution)
      }
      for (const contribution of wanted) {
        if (mounted.has(contribution)) continue
        const host = element.ownerDocument.createElement('div')
        host.dataset.iotaShellContribution = contribution.key
        const entry = { host, cleanup: undefined as void | (() => void) }
        try {
          entry.cleanup = contribution.mount(host)
        } catch (error) {
          host.remove()
          element.ownerDocument.defaultView?.console.error(`Shell contribution ${contribution.key} failed to mount`, error)
          continue
        }
        mounted.set(contribution, entry)
      }
      for (const contribution of wanted) {
        const entry = mounted.get(contribution)
        if (entry) element.append(entry.host)
      }
    }
    sync()
    const unsubscribe = this.subscribe(sync)
    return () => {
      unsubscribe()
      for (const entry of mounted.values()) this.cleanup(entry)
      mounted.clear()
    }
  }

  private cleanup(entry: { host: HTMLElement; cleanup: void | (() => void) }): void {
    try { entry.cleanup?.() } finally { entry.host.remove() }
  }

  private activeContributions(): Owned<ShellContribution>[] {
    const seen = new Set<string>()
    return this.contributionList.filter(({ value }) => {
      if (seen.has(value.key)) return false
      seen.add(value.key)
      return true
    })
  }

  private contributionDiagnostics(): ShellDiagnostic[] {
    const first = new Map<string, Owned<ShellContribution>>()
    const out: ShellDiagnostic[] = []
    for (const entry of this.contributionList) {
      const existing = first.get(entry.value.key)
      if (!existing) { first.set(entry.value.key, entry); continue }
      out.push({
        code: 'duplicate-contribution-key',
        owner: entry.owner,
        key: entry.value.key,
        message: `Contribution ${entry.value.key} from ${entry.owner} is ignored; ${existing.owner} registered it first`,
      })
    }
    return out
  }

  private resolve(): { tree: ShellNavigationNode[]; diagnostics: ShellDiagnostic[] } {
    const diagnostics: ShellDiagnostic[] = []
    const nodes = new Map<string, Owned<ShellNavigationItem> & { parent: string | undefined; hidden: boolean; title: string; workspace: string | undefined; order: number | undefined }>()
    for (const entry of this.items) {
      const { key } = entry.value
      if (nodes.has(key)) {
        diagnostics.push({ code: 'duplicate-navigation-key', owner: entry.owner, key, message: `Navigation item ${key} from ${entry.owner} is ignored; it is already registered` })
        continue
      }
      nodes.set(key, { ...entry, parent: entry.value.parent, hidden: false, title: entry.value.title, workspace: entry.value.workspace, order: entry.value.order })
    }

    const claimed = new Map<string, string>()
    for (const patch of [...this.patches].sort((a, b) => a.seq - b.seq)) {
      const node = nodes.get(patch.key)
      if (!node) {
        diagnostics.push({ code: 'unknown-navigation-target', owner: patch.owner, key: patch.key, message: `Navigation patch from ${patch.owner} targets unknown item ${patch.key}` })
        continue
      }
      for (const field of patchFields) {
        if (!(field in patch.value) || patch.value[field] === undefined) continue
        const claim = `${patch.key}\0${field}`
        const holder = claimed.get(claim)
        if (holder !== undefined && holder !== patch.owner) {
          diagnostics.push({ code: 'navigation-patch-conflict', owner: patch.owner, key: patch.key, message: `Navigation patch ${field} on ${patch.key} from ${patch.owner} is ignored; ${holder} already changed it` })
          continue
        }
        claimed.set(claim, patch.owner)
        if (field === 'title') node.title = patch.value.title!
        else if (field === 'hidden') node.hidden = patch.value.hidden!
        else if (field === 'parent') node.parent = patch.value.parent ?? undefined
        else if (field === 'workspace') node.workspace = patch.value.workspace
        else node.order = patch.value.order
      }
    }

    for (const [key, node] of nodes) {
      if (node.parent === undefined) continue
      if (!nodes.has(node.parent)) {
        diagnostics.push({ code: 'unknown-navigation-parent', owner: node.owner, key, message: `Navigation item ${key} references unknown parent ${node.parent}; shown at the root` })
        node.parent = undefined
      }
    }
    for (const [key, node] of nodes) {
      const chain = new Set([key])
      for (let parent = node.parent; parent !== undefined; parent = nodes.get(parent)?.parent) {
        if (chain.has(parent)) {
          diagnostics.push({ code: 'navigation-cycle', owner: node.owner, key, message: `Navigation item ${key} would form a parent cycle; shown at the root` })
          node.parent = undefined
          break
        }
        chain.add(parent)
      }
    }

    const children = new Map<string | undefined, typeof nodes extends Map<string, infer V> ? V[] : never>()
    for (const node of nodes.values()) {
      const list = children.get(node.parent) ?? []
      list.push(node)
      children.set(node.parent, list)
    }
    const build = (parent: string | undefined): ShellNavigationNode[] => (children.get(parent) ?? [])
      .filter((node) => !node.hidden)
      .sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.seq - b.seq)
      .map((node) => {
        const { parent: _parent, ...rest } = node.value
        return {
          ...rest,
          title: node.title,
          ...(node.workspace === undefined ? {} : { workspace: node.workspace }),
          ...(node.order === undefined ? {} : { order: node.order }),
          children: build(node.value.key),
        }
      })
    return { tree: build(undefined), diagnostics }
  }

  private remove<T>(list: Owned<T>[], entries: Owned<T>[]): void {
    let changed = false
    for (const entry of entries) {
      const index = list.indexOf(entry)
      if (index < 0) continue
      list.splice(index, 1)
      changed = true
    }
    if (changed) this.changed()
  }

  private changed(): void {
    for (const listener of [...this.listeners]) listener()
  }
}
