// Manifest JSON and wire value types. Mirrors pkg/extruntime/manifest (Go) exactly:
// field names and omitempty semantics (false/0/""/[] are omitted, never emitted).

export type Json = null | boolean | number | string | readonly Json[] | { readonly [key: string]: Json | undefined }

/** A JSON value other than `null` (the wire rejects `null` for a `json` type unless it is `.nullable()`). */
export type JsonNotNull = Exclude<Json, null>

/**
 * `T` when every member of `T` is JSON, otherwise a type `T` is not assignable to. Unlike `T extends Json`
 * it accepts `interface`s (which have no implicit index signature) and ignores symbol-keyed phantom brands
 * such as the one on `Ref`. Use as `T extends JsonOf<T>`.
 */
export type JsonOf<T> = T extends string | number | boolean | null | undefined | void
  ? T
  : T extends bigint | symbol | ((...args: never[]) => unknown)
    ? never
    : T extends readonly (infer U)[]
      ? readonly JsonOf<U>[]
      : T extends object
        ? { [K in keyof T as K extends symbol ? never : K]: JsonOf<T[K]> }
        : never

declare const int64Brand: unique symbol
declare const decimalBrand: unique symbol
declare const dateBrand: unique symbol
declare const datetimeBrand: unique symbol

/** Decimal string of an integer within the signed 64-bit range. Never a JS number. */
export type Int64 = string & { readonly [int64Brand]: true }
/** Exact decimal string, e.g. "12.50". */
export type Decimal = string & { readonly [decimalBrand]: true }
/** "YYYY-MM-DD". */
export type ISODate = string & { readonly [dateBrand]: true }
/** RFC 3339 with offset. */
export type ISODateTime = string & { readonly [datetimeBrand]: true }

export type Label = Readonly<Record<string, string>>

export type TypeKind =
  | 'string' | 'int' | 'int64' | 'decimal' | 'bool' | 'date' | 'datetime'
  | 'uuid' | 'enum' | 'json' | 'ref' | 'object' | 'array'

export interface TypeJSON {
  kind: TypeKind
  optional?: true
  nullable?: true
  values?: string[]
  entity?: string
  properties?: PropertyJSON[]
  items?: TypeJSON
  precision?: number
  scale?: number
  maxLength?: number
}

export interface PropertyJSON {
  name: string
  type: TypeJSON
}

export type ResourceKind =
  | 'entity' | 'overlay' | 'query' | 'command' | 'operation' | 'event' | 'workflow'
  | 'notice' | 'route' | 'permission' | 'issue' | 'domainError' | 'column'
  | 'page' | 'navigation' | 'slot' | 'cell' | 'shell' | 'contactResolver' | 'deliveryPolicy'

export interface FieldJSON {
  id: string
  type: TypeJSON
  unique?: true
  index?: true
  default?: Json
  renamedFrom?: string[]
  label?: Label
}

export interface ConditionJSON {
  eq?: Json
  ne?: Json
  in?: Json[]
  lt?: Json
  lte?: Json
  gt?: Json
  gte?: Json
  isNull?: boolean
  like?: string
}

export interface FilterJSON {
  fields?: Record<string, ConditionJSON>
  or?: FilterJSON[]
}

export interface ReadRuleJSON {
  permission?: string
  all?: true
  rows?: FilterJSON
}

export interface AccessJSON {
  read?: ReadRuleJSON[]
  fields?: Record<string, string[]>
}

export interface NavJSON {
  label: Label
  group?: string
  parent?: string
  icon?: string
  order?: number
}

export interface NavPatchJSON {
  ref: string
  hide?: true
  rename?: Label
  parent?: string
  order?: number
}

export interface NavGroupJSON {
  id: string
  label: Label
  icon?: string
  order?: number
}

export interface WorkspaceJSON {
  id: string
  label: Label
  icon?: string
  order?: number
}

export interface PaletteEntryJSON {
  id: string
  label: Label
  target: string
  keywords?: string[]
}

export interface ResourceJSON {
  id: string
  kind: ResourceKind
  label?: Label
  description?: string
  fields?: FieldJSON[]
  base?: string
  access?: AccessJSON
  /** Native entities only: the type of the record key (package entities are always uuid). */
  keyType?: 'int64' | 'uuid' | 'string'
  atomic?: true
  input?: TypeJSON
  output?: TypeJSON
  errors?: string[]
  params?: TypeJSON
  payload?: TypeJSON
  subject?: string
  signals?: Record<string, TypeJSON>
  permission?: string
  path?: string
  route?: string
  nav?: NavJSON
  chunk?: string
  export?: string
  list?: string
  overlay?: string
  field?: string
  filterable?: true
  sortable?: true
  exportable?: true
  groups?: NavGroupJSON[]
  patches?: NavPatchJSON[]
  workspaces?: WorkspaceJSON[]
  palette?: PaletteEntryJSON[]
  /** Slot contribution: the slot ID; cell: the column ID. */
  target?: string
  /** Slot declaration: the typed props its host page passes to every contribution. */
  props?: TypeJSON
  /** Slot contribution: position among the contributions of the slot (ascending, default 0). */
  order?: number
}

export type ImplKind =
  | 'command' | 'atomic' | 'operation' | 'query' | 'workflow' | 'contactResolver' | 'deliveryPolicy'

export interface ImplementationJSON {
  export: string
  implements: string
  kind: ImplKind
}

export interface SubscriptionJSON {
  id: string
  export: string
  event: string
}

export type GuardPhase = 'input' | 'operation' | 'write'

export interface GuardJSON {
  id: string
  export: string
  target: string
  phase: GuardPhase
}

export type Channel = 'sms' | 'email' | 'inApp' | 'push'

export interface TemplateJSON {
  export: string
  notice: string
  channels: Channel[]
}

export interface ScheduleJSON {
  id: string
  export: string
  cron: string
  timezone?: string
}

export interface EndpointJSON {
  id: string
  export: string
  method: string
  path: string
  public?: true
}

export interface CapabilitiesJSON {
  network?: string[]
  secrets?: string[]
}

/** One document of a package in one locale (see build/docs.ts and manifest.Doc). */
export interface DocJSON {
  path: string
  locale: string
  title: string
  order?: number
  audience?: string
  permission?: string
}

export interface ManifestJSON {
  schema: 'iota.manifest/1'
  namespace: string
  resources: ResourceJSON[]
  implementations?: ImplementationJSON[]
  subscriptions?: SubscriptionJSON[]
  guards?: GuardJSON[]
  templates?: TemplateJSON[]
  schedules?: ScheduleJSON[]
  endpoints?: EndpointJSON[]
  capabilities?: CapabilitiesJSON
  docs?: DocJSON[]
  dependencies?: Record<string, string>
}

export const MANIFEST_SCHEMA = 'iota.manifest/1'
