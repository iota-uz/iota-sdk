import type { NodeKey } from '../../contract'

interface EChartsEvent {
  data?: unknown
}

export function nodeKeyFromEvent(event: EChartsEvent): NodeKey | undefined {
  if (!event.data || typeof event.data !== 'object') return undefined
  const nodeKey = (event.data as Record<string, unknown>).nodeKey
  return typeof nodeKey === 'string' && nodeKey !== '' ? nodeKey : undefined
}
