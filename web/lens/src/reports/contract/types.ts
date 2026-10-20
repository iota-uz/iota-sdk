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
