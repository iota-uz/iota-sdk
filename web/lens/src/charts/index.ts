import type { ChartAdapter } from './adapter'

export type * from './adapter'

export async function getChartAdapter(): Promise<ChartAdapter> {
  const { echartsAdapter } = await import('./echarts/adapter')
  return echartsAdapter
}
