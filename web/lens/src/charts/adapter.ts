import type { Encoding, Frame, GeoJSONFeatureCollection, NodeKey, PanelKind, PanelTemporal, Presentation, RadialConfig, Theme, ValueAxis } from '../contract'

export type ChartKind = Extract<PanelKind, 'pie' | 'donut' | 'radial' | 'bar' | 'hbar' | 'line' | 'area' | 'histogram' | 'boxplot' | 'heatmap' | 'map'>
export type ChartFormatResolver = (field: string, value: unknown) => string

export interface ChartLabels {
  previous?: string
  trend?: string
  movingAverage?: (window: number) => string
  estimate?: string
  ytd?: string
  forecast?: string
  forecastLower?: (forecast: string) => string
  forecastConfidence?: (forecast: string) => string
  boxplot?: [min: string, q1: string, median: string, q3: string, max: string]
  /** What a mark with no reading behind it says instead of a formatted zero. */
  noData: string
  /** The tooltip's copy button, at rest and for the moment after it is used. */
  copyValue?: string
  copied?: string
}

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
  formatTooltip?: (field: string, value: unknown, reference: number) => string
  /**
   * The unabbreviated figure behind a compact one, for tooltips. A plot label
   * has to fit beside a mark and abbreviates to «53,89 млрд UZS»; a reader who
   * has stopped to point at that mark is asking for the whole number.
   * Undefined when the field is not compact — nothing was abbreviated away.
   */
  formatExact?: (field: string, value: unknown) => string | undefined
  /**
   * The machine value behind a formatted one — plain digits in the unit the
   * figure is drawn in — for the tooltip's copy button. Resolved here rather
   * than in the adapter because the scaling is a property of the field format,
   * which is the runtime's knowledge, not the chart library's.
   */
  rawValue?: (field: string, value: unknown) => string | undefined
  /** The reader's language, for the shares the chart writes itself. */
  locale?: string
  /** Producer-pinned separator shared by values, legend shares, and slice labels. */
  shareDecimalSeparator?: string
  /** Localized label used for the sum of a stacked column in its tooltip. */
  tooltipTotalLabel?: string
  /**
   * What a click on a mark does, when it does anything — printed at the foot of
   * every tooltip this chart draws.
   *
   * It used to be a glyph in the notes row above the plot, which is the corner
   * of the card furthest from wherever the reader is actually pointing. An
   * affordance belongs where the act it describes is available: the tooltip is
   * already open, already under the cursor, and already naming the mark that
   * the click would open. Absent when the plot is inert, so a tooltip never
   * offers something that will not happen.
   */
  actionHint?: string
  /** Localized chart-generated labels that are not supplied by the producer. */
  labels?: ChartLabels
  theme: Theme
  selectedKey?: NodeKey
  /** Opt-in density hints; absent hints keep the default chart treatment. */
  presentation?: Presentation
  /** Numeric-axis scale requested by the dashboard producer. */
  valueAxis?: ValueAxis
  /**
   * The unit the axis ticks no longer carry — a currency, normally — so the
   * axis can state it once in its name instead of on every gridline or, as
   * before, nowhere at all.
   */
  valueUnit?: string
  /** Required geometry contract for radial charts. */
  radial?: RadialConfig
  /**
   * Opt-in readability overlays for line and area time series, as the panel
   * declared them. Every overlay stays here whether or not it is currently
   * drawn: `hiddenOverlays` decides that, so the legend can list an overlay
   * the reader has switched off and offer it back.
   */
  temporal?: PanelTemporal
  /**
   * Overlay ids (see `charts/overlays`) the reader has switched off. Filtering
   * happens here rather than in the panel so that one list of overlays governs
   * both what is drawn and what the legend prints.
   */
  hiddenOverlays?: ReadonlySet<string>
  /** Registered geometry and exact feature-property join for a map panel. */
  map?: {
    name: string
    geoJSON: GeoJSONFeatureCollection
    featureProperty: string
    labelProperty?: string
    /**
     * The property to read a feature's name from when `labelProperty` — the
     * locale-specific one — is absent on that feature. A boundary file that
     * carries English names for every region and Russian names for most is the
     * normal state of external geometry, and it must cost that one region its
     * translation, not the whole map.
     */
    fallbackLabelProperty?: string
  }
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
  /** Current plot width, supplied by the mounted adapter for responsive labels. */
  viewportWidth?: number
  /** Whether the producer declared formatting for the category/time field. */
  categoryFormatDefined?: boolean
}

/** Viewport coordinates of the activated mark, used to anchor an overlay. */
export interface ChartAnchor {
  x: number
  y: number
}

export interface ChartActivation {
  newTab: boolean
}

export interface ChartEvents {
  onSelect(key: NodeKey, anchor?: ChartAnchor, activation?: ChartActivation): void
  onHover(key: NodeKey | null): void
}

export interface ChartInstance {
  update(input: ChartInput): void
  resetZoom?(): void
  dispose(): void
}

export interface ChartAdapter {
  mount(el: HTMLElement, input: ChartInput, events: ChartEvents): ChartInstance
}
