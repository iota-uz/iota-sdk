// GENERATED — do not edit

export const CONTRACT_VERSION = "1.0.0"

export interface Action {
  kind: ActionKind
  method?: string
  urlTemplate?: string
  urlSource?: Source
  event?: string
  params: Array<ActionParam>
  payload: Record<string, Source>
  preserveQuery?: boolean
}

export type ActionKind = "emit_event" | "navigate" | "navigate_to_leaf" | "open_drawer"

export interface ActionParam {
  name: string
  source: Source
}

export type Availability = "available" | "config_required" | "empty_source" | "unavailable"

export type BridgeLayout = "waterfall"

export type CascadeTone = "inflow" | "negative" | "neutral" | "positive"

export type ColorBy = "category"

export interface Column {
  name: string
  type: ColumnType
}

export type ColumnType = "bool" | "number" | "string" | "time"

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
  endpoints: Endpoints
  i18n: Record<string, string>
  theme: Theme
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
  export?: string
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
}

export type FilterKind = "period"

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
  group?: LayoutGroup
  groups?: Array<LayoutGroup>
}

export interface LayoutRow {
  heading?: string
  class?: string
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
  presentation?: Presentation
  metricFlow?: MetricFlowConfig
  metricHierarchy?: MetricHierarchyConfig
  metricRelationship?: MetricRelationshipConfig
  radial?: RadialConfig
  confidence?: Confidence
  availability?: Availability
}

export type PanelKind = "area" | "bar" | "cascade" | "coverage" | "donut" | "hbar" | "line" | "metric_flow" | "metric_hierarchy" | "metric_relationship" | "pie" | "radial" | "stat" | "table"

export interface PanelStatus {
  label: string
  tone?: StatusTone
}

export interface PanelTarget {
  value: number
  label?: string
}

export interface PanelTrend {
  percent: number
  label?: string
  invert?: boolean
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
  page?: number
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

export type Semantics = "evidence" | "partition" | "reconciliation" | "series"

export type SliceLabels = "label" | "percent"

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
}

export interface Theme {
  palette: Record<string, string>
  series: Record<string, string>
}

export type TotalBadgePlacement = "header" | "none" | "plot"

export type ValueSourceKind = "field" | "literal" | "variable"

