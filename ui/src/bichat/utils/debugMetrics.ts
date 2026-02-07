import type { DebugUsage } from '../types'

export function formatGenerationDuration(generationMs: number): string {
  return generationMs > 1000 ? `${(generationMs / 1000).toFixed(2)}s` : `${generationMs}ms`
}

export function formatDuration(durationMs: number): string {
  return durationMs > 1000 ? `${(durationMs / 1000).toFixed(2)}s` : `${durationMs}ms`
}

export function calculateCompletionTokensPerSecond(
  usage?: DebugUsage,
  generationMs?: number
): number | null {
  if (!usage || !generationMs || generationMs <= 0 || usage.completionTokens <= 0) {
    return null
  }

  return usage.completionTokens / (generationMs / 1000)
}

export function calculateContextUsagePercent(
  promptTokens: number,
  effectiveMaxTokens?: number
): number | null {
  if (!effectiveMaxTokens || effectiveMaxTokens <= 0 || promptTokens <= 0) {
    return null
  }

  return (promptTokens / effectiveMaxTokens) * 100
}
