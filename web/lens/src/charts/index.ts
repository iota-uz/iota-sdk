import type { ChartAdapter, ChartInput } from './adapter'

export type * from './adapter'

interface EChartsAdapterModule {
  echartsAdapter: ChartAdapter
  prepareEChartsKind: (kind: ChartInput['kind']) => Promise<void>
}

export async function resolveChartAdapter(
  kind: ChartInput['kind'],
  load: () => Promise<EChartsAdapterModule> = () => import('./echarts/adapter'),
): Promise<ChartAdapter> {
  const module = await load()
  await module.prepareEChartsKind(kind)
  return module.echartsAdapter
}

export function getChartAdapter(kind: ChartInput['kind']): Promise<ChartAdapter> {
  return resolveChartAdapter(kind)
}
