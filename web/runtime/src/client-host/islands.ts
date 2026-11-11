// Islands: the places Go-rendered (Templ/HTMX) pages hand to package components. The server writes a mount
// element with its whole configuration (`data-iota-slot` for a slot, `data-iota-cell` for a custom list cell);
// this host mounts each into its own Solid root and disposes it when the element leaves the document (an HTMX
// swap, a boosted navigation, the page going away). The island is never touched by HTMX: the server marks the
// element `hx-disable`, and the component renders into its own child.
import { createComponent, createSignal, ErrorBoundary, onCleanup, type Component } from 'solid-js'
import { render } from 'solid-js/web'
import type { Json } from '../model.ts'
import { RuntimeProvider } from './hooks.ts'
import { createSlotSource, Slot, SlotsProvider, type SlotContribution } from './slots.ts'
import type { RuntimeTransport } from './transport.ts'

export const SLOT_ATTR = 'data-iota-slot'
export const SLOT_CONFIG_ATTR = 'data-iota-slot-config'
export const CELL_ATTR = 'data-iota-cell'
export const CELL_CONFIG_ATTR = 'data-iota-cell-config'
/** The child an island renders into; anything else inside the element is the server-rendered fallback. */
export const OUTPUT_ATTR = 'data-iota-island-output'

/** What an island needs of the runtime, repeated on each element so a fragment is self-contained. */
export interface IslandEnv {
  rpc: string
  locale: string
  protocolVersion: string
  sdkReleaseVersion: string
  sdkCommit: string
}

export interface SlotElementConfig {
  props: Record<string, Json>
  contributions: SlotContribution[]
  env: IslandEnv
}

export interface CellElementConfig {
  contribution: SlotContribution
  /** The cell's value as wire JSON (null for NULL); exports always use the raw value, never this renderer. */
  value: Json
  /** The row's contributed values by column ID, plus `ref` (the native record). */
  row: Record<string, Json>
  env: IslandEnv
}

export interface IslandHostOptions {
  window: Window & typeof globalThis
  /**
   * Returns the transport for an island, or undefined to refuse it (an SDK identity mismatch, for example).
   * Called once per island; implementations cache.
   */
  prepare(env: IslandEnv): RuntimeTransport | undefined
  importModule?: (url: string) => Promise<Record<string, unknown>>
}

function parse<T>(element: Element, attr: string, win: Window & typeof globalThis): T | undefined {
  try {
    return JSON.parse(element.getAttribute(attr) ?? '') as T
  } catch (cause) {
    win.console.error(`Island ${element.getAttribute(SLOT_ATTR) ?? element.getAttribute(CELL_ATTR) ?? ''}: invalid configuration`, cause)
    return undefined
  }
}

function CellView(p: {
  config: CellElementConfig
  load: (c: SlotContribution) => Promise<Component<never> | undefined>
  ready: () => void
}) {
  const [component, setComponent] = createSignal<Component<never>>()
  let disposed = false
  onCleanup(() => { disposed = true })
  void p.load(p.config.contribution).then((loaded) => {
    if (disposed || !loaded) return // late or broken module: the server-rendered cell stays
    setComponent(() => loaded)
  })
  return createComponent(ErrorBoundary, {
    fallback: (cause: unknown) => {
      console.error(`Cell renderer ${p.config.contribution.id} failed`, cause)
      return undefined
    },
    get children() {
      const Cell = component() as Component<{ value: Json; row: Record<string, Json> }> | undefined
      if (!Cell) return undefined
      // A renderer that throws here lands in the boundary's fallback and never gets this far: the standard
      // cell is removed only once the renderer has produced its output.
      const output = createComponent(Cell, { value: p.config.value, row: p.config.row })
      queueMicrotask(() => { if (!disposed) p.ready() })
      return output
    },
  })
}

/**
 * Starts mounting islands: the ones in the document now and every one that appears later. Returns the stop
 * function, which disposes every mounted island.
 */
export function startIslandHost(options: IslandHostOptions): () => void {
  const win = options.window
  const doc = win.document
  const mounted = new Map<Element, () => void>()
  const selector = `[${SLOT_ATTR}],[${CELL_ATTR}]`
  const importModule = options.importModule

  const release = (element: Element, dispose: () => void) => {
    mounted.delete(element)
    try {
      dispose()
    } catch (cause) {
      win.console.error('Island disposal failed', cause)
    }
  }

  const output = (element: Element): HTMLElement => {
    // A restored history snapshot carries the rendered output of a root that no longer exists.
    for (const stale of Array.from(element.querySelectorAll(`:scope > [${OUTPUT_ATTR}]`))) stale.remove()
    const out = doc.createElement('div')
    out.setAttribute(OUTPUT_ATTR, '')
    out.style.display = 'contents'
    element.append(out)
    return out
  }

  const mountSlot = (element: Element): (() => void) => {
    const config = parse<SlotElementConfig>(element, SLOT_CONFIG_ATTR, win)
    const slotId = element.getAttribute(SLOT_ATTR) ?? ''
    const transport = config && options.prepare(config.env)
    if (!config || !transport) return () => {}
    const source = createSlotSource({
      contributions: { [slotId]: config.contributions },
      ...(importModule ? { importModule } : {}),
      console: win.console,
    })
    return render(
      () => createComponent(RuntimeProvider, {
        transport,
        get children() {
          return createComponent(SlotsProvider, {
            source,
            get children() { return createComponent(Slot, { slot: { slotId }, props: config.props }) },
          })
        },
      }),
      output(element),
    )
  }

  const mountCell = (element: Element): (() => void) => {
    const config = parse<CellElementConfig>(element, CELL_CONFIG_ATTR, win)
    const transport = config && options.prepare(config.env)
    if (!config || !transport) return () => {}
    const fallback = Array.from(element.childNodes)
    const source = createSlotSource({ contributions: {}, ...(importModule ? { importModule } : {}), console: win.console })
    return render(
      () => createComponent(RuntimeProvider, {
        transport,
        get children() {
          return createComponent(CellView, { config, load: (c) => source.load(c), ready: () => fallback.forEach((n) => n.remove()) })
        },
      }),
      output(element),
    )
  }

  const reconcile = () => {
    for (const [element, dispose] of [...mounted]) if (!element.isConnected) release(element, dispose)
    for (const element of Array.from(doc.querySelectorAll(selector))) {
      if (mounted.has(element)) continue
      // Reserve first: a failure below must not be retried on every mutation.
      mounted.set(element, () => {})
      let dispose: () => void
      try {
        dispose = element.hasAttribute(SLOT_ATTR) ? mountSlot(element) : mountCell(element)
      } catch (cause) {
        win.console.error('Island could not be mounted', cause)
        continue
      }
      mounted.set(element, dispose)
    }
  }

  const touches = (nodes: NodeList): boolean => {
    for (const node of Array.from(nodes)) {
      if (node.nodeType !== 1) continue
      const element = node as Element
      if (element.matches(selector) || element.querySelector(selector)) return true
    }
    return false
  }

  const observer = new win.MutationObserver((records) => {
    if (records.some((r) => (mounted.size > 0 && r.removedNodes.length > 0) || touches(r.addedNodes))) reconcile()
  })
  observer.observe(doc.documentElement, { childList: true, subtree: true })

  let stopped = false
  const stop = () => {
    if (stopped) return
    stopped = true
    win.removeEventListener('pagehide', stop)
    observer.disconnect()
    for (const [element, dispose] of [...mounted]) release(element, dispose)
  }
  win.addEventListener('pagehide', stop, { once: true })
  reconcile()
  return stop
}
