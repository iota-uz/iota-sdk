// GENERATED — do not edit

export const CONTRACT_VERSION = "1.0.0"

export interface Action {
  kind: ActionKind
  method?: string
  urlTemplate?: string
  urlSource?: Source
  drawerKey?: Source
  event?: string
  params: Array<ActionParam>
  payload: Record<string, Source>
  preserveQuery?: boolean
  filter?: ActionFilter
}

export interface ActionFilter {
  dimension: string
  value: Source
  groupBy?: string
}

export type ActionKind = "cross_filter" | "cube_drill" | "emit_event" | "navigate" | "navigate_to_leaf" | "open_drawer"

export interface ActionParam {
  name: string
  source: Source
}

export interface ActiveFilter {
  dimension: string
  label: string
  value: string
  removeUrl: string
}

export type Availability = "available" | "config_required" | "empty_source" | "unavailable"

export type AxisScale = "linear" | "logarithmic"

export type BridgeLayout = "waterfall"

export type CascadeTone = "inflow" | "negative" | "neutral" | "positive"

export type ColorBy = "category" | "sequence"

export interface Column {
  name: string
  type: ColumnType
}

export type ColumnType = "bool" | "number" | "string" | "time"

export interface CompareFilter {
  modeParam: string
  startParam: string
  endParam: string
  compareTo: string
  value: CompareValue
}

export type CompareMode = "custom" | "off" | "previous_period" | "year_ago"

export interface CompareValue {
  mode: CompareMode
  start?: string
  end?: string
}

export type Confidence = "calculated" | "proxy" | "requires_reconciliation" | "verified"

export interface DashboardDocument {
  version: string
  snapshotId: string
  meta: Meta
  layout: Layout
  panels: Array<Panel>
  frames: Record<FrameRef, Frame>
  drill: Drill
  perspectives: Array<Perspective>
  filters?: Array<Filter>
  activeFilters?: Array<ActiveFilter>
  resetFiltersUrl?: string
  endpoints: Endpoints
  i18n: Record<string, string>
  theme: Theme
  urlState?: URLStateContract
  header?: DocumentHeader
  drawer?: DrawerHeader
}

export interface DocumentHeader {
  title?: string
  subtitle?: string
}

export interface DrawerHeader {
  eyebrow?: string
  title?: string
  caption?: string
  size?: DrawerSize
}

export interface DrawerResolveRequest {
  snapshotId: string
  metricKey: string
  params?: Record<string, unknown>
}

export interface DrawerResolveResponse {
  url: string
}

export type DrawerSize = "wide"

export interface Drill {
  edges: Record<NodeKey, Level>
  inlineDepth: number
}

export interface DynamicChildren {
  key: Source
  label: Source
  target?: Source
  action?: Action
}

export interface Encoding {
  label?: string
  value?: string
  previous?: string
  lower?: string
  q1?: string
  median?: string
  q3?: string
  upper?: string
  id?: string
  series?: string
  category?: string
  cut?: string
  cutLabel?: string
  final?: string
  annotation?: string
  tone?: string
  split?: string
  splitLabel?: string
  share?: string
  confidence?: string
  availability?: string
}

export interface Endpoints {
  query?: string
  panel?: string
  drawer?: string
  export?: string
  release?: string
}

export interface FacetFilter {
  dimension: string
  optionsEndpoint: string
  searchParam?: string
  selections?: Array<FacetSelection>
  clearUrl?: string
}

export interface FacetSelection {
  label: string
  removeUrl: string
}

export interface FieldFormat {
  kind: FormatKind
  currency?: string
  minorUnits: boolean
  precision?: number
  layout?: string
  compact?: boolean
  decimalSeparator?: string
  symbol?: string
}

export interface Filter {
  id: string
  kind: FilterKind
  label?: string
  period?: PeriodFilter
  facet?: FacetFilter
  compare?: CompareFilter
  segmented?: SegmentedFilter
}

export type FilterKind = "compare" | "facet" | "period" | "segmented"

export interface FlowReconciliation {
  tolerance?: number
}

export type FocusMode = "canvas"

export type FormatKind = "date" | "money" | "number" | "percent" | "string"

export interface Frame {
  columns: Array<Column>
  rows: Array<Array<unknown>>
  children?: Array<Node>
  total?: number
  presentation?: Presentation
  colors?: Array<string>
}

export type FrameRef = string

export interface GeoJSONFeature {
  type: string
  properties: Record<string, unknown>
  geometry: Record<string, unknown>
}

export interface GeoJSONFeatureCollection {
  type: string
  features: Array<GeoJSONFeature>
}

export interface GeoJSONSource {
  inline?: GeoJSONFeatureCollection
  url?: string
  maxBytes?: number
}

export interface HierarchyReconciliation {
  tolerance?: number
}

export interface Layout {
  rows: Array<LayoutRow>
}

export interface LayoutGroup {
  id: string
  kind: LayoutGroupKind
  caption?: string
  label?: string
  layout?: LayoutGroupLayout
  span: number
  tab?: string
  status?: PanelStatus
}

export type LayoutGroupKind = "metrics" | "tabs"

export type LayoutGroupLayout = "columns" | "rows"

export interface LayoutItem {
  panelId: string
  span: number
  groups?: Array<LayoutGroup>
}

export interface LayoutRow {
  heading?: string
  class?: string
  anchor?: string
  panels: Array<LayoutItem>
}

export type LegendPlacement = "below"

export type LegendValue = "percent" | "value"

export interface Level {
  path: NodePath
  label: string
  children: Array<Node>
  dynamicChildren?: DynamicChildren
  frame?: FrameRef
  encoding?: Encoding
  perspectives: Array<PerspectiveRef>
  defaultPerspective?: string
  view?: PanelKind
  presentation?: Presentation
  status?: PanelStatus
  source?: LevelSource
}

export interface LevelSource {
  label?: string
  frame: FrameRef
  columns?: Array<TableColumn>
  format?: Record<string, FieldFormat>
}

export interface MapConfig {
  source: GeoJSONSource
  featureProperty: string
  labelProperty?: string
  labelProperties?: Record<string, string>
  attribution?: string
}

export interface Meta {
  dashboardId: string
  title: string
  generatedAt: string
  locale: string
}

export interface MetricFlowConfig {
  stages: Array<MetricFlowStage>
  reconcile?: FlowReconciliation
}

export interface MetricFlowStage {
  key: string
  label: string
  role: MetricFlowStageRole
  caption?: string
  confidence?: Confidence
  availability?: Availability
  action?: Action
}

export type MetricFlowStageRole = "add" | "input" | "intermediate" | "result" | "subtract"

export interface MetricHierarchyConfig {
  rows: Array<MetricHierarchyRow>
  reconcile?: HierarchyReconciliation
}

export interface MetricHierarchyRow {
  key: string
  label: string
  description?: string
  parent?: string
  depth?: number
  unallocated?: boolean
  selected?: boolean
  confidence?: Confidence
  availability?: Availability
  action?: Action
}

export interface MetricRelationshipConfig {
  source: MetricRelationshipEnd
  target: MetricRelationshipEnd
  type: MetricRelationshipType
  direction?: MetricRelationshipDirection
  note?: string
}

export type MetricRelationshipDirection = "bidirectional" | "source_to_target" | "target_to_source"

export interface MetricRelationshipEnd {
  key: string
  label: string
  confidence?: Confidence
  availability?: Availability
  action?: Action
}

export type MetricRelationshipType = "association" | "derivation" | "reconciliation"

export interface Node {
  key: NodeKey
  path: NodePath
  label: string
  target?: NodeKey
  action?: Action
}

export type NodeKey = string

export type NodePath = Array<NodeKey>

export interface Panel {
  id: string
  kind: PanelKind
  title: string
  semantics: Semantics
  frame: FrameRef
  encoding: Encoding
  format: Record<string, FieldFormat>
  total?: number
  columns?: Array<TableColumn>
  table?: TableOptions
  drillRoot?: NodeKey
  actions: Array<Action>
  accent?: string
  status?: PanelStatus
  caption?: string
  info?: string
  headline?: number
  trend?: PanelTrend
  sparkline?: Sparkline
  target?: PanelTarget
  temporal?: PanelTemporal
  presentation?: Presentation
  valueAxis?: ValueAxis
  metricFlow?: MetricFlowConfig
  metricHierarchy?: MetricHierarchyConfig
  metricRelationship?: MetricRelationshipConfig
  radial?: RadialConfig
  map?: MapConfig
  confidence?: Confidence
  availability?: Availability
  deferred?: boolean
  terminal?: boolean
  comparisonUnsupported?: boolean
}

export interface PanelBatchRequest {
  snapshotId: string
  panels: Array<PanelRequest>
}

export interface PanelBatchResponse {
  panels: Record<string, PanelBatchResult>
}

export interface PanelBatchResult {
  frames?: Record<FrameRef, Frame>
  calculation?: PanelCalculation
  summary?: TableSummary
  page?: QueryPage
  error?: QueryErrorResponse
}

export interface PanelBatchStreamEvent {
  panelId?: string
  result?: PanelBatchResult
  complete?: boolean
}

export interface PanelCalculation {
  durationMs: number
  cacheHit: boolean
  calculatedAt: string
}

export type PanelKind = "area" | "bar" | "boxplot" | "cascade" | "coverage" | "donut" | "gauge" | "hbar" | "heatmap" | "histogram" | "line" | "map" | "metric_flow" | "metric_hierarchy" | "metric_relationship" | "pie" | "radial" | "stat" | "table"

export interface PanelRequest {
  panelId: string
  recompute?: boolean
  search?: string
  sort?: TableSort
  page?: number
}

export interface PanelResponse {
  frames: Record<FrameRef, Frame>
  calculation: PanelCalculation
  summary?: TableSummary
  page?: QueryPage
}

export interface PanelStatus {
  label: string
  tone?: StatusTone
}

export interface PanelTarget {
  value: number
  label?: string
}

export interface PanelTemporal {
  regression?: TemporalSeries
  movingAverages?: Array<TemporalMovingAverage>
  referenceLines?: Array<PanelTarget>
  period?: TemporalPeriod
  annotations?: Array<TemporalAnnotation>
  forecast?: TemporalForecast
}

export interface PanelTrend {
  percent: number
  label?: string
  polarity?: TrendPolarity
  invert?: boolean
  absoluteField?: string
  percentField?: string
  absoluteDeltaUnit?: TrendDeltaUnit
}

export interface PeriodFilter {
  startParam: string
  endParam: string
  value: PeriodValue
  min?: string
  max?: string
  allowEmpty?: boolean
  presets?: Array<PeriodPreset>
}

export interface PeriodPreset {
  id: string
  label: string
  value: PeriodValue
}

export interface PeriodValue {
  start: string
  end: string
}

export interface Perspective {
  id: string
  explorerId: string
  branchKey: NodeKey
  key: string
  label: string
  semantics: Semantics
  root: NodeKey
}

export interface PerspectiveRef {
  id: string
}

export interface Presentation {
  dataLabels?: boolean
  legend?: LegendPlacement
  legendValue?: LegendValue
  sliceLabels?: SliceLabels
  totalBadge?: TotalBadgePlacement
  colorBy?: ColorBy
  fill?: boolean
  barWidthPx?: number
  bridgeLayout?: BridgeLayout
  sortable?: boolean
  expandable?: boolean
  exportable?: boolean
  rowGroupField?: string
  focus?: FocusMode
  stack?: boolean
  lineSeries?: Array<string>
}

export type QueryErrorCode = "bad_request" | "internal" | "snapshot_gone"

export interface QueryErrorResponse {
  error: QueryErrorCode
  message: string
}

export interface QueryPage {
  number: number
  size: number
  hasNext?: boolean
}

export interface QueryRequest {
  snapshotId: string
  path: NodePath
  perspective?: string
  revision?: number
  prefetch?: boolean
  page?: number
  sort?: TableSort
}

export interface QueryResponse {
  frames: Record<FrameRef, Frame>
  page?: QueryPage
}

export interface RadialConfig {
  mode: RadialMode
  max?: number
  rings?: Array<RadialRing>
  tolerance?: number
}

export type RadialMode = "partition" | "progress"

export interface RadialRing {
  key: string
  label: string
  order?: number
  total: number
}

export interface SegmentedFilter {
  param: string
  value: string
  options: Array<SegmentedOption>
}

export interface SegmentedOption {
  value: string
  label: string
}

export type Semantics = "evidence" | "partition" | "reconciliation" | "series"

export type SliceLabels = "label" | "percent"

export type SortDirection = "asc" | "desc"

export interface Source {
  kind: ValueSourceKind
  name?: string
  value?: unknown
  fallback?: unknown
}

export interface Sparkline {
  values: Array<number>
  color?: string
}

export type StatusTone = "neutral" | "positive" | "warning"

export type TableAffordance = "pill" | "quiet"

export type TableAlign = "left" | "right"

export interface TableCell {
  kind: TableCellKind
  secondaryField?: string
  layout?: TableCellLayout
  toneField?: string
}

export type TableCellKind = "bar" | "delta" | "plain" | "underline"

export type TableCellLayout = "stacked"

export interface TableColumn {
  field: string
  label: string
  align?: TableAlign
  cell: TableCell
  action?: Action
  text?: string
  widthPx?: number
  clamp?: number
  affordance?: TableAffordance
  badgeField?: string
  heat?: boolean
  sampleSizeField?: string
  minSampleSize?: number
  total?: boolean
  shareOf?: string
}

export interface TableOptions {
  searchable?: boolean
}

export interface TableSort {
  field: string
  direction: SortDirection
}

export interface TableSummary {
  values: Record<string, unknown>
  fullValues?: Record<string, unknown>
  filteredRows: number
  totalRows: number
}

export interface TemporalAnnotation {
  at: string
  label: string
}

export interface TemporalForecast {
  start: string
  valueField: string
  lowerField: string
  upperField: string
  label?: string
}

export interface TemporalMovingAverage {
  window: number
  field: string
  label?: string
}

export interface TemporalPeriod {
  category: string
  state: TemporalPeriodState
  label?: string
  annualizedField?: string
}

export type TemporalPeriodState = "annualized" | "ytd"

export interface TemporalSeries {
  field: string
  label?: string
}

export interface Theme {
  palette: Record<string, string>
  series: Record<string, string>
  debounceMs?: number
}

export type TotalBadgePlacement = "header" | "none" | "plot"

export type TrendDeltaUnit = "percentage_points" | "value"

export type TrendPolarity = "higher_better" | "lower_better" | "neutral"

export interface URLStateContract {
  version: number
  param: string
  maxBytes: number
}

export interface ValueAxis {
  scale: AxisScale
  logBase?: number
}

export type ValueSourceKind = "field" | "literal" | "variable"

