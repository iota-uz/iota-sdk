// Public contract of @iota-uz/sdk/automation-editor (issue #1331).
// The graph format and catalog view follow pkg/automation/graph (#1322).
// No type of @xyflow/system appears here or anywhere in the public surface.
import type { Accessor, JSX } from 'solid-js'

// Graph format of #1322 as JSON.
export type AutomationGraph = {
  name: string
  description?: string
  run_as: 'owner' | 'initiator'
  triggers: GraphTrigger[]
  nodes: GraphNode[]
  edges: GraphEdge[]
}
export type GraphTrigger = { id: string; event: string; filter?: string; params?: Record<string, unknown> }
export type GraphNode = { id: string; type: string; params: Record<string, unknown> }
/** `to` is a node id or `"ignore"`; a trigger's only outcome is `"fired"`. */
export type GraphEdge = { from: { node: string; outcome: string }; to: string }
export type XY = { x: number; y: number }
/** node/trigger id → position; one per automation. */
export type AutomationLayout = Record<string, XY>

export type JsonSchema = Record<string, unknown>
export type ExpressionType = 'bool' | 'string' | 'map' | 'list' | 'dyn'
export type OutcomeRule =
  | { fixed: string[]; optional?: { param: string; outcome: string } }
  | { from: { param: string; field?: string }; fixed?: string[]; optional?: { param: string; outcome: string } }

/** Catalog view of #1322 for one tenant and author (already filtered by connections and permissions). */
export type EditorCatalog = {
  revision: string
  events: CatalogEvent[]
  nodes: CatalogNode[]
  context_schema: JsonSchema
  /** Secret names the tenant may declare in Code nodes. */
  code?: { secrets: string[] }
}
export type CatalogEvent = {
  /** "core.user.created.v1" (EventView.Key of #1322) */
  key: string
  /** Localized by the host. */
  title: string
  /** Palette group, localized. */
  group: string
  /** "core.user" | "conversation" | "none" */
  subject: string
  /** Projected for the author (sensitive fields removed). */
  payload: JsonSchema
  trigger_params: JsonSchema | null
  /** Manual trigger with an input form. */
  manual?: boolean
  schedule?: boolean
  /** false → the palette marks the event "no connection" and does not offer it. */
  connected?: boolean
}
export type NodeBehavior =
  | 'branch' | 'switch' | 'wait' | 'call' | 'effect' | 'end' | 'parallel' | 'join' | 'foreach' | 'collect'
export type CatalogNode = {
  type: string
  title: string
  description: string
  /** "logic" | "data" | "human" | "sdk" | connector kind … */
  group: string
  icon?: string
  behavior: NodeBehavior
  params: JsonSchema
  outcomes: string[]
  outcome_rule: OutcomeRule
  expressions: { param: string; type: ExpressionType }[]
  templates: string[]
  output: JsonSchema | null
  /** ConnectorKind; the host fills the connection picker through fieldSlot. */
  connector?: string
  /** false → the palette marks the type "no connection" and does not offer it. */
  connected?: boolean
}

export type GraphError = { path: string; message: string; line?: number; column?: number; code?: string }
export type Subject = { entity: string; key: string; label?: string }

// Types of runs, dry run and test node; the editor ships only the types and the overlay prop.
export type TraceStep = {
  node: string; token?: string; kind: string; outcome?: string | null
  path?: string; message?: string; effects?: unknown[]
}
export type TestNodeResult = {
  outcome: string; output?: unknown; logs?: string[]; duration_ms?: number
  error?: { reason: string; message: string; line?: number }
}
export type SaveResult =
  /** Saved; with pending_review the enabled version stays until review. */
  | { ok: true; graph: AutomationGraph; version: number; pending_review?: boolean }
  | { errors: GraphError[]; warnings?: GraphError[] }
  | { conflict: { by: string; version: number } }

/** A choice of a field that names an options source (`x-options-source` in its schema): a chat of a bot. */
export type EditorOption = { value: string; label: string; /** A key the editor names: `private`, `group`. */ group?: string }

export interface EditorApi {
  validate(graph: AutomationGraph): Promise<{ ok: boolean; errors: GraphError[]; warnings?: GraphError[] }>
  dryRun(input: { graph: AutomationGraph; subject: Subject; context?: Record<string, unknown>; events: Record<string, unknown>[] }): Promise<{ trace: TraceStep[] }>
  testNode(input: { graph: AutomationGraph; node_id: string; subject?: Subject; context?: Record<string, unknown> }): Promise<TestNodeResult>
  searchSubjects?(entity: string, query: string): Promise<{ subject: Subject; label: string; note?: string }[]>
  /** Choices for a field with `x-options-source`; `params` are the step's params (its `connection`). */
  searchOptions?(source: string, query: string, ctx: { nodeType: string; params: Record<string, unknown> }): Promise<EditorOption[]>
}

export type ItemState = 'passed' | 'running' | 'waiting' | 'failed' | 'skipped' | 'cancelled'
/** taken: "node.outcome" */
export type Overlay = { states: Map<string, ItemState>; badges: Map<string, string>; taken: Set<string> }

export type FieldSlot = (field: {
  nodeType: string; nodeId: string; param: string
  value: unknown; onChange(value: unknown): void
  params: Record<string, unknown>; onParamsChange(params: Record<string, unknown>): void
  readOnly: boolean; graph: AutomationGraph
}) => JSX.Element | undefined // undefined → the editor's own field

export type VersionEntry = { version: number; note?: string; enabled?: boolean; by?: string }

export type EditorLocale = 'en' | 'ru' | 'uz' | 'uz-Cyrl'

export interface EditorProps {
  /** As saved; the editor keeps its own draft. */
  graph: AutomationGraph
  /** Base of the next save; 0 for a new graph. */
  version: number
  catalog: EditorCatalog
  api: EditorApi
  layout: AutomationLayout
  save(graph: AutomationGraph, layout: AutomationLayout, base: number): Promise<SaveResult>
  /** Absent for an unsaved graph and read-only. */
  saveLayout?(positions: AutomationLayout): Promise<void>
  reload(): Promise<{ graph: AutomationGraph; version: number; layout: AutomationLayout }>
  versions?: VersionEntry[]; onOpenVersion?(version: number): void
  enabled: boolean
  /** The host asks for confirmation; absent → no toggle. */
  toggleEnabled?(): void
  readOnly?: { reason: string }
  fieldSlot?: FieldSlot
  overlay?: Overlay | null
  tab?: 'edit' | 'runs'; onTab?(tab: 'edit' | 'runs'): void; runsCount?: number; runsBody?: JSX.Element
  /** Extra docked panels from the host (Dry run, Test node). */
  panels?: JSX.Element
  crumb?: JSX.Element; actions?: JSX.Element; banner?: JSX.Element
  locale: EditorLocale
  /** The selected step, for a host that keeps it in the URL (D1); omitted → the editor's own state. */
  selected?: string | null
  onSelect?(id: string | null): void
}

export interface CanvasApi {
  center(): XY; fit(): void; zoomIn(): void; zoomOut(): void; zoomReset(): void
  selectAll(): void; selection(): string[]; focus(id: string): void; toBox(flow: XY): XY
}
export type CanvasDrop = { from: { node: string; outcome: string }; x: number; y: number; flow: XY }
export type CanvasMenu = { x: number; y: number; flow: XY; item: string | null }
export type CanvasItem = {
  id: string; kind: 'trigger' | 'node'; type: string
  outs: { outcome: string; to: string | null; ignore: boolean }[]
  state?: ItemState; error?: string
}
export interface CanvasProps {
  graph: AutomationGraph; catalog: EditorCatalog; layout: AutomationLayout
  errors?: GraphError[]; overlay?: Overlay | null
  selected: string | null; onSelect(id: string | null): void
  /** Absent → read-only. */
  onChange?(graph: AutomationGraph): void
  /** Once per drag end, snapped. */
  onMove?(positions: AutomationLayout): void
  /** Outcome released on empty canvas → palette. */
  onDrop?(drop: CanvasDrop): void
  /** Right click, long press, context-menu key. */
  onMenu?(menu: CanvasMenu): void
  onReady?(api: CanvasApi): void; onEscape?(): boolean; onOpen?(id: string): void
  selecting?: boolean; snap?: boolean; minimap?: boolean
  /** Shows the Tidy button next to Fit; absent → no button. */
  onTidy?(): void
  /** Bump it just before a layout change that should animate (Tidy): the cards glide for 200 ms. */
  layoutTransition?: number
  renderNode?(item: Accessor<CanvasItem>): JSX.Element
  label?: string
  /** Locale of the canvas' own strings (aria labels, hints); defaults to en. */
  locale?: EditorLocale
}

export interface OutlineProps {
  graph: AutomationGraph
  catalog: EditorCatalog
  overlay?: Overlay | null
  locale?: EditorLocale
}
