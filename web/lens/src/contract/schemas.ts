// GENERATED — do not edit

import { z } from 'zod'
import { CONTRACT_VERSION } from './types'
import type * as Contract from './types'

const CONTRACT_MAJOR_VERSION = CONTRACT_VERSION.split('.', 1)[0]!

function contractMajor(version: string): string {
  return version.split('.', 1)[0]!
}

export class ContractVersionMismatchError extends Error {
  readonly code = 'CONTRACT_VERSION_MISMATCH'
  readonly expectedMajor = CONTRACT_MAJOR_VERSION

  constructor(readonly actualVersion: string) {
    super(`Lens contract major version ${contractMajor(actualVersion)} is incompatible with expected major ${CONTRACT_MAJOR_VERSION}`)
    this.name = 'ContractVersionMismatchError'
  }
}

export const ContractVersionSchema: z.ZodType<string> = z.string().refine(
  (version) => contractMajor(version) === CONTRACT_MAJOR_VERSION,
  { message: `Expected Lens contract major version ${CONTRACT_MAJOR_VERSION}` },
)

export const ActionSchema: z.ZodType<Contract.Action> = z.lazy(() => z.object({
  kind: z.lazy(() => ActionKindSchema),
  method: z.string().optional(),
  urlTemplate: z.string().optional(),
  urlSource: z.lazy(() => SourceSchema).optional(),
  drawerKey: z.lazy(() => SourceSchema).optional(),
  event: z.string().optional(),
  params: z.array(z.lazy(() => ActionParamSchema)),
  payload: z.record(z.string(), z.lazy(() => SourceSchema)),
  preserveQuery: z.boolean().optional(),
  filter: z.lazy(() => ActionFilterSchema).optional(),
}).strict())

export const ActionFilterSchema: z.ZodType<Contract.ActionFilter> = z.lazy(() => z.object({
  dimension: z.string(),
  value: z.lazy(() => SourceSchema),
  groupBy: z.string().optional(),
}).strict())

export const ActionKindSchema: z.ZodType<Contract.ActionKind> = z.enum(["cross_filter", "cube_drill", "emit_event", "navigate", "navigate_to_leaf", "open_drawer"])

export const ActionParamSchema: z.ZodType<Contract.ActionParam> = z.lazy(() => z.object({
  name: z.string(),
  source: z.lazy(() => SourceSchema),
}).strict())

export const ActiveFilterSchema: z.ZodType<Contract.ActiveFilter> = z.object({
  dimension: z.string(),
  label: z.string(),
  value: z.string(),
  removeUrl: z.string(),
}).strict()

export const AvailabilitySchema: z.ZodType<Contract.Availability> = z.enum(["available", "config_required", "empty_source", "unavailable"])

export const AxisScaleSchema: z.ZodType<Contract.AxisScale> = z.enum(["linear", "logarithmic"])

export const BridgeLayoutSchema: z.ZodType<Contract.BridgeLayout> = z.enum(["waterfall"])

export const CascadeToneSchema: z.ZodType<Contract.CascadeTone> = z.enum(["inflow", "negative", "neutral", "positive"])

export const ColorBySchema: z.ZodType<Contract.ColorBy> = z.enum(["category"])

export const ColumnSchema: z.ZodType<Contract.Column> = z.lazy(() => z.object({
  name: z.string(),
  type: z.lazy(() => ColumnTypeSchema),
}).strict())

export const ColumnTypeSchema: z.ZodType<Contract.ColumnType> = z.enum(["bool", "number", "string", "time"])

export const CompareFilterSchema: z.ZodType<Contract.CompareFilter> = z.lazy(() => z.object({
  modeParam: z.string(),
  startParam: z.string(),
  endParam: z.string(),
  compareTo: z.string(),
  value: z.lazy(() => CompareValueSchema),
}).strict())

export const CompareModeSchema: z.ZodType<Contract.CompareMode> = z.enum(["custom", "off", "previous_period", "year_ago"])

export const CompareValueSchema: z.ZodType<Contract.CompareValue> = z.object({
  mode: z.lazy(() => CompareModeSchema),
  start: z.string().optional(),
  end: z.string().optional(),
}).strict()

export const ConfidenceSchema: z.ZodType<Contract.Confidence> = z.enum(["calculated", "proxy", "requires_reconciliation", "verified"])

export const DashboardDocumentSchema: z.ZodType<Contract.DashboardDocument> = z.lazy(() => z.object({
  version: ContractVersionSchema,
  snapshotId: z.string(),
  meta: z.lazy(() => MetaSchema),
  layout: z.lazy(() => LayoutSchema),
  panels: z.array(z.lazy(() => PanelSchema)),
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  drill: z.lazy(() => DrillSchema),
  perspectives: z.array(z.lazy(() => PerspectiveSchema)),
  filters: z.array(z.lazy(() => FilterSchema)).optional(),
  activeFilters: z.array(z.lazy(() => ActiveFilterSchema)).optional(),
  resetFiltersUrl: z.string().optional(),
  endpoints: z.lazy(() => EndpointsSchema),
  i18n: z.record(z.string(), z.string()),
  theme: z.lazy(() => ThemeSchema),
  urlState: z.lazy(() => URLStateContractSchema).optional(),
  header: z.lazy(() => DocumentHeaderSchema).optional(),
  drawer: z.lazy(() => DrawerHeaderSchema).optional(),
}).strict())

export const DocumentHeaderSchema: z.ZodType<Contract.DocumentHeader> = z.object({
  title: z.string().optional(),
  subtitle: z.string().optional(),
}).strict()

export const DrawerHeaderSchema: z.ZodType<Contract.DrawerHeader> = z.lazy(() => z.object({
  eyebrow: z.string().optional(),
  title: z.string().optional(),
  caption: z.string().optional(),
  size: z.lazy(() => DrawerSizeSchema).optional(),
}).strict())

export const DrawerResolveRequestSchema: z.ZodType<Contract.DrawerResolveRequest> = z.object({
  snapshotId: z.string(),
  metricKey: z.string(),
  params: z.record(z.string(), z.unknown()).optional(),
}).strict()

export const DrawerResolveResponseSchema: z.ZodType<Contract.DrawerResolveResponse> = z.object({
  url: z.string(),
}).strict()

export const DrawerSizeSchema: z.ZodType<Contract.DrawerSize> = z.enum(["wide"])

export const DrillSchema: z.ZodType<Contract.Drill> = z.lazy(() => z.object({
  edges: z.record(z.lazy(() => NodeKeySchema), z.lazy(() => LevelSchema)),
  inlineDepth: z.number().int(),
}).strict())

export const DynamicChildrenSchema: z.ZodType<Contract.DynamicChildren> = z.lazy(() => z.object({
  key: z.lazy(() => SourceSchema),
  label: z.lazy(() => SourceSchema),
  target: z.lazy(() => SourceSchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict())

export const EncodingSchema: z.ZodType<Contract.Encoding> = z.object({
  label: z.string().optional(),
  value: z.string().optional(),
  previous: z.string().optional(),
  lower: z.string().optional(),
  q1: z.string().optional(),
  median: z.string().optional(),
  q3: z.string().optional(),
  upper: z.string().optional(),
  id: z.string().optional(),
  series: z.string().optional(),
  category: z.string().optional(),
  cut: z.string().optional(),
  cutLabel: z.string().optional(),
  final: z.string().optional(),
  annotation: z.string().optional(),
  tone: z.string().optional(),
  split: z.string().optional(),
  splitLabel: z.string().optional(),
  share: z.string().optional(),
  confidence: z.string().optional(),
  availability: z.string().optional(),
}).strict()

export const EndpointsSchema: z.ZodType<Contract.Endpoints> = z.object({
  query: z.string().optional(),
  panel: z.string().optional(),
  drawer: z.string().optional(),
  export: z.string().optional(),
}).strict()

export const FacetFilterSchema: z.ZodType<Contract.FacetFilter> = z.lazy(() => z.object({
  dimension: z.string(),
  optionsEndpoint: z.string(),
  searchParam: z.string().optional(),
  selections: z.array(z.lazy(() => FacetSelectionSchema)).optional(),
  clearUrl: z.string().optional(),
}).strict())

export const FacetSelectionSchema: z.ZodType<Contract.FacetSelection> = z.object({
  label: z.string(),
  removeUrl: z.string(),
}).strict()

export const FieldFormatSchema: z.ZodType<Contract.FieldFormat> = z.lazy(() => z.object({
  kind: z.lazy(() => FormatKindSchema),
  currency: z.string().optional(),
  minorUnits: z.boolean(),
  precision: z.number().int().optional(),
  layout: z.string().optional(),
  compact: z.boolean().optional(),
  decimalSeparator: z.string().optional(),
  symbol: z.string().optional(),
}).strict())

export const FilterSchema: z.ZodType<Contract.Filter> = z.lazy(() => z.object({
  id: z.string(),
  kind: z.lazy(() => FilterKindSchema),
  label: z.string().optional(),
  period: z.lazy(() => PeriodFilterSchema).optional(),
  facet: z.lazy(() => FacetFilterSchema).optional(),
  compare: z.lazy(() => CompareFilterSchema).optional(),
}).strict())

export const FilterKindSchema: z.ZodType<Contract.FilterKind> = z.enum(["compare", "facet", "period"])

export const FlowReconciliationSchema: z.ZodType<Contract.FlowReconciliation> = z.object({
  tolerance: z.number().optional(),
}).strict()

export const FocusModeSchema: z.ZodType<Contract.FocusMode> = z.enum(["canvas"])

export const FormatKindSchema: z.ZodType<Contract.FormatKind> = z.enum(["date", "money", "number", "percent", "string"])

export const FrameSchema: z.ZodType<Contract.Frame> = z.lazy(() => z.object({
  columns: z.array(z.lazy(() => ColumnSchema)),
  rows: z.array(z.array(z.unknown())),
  children: z.array(z.lazy(() => NodeSchema)).optional(),
  total: z.number().optional(),
  presentation: z.lazy(() => PresentationSchema).optional(),
  colors: z.array(z.string()).optional(),
}).strict())

export const FrameRefSchema: z.ZodType<Contract.FrameRef> = z.string()

export const GeoJSONFeatureSchema: z.ZodType<Contract.GeoJSONFeature> = z.object({
  type: z.string(),
  properties: z.record(z.string(), z.unknown()),
  geometry: z.record(z.string(), z.unknown()),
}).strict()

export const GeoJSONFeatureCollectionSchema: z.ZodType<Contract.GeoJSONFeatureCollection> = z.object({
  type: z.string(),
  features: z.array(z.lazy(() => GeoJSONFeatureSchema)),
}).strict()

export const GeoJSONSourceSchema: z.ZodType<Contract.GeoJSONSource> = z.object({
  inline: z.lazy(() => GeoJSONFeatureCollectionSchema).optional(),
  url: z.string().optional(),
  maxBytes: z.number().int().optional(),
}).strict()

export const HierarchyReconciliationSchema: z.ZodType<Contract.HierarchyReconciliation> = z.object({
  tolerance: z.number().optional(),
}).strict()

export const LayoutSchema: z.ZodType<Contract.Layout> = z.lazy(() => z.object({
  rows: z.array(z.lazy(() => LayoutRowSchema)),
}).strict())

export const LayoutGroupSchema: z.ZodType<Contract.LayoutGroup> = z.lazy(() => z.object({
  id: z.string(),
  kind: z.lazy(() => LayoutGroupKindSchema),
  caption: z.string().optional(),
  label: z.string().optional(),
  layout: z.lazy(() => LayoutGroupLayoutSchema).optional(),
  span: z.number().int(),
  tab: z.string().optional(),
  status: z.lazy(() => PanelStatusSchema).optional(),
}).strict())

export const LayoutGroupKindSchema: z.ZodType<Contract.LayoutGroupKind> = z.enum(["metrics", "tabs"])

export const LayoutGroupLayoutSchema: z.ZodType<Contract.LayoutGroupLayout> = z.enum(["columns", "rows"])

export const LayoutItemSchema: z.ZodType<Contract.LayoutItem> = z.object({
  panelId: z.string(),
  span: z.number().int(),
  groups: z.array(z.lazy(() => LayoutGroupSchema)).optional(),
}).strict()

export const LayoutRowSchema: z.ZodType<Contract.LayoutRow> = z.object({
  heading: z.string().optional(),
  class: z.string().optional(),
  anchor: z.string().optional(),
  panels: z.array(z.lazy(() => LayoutItemSchema)),
}).strict()

export const LegendPlacementSchema: z.ZodType<Contract.LegendPlacement> = z.enum(["below"])

export const LegendValueSchema: z.ZodType<Contract.LegendValue> = z.enum(["percent", "value"])

export const LevelSchema: z.ZodType<Contract.Level> = z.lazy(() => z.object({
  path: z.lazy(() => NodePathSchema),
  label: z.string(),
  children: z.array(z.lazy(() => NodeSchema)),
  dynamicChildren: z.lazy(() => DynamicChildrenSchema).optional(),
  frame: z.lazy(() => FrameRefSchema).optional(),
  encoding: z.lazy(() => EncodingSchema).optional(),
  perspectives: z.array(z.lazy(() => PerspectiveRefSchema)),
  defaultPerspective: z.string().optional(),
  view: z.lazy(() => PanelKindSchema).optional(),
  presentation: z.lazy(() => PresentationSchema).optional(),
  status: z.lazy(() => PanelStatusSchema).optional(),
  source: z.lazy(() => LevelSourceSchema).optional(),
}).strict())

export const LevelSourceSchema: z.ZodType<Contract.LevelSource> = z.lazy(() => z.object({
  label: z.string().optional(),
  frame: z.lazy(() => FrameRefSchema),
  columns: z.array(z.lazy(() => TableColumnSchema)).optional(),
  format: z.record(z.string(), z.lazy(() => FieldFormatSchema)).optional(),
}).strict())

export const MapConfigSchema: z.ZodType<Contract.MapConfig> = z.object({
  source: z.lazy(() => GeoJSONSourceSchema),
  featureProperty: z.string(),
  labelProperty: z.string().optional(),
  labelProperties: z.record(z.string(), z.string()).optional(),
  attribution: z.string().optional(),
}).strict()

export const MetaSchema: z.ZodType<Contract.Meta> = z.object({
  dashboardId: z.string(),
  title: z.string(),
  generatedAt: z.string().datetime({ offset: true }),
  locale: z.string(),
}).strict()

export const MetricFlowConfigSchema: z.ZodType<Contract.MetricFlowConfig> = z.lazy(() => z.object({
  stages: z.array(z.lazy(() => MetricFlowStageSchema)),
  reconcile: z.lazy(() => FlowReconciliationSchema).optional(),
}).strict())

export const MetricFlowStageSchema: z.ZodType<Contract.MetricFlowStage> = z.lazy(() => z.object({
  key: z.string(),
  label: z.string(),
  role: z.lazy(() => MetricFlowStageRoleSchema),
  caption: z.string().optional(),
  confidence: z.lazy(() => ConfidenceSchema).optional(),
  availability: z.lazy(() => AvailabilitySchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict())

export const MetricFlowStageRoleSchema: z.ZodType<Contract.MetricFlowStageRole> = z.enum(["add", "input", "intermediate", "result", "subtract"])

export const MetricHierarchyConfigSchema: z.ZodType<Contract.MetricHierarchyConfig> = z.lazy(() => z.object({
  rows: z.array(z.lazy(() => MetricHierarchyRowSchema)),
  reconcile: z.lazy(() => HierarchyReconciliationSchema).optional(),
}).strict())

export const MetricHierarchyRowSchema: z.ZodType<Contract.MetricHierarchyRow> = z.object({
  key: z.string(),
  label: z.string(),
  description: z.string().optional(),
  parent: z.string().optional(),
  depth: z.number().int().optional(),
  unallocated: z.boolean().optional(),
  selected: z.boolean().optional(),
  confidence: z.lazy(() => ConfidenceSchema).optional(),
  availability: z.lazy(() => AvailabilitySchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict()

export const MetricRelationshipConfigSchema: z.ZodType<Contract.MetricRelationshipConfig> = z.lazy(() => z.object({
  source: z.lazy(() => MetricRelationshipEndSchema),
  target: z.lazy(() => MetricRelationshipEndSchema),
  type: z.lazy(() => MetricRelationshipTypeSchema),
  direction: z.lazy(() => MetricRelationshipDirectionSchema).optional(),
  note: z.string().optional(),
}).strict())

export const MetricRelationshipDirectionSchema: z.ZodType<Contract.MetricRelationshipDirection> = z.enum(["bidirectional", "source_to_target", "target_to_source"])

export const MetricRelationshipEndSchema: z.ZodType<Contract.MetricRelationshipEnd> = z.object({
  key: z.string(),
  label: z.string(),
  confidence: z.lazy(() => ConfidenceSchema).optional(),
  availability: z.lazy(() => AvailabilitySchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict()

export const MetricRelationshipTypeSchema: z.ZodType<Contract.MetricRelationshipType> = z.enum(["association", "derivation", "reconciliation"])

export const NodeSchema: z.ZodType<Contract.Node> = z.lazy(() => z.object({
  key: z.lazy(() => NodeKeySchema),
  path: z.lazy(() => NodePathSchema),
  label: z.string(),
  target: z.lazy(() => NodeKeySchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict())

export const NodeKeySchema: z.ZodType<Contract.NodeKey> = z.string()

export const NodePathSchema: z.ZodType<Contract.NodePath> = z.array(z.lazy(() => NodeKeySchema))

export const PanelSchema: z.ZodType<Contract.Panel> = z.lazy(() => z.object({
  id: z.string(),
  kind: z.lazy(() => PanelKindSchema),
  title: z.string(),
  semantics: z.lazy(() => SemanticsSchema),
  frame: z.lazy(() => FrameRefSchema),
  encoding: z.lazy(() => EncodingSchema),
  format: z.record(z.string(), z.lazy(() => FieldFormatSchema)),
  total: z.number().optional(),
  columns: z.array(z.lazy(() => TableColumnSchema)).optional(),
  table: z.lazy(() => TableOptionsSchema).optional(),
  drillRoot: z.lazy(() => NodeKeySchema).optional(),
  actions: z.array(z.lazy(() => ActionSchema)),
  accent: z.string().optional(),
  status: z.lazy(() => PanelStatusSchema).optional(),
  caption: z.string().optional(),
  info: z.string().optional(),
  headline: z.number().optional(),
  trend: z.lazy(() => PanelTrendSchema).optional(),
  sparkline: z.lazy(() => SparklineSchema).optional(),
  target: z.lazy(() => PanelTargetSchema).optional(),
  temporal: z.lazy(() => PanelTemporalSchema).optional(),
  presentation: z.lazy(() => PresentationSchema).optional(),
  valueAxis: z.lazy(() => ValueAxisSchema).optional(),
  metricFlow: z.lazy(() => MetricFlowConfigSchema).optional(),
  metricHierarchy: z.lazy(() => MetricHierarchyConfigSchema).optional(),
  metricRelationship: z.lazy(() => MetricRelationshipConfigSchema).optional(),
  radial: z.lazy(() => RadialConfigSchema).optional(),
  map: z.lazy(() => MapConfigSchema).optional(),
  confidence: z.lazy(() => ConfidenceSchema).optional(),
  availability: z.lazy(() => AvailabilitySchema).optional(),
  deferred: z.boolean().optional(),
  terminal: z.boolean().optional(),
  comparisonUnsupported: z.boolean().optional(),
}).strict())

export const PanelBatchRequestSchema: z.ZodType<Contract.PanelBatchRequest> = z.lazy(() => z.object({
  snapshotId: z.string(),
  panels: z.array(z.lazy(() => PanelRequestSchema)),
}).strict())

export const PanelBatchResponseSchema: z.ZodType<Contract.PanelBatchResponse> = z.lazy(() => z.object({
  panels: z.record(z.string(), z.lazy(() => PanelBatchResultSchema)),
}).strict())

export const PanelBatchResultSchema: z.ZodType<Contract.PanelBatchResult> = z.lazy(() => z.object({
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)).optional(),
  calculation: z.lazy(() => PanelCalculationSchema).optional(),
  summary: z.lazy(() => TableSummarySchema).optional(),
  page: z.lazy(() => QueryPageSchema).optional(),
  error: z.lazy(() => QueryErrorResponseSchema).optional(),
}).strict())

export const PanelBatchStreamEventSchema: z.ZodType<Contract.PanelBatchStreamEvent> = z.object({
  panelId: z.string().optional(),
  result: z.lazy(() => PanelBatchResultSchema).optional(),
  complete: z.boolean().optional(),
}).strict()

export const PanelCalculationSchema: z.ZodType<Contract.PanelCalculation> = z.object({
  durationMs: z.number().int(),
  cacheHit: z.boolean(),
  calculatedAt: z.string().datetime({ offset: true }),
}).strict()

export const PanelKindSchema: z.ZodType<Contract.PanelKind> = z.enum(["area", "bar", "boxplot", "cascade", "coverage", "donut", "gauge", "hbar", "heatmap", "histogram", "line", "map", "metric_flow", "metric_hierarchy", "metric_relationship", "pie", "radial", "stat", "table"])

export const PanelRequestSchema: z.ZodType<Contract.PanelRequest> = z.lazy(() => z.object({
  panelId: z.string(),
  recompute: z.boolean().optional(),
  search: z.string().optional(),
  sort: z.lazy(() => TableSortSchema).optional(),
  page: z.number().int().optional(),
}).strict())

export const PanelResponseSchema: z.ZodType<Contract.PanelResponse> = z.lazy(() => z.object({
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  calculation: z.lazy(() => PanelCalculationSchema),
  summary: z.lazy(() => TableSummarySchema).optional(),
  page: z.lazy(() => QueryPageSchema).optional(),
}).strict())

export const PanelStatusSchema: z.ZodType<Contract.PanelStatus> = z.lazy(() => z.object({
  label: z.string(),
  tone: z.lazy(() => StatusToneSchema).optional(),
}).strict())

export const PanelTargetSchema: z.ZodType<Contract.PanelTarget> = z.object({
  value: z.number(),
  label: z.string().optional(),
}).strict()

export const PanelTemporalSchema: z.ZodType<Contract.PanelTemporal> = z.lazy(() => z.object({
  regression: z.lazy(() => TemporalSeriesSchema).optional(),
  movingAverages: z.array(z.lazy(() => TemporalMovingAverageSchema)).optional(),
  referenceLines: z.array(z.lazy(() => PanelTargetSchema)).optional(),
  period: z.lazy(() => TemporalPeriodSchema).optional(),
  annotations: z.array(z.lazy(() => TemporalAnnotationSchema)).optional(),
  forecast: z.lazy(() => TemporalForecastSchema).optional(),
}).strict())

export const PanelTrendSchema: z.ZodType<Contract.PanelTrend> = z.lazy(() => z.object({
  percent: z.number(),
  label: z.string().optional(),
  invert: z.boolean().optional(),
  absoluteField: z.string().optional(),
  percentField: z.string().optional(),
  absoluteDeltaUnit: z.lazy(() => TrendDeltaUnitSchema).optional(),
}).strict())

export const PeriodFilterSchema: z.ZodType<Contract.PeriodFilter> = z.lazy(() => z.object({
  startParam: z.string(),
  endParam: z.string(),
  value: z.lazy(() => PeriodValueSchema),
  min: z.string().optional(),
  max: z.string().optional(),
  allowEmpty: z.boolean().optional(),
  presets: z.array(z.lazy(() => PeriodPresetSchema)).optional(),
}).strict())

export const PeriodPresetSchema: z.ZodType<Contract.PeriodPreset> = z.lazy(() => z.object({
  id: z.string(),
  label: z.string(),
  value: z.lazy(() => PeriodValueSchema),
}).strict())

export const PeriodValueSchema: z.ZodType<Contract.PeriodValue> = z.object({
  start: z.string(),
  end: z.string(),
}).strict()

export const PerspectiveSchema: z.ZodType<Contract.Perspective> = z.lazy(() => z.object({
  id: z.string(),
  explorerId: z.string(),
  branchKey: z.lazy(() => NodeKeySchema),
  key: z.string(),
  label: z.string(),
  semantics: z.lazy(() => SemanticsSchema),
  root: z.lazy(() => NodeKeySchema),
}).strict())

export const PerspectiveRefSchema: z.ZodType<Contract.PerspectiveRef> = z.object({
  id: z.string(),
}).strict()

export const PresentationSchema: z.ZodType<Contract.Presentation> = z.lazy(() => z.object({
  dataLabels: z.boolean().optional(),
  legend: z.lazy(() => LegendPlacementSchema).optional(),
  legendValue: z.lazy(() => LegendValueSchema).optional(),
  sliceLabels: z.lazy(() => SliceLabelsSchema).optional(),
  totalBadge: z.lazy(() => TotalBadgePlacementSchema).optional(),
  colorBy: z.lazy(() => ColorBySchema).optional(),
  fill: z.boolean().optional(),
  barWidthPx: z.number().int().optional(),
  bridgeLayout: z.lazy(() => BridgeLayoutSchema).optional(),
  sortable: z.boolean().optional(),
  expandable: z.boolean().optional(),
  exportable: z.boolean().optional(),
  rowGroupField: z.string().optional(),
  focus: z.lazy(() => FocusModeSchema).optional(),
  stack: z.boolean().optional(),
  lineSeries: z.array(z.string()).optional(),
}).strict())

export const QueryErrorCodeSchema: z.ZodType<Contract.QueryErrorCode> = z.enum(["bad_request", "internal", "snapshot_gone"])

export const QueryErrorResponseSchema: z.ZodType<Contract.QueryErrorResponse> = z.object({
  error: z.lazy(() => QueryErrorCodeSchema),
  message: z.string(),
}).strict()

export const QueryPageSchema: z.ZodType<Contract.QueryPage> = z.object({
  number: z.number().int(),
  size: z.number().int(),
  hasNext: z.boolean().optional(),
}).strict()

export const QueryRequestSchema: z.ZodType<Contract.QueryRequest> = z.lazy(() => z.object({
  snapshotId: z.string(),
  path: z.lazy(() => NodePathSchema),
  perspective: z.string().optional(),
  page: z.number().int().optional(),
  sort: z.lazy(() => TableSortSchema).optional(),
}).strict())

export const QueryResponseSchema: z.ZodType<Contract.QueryResponse> = z.object({
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  page: z.lazy(() => QueryPageSchema).optional(),
}).strict()

export const RadialConfigSchema: z.ZodType<Contract.RadialConfig> = z.lazy(() => z.object({
  mode: z.lazy(() => RadialModeSchema),
  max: z.number().optional(),
  rings: z.array(z.lazy(() => RadialRingSchema)).optional(),
  tolerance: z.number().optional(),
}).strict())

export const RadialModeSchema: z.ZodType<Contract.RadialMode> = z.enum(["partition", "progress"])

export const RadialRingSchema: z.ZodType<Contract.RadialRing> = z.object({
  key: z.string(),
  label: z.string(),
  order: z.number().int().optional(),
  total: z.number(),
}).strict()

export const SemanticsSchema: z.ZodType<Contract.Semantics> = z.enum(["evidence", "partition", "reconciliation", "series"])

export const SliceLabelsSchema: z.ZodType<Contract.SliceLabels> = z.enum(["label", "percent"])

export const SortDirectionSchema: z.ZodType<Contract.SortDirection> = z.enum(["asc", "desc"])

export const SourceSchema: z.ZodType<Contract.Source> = z.lazy(() => z.object({
  kind: z.lazy(() => ValueSourceKindSchema),
  name: z.string().optional(),
  value: z.unknown().optional(),
  fallback: z.unknown().optional(),
}).strict())

export const SparklineSchema: z.ZodType<Contract.Sparkline> = z.object({
  values: z.array(z.number()),
  color: z.string().optional(),
}).strict()

export const StatusToneSchema: z.ZodType<Contract.StatusTone> = z.enum(["neutral", "positive", "warning"])

export const TableAffordanceSchema: z.ZodType<Contract.TableAffordance> = z.enum(["pill", "quiet"])

export const TableAlignSchema: z.ZodType<Contract.TableAlign> = z.enum(["left", "right"])

export const TableCellSchema: z.ZodType<Contract.TableCell> = z.lazy(() => z.object({
  kind: z.lazy(() => TableCellKindSchema),
  secondaryField: z.string().optional(),
  layout: z.lazy(() => TableCellLayoutSchema).optional(),
  toneField: z.string().optional(),
}).strict())

export const TableCellKindSchema: z.ZodType<Contract.TableCellKind> = z.enum(["bar", "delta", "plain", "underline"])

export const TableCellLayoutSchema: z.ZodType<Contract.TableCellLayout> = z.enum(["stacked"])

export const TableColumnSchema: z.ZodType<Contract.TableColumn> = z.object({
  field: z.string(),
  label: z.string(),
  align: z.lazy(() => TableAlignSchema).optional(),
  cell: z.lazy(() => TableCellSchema),
  action: z.lazy(() => ActionSchema).optional(),
  text: z.string().optional(),
  widthPx: z.number().int().optional(),
  clamp: z.number().int().optional(),
  affordance: z.lazy(() => TableAffordanceSchema).optional(),
  badgeField: z.string().optional(),
  heat: z.boolean().optional(),
  sampleSizeField: z.string().optional(),
  minSampleSize: z.number().int().optional(),
  total: z.boolean().optional(),
  shareOf: z.string().optional(),
}).strict()

export const TableOptionsSchema: z.ZodType<Contract.TableOptions> = z.object({
  searchable: z.boolean().optional(),
}).strict()

export const TableSortSchema: z.ZodType<Contract.TableSort> = z.object({
  field: z.string(),
  direction: z.lazy(() => SortDirectionSchema),
}).strict()

export const TableSummarySchema: z.ZodType<Contract.TableSummary> = z.object({
  values: z.record(z.string(), z.unknown()),
  fullValues: z.record(z.string(), z.unknown()).optional(),
  filteredRows: z.number().int(),
  totalRows: z.number().int(),
}).strict()

export const TemporalAnnotationSchema: z.ZodType<Contract.TemporalAnnotation> = z.object({
  at: z.string(),
  label: z.string(),
}).strict()

export const TemporalForecastSchema: z.ZodType<Contract.TemporalForecast> = z.object({
  start: z.string(),
  valueField: z.string(),
  lowerField: z.string(),
  upperField: z.string(),
  label: z.string().optional(),
}).strict()

export const TemporalMovingAverageSchema: z.ZodType<Contract.TemporalMovingAverage> = z.object({
  window: z.number().int(),
  field: z.string(),
  label: z.string().optional(),
}).strict()

export const TemporalPeriodSchema: z.ZodType<Contract.TemporalPeriod> = z.lazy(() => z.object({
  category: z.string(),
  state: z.lazy(() => TemporalPeriodStateSchema),
  label: z.string().optional(),
  annualizedField: z.string().optional(),
}).strict())

export const TemporalPeriodStateSchema: z.ZodType<Contract.TemporalPeriodState> = z.enum(["annualized", "ytd"])

export const TemporalSeriesSchema: z.ZodType<Contract.TemporalSeries> = z.object({
  field: z.string(),
  label: z.string().optional(),
}).strict()

export const ThemeSchema: z.ZodType<Contract.Theme> = z.object({
  palette: z.record(z.string(), z.string()),
  series: z.record(z.string(), z.string()),
  debounceMs: z.number().int().optional(),
}).strict()

export const TotalBadgePlacementSchema: z.ZodType<Contract.TotalBadgePlacement> = z.enum(["header", "none", "plot"])

export const TrendDeltaUnitSchema: z.ZodType<Contract.TrendDeltaUnit> = z.enum(["percentage_points", "value"])

export const URLStateContractSchema: z.ZodType<Contract.URLStateContract> = z.object({
  version: z.number().int(),
  param: z.string(),
  maxBytes: z.number().int(),
}).strict()

export const ValueAxisSchema: z.ZodType<Contract.ValueAxis> = z.object({
  scale: z.lazy(() => AxisScaleSchema),
  logBase: z.number().int().optional(),
}).strict()

export const ValueSourceKindSchema: z.ZodType<Contract.ValueSourceKind> = z.enum(["field", "literal", "variable"])

const DocumentVersionSchema = z.object({ version: z.string() }).passthrough()

function panelIsActionable(panel: Contract.Panel): boolean {
  return Boolean(
    panel.drillRoot || panel.actions.length > 0 || panel.columns?.some((column) => column.action) ||
    panel.metricFlow?.stages.some((stage) => stage.action) ||
    panel.metricHierarchy?.rows.some((row) => row.action) ||
    panel.metricRelationship?.source.action || panel.metricRelationship?.target.action
  )
}

function assertPanelInteractionContract(document: Contract.DashboardDocument): void {
  document.panels.forEach((panel, index) => {
    const actionable = panelIsActionable(panel)
    if (panel.terminal && actionable) {
      throw new z.ZodError([{ code: 'custom', path: ['panels', index], message: 'panel ' + panel.id + ' cannot be terminal and actionable' }])
    }
    if (!panel.terminal && !actionable) {
      throw new z.ZodError([{ code: 'custom', path: ['panels', index], message: 'panel ' + panel.id + ' must be actionable or explicitly terminal' }])
    }
  })
}

export function parseDocument(input: unknown): Contract.DashboardDocument {
  const version = DocumentVersionSchema.safeParse(input)
  if (version.success && contractMajor(version.data.version) !== CONTRACT_MAJOR_VERSION) {
    throw new ContractVersionMismatchError(version.data.version)
  }
  const document = DashboardDocumentSchema.parse(input)
  assertPanelInteractionContract(document)
  return document
}
