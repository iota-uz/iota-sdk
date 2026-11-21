import type { AutomationGraph, AutomationLayout } from '../src/types'

declare global {
  interface Window {
    __fixture: {
      saves: { base: number; graph: AutomationGraph }[]
      saveLayouts: AutomationLayout[]
      validates: number
      moves: AutomationLayout[]
      changes: AutomationGraph[]
      reloads: number
      graph: AutomationGraph
      layout: AutomationLayout
      mountMs?: number
      failNext(kind: string): void
      recover(kind: string): void
      setGraph?(graph: AutomationGraph): void
    }
  }
}
