import type { Encoding, Frame, NodeKey, PanelKind, Presentation, RadialConfig, Theme } from '../contract'

export type ChartKind = Extract<PanelKind, 'pie' | 'donut' | 'radial' | 'bar' | 'hbar' | 'line' | 'area'>
export type ChartFormatResolver = (field: string, value: unknown) => string

/** Stable mark identity for a category within a specific partition ring. */
export function radialNodeKey(ringKey: string, categoryKey: string): NodeKey {
  return `radial:${JSON.stringify([ringKey, categoryKey])}`
}

export interface ChartInput {
  kind: ChartKind
  frame: Frame
  encoding: Encoding
  format: ChartFormatResolver
  /** Compact, locale-aware value formatter for axis ticks. Falls back to `format`. */
  formatAxis?: ChartFormatResolver
  theme: Theme
  selectedKey?: NodeKey
  /** Opt-in density hints; absent hints keep the default chart treatment. */
  presentation?: Presentation
  /** Required geometry contract for radial charts. */
  radial?: RadialConfig
  /**
   * Whether a point expands into a further level, keyed by its id (falling
   * back to its category label). Used for the per-mark cursor: with a Pareto
   * tail collapsed recursively, neighbouring slices of one ring differ in
   * whether they go anywhere, and nothing else on the plot says so.
   *
   * Absent means "unknown", which keeps the whole-chart treatment.
   */
  expandable?: (key: string) => boolean
  // Per-row palette shipped with a served frame, indexed like its rows. It
  // outranks the document palette so a level can paint its remainder neutral
  // without the placeholder panel knowing a remainder exists.
  colors?: string[]
}

/** Viewport coordinates of the activated mark, used to anchor an overlay. */
export interface ChartAnchor {
  x: number
  y: number
}

export interface ChartEvents {
  onSelect(key: NodeKey, anchor?: ChartAnchor): void
  onHover(key: NodeKey | null): void
}

export interface ChartInstance {
  update(input: ChartInput): void
  dispose(): void
}

export interface ChartAdapter {
  mount(el: HTMLElement, input: ChartInput, events: ChartEvents): ChartInstance
}
