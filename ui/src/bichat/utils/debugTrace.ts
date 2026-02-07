import type { ConversationTurn, DebugTrace, SessionDebugUsage } from '../types'

export function hasMeaningfulUsage(trace?: DebugTrace['usage']): boolean {
  if (!trace) return false
  return (
    trace.promptTokens > 0 ||
    trace.completionTokens > 0 ||
    trace.totalTokens > 0 ||
    (trace.cachedTokens ?? 0) > 0 ||
    (trace.cost ?? 0) > 0
  )
}

export function hasDebugTrace(trace: DebugTrace): boolean {
  return trace.tools.length > 0 || hasMeaningfulUsage(trace.usage) || !!trace.generationMs
}

export function getSessionDebugUsage(turns: ConversationTurn[]): SessionDebugUsage {
  let promptTokens = 0
  let completionTokens = 0
  let totalTokens = 0
  let turnsWithUsage = 0
  let latestPromptTokens = 0
  let latestCompletionTokens = 0
  let latestTotalTokens = 0

  for (const turn of turns) {
    const usage = turn.assistantTurn?.debug?.usage
    if (!hasMeaningfulUsage(usage) || !usage) {
      continue
    }

    turnsWithUsage++
    promptTokens += usage.promptTokens
    completionTokens += usage.completionTokens
    totalTokens += usage.totalTokens
    latestPromptTokens = usage.promptTokens
    latestCompletionTokens = usage.completionTokens
    latestTotalTokens = usage.totalTokens
  }

  return {
    promptTokens,
    completionTokens,
    totalTokens,
    turnsWithUsage,
    latestPromptTokens,
    latestCompletionTokens,
    latestTotalTokens,
  }
}

