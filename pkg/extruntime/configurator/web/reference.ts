import type { ReferenceItem, ReferenceModel, ReferenceNamespace } from './types'

/** The Help Center document of a namespace's reference (the Go source mounts it at "extensions"). */
export function helpDocUrl(namespace: string): string {
  return `/help/doc/extensions/reference/${encodeURIComponent(namespace)}`
}

/**
 * pickNamespace chooses the namespace the tab shows: the one the user picked if
 * it is still in the model, else the namespace of the selected package, else
 * the first one.
 */
export function pickNamespace(model: ReferenceModel | undefined, chosen: string | undefined, current: string | undefined): ReferenceNamespace | undefined {
  const all = model?.namespaces ?? []
  return all.find((n) => n.name === chosen) ?? all.find((n) => n.name === current) ?? all[0]
}

/** Resource kinds in the order the reference lists them. */
export const KIND_ORDER = [
  'entity', 'overlay', 'command', 'operation', 'query', 'event', 'workflow', 'notice', 'issue', 'domainError',
  'permission', 'route', 'page', 'navigation', 'slot', 'cell', 'column', 'shell', 'contactResolver', 'deliveryPolicy',
] as const

export interface KindGroup { kind: string; items: ReferenceItem[] }

/** groupByKind splits a namespace's items by kind, in KIND_ORDER (unknown kinds last). */
export function groupByKind(items: readonly ReferenceItem[]): KindGroup[] {
  const rank = (kind: string): number => {
    const i = (KIND_ORDER as readonly string[]).indexOf(kind)
    return i < 0 ? KIND_ORDER.length : i
  }
  const byKind = new Map<string, ReferenceItem[]>()
  for (const it of items) byKind.set(it.kind, [...(byKind.get(it.kind) ?? []), it])
  return [...byKind.entries()]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([kind, list]) => ({ kind, items: list }))
}

/** typeDocs lists the typed parts of an item as [label, doc] pairs, in reading order. */
export function typeDocs(it: ReferenceItem): Array<[string, NonNullable<ReferenceItem['input']>]> {
  const out: Array<[string, NonNullable<ReferenceItem['input']>]> = []
  const add = (label: string, doc: ReferenceItem['input']) => { if (doc) out.push([label, doc]) }
  add('record', it.record)
  add('input', it.input)
  add('output', it.output)
  add('payload', it.payload)
  add('params', it.params)
  add('props', it.props)
  for (const s of it.signals ?? []) out.push([`signal ${s.name}`, s.payload])
  return out
}

/** exampleText pretty-prints an example value. */
export function exampleText(value: unknown): string {
  return JSON.stringify(value, null, 2) ?? 'null'
}

/** matches reports whether an item satisfies a free-text filter (id, label, kind, description). */
export function matches(it: ReferenceItem, filter: string): boolean {
  const q = filter.trim().toLowerCase()
  if (!q) return true
  return [it.id, it.label, it.kind, it.description, it.permission].some((s) => s?.toLowerCase().includes(q))
}
