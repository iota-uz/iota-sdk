import type { Encoding, Frame, NodeKey, PanelKind, Presentation, Theme } from '../contract'

export type ChartKind = Extract<PanelKind, 'pie' | 'donut' | 'bar' | 'hbar' | 'line' | 'area'>
export type ChartFormatResolver = (field: string, value: unknown) => string

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
