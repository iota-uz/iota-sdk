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
  /** The reader's language, for the shares the chart writes itself. */
  locale?: string
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
  /**
   * The panel's own colour resolver — the same function its legend uses. The
   * chart cannot derive it: a document pins colours positionally, per panel
   * (`panelId:index`), and the chart knows neither which panel it is drawing
   * nor that such pins exist. Without it a panel's declared colours reach the
   * legend and never the plot, and the two disagree in front of the reader.
   *
   * Indexed by series (or, for a distributed bar, by category) ordinal.
   */
  seriesColor?: (label: string, index: number) => string | undefined
  /**
   * The same resolver indexed by frame *row*, which is what a part-to-whole
   * chart draws. It also carries the per-row palette a served frame ships —
   * that palette outranks the document's, so a drill level can paint its
   * remainder neutral without the placeholder panel knowing a remainder exists.
   */
  rowColor?: (label: string, index: number, nodeKey?: string) => string | undefined
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
