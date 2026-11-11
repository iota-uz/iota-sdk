// Slots: where a page that is not the package's own lets packages mount components. The page declares the
// slot (stable ID, typed props, optional permission); every activated package may contribute to it. The server
// decides WHICH contributions a viewer gets (activation, order, each contribution's permission); this module
// only loads and renders them. It imports `solid-js` only, because the author-facing `Slot` is re-exported
// from the client entry, which the build also evaluates in Node.
import { createComponent, createContext, createSignal, ErrorBoundary, For, onCleanup, useContext, type Component, type JSX } from 'solid-js'
import type { SlotRef } from '../contracts.ts'

/** One contribution the server resolved for a slot: where its module is and which export holds the component. */
export interface SlotContribution {
  readonly id: string
  readonly package: string
  readonly version: string
  /** Immutable asset URL of the package's client entry chunk. */
  readonly module: string
  /** Export of `module` holding the `slot()` value; default export when absent. */
  readonly export?: string
}

/** What the host gives `<Slot>`: the contributions of a slot and the way to load one. */
export interface SlotSource {
  contributions(slotId: string): readonly SlotContribution[]
  /** The component of a contribution; undefined when its module is unusable (reported, never thrown). */
  load(contribution: SlotContribution): Promise<Component<never> | undefined>
}

const SlotContext = createContext<SlotSource>()

export interface SlotsProviderProps {
  source: SlotSource
  children: JSX.Element
}

/** Makes `<Slot>` work in a subtree. The runtime host installs it around every page, shell and island. */
export const SlotsProvider: Component<SlotsProviderProps> = (props) =>
  createComponent(SlotContext.Provider, {
    get value() { return props.source },
    get children() { return props.children },
  })

export interface SlotProps<P extends object> {
  /**
   * The slot, as the kit (or `defineSlot`) exports it; it types `props`. (Not named `ref`: Solid's compiler
   * reserves that prop on every component and turns it into an assignment.)
   */
  readonly slot: SlotRef<P>
  /** Passed to every contribution. Must match the slot's declared props. */
  readonly props: P
}

/**
 * Mounts the contributions of a slot, in the order the server resolved them (each filtered by its own
 * permission). Nothing renders while a contribution loads or when none applies; a contribution that fails to
 * load or throws is reported to the console and leaves its neighbours and the page alone.
 */
export function Slot<P extends object>(props: SlotProps<P>): JSX.Element {
  const source = useContext(SlotContext)
  if (!source) return undefined
  return createComponent(For, {
    get each() { return source.contributions(props.slot.slotId) },
    children: (contribution: SlotContribution) =>
      createComponent(ContributionView, { contribution, source, get props() { return props.props } }),
  })
}

function ContributionView(p: { contribution: SlotContribution; source: SlotSource; props: object }): JSX.Element {
  const [component, setComponent] = createSignal<Component<never>>()
  let disposed = false
  onCleanup(() => { disposed = true })
  // A module that arrives after the slot went away (navigation, swap) is dropped, never mounted.
  void p.source.load(p.contribution).then((loaded) => {
    if (!disposed && loaded) setComponent(() => loaded)
  })
  return createComponent(ErrorBoundary, {
    fallback: (cause: unknown) => {
      console.error(`Slot contribution ${p.contribution.id} failed`, cause)
      return undefined
    },
    get children() {
      const Contribution = component() as Component<object> | undefined
      return Contribution ? createComponent(Contribution, p.props) : undefined
    },
  })
}

/** The loaded module's export, when it is a client value with a component (`slot()`, `cell()`). */
export function resolveComponent(module: Record<string, unknown>, name: string | undefined): Component<never> | undefined {
  const value = (name ? module[name] : module.default) as { component?: unknown } | undefined
  return value && typeof value.component === 'function' ? (value.component as Component<never>) : undefined
}

export interface SlotSourceOptions {
  /** Contributions by slot ID, as the server resolved them for this viewer. */
  contributions: Readonly<Record<string, readonly SlotContribution[]>>
  /** Dynamic import, injectable for tests. */
  importModule?: (url: string) => Promise<Record<string, unknown>>
  console?: Pick<Console, 'error'>
}

const EMPTY: readonly SlotContribution[] = []

/** A SlotSource over a fixed server-resolved set; modules are imported once per URL. */
export function createSlotSource(options: SlotSourceOptions): SlotSource {
  const importModule = options.importModule ?? ((url: string) => import(/* @vite-ignore */ url) as Promise<Record<string, unknown>>)
  const log = options.console ?? console
  const modules = new Map<string, Promise<Record<string, unknown>>>()
  return {
    contributions: (slotId) => options.contributions[slotId] ?? EMPTY,
    async load(contribution) {
      let pending = modules.get(contribution.module)
      if (!pending) {
        pending = importModule(contribution.module)
        modules.set(contribution.module, pending)
      }
      try {
        const component = resolveComponent(await pending, contribution.export)
        if (!component) log.error(`Slot contribution ${contribution.id}: module ${contribution.module} has no component export "${contribution.export ?? 'default'}"`)
        return component
      } catch (cause) {
        log.error(`Slot contribution ${contribution.id}: module ${contribution.module} could not be loaded`, cause)
        return undefined
      }
    },
  }
}
