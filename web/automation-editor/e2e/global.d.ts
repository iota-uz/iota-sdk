import type { AutomationGraph, AutomationLayout } from '../src/types'

declare global {
  interface Window {
    __fixture: {
      saves: { base: number; graph: AutomationGraph }[]
      saveLayouts: AutomationLayout[]
      saveLayoutCalls: number
      openedVersions: number[]
      initialLayout: AutomationLayout
      validates: number
      moves: AutomationLayout[]
      changes: AutomationGraph[]
      reloads: number
      runCalls: number
      runsCalls: { state?: string[]; subject?: string; node?: string; cursor?: string }[]
      ops: string[]
      dryRuns: unknown[]
      tests: unknown[]
      graph: AutomationGraph
      layout: AutomationLayout
      mountMs?: number
      failNext(kind: string): void
      recover(kind: string): void
      setGraph?(graph: AutomationGraph): void
      replace?(): number
    }
  }
}
