/**
 * DebugPanel Component
 * Renders debug trace information (generation time, token usage, tool calls)
 * Extracted from AssistantMessage for reuse and clarity.
 */

import type { DebugTrace } from '../types'
import { hasMeaningfulUsage, hasDebugTrace } from '../utils/debugTrace'
import { useTranslation } from '../hooks/useTranslation'

export interface DebugPanelProps {
  trace?: DebugTrace
}

export function DebugPanel({ trace }: DebugPanelProps) {
  const { t } = useTranslation()
  const hasData = !!trace && hasDebugTrace(trace)
  const formatGenerationDuration = (durationMs: number): string => {
    if (durationMs > 1000) {
      return `${(durationMs / 1000).toFixed(2)}s`
    }
    return `${durationMs}ms`
  }
  const getTokensPerSecond = (): number | null => {
    if (!trace?.generationMs || trace.generationMs <= 0 || !trace.usage) {
      return null
    }

    const outputTokens = trace.usage.completionTokens > 0 ? trace.usage.completionTokens : trace.usage.totalTokens
    if (outputTokens <= 0) {
      return null
    }

    return outputTokens / (trace.generationMs / 1000)
  }
  const tokensPerSecond = getTokensPerSecond()

  return (
    <div className="mt-4 border-t border-gray-100 dark:border-gray-700 pt-4">
      <p className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-2">
        {t('slash.debugPanelTitle')}
      </p>
      <div className="space-y-2 text-xs text-gray-600 dark:text-gray-300">
        {hasData && trace && trace.generationMs !== undefined && (
          <p>
            {t('slash.debugGeneration')}:{' '}
            <span className="font-mono">{formatGenerationDuration(trace.generationMs)}</span>
          </p>
        )}
        {hasData && tokensPerSecond !== null && (
          <p>
            {t('slash.debugTokensPerSecond')}:{' '}
            <span className="font-mono">{tokensPerSecond.toFixed(2)}</span>
          </p>
        )}
        {hasData && trace && hasMeaningfulUsage(trace.usage) && trace.usage && (
          <p>
            {t('slash.debugUsage')}:{' '}
            <span className="font-mono">
              {trace.usage.promptTokens}/{trace.usage.completionTokens}/{trace.usage.totalTokens}
            </span>
          </p>
        )}
        {hasData && trace && trace.tools.length > 0 && (
          <div className="space-y-1">
            <p>{t('slash.debugTools')}</p>
            {trace.tools.map((tool, idx) => (
              <div
                key={`${tool.callId || tool.name}-${idx}`}
                className="rounded-md bg-gray-50 dark:bg-gray-900/40 p-2 border border-gray-200 dark:border-gray-700"
              >
                <p className="font-mono text-[11px] text-gray-700 dark:text-gray-200">
                  {tool.name} {tool.callId ? `(${tool.callId})` : ''}
                </p>
                {tool.durationMs !== undefined && (
                  <p className="text-[11px] text-gray-500 dark:text-gray-400">{tool.durationMs}ms</p>
                )}
                {tool.arguments && (
                  <pre className="mt-1 whitespace-pre-wrap break-all text-[11px] text-gray-600 dark:text-gray-300">
                    {tool.arguments.slice(0, 500)}
                  </pre>
                )}
                {tool.result && (
                  <pre className="mt-1 whitespace-pre-wrap break-all text-[11px] text-gray-600 dark:text-gray-300">
                    {tool.result.slice(0, 500)}
                  </pre>
                )}
                {tool.error && (
                  <p className="mt-1 text-[11px] text-red-600 dark:text-red-400">{tool.error}</p>
                )}
              </div>
            ))}
          </div>
        )}
        {!hasData && (
          <p className="text-[11px] text-gray-500 dark:text-gray-400">
            {t('slash.debugUnavailable')}
          </p>
        )}
      </div>
    </div>
  )
}

export default DebugPanel
