// One function names every item by meaning (C3), shared by the canvas, the outline, the panel and
// the palette: a trigger is "Every day at 20:00" or the event's title, a step is its catalog title.
import { describeSchedule, nextRuns } from './cron'
import { catalogEvent, catalogNode, findItem } from './graph'
import { intlLocale } from './locale'
import type { AutomationGraph, EditorCatalog, EditorLocale } from './types'

export type ItemName = { title: string; subtitle: string; kind: 'trigger' | 'node'; icon?: string; group?: string }

function short(text: string, max = 64): string {
  const line = text.replace(/\s+/g, ' ').trim()
  return line.length > max ? `${line.slice(0, max - 1)}…` : line
}

export function scheduleParams(params: Record<string, unknown> | undefined): { cron: string; timezone: string } {
  const cron = typeof params?.cron === 'string' ? params.cron : ''
  const timezone = typeof params?.timezone === 'string' && params.timezone ? params.timezone : 'UTC'
  return { cron, timezone }
}

export function nextRunLabel(cron: string, timezone: string, locale: EditorLocale, now = new Date()): string | null {
  let runs: Date[]
  try {
    runs = nextRuns(cron, timezone, now, 1)
  } catch {
    return null
  }
  const next = runs[0]
  if (!next) return null
  return new Intl.DateTimeFormat(intlLocale(locale), { timeZone: timezone, month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(next)
}

export function describeItem(graph: AutomationGraph, catalog: EditorCatalog, id: string, locale: EditorLocale, now?: Date): ItemName {
  const ref = findItem(graph, id)
  if (!ref) return { title: id, subtitle: '', kind: 'node' }
  if (ref.kind === 'trigger') {
    const event = catalogEvent(catalog, ref.item.event)
    if (event?.schedule) {
      const { cron, timezone } = scheduleParams(ref.item.params)
      let title = event.title
      try {
        if (cron) title = describeSchedule(cron, locale)
      } catch {
        title = event.title
      }
      const next = cron ? nextRunLabel(cron, timezone, locale, now) : null
      return { title, subtitle: next ? `${timezone} · ${next}` : timezone, kind: 'trigger', icon: 'schedule', group: event.group }
    }
    const filter = ref.item.filter ? short(ref.item.filter, 48) : ''
    return {
      title: event?.title ?? ref.item.event,
      subtitle: filter ? `${event?.group ?? ''}${event?.group ? ' · ' : ''}${filter}` : event?.group ?? ref.item.event,
      kind: 'trigger',
      icon: event?.manual ? 'manual' : 'event',
      group: event?.group,
    }
  }
  const spec = catalogNode(catalog, ref.item.type)
  if (!spec) return { title: ref.item.type, subtitle: ref.item.id, kind: 'node' }
  const summaryParam = spec.expressions[0]?.param ?? spec.templates[0]
  const summary = summaryParam ? ref.item.params[summaryParam] : undefined
  return {
    title: spec.title,
    subtitle: typeof summary === 'string' && summary.trim() ? short(summary, 48) : ref.item.id,
    kind: 'node',
    icon: spec.icon,
    group: spec.group,
  }
}

/** Names of every item at once (one pass over the graph), for the canvas. */
export function describeAll(graph: AutomationGraph, catalog: EditorCatalog, locale: EditorLocale, now?: Date): Map<string, ItemName> {
  const out = new Map<string, ItemName>()
  const nodes = new Map(catalog.nodes.map((n) => [n.type, n]))
  for (const trigger of graph.triggers) out.set(trigger.id, describeItem({ ...graph, nodes: [], triggers: [trigger] }, catalog, trigger.id, locale, now))
  for (const node of graph.nodes) {
    const spec = nodes.get(node.type)
    if (!spec) {
      out.set(node.id, { title: node.type, subtitle: node.id, kind: 'node' })
      continue
    }
    const summaryParam = spec.expressions[0]?.param ?? spec.templates[0]
    const summary = summaryParam ? node.params[summaryParam] : undefined
    out.set(node.id, {
      title: spec.title,
      subtitle: typeof summary === 'string' && summary.trim() ? short(summary, 48) : node.id,
      kind: 'node',
      icon: spec.icon,
      group: spec.group,
    })
  }
  return out
}
