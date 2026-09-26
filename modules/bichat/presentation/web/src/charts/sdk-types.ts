import type { Artifact } from '../rpc.generated'

export interface ChartDataSeries {
  name: string
  data: number[]
}

/** Shape previously provided by @iota-uz/sdk/bichat's ChartData. */
export interface ChartData {
  chartType: 'line' | 'area' | 'bar' | 'pie' | 'donut'
  title: string
  categories?: string[]
  series: ChartDataSeries[]
  colors?: string[]
  xaxis?: Record<string, unknown>
}

export type RichChartData = ChartData & {
  height?: number
  options?: Record<string, unknown>
  warnings?: string[]
  meta?: Record<string, unknown>
}

export interface AssistantTurnWithCharts {
  charts?: RichChartData[]
}

export type SessionArtifact = Artifact
