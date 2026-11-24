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
  /** Set for a call node: false → a Test node is confirmed as an irreversible effect. */
  idempotent?: boolean
  /** What a dry run does with the node. */
  dry_run?: 'execute' | 'capture' | 'forbid'
}

export type GraphError = { path: string; message: string; line?: number; column?: number; code?: string }
/** url: the subject's page, when the host knows it (a run's «Open <subject>»). */
export type Subject = { entity: string; key: string; label?: string; url?: string }

// ——— runs, dry run and test node (#1347) ———
/**
 * A step of a dry run. kind: triggered | not_triggered | step | wait | woke | timeout | joined | cancelled
 * | ended | error. status: captured (an effect not made, its params rendered in effects) or executed (a
 * call made for real, a live decision).
 */
export type TraceStep = {
  node: string; token?: string; kind: string; outcome?: string | null
  path?: string; message?: string; effects?: unknown[]
  status?: 'captured' | 'executed' | string; output?: unknown
}
/** How a dry run's execute nodes answer (#1336): from the cache, a live call, or the given context. */
export type DecisionMode = 'cached' | 'live' | 'scripted'
export type TestNodeResult = {
  outcome: string; output?: unknown; logs?: string[]; duration_ms?: number
  /** reason: exception | timeout | network_blocked | connection_unavailable | secret_missing | … */
  error?: { reason: string; message: string; line?: number }
}

export type RunState = 'running' | 'waiting' | 'done' | 'failed' | 'cancelled' | 'blocked'
export type RunSummary = {
  run_id: string
  version: number
  state: RunState
  subject: Subject | null
  /** The id of the graph's trigger that started the run. */
  trigger: string
  /** The phrase of the run's error, for a failed run. */
  error: string | null
  /** RFC 3339 */
  created_at: string
  updated_at: string
  /** Nodes where the run's tokens stand now. */
  current_nodes?: string[]
}
export type RunToken = {
  token_id: string; node: string | null; state: 'active' | 'waiting' | 'done' | 'cancelled'
  parent?: string | null; foreach_index?: number | null
}
export type RunStep = {
  node: string
  token_id: string
  attempt: number
  input: unknown
  outcome: string | null
  output: unknown
  /** Already masked by the server. */
  logs: string[]
  error: { code: string; message: string } | null
  started_at: string
  ended_at: string | null
  /** The viewer may not read the step's data (a sensitive field). */
  hidden?: boolean
}
export type RunWait = {
  node: string; token_id: string; kind: 'event' | 'timer' | 'human' | 'call'
  since: string; timeout_at: string | null
  /** What Retry and Resolve name; a blocked call's wait. */
  wait_id?: string
}
export type RunData = RunSummary & { tokens: RunToken[]; steps: RunStep[]; waits: RunWait[] }
export type RunsQuery = { automation_id: string; version?: number; state?: RunState[]; subject?: string; cursor?: string; limit?: number }

/** The Runs tab's view state; a host that keeps it in the URL controls it (D1). */
export type RunsView = { run?: string | null; node?: string | null; state?: RunState[]; subject?: string }
export type SaveResult =
  /** Saved; with pending_review the enabled version stays until review. */
  | { ok: true; graph: AutomationGraph; version: number; pending_review?: boolean }
  | { errors: GraphError[]; warnings?: GraphError[] }
  | { conflict: { by: string; version: number } }

export interface EditorApi {
  validate(graph: AutomationGraph): Promise<{ ok: boolean; errors: GraphError[]; warnings?: GraphError[] }>
  dryRun(input: { graph: AutomationGraph; subject: Subject; context?: Record<string, unknown>; events: Record<string, unknown>[]; decisions?: DecisionMode }): Promise<{ trace: TraceStep[] }>
  testNode(input: { graph: AutomationGraph; node_id: string; subject?: Subject; context?: Record<string, unknown> }): Promise<TestNodeResult>
  searchSubjects?(entity: string, query: string): Promise<{ subject: Subject; label: string; note?: string }[]>
  /** The runs of a graph, newest first (limit ≤ 50). Both runs and run → the Runs tab. */
  runs?(input: RunsQuery): Promise<{ items: RunSummary[]; next_cursor: string | null }>
  /** A run; a run that is deleted or another tenant's rejects with an error whose code is "not_found". */
  run?(run_id: string): Promise<RunData>
  /** The graph of a version, which a run of another version is drawn with. */
  version?(version: number): Promise<{ graph: AutomationGraph; layout: AutomationLayout }>
  // Operate (Automations.Operate): without a method its button is not rendered.
  cancelRun?(run_id: string): Promise<void>
  /** A blocked run: repeat the blocked call. */
  retryRun?(input: { run_id: string; wait_id?: string }): Promise<void>
  /** A blocked run: go on with the chosen outcome of the blocked node. */
  resolveRun?(input: { run_id: string; outcome: string; wait_id?: string }): Promise<void>
}

export type ItemState = 'passed' | 'running' | 'waiting' | 'failed' | 'skipped' | 'cancelled'
/** taken: "node.outcome"; notes: a line under a card's title (the message of a failed branch). */
export type Overlay = { states: Map<string, ItemState>; badges: Map<string, string>; taken: Set<string>; notes?: Map<string, string> }

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
  /** Absent for an unsaved graph and read-only. A rejection leaves the positions on screen unsaved. */
  saveLayout?(positions: AutomationLayout): Promise<void>
  /**
   * Positions the host read from storage (another tab saved them): each new object replaces the
   * positions on screen and becomes the saved layout, without a history step or a layout save.
   */
  remoteLayout?: { positions: AutomationLayout } | null
  /** Called when the draft starts or stops differing from the saved graph (the host guards navigation). */
  onDirtyChange?(dirty: boolean): void
  reload(): Promise<{ graph: AutomationGraph; version: number; layout: AutomationLayout }>
  versions?: VersionEntry[]; onOpenVersion?(version: number): void
  enabled: boolean
  /** The host asks for confirmation; absent → no toggle. */
  toggleEnabled?(): void
  readOnly?: { reason: string }
  fieldSlot?: FieldSlot
  overlay?: Overlay | null
  tab?: 'edit' | 'runs'; onTab?(tab: 'edit' | 'runs'): void; runsCount?: number
  /** The host's own Runs tab; absent and api.runs/api.run given → the editor's runs view. */
  runsBody?: JSX.Element
  /** The graph's id, which api.runs is asked with. */
  automationId?: string
  /** Rights of the viewer (the host's RBAC); absent → author unless read-only, operate by the api's methods. */
  can?: { author?: boolean; operate?: boolean }
  /** The Runs tab's state (run, node, filters); with onRunsView the host keeps it (in the URL). */
  runsView?: RunsView
  /** push: a new history entry (a run or node opened); otherwise the current one is replaced. */
  onRunsView?(view: RunsView, push: boolean): void
  /** A link to a run's subject («Open <subject>»); undefined → no link. */
  subjectHref?(subject: Subject): string | undefined
  /** «Run» on the empty Runs tab of a graph with a manual trigger; absent → no button. */
  onStartRun?(): void
  /** The name of a connection by id (the Test node confirmation names it). */
  connectionName?(id: string): string | undefined
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
  /** Makes each item a button (a run's outline opens the node's drawer). */
  onPick?(id: string): void
}
