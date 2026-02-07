/**
 * DebugPanel Component
 * Polished debug trace viewer with expandable tool cards,
 * metric grid, and beautiful dark mode styling.
 */

import { useState, useRef, useEffect } from 'react'
import {
  Bug,
  Timer,
  Lightning,
  Wrench,
  CaretDown,
  CheckCircle,
  XCircle,
  Copy,
  Check,
  CircleNotch,
  ChartBar,
} from '@phosphor-icons/react'
import type { DebugTrace, StreamToolPayload } from '../types'
import { hasMeaningfulUsage, hasDebugTrace } from '../utils/debugTrace'
import { useTranslation } from '../hooks/useTranslation'

export interface DebugPanelProps {
  trace?: DebugTrace
}

const formatDuration = (durationMs: number): string => {
  if (durationMs > 1000) {
    return `${(durationMs / 1000).toFixed(2)}s`
  }
  return `${durationMs}ms`
}

interface MetricCardProps {
  icon: React.ReactNode
  value: string | number
  label: string
  accentColor: string
}

function MetricCard({ icon, value, label, accentColor }: MetricCardProps) {
  return (
    <div className="flex items-start gap-2 p-2.5 rounded-lg border border-gray-200/80 dark:border-gray-700/60 bg-white dark:bg-gray-800/60">
      <div
        className={`flex-shrink-0 flex items-center justify-center w-6 h-6 rounded-md ${accentColor}`}
      >
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="font-mono font-medium text-xs text-gray-900 dark:text-gray-100">
          {value}
        </div>
        <div className="text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500 mt-0.5">
          {label}
        </div>
      </div>
    </div>
  )
}

interface ToolCardProps {
  tool: StreamToolPayload
}

function ToolCard({ tool }: ToolCardProps) {
  const [expanded, setExpanded] = useState(false)
  const [copiedArgs, setCopiedArgs] = useState(false)
  const [copiedResult, setCopiedResult] = useState(false)
  const { t } = useTranslation()
  const copiedArgsRef = useRef<number | null>(null)
  const copiedResultRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (copiedArgsRef.current !== null) clearTimeout(copiedArgsRef.current)
      if (copiedResultRef.current !== null) clearTimeout(copiedResultRef.current)
    }
  }, [])

  const hasResult = !!tool.result && !tool.error
  const hasError = !!tool.error

  const handleCopy = async (
    text: string,
    setCopied: (v: boolean) => void,
    timerRef: React.MutableRefObject<number | null>
  ) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      if (timerRef.current !== null) clearTimeout(timerRef.current)
      timerRef.current = window.setTimeout(() => {
        setCopied(false)
        timerRef.current = null
      }, 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  return (
    <div className="rounded-lg border border-gray-200/80 dark:border-gray-700/60 bg-white dark:bg-gray-800/60 overflow-hidden">
      {/* Collapsed header */}
      <button
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label={`${tool.name} — ${hasError ? 'error' : hasResult ? 'success' : 'pending'}`}
        className="w-full flex items-center gap-2.5 px-3.5 py-2.5 hover:bg-gray-50/60 dark:hover:bg-gray-700/30 transition-colors"
      >
        {/* Status icon */}
        <div className="flex-shrink-0">
          {hasError ? (
            <XCircle size={16} weight="fill" className="text-red-500 dark:text-red-400" />
          ) : hasResult ? (
            <CheckCircle size={16} weight="fill" className="text-green-500 dark:text-green-400" />
          ) : (
            <CircleNotch size={16} weight="bold" className="text-gray-400 dark:text-gray-500 animate-spin" />
          )}
        </div>

        {/* Tool name */}
        <div className="flex-1 min-w-0 text-left font-mono text-xs text-gray-900 dark:text-gray-100 truncate">
          {tool.name}
        </div>

        {/* Duration chip */}
        {tool.durationMs !== undefined && (
          <div className="flex-shrink-0 px-2 py-0.5 rounded bg-gray-100/80 dark:bg-gray-700/60 text-[10px] font-medium text-gray-600 dark:text-gray-400">
            {formatDuration(tool.durationMs)}
          </div>
        )}

        {/* Expand chevron */}
        <CaretDown
          size={14}
          weight="bold"
          className={`flex-shrink-0 text-gray-400 dark:text-gray-500 transition-transform duration-200 ${
            expanded ? 'rotate-180' : ''
          }`}
        />
      </button>

      {/* Expanded content */}
      {expanded && (
        <div className="border-t border-gray-200/80 dark:border-gray-700/60 px-3.5 py-3 space-y-3">
          {/* Arguments section */}
          {tool.arguments && (
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-[10px] uppercase tracking-wider font-medium text-gray-500 dark:text-gray-400">
                  {t('slash.debugArguments')}
                </span>
                <button
                  onClick={() => handleCopy(tool.arguments || '', setCopiedArgs, copiedArgsRef)}
                  className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                  title={t('slash.debugCopyTrace')}
                >
                  {copiedArgs ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
              <pre className="text-[11px] font-mono text-gray-700 dark:text-gray-300 bg-gray-50/60 dark:bg-gray-900/40 rounded-md p-2.5 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all">
                {tool.arguments}
              </pre>
            </div>
          )}

          {/* Result section */}
          {tool.result && (
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-[10px] uppercase tracking-wider font-medium text-gray-500 dark:text-gray-400">
                  {t('slash.debugResult')}
                </span>
                <button
                  onClick={() => handleCopy(tool.result || '', setCopiedResult, copiedResultRef)}
                  className="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                  title={t('slash.debugCopyTrace')}
                >
                  {copiedResult ? <Check size={12} /> : <Copy size={12} />}
                </button>
              </div>
              <pre className="text-[11px] font-mono text-gray-700 dark:text-gray-300 bg-gray-50/60 dark:bg-gray-900/40 rounded-md p-2.5 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all">
                {tool.result}
              </pre>
            </div>
          )}

          {/* Error section */}
          {tool.error && (
            <div>
              <div className="mb-1.5">
                <span className="text-[10px] uppercase tracking-wider font-medium text-red-500 dark:text-red-400">
                  {t('slash.debugError')}
                </span>
              </div>
              <pre className="text-[11px] font-mono text-red-600 dark:text-red-400 bg-red-50/60 dark:bg-red-950/20 rounded-md p-2.5 overflow-x-auto whitespace-pre-wrap break-all">
                {tool.error}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export function DebugPanel({ trace }: DebugPanelProps) {
  const { t } = useTranslation()
  const [copiedTrace, setCopiedTrace] = useState(false)
  const copiedTraceRef = useRef<number | null>(null)
  const hasData = !!trace && hasDebugTrace(trace)

  useEffect(() => {
    return () => {
      if (copiedTraceRef.current !== null) clearTimeout(copiedTraceRef.current)
    }
  }, [])

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

  const handleCopyTrace = async () => {
    if (!trace) return
    try {
      await navigator.clipboard.writeText(JSON.stringify(trace, null, 2))
      setCopiedTrace(true)
      if (copiedTraceRef.current !== null) clearTimeout(copiedTraceRef.current)
      copiedTraceRef.current = window.setTimeout(() => {
        setCopiedTrace(false)
        copiedTraceRef.current = null
      }, 2000)
    } catch (err) {
      console.error('Failed to copy trace:', err)
    }
  }

  const tokensPerSecond = getTokensPerSecond()

  return (
    <div className="mt-4 border-t border-gray-100 dark:border-gray-700 pt-4">
      {/* Section header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Bug size={16} weight="duotone" className="text-gray-500 dark:text-gray-400" />
          <h3 className="text-xs uppercase tracking-wide font-medium text-gray-500 dark:text-gray-400">
            {t('slash.debugPanelTitle')}
          </h3>
        </div>
        {hasData && (
          <button
            onClick={handleCopyTrace}
            className="flex items-center gap-1.5 px-2 py-1 text-[10px] uppercase tracking-wider font-medium text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors rounded hover:bg-gray-100 dark:hover:bg-gray-700/40"
            title={t('slash.debugCopyTrace')}
          >
            {copiedTrace ? (
              <>
                <Check size={12} />
                <span>{t('slash.debugCopied')}</span>
              </>
            ) : (
              <>
                <Copy size={12} />
                <span>{t('slash.debugCopyTrace')}</span>
              </>
            )}
          </button>
        )}
      </div>

      {/* Content */}
      {hasData && trace ? (
        <div className="space-y-4">
          {/* Metric cards grid */}
          {(trace.generationMs !== undefined || tokensPerSecond !== null || hasMeaningfulUsage(trace.usage)) && (
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {trace.generationMs !== undefined && (
                <MetricCard
                  icon={<Timer size={14} weight="duotone" className="text-blue-600 dark:text-blue-400" />}
                  value={formatDuration(trace.generationMs)}
                  label={t('slash.debugGeneration')}
                  accentColor="bg-blue-50 dark:bg-blue-950/20"
                />
              )}
              {tokensPerSecond !== null && (
                <MetricCard
                  icon={<Lightning size={14} weight="fill" className="text-amber-600 dark:text-amber-400" />}
                  value={tokensPerSecond.toFixed(2)}
                  label={t('slash.debugTokensPerSecond')}
                  accentColor="bg-amber-50 dark:bg-amber-950/20"
                />
              )}
              {hasMeaningfulUsage(trace.usage) && trace.usage && (
                <>
                  <MetricCard
                    icon={<ChartBar size={14} weight="duotone" className="text-purple-600 dark:text-purple-400" />}
                    value={trace.usage.totalTokens.toLocaleString()}
                    label={t('slash.debugTotalTokens')}
                    accentColor="bg-purple-50 dark:bg-purple-950/20"
                  />
                  <MetricCard
                    icon={<ChartBar size={14} weight="duotone" className="text-emerald-600 dark:text-emerald-400" />}
                    value={trace.usage.promptTokens.toLocaleString()}
                    label={t('slash.debugPromptTokens')}
                    accentColor="bg-emerald-50 dark:bg-emerald-950/20"
                  />
                  <MetricCard
                    icon={<ChartBar size={14} weight="duotone" className="text-cyan-600 dark:text-cyan-400" />}
                    value={trace.usage.completionTokens.toLocaleString()}
                    label={t('slash.debugCompletionTokens')}
                    accentColor="bg-cyan-50 dark:bg-cyan-950/20"
                  />
                  {trace.usage.cachedTokens !== undefined && trace.usage.cachedTokens > 0 && (
                    <MetricCard
                      icon={<ChartBar size={14} weight="duotone" className="text-rose-600 dark:text-rose-400" />}
                      value={trace.usage.cachedTokens.toLocaleString()}
                      label={t('slash.debugCachedTokens')}
                      accentColor="bg-rose-50 dark:bg-rose-950/20"
                    />
                  )}
                </>
              )}
            </div>
          )}

          {/* Tool calls section */}
          {trace.tools.length > 0 && (
            <div>
              <div className="flex items-center gap-2 mb-2.5">
                <Wrench size={14} weight="duotone" className="text-gray-500 dark:text-gray-400" />
                <h4 className="text-xs font-medium text-gray-700 dark:text-gray-300">
                  {t('slash.debugToolCalls')} ({trace.tools.length})
                </h4>
              </div>
              <div className="space-y-2">
                {trace.tools.map((tool, idx) => (
                  <ToolCard key={`${tool.callId || tool.name}-${idx}`} tool={tool} />
                ))}
              </div>
            </div>
          )}
        </div>
      ) : (
        <p className="text-xs text-gray-500 dark:text-gray-400">
          {t('slash.debugUnavailable')}
        </p>
      )}
    </div>
  )
}

export default DebugPanel
