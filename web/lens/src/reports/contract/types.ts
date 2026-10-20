// GENERATED — do not edit

export const CONTRACT_VERSION = "1"

export interface Capabilities {
  operationDimensions?: Record<string, Array<string>>
  dimensions?: Array<string>
  viewDimensions?: Record<string, Array<string>>
  filters: Array<string>
  views: Array<string>
  drill: boolean
  save: boolean
  export: boolean
}

export interface CatalogEntry {
  owner?: CatalogOwner
  id: string
  revision: string
  title: string
  description?: string
  source?: string
  ownership: Ownership
  restriction?: string
  members?: number
  placed?: number
}

export interface CatalogOwner {
  id: string
  name: string
}

export interface CatalogPage {
  entries: Array<CatalogEntry>
  next?: string
}

export interface CatalogQuery {
  search?: string
  scope?: CatalogScope
  cursor?: string
  target?: string
  limit?: number
}

export type CatalogScope = "all" | "personal" | "shared"

export interface Composition {
  context?: ContextBoard
  id: string
  revision: string
  title: string
  parameters?: Parameters
  rows: Array<Array<Placement>>
}

export interface ContextBoard {
  calendar: ContextCalendar
  dashboard: ContextFilters
  cards?: Record<string, ContextFilters>
  sections?: Record<string, ContextFilters>
  groups?: Record<string, Array<ContextLayer>>
}

export interface ContextCalendar {
  timezone: string
  asOf: string
  defaultPeriod: ContextPeriod
}

export interface ContextConstraint {
  dimension: string
  values: Array<string>
}

export interface ContextFilters {
  period?: ContextPeriod
  constraints?: Array<ContextConstraint>
}

export interface ContextLayer {
  origin: ContextOrigin
  filters: ContextFilters
}

export interface ContextOrigin {
  scope: string
  id: string
}

export interface ContextPeriod {
  mode: string
  start?: string
  end?: string
  days?: number
  months?: number
}

export interface Definition {
  filters?: ContextFilters
  id: string
  revision: string
  title: string
  source: SourceRef
  view: string
  parameters?: Parameters
}

export interface Destination {
  id: string
  title: string
  placed?: number
}

export type Ownership = "personal" | "shared" | "system"

export type Parameters = Record<string, unknown>

export interface Placement {
  filters?: ContextFilters
  sectionId?: string
  id: string
  reportId: string
  title?: string
  span?: number
  parameters?: Parameters
}

export interface Proposal {
  id: string
  base: string
  summary?: string
  composition: Composition
}

export interface SourceDescriptor {
  id: string
  version: number
  revision: string
  capabilities: Capabilities
}

export interface SourceRef {
  id: string
  version: number
}

export type VersionStatus = "current" | "incompatible" | "update-available"
