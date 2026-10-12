// GENERATED — do not edit

export const CONTRACT_VERSION = "1"

export interface Capabilities {
  filters: Array<string>
  views: Array<string>
  drill: boolean
  save: boolean
  export: boolean
}

export interface Composition {
  id: string
  revision: string
  title: string
  parameters?: Parameters
  rows: Array<Array<Placement>>
}

export interface Definition {
  id: string
  revision: string
  title: string
  source: SourceRef
  view: string
  parameters?: Parameters
}

export type Parameters = Record<string, unknown>

export interface Placement {
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
