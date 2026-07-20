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

export type ActionKind = "emit_event" | "navigate" | "navigate_to_leaf"

export interface ActionParam {
  name: string
  source: Source
}

export type ColorBy = "category"

export interface Column {
  name: string
  type: ColumnType
}

export type ColumnType = "bool" | "number" | "string" | "time"

export interface DashboardDocument {
  version: string
  snapshotId: string
  meta: Meta
  layout: Layout
  panels: Array<Panel>
  frames: Record<FrameRef, Frame>
  drill: Drill
  perspectives: Array<Perspective>
  endpoints: Endpoints
  i18n: Record<string, string>
  theme: Theme
}

export interface Drill {
  edges: Record<NodeKey, Level>
  inlineDepth: number
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

export type FormatKind = "date" | "money" | "number" | "percent" | "string"

export interface Frame {
  columns: Array<Column>
  rows: Array<Array<unknown>>
}

export type FrameRef = string

export interface Layout {
  rows: Array<LayoutRow>
}

export interface LayoutGroup {
  id: string
  kind: LayoutGroupKind
  label?: string
  layout?: LayoutGroupLayout
  span: number
  tab?: string
}

export type LayoutGroupKind = "metrics" | "tabs"

export type LayoutGroupLayout = "columns" | "rows"

export interface LayoutItem {
  panelId: string
  span: number
  group?: LayoutGroup
}

export interface LayoutRow {
  heading?: string
  class?: string
  panels: Array<LayoutItem>
}

export type LegendPlacement = "below"

export interface Level {
  path: NodePath
  label: string
  children: Array<Node>
  frame?: FrameRef
  encoding?: Encoding
  perspectives: Array<PerspectiveRef>
}

export interface Meta {
  dashboardId: string
  title: string
  generatedAt: string
  locale: string
}

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
  headline?: number
  trend?: PanelTrend
  presentation?: Presentation
}

export type PanelKind = "area" | "bar" | "cascade" | "coverage" | "donut" | "hbar" | "line" | "pie" | "stat" | "table"

export interface PanelStatus {
  label: string
  tone?: StatusTone
}

export interface PanelTrend {
  percent: number
  label?: string
  invert?: boolean
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
  sliceLabels?: SliceLabels
  totalBadge?: TotalBadgePlacement
  colorBy?: ColorBy
  fill?: boolean
  barWidthPx?: number
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

export type Semantics = "evidence" | "partition" | "reconciliation" | "series"

export type SliceLabels = "percent"

export interface Source {
  kind: ValueSourceKind
  name?: string
  value?: unknown
  fallback?: unknown
}

export type StatusTone = "neutral" | "positive" | "warning"

export type TableAffordance = "pill"

export type TableAlign = "left" | "right"

export interface TableCell {
  kind: TableCellKind
  secondaryField?: string
  layout?: TableCellLayout
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
}

export interface Theme {
  palette: Record<string, string>
  series: Record<string, string>
}

export type TotalBadgePlacement = "header" | "none" | "plot"

export type ValueSourceKind = "field" | "literal" | "variable"

