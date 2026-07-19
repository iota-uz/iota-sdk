// GENERATED — do not edit

export const CONTRACT_VERSION = "1.0.0"

export interface Action {
  kind: ActionKind
  method?: string
  urlTemplate?: string
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

export interface LayoutItem {
  panelId: string
  span: number
}

export interface LayoutRow {
  heading?: string
  class?: string
  panels: Array<LayoutItem>
}

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
  drillRoot?: NodeKey
  actions: Array<Action>
}

export type PanelKind = "area" | "bar" | "cascade" | "donut" | "hbar" | "line" | "pie" | "stat" | "table"

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

export type QueryErrorCode = "bad_request" | "internal" | "snapshot_gone"

export interface QueryErrorResponse {
  error: string
  message: string
}

export interface QueryPage {
  number: number
  size: number
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

export interface Source {
  kind: ValueSourceKind
  name?: string
  value?: unknown
  fallback?: unknown
}

export interface Theme {
  palette: Record<string, string>
  series: Record<string, string>
}

export type ValueSourceKind = "field" | "literal" | "variable"

