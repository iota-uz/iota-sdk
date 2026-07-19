// GENERATED — do not edit

import { z } from 'zod'
import { CONTRACT_VERSION } from './types'
import type * as Contract from './types'

const CONTRACT_MAJOR_VERSION = CONTRACT_VERSION.split('.', 1)[0] ?? CONTRACT_VERSION

function contractMajor(version: string): string {
  return version.split('.', 1)[0] ?? version
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
  event: z.string().optional(),
  params: z.array(z.lazy(() => ActionParamSchema)),
  payload: z.record(z.string(), z.lazy(() => SourceSchema)),
  preserveQuery: z.boolean().optional(),
}).strict())

export const ActionKindSchema: z.ZodType<Contract.ActionKind> = z.lazy(() => z.enum(["emit_event", "navigate", "navigate_to_leaf"]))

export const ActionParamSchema: z.ZodType<Contract.ActionParam> = z.lazy(() => z.object({
  name: z.string(),
  source: z.lazy(() => SourceSchema),
}).strict())

export const ColumnSchema: z.ZodType<Contract.Column> = z.lazy(() => z.object({
  name: z.string(),
  type: z.lazy(() => ColumnTypeSchema),
}).strict())

export const ColumnTypeSchema: z.ZodType<Contract.ColumnType> = z.lazy(() => z.enum(["bool", "number", "string", "time"]))

export const DashboardDocumentSchema: z.ZodType<Contract.DashboardDocument> = z.lazy(() => z.object({
  version: ContractVersionSchema,
  snapshotId: z.string(),
  meta: z.lazy(() => MetaSchema),
  layout: z.lazy(() => LayoutSchema),
  panels: z.array(z.lazy(() => PanelSchema)),
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  drill: z.lazy(() => DrillSchema),
  perspectives: z.array(z.lazy(() => PerspectiveSchema)),
  endpoints: z.lazy(() => EndpointsSchema),
  i18n: z.record(z.string(), z.string()),
  theme: z.lazy(() => ThemeSchema),
}).strict())

export const DrillSchema: z.ZodType<Contract.Drill> = z.lazy(() => z.object({
  edges: z.record(z.lazy(() => NodeKeySchema), z.lazy(() => LevelSchema)),
  inlineDepth: z.number().int(),
}).strict())

export const EncodingSchema: z.ZodType<Contract.Encoding> = z.lazy(() => z.object({
  label: z.string().optional(),
  value: z.string().optional(),
  id: z.string().optional(),
  series: z.string().optional(),
  category: z.string().optional(),
  cut: z.string().optional(),
  cutLabel: z.string().optional(),
  final: z.string().optional(),
}).strict())

export const EndpointsSchema: z.ZodType<Contract.Endpoints> = z.lazy(() => z.object({
  query: z.string().optional(),
  export: z.string().optional(),
}).strict())

export const FieldFormatSchema: z.ZodType<Contract.FieldFormat> = z.lazy(() => z.object({
  kind: z.lazy(() => FormatKindSchema),
  currency: z.string().optional(),
  minorUnits: z.boolean(),
  precision: z.number().int().optional(),
  layout: z.string().optional(),
}).strict())

export const FormatKindSchema: z.ZodType<Contract.FormatKind> = z.lazy(() => z.enum(["date", "money", "number", "percent", "string"]))

export const FrameSchema: z.ZodType<Contract.Frame> = z.lazy(() => z.object({
  columns: z.array(z.lazy(() => ColumnSchema)),
  rows: z.array(z.array(z.unknown())),
}).strict())

export const FrameRefSchema: z.ZodType<Contract.FrameRef> = z.lazy(() => z.string())

export const LayoutSchema: z.ZodType<Contract.Layout> = z.lazy(() => z.object({
  rows: z.array(z.lazy(() => LayoutRowSchema)),
}).strict())

export const LayoutItemSchema: z.ZodType<Contract.LayoutItem> = z.lazy(() => z.object({
  panelId: z.string(),
  span: z.number().int(),
}).strict())

export const LayoutRowSchema: z.ZodType<Contract.LayoutRow> = z.lazy(() => z.object({
  heading: z.string().optional(),
  class: z.string().optional(),
  panels: z.array(z.lazy(() => LayoutItemSchema)),
}).strict())

export const LevelSchema: z.ZodType<Contract.Level> = z.lazy(() => z.object({
  path: z.lazy(() => NodePathSchema),
  label: z.string(),
  children: z.array(z.lazy(() => NodeSchema)),
  frame: z.lazy(() => FrameRefSchema).optional(),
  encoding: z.lazy(() => EncodingSchema).optional(),
  perspectives: z.array(z.lazy(() => PerspectiveRefSchema)),
}).strict())

export const MetaSchema: z.ZodType<Contract.Meta> = z.lazy(() => z.object({
  dashboardId: z.string(),
  title: z.string(),
  generatedAt: z.string().datetime({ offset: true }),
  locale: z.string(),
}).strict())

export const NodeSchema: z.ZodType<Contract.Node> = z.lazy(() => z.object({
  key: z.lazy(() => NodeKeySchema),
  path: z.lazy(() => NodePathSchema),
  label: z.string(),
  target: z.lazy(() => NodeKeySchema).optional(),
  action: z.lazy(() => ActionSchema).optional(),
}).strict())

export const NodeKeySchema: z.ZodType<Contract.NodeKey> = z.lazy(() => z.string())

export const NodePathSchema: z.ZodType<Contract.NodePath> = z.lazy(() => z.array(z.lazy(() => NodeKeySchema)))

export const PanelSchema: z.ZodType<Contract.Panel> = z.lazy(() => z.object({
  id: z.string(),
  kind: z.lazy(() => PanelKindSchema),
  title: z.string(),
  semantics: z.lazy(() => SemanticsSchema),
  frame: z.lazy(() => FrameRefSchema),
  encoding: z.lazy(() => EncodingSchema),
  format: z.record(z.string(), z.lazy(() => FieldFormatSchema)),
  drillRoot: z.lazy(() => NodeKeySchema).optional(),
  actions: z.array(z.lazy(() => ActionSchema)),
}).strict())

export const PanelKindSchema: z.ZodType<Contract.PanelKind> = z.lazy(() => z.enum(["area", "bar", "cascade", "donut", "hbar", "line", "pie", "stat", "table"]))

export const PerspectiveSchema: z.ZodType<Contract.Perspective> = z.lazy(() => z.object({
  id: z.string(),
  explorerId: z.string(),
  branchKey: z.lazy(() => NodeKeySchema),
  key: z.string(),
  label: z.string(),
  semantics: z.lazy(() => SemanticsSchema),
  root: z.lazy(() => NodeKeySchema),
}).strict())

export const PerspectiveRefSchema: z.ZodType<Contract.PerspectiveRef> = z.lazy(() => z.object({
  id: z.string(),
}).strict())

export const QueryErrorCodeSchema: z.ZodType<Contract.QueryErrorCode> = z.lazy(() => z.enum(["bad_request", "internal", "snapshot_gone"]))

export const QueryErrorResponseSchema: z.ZodType<Contract.QueryErrorResponse> = z.lazy(() => z.object({
  error: z.string(),
  message: z.string(),
}).strict())

export const QueryPageSchema: z.ZodType<Contract.QueryPage> = z.lazy(() => z.object({
  number: z.number().int(),
  size: z.number().int(),
}).strict())

export const QueryRequestSchema: z.ZodType<Contract.QueryRequest> = z.lazy(() => z.object({
  snapshotId: z.string(),
  path: z.lazy(() => NodePathSchema),
  perspective: z.string().optional(),
  page: z.number().int().optional(),
}).strict())

export const QueryResponseSchema: z.ZodType<Contract.QueryResponse> = z.lazy(() => z.object({
  frames: z.record(z.lazy(() => FrameRefSchema), z.lazy(() => FrameSchema)),
  page: z.lazy(() => QueryPageSchema).optional(),
}).strict())

export const SemanticsSchema: z.ZodType<Contract.Semantics> = z.lazy(() => z.enum(["evidence", "partition", "reconciliation", "series"]))

export const SourceSchema: z.ZodType<Contract.Source> = z.lazy(() => z.object({
  kind: z.lazy(() => ValueSourceKindSchema),
  name: z.string().optional(),
  value: z.unknown().optional(),
  fallback: z.unknown().optional(),
}).strict())

export const ThemeSchema: z.ZodType<Contract.Theme> = z.lazy(() => z.object({
  palette: z.record(z.string(), z.string()),
  series: z.record(z.string(), z.string()),
}).strict())

export const ValueSourceKindSchema: z.ZodType<Contract.ValueSourceKind> = z.lazy(() => z.enum(["field", "literal", "variable"]))

const DocumentVersionSchema = z.object({ version: z.string() }).passthrough()

export function parseDocument(input: unknown): Contract.DashboardDocument {
  const version = DocumentVersionSchema.safeParse(input)
  if (version.success && contractMajor(version.data.version) !== CONTRACT_MAJOR_VERSION) {
    throw new ContractVersionMismatchError(version.data.version)
  }
  return DashboardDocumentSchema.parse(input)
}
