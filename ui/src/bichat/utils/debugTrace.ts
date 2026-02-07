/**
 * Debug trace utility functions
 * Extracted from ChatContext.tsx for reuse across components
 */

import type { DebugTrace, ToolCall, ConversationTurn, SessionDebugUsage } from '../types'

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

export function inferDebugTraceFromToolCalls(toolCalls?: ToolCall[]): DebugTrace | null {
  if (!toolCalls || toolCalls.length === 0) {
    return null
  }

  const tools = toolCalls
    .filter((call) => !!call.name)
    .map((call) => ({
      callId: call.id,
      name: call.name,
      arguments: call.arguments,
      result: call.result,
      error: call.error,
      durationMs: call.durationMs,
    }))

  if (tools.length === 0) {
    return null
  }

  return { tools }
}

export function hydrateDebugTraceFromToolCalls(turns: ConversationTurn[]): ConversationTurn[] {
  let changed = false
  const hydrated = turns.map((turn) => {
    const assistantTurn = turn.assistantTurn
    if (!assistantTurn) {
      return turn
    }

    const inferred = inferDebugTraceFromToolCalls(assistantTurn.toolCalls)
    if (!inferred) {
      return turn
    }

    if (!assistantTurn.debug) {
      changed = true
      return {
        ...turn,
        assistantTurn: {
          ...assistantTurn,
          debug: inferred,
        },
      }
    }

    if (assistantTurn.debug.tools.length > 0) {
      return turn
    }

    changed = true
    return {
      ...turn,
      assistantTurn: {
        ...assistantTurn,
        debug: {
          ...assistantTurn.debug,
          tools: inferred.tools,
        },
      },
    }
  })

  return changed ? hydrated : turns
}

export function attachDebugTraceToLatestTurn(turns: ConversationTurn[], trace: DebugTrace): ConversationTurn[] {
  if (!hasDebugTrace(trace)) {
    return turns
  }

  for (let i = turns.length - 1; i >= 0; i--) {
    const turn = turns[i]
    if (!turn.assistantTurn) {
      continue
    }

    const next = [...turns]
    next[i] = {
      ...turn,
      assistantTurn: {
        ...turn.assistantTurn,
        debug: trace,
      },
    }
    return next
  }

  return turns
}

export function mergeDebugTraceFromPreviousTurns(previousTurns: ConversationTurn[], nextTurns: ConversationTurn[]): ConversationTurn[] {
  if (previousTurns.length === 0 || nextTurns.length === 0) {
    return nextTurns
  }

  const debugByAssistantTurnID = new Map<string, DebugTrace>()
  for (const turn of previousTurns) {
    const assistantTurn = turn.assistantTurn
    if (assistantTurn?.debug) {
      debugByAssistantTurnID.set(assistantTurn.id, assistantTurn.debug)
    }
  }

  if (debugByAssistantTurnID.size === 0) {
    return nextTurns
  }

  let changed = false
  const merged = nextTurns.map((turn) => {
    const assistantTurn = turn.assistantTurn
    if (!assistantTurn) {
      return turn
    }

    if (assistantTurn.debug) {
      return turn
    }

    const debug = debugByAssistantTurnID.get(assistantTurn.id)
    if (!debug) {
      return turn
    }

    changed = true
    return {
      ...turn,
      assistantTurn: {
        ...assistantTurn,
        debug,
      },
    }
  })

  return changed ? merged : nextTurns
}
