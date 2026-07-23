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
  event: z.string().optional(),
  params: z.array(z.lazy(() => ActionParamSchema)),
  payload: z.record(z.string(), z.lazy(() => SourceSchema)),
  preserveQuery: z.boolean().optional(),
}).strict())

export const ActionKindSchema: z.ZodType<Contract.ActionKind> = z.enum(["emit_event", "navigate", "navigate_to_leaf", "open_drawer"])

export const ActionParamSchema: z.ZodType<Contract.ActionParam> = z.lazy(() => z.object({
  name: z.string(),
  source: z.lazy(() => SourceSchema),
}).strict())

export const ColorBySchema: z.ZodType<Contract.ColorBy> = z.enum(["category"])

export const ColumnSchema: z.ZodType<Contract.Column> = z.lazy(() => z.object({
  name: z.string(),
  type: z.lazy(() => ColumnTypeSchema),
}).strict())

export const ColumnTypeSchema: z.ZodType<Contract.ColumnType> = z.enum(["bool", "number", "string", "time"])

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
  endpoints: z.lazy(() => EndpointsSchema),
  i18n: z.record(z.string(), z.string()),
  theme: z.lazy(() => ThemeSchema),
  header: z.lazy(() => DocumentHeaderSchema).optional(),
  drawer: z.lazy(() => DrawerHeaderSchema).optional(),
}).strict())

export const DocumentHeaderSchema: z.ZodType<Contract.DocumentHeader> = z.object({
  title: z.string().optional(),
  subtitle: z.string().optional(),
}).strict()

export const DrawerHeaderSchema: z.ZodType<Contract.DrawerHeader> = z.object({
  eyebrow: z.string().optional(),
  title: z.string().optional(),
  caption: z.string().optional(),
}).strict()

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
  id: z.string().optional(),
  series: z.string().optional(),
  category: z.string().optional(),
  cut: z.string().optional(),
  cutLabel: z.string().optional(),
  final: z.string().optional(),
}).strict()

export const EndpointsSchema: z.ZodType<Contract.Endpoints> = z.object({
  query: z.string().optional(),
  export: z.string().optional(),
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
}).strict())

export const FilterKindSchema: z.ZodType<Contract.FilterKind> = z.enum(["period"])

export const FormatKindSchema: z.ZodType<Contract.FormatKind> = z.enum(["date", "money", "number", "percent", "string"])

export const FrameSchema: z.ZodType<Contract.Frame> = z.lazy(() => z.object({
  columns: z.array(z.lazy(() => ColumnSchema)),
  rows: z.array(z.array(z.unknown())),
  children: z.array(z.lazy(() => NodeSchema)).optional(),
}).strict())

export const FrameRefSchema: z.ZodType<Contract.FrameRef> = z.string()

export const LayoutSchema: z.ZodType<Contract.Layout> = z.lazy(() => z.object({
  rows: z.array(z.lazy(() => LayoutRowSchema)),
}).strict())

export const LayoutGroupSchema: z.ZodType<Contract.LayoutGroup> = z.lazy(() => z.object({
  id: z.string(),
  kind: z.lazy(() => LayoutGroupKindSchema),
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
  group: z.lazy(() => LayoutGroupSchema).optional(),
}).strict()

export const LayoutRowSchema: z.ZodType<Contract.LayoutRow> = z.object({
  heading: z.string().optional(),
  class: z.string().optional(),
  panels: z.array(z.lazy(() => LayoutItemSchema)),
}).strict()

export const LegendPlacementSchema: z.ZodType<Contract.LegendPlacement> = z.enum(["below"])

export const LevelSchema: z.ZodType<Contract.Level> = z.lazy(() => z.object({
  path: z.lazy(() => NodePathSchema),
  label: z.string(),
  children: z.array(z.lazy(() => NodeSchema)),
  dynamicChildren: z.lazy(() => DynamicChildrenSchema).optional(),
  frame: z.lazy(() => FrameRefSchema).optional(),
  encoding: z.lazy(() => EncodingSchema).optional(),
  perspectives: z.array(z.lazy(() => PerspectiveRefSchema)),
}).strict())

export const MetaSchema: z.ZodType<Contract.Meta> = z.object({
  dashboardId: z.string(),
  title: z.string(),
  generatedAt: z.string().datetime({ offset: true }),
  locale: z.string(),
}).strict()

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
  drillRoot: z.lazy(() => NodeKeySchema).optional(),
  actions: z.array(z.lazy(() => ActionSchema)),
  accent: z.string().optional(),
  status: z.lazy(() => PanelStatusSchema).optional(),
  caption: z.string().optional(),
  headline: z.number().optional(),
  trend: z.lazy(() => PanelTrendSchema).optional(),
  presentation: z.lazy(() => PresentationSchema).optional(),
}).strict())

export const PanelKindSchema: z.ZodType<Contract.PanelKind> = z.enum(["area", "bar", "cascade", "coverage", "donut", "hbar", "line", "pie", "stat", "table"])

export const PanelStatusSchema: z.ZodType<Contract.PanelStatus> = z.lazy(() => z.object({
  label: z.string(),
  tone: z.lazy(() => StatusToneSchema).optional(),
}).strict())

export const PanelTrendSchema: z.ZodType<Contract.PanelTrend> = z.object({
  percent: z.number(),
  label: z.string().optional(),
  invert: z.boolean().optional(),
}).strict()

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
  legend: z.lazy(() => LegendPlacementSchema).optional(),
  sliceLabels: z.lazy(() => SliceLabelsSchema).optional(),
  totalBadge: z.lazy(() => TotalBadgePlacementSchema).optional(),
  colorBy: z.lazy(() => ColorBySchema).optional(),
  fill: z.boolean().optional(),
  barWidthPx: z.number().int().optional(),
  sortable: z.boolean().optional(),
  expandable: z.boolean().optional(),
  exportable: z.boolean().optional(),
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

export const QueryRequestSchema: z.ZodType<Contract.QueryRequest> = z.object({
  snapshotId: z.string(),
  path: z.lazy(() => NodePathSchema),
  perspective: z.string().optional(),
  page: z.number().int().optional(),
}).strict()

export const QueryResponseSchema: z.ZodType<Contract.QueryResponse> = z.object({
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  page: z.lazy(() => QueryPageSchema).optional(),
}).strict()

export const SemanticsSchema: z.ZodType<Contract.Semantics> = z.enum(["evidence", "partition", "reconciliation", "series"])

export const SliceLabelsSchema: z.ZodType<Contract.SliceLabels> = z.enum(["percent"])

export const SourceSchema: z.ZodType<Contract.Source> = z.lazy(() => z.object({
  kind: z.lazy(() => ValueSourceKindSchema),
  name: z.string().optional(),
  value: z.unknown().optional(),
  fallback: z.unknown().optional(),
}).strict())

export const StatusToneSchema: z.ZodType<Contract.StatusTone> = z.enum(["neutral", "positive", "warning"])

export const TableAffordanceSchema: z.ZodType<Contract.TableAffordance> = z.enum(["pill"])

export const TableAlignSchema: z.ZodType<Contract.TableAlign> = z.enum(["left", "right"])

export const TableCellSchema: z.ZodType<Contract.TableCell> = z.lazy(() => z.object({
  kind: z.lazy(() => TableCellKindSchema),
  secondaryField: z.string().optional(),
  layout: z.lazy(() => TableCellLayoutSchema).optional(),
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
}).strict()

export const ThemeSchema: z.ZodType<Contract.Theme> = z.object({
  palette: z.record(z.string(), z.string()),
  series: z.record(z.string(), z.string()),
}).strict()

export const TotalBadgePlacementSchema: z.ZodType<Contract.TotalBadgePlacement> = z.enum(["header", "none", "plot"])

export const ValueSourceKindSchema: z.ZodType<Contract.ValueSourceKind> = z.enum(["field", "literal", "variable"])

const DocumentVersionSchema = z.object({ version: z.string() }).passthrough()

export function parseDocument(input: unknown): Contract.DashboardDocument {
  const version = DocumentVersionSchema.safeParse(input)
  if (version.success && contractMajor(version.data.version) !== CONTRACT_MAJOR_VERSION) {
    throw new ContractVersionMismatchError(version.data.version)
  }
  return DashboardDocumentSchema.parse(input)
}
