/**
 * DebugPanel Component
 * Beautiful debug trace viewer with metric cards, expandable tool calls,
 * and terminal-inspired code blocks.
 */

import { useState, useRef, useEffect, type ReactNode } from 'react'
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
  ArrowUp,
  ArrowDown,
  Stack,
  Database,
} from '@phosphor-icons/react'
import type { DebugTrace, StreamToolPayload } from '../types'
import { hasMeaningfulUsage, hasDebugTrace } from '../utils/debugTrace'
import {
  calculateCompletionTokensPerSecond,
  formatDuration,
  formatGenerationDuration,
} from '../utils/debugMetrics'
import { useTranslation } from '../hooks/useTranslation'

export interface DebugPanelProps {
  trace?: DebugTrace
}

// ─── CopyPill ───────────────────────────────────────────────

function CopyPill({ text, label, copiedLabel }: { text: string; label: string; copiedLabel: string }) {
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)

  useEffect(() => () => {
    if (timerRef.current !== null) clearTimeout(timerRef.current)
  }, [])

  const handleCopy = async (e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      if (timerRef.current !== null) clearTimeout(timerRef.current)
      timerRef.current = window.setTimeout(() => {
        setCopied(false)
        timerRef.current = null
      }, 2000)
    } catch (err) {
      console.error('Copy failed:', err)
    }
  }

  return (
    <button
      onClick={handleCopy}
      className={[
        'flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium',
        'transition-all duration-200',
        copied
          ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400'
          : 'text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700/50',
      ].join(' ')}
    >
      {copied ? <Check size={10} weight="bold" /> : <Copy size={10} />}
      <span>{copied ? copiedLabel : label}</span>
    </button>
  )
}

// ─── MetricCard ─────────────────────────────────────────────

interface MetricCardProps {
  icon: ReactNode
  value: string
  label: string
  accentBorder: string
  accentBg: string
}

function MetricCard({ icon, value, label, accentBorder, accentBg }: MetricCardProps) {
  return (
    <div
      className={[
        'flex items-center gap-2.5 p-2.5 rounded-lg',
        'border border-gray-200/60 dark:border-gray-700/40',
        'border-l-2', accentBorder,
        'bg-gray-50/40 dark:bg-gray-800/30',
        'hover:bg-white dark:hover:bg-gray-800/60',
        'hover:shadow-[0_1px_3px_rgba(0,0,0,0.04)] dark:hover:shadow-[0_1px_3px_rgba(0,0,0,0.2)]',
        'transition-all duration-150',
      ].join(' ')}
    >
      <div
        className={[
          'flex-shrink-0 flex items-center justify-center w-7 h-7 rounded-lg',
          accentBg,
        ].join(' ')}
      >
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="font-mono font-semibold text-sm text-gray-900 dark:text-gray-50 tabular-nums leading-none">
          {value}
        </div>
        <div className="text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500 mt-1 leading-none">
          {label}
        </div>
      </div>
    </div>
  )
}

// ─── ToolCard ───────────────────────────────────────────────

function ToolCard({ tool }: { tool: StreamToolPayload }) {
  const [expanded, setExpanded] = useState(false)
  const { t } = useTranslation()

  const hasResult = !!tool.result && !tool.error
  const hasError = !!tool.error

  const status = hasError
    ? {
        icon: <XCircle size={12} weight="fill" />,
        pillBg: 'bg-red-50 dark:bg-red-950/30',
        pillText: 'text-red-500 dark:text-red-400',
        borderColor: 'border-l-red-400 dark:border-l-red-500',
      }
    : hasResult
    ? {
        icon: <CheckCircle size={12} weight="fill" />,
        pillBg: 'bg-emerald-50 dark:bg-emerald-950/30',
        pillText: 'text-emerald-500 dark:text-emerald-400',
        borderColor: 'border-l-emerald-400 dark:border-l-emerald-500',
      }
    : {
        icon: <CircleNotch size={12} weight="bold" className="animate-spin" />,
        pillBg: 'bg-gray-100 dark:bg-gray-800',
        pillText: 'text-gray-400 dark:text-gray-500',
        borderColor: 'border-l-gray-300 dark:border-l-gray-600',
      }

  return (
    <div
      className={[
        'rounded-lg overflow-hidden',
        'border border-gray-200/60 dark:border-gray-700/40',
        'border-l-2', status.borderColor,
        'bg-white dark:bg-gray-800/50',
        'transition-all duration-150',
      ].join(' ')}
    >
      {/* Header */}
      <button
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label={`${tool.name} — ${hasError ? 'error' : hasResult ? 'success' : 'pending'}`}
        className="w-full flex items-center gap-2.5 px-3 py-2.5 hover:bg-gray-50/60 dark:hover:bg-gray-700/20 transition-colors cursor-pointer"
      >
        {/* Status pill */}
        <span className={`flex items-center justify-center w-5 h-5 rounded-full ${status.pillBg} ${status.pillText}`}>
          {status.icon}
        </span>

        {/* Tool name */}
        <span className="flex-1 min-w-0 text-left font-mono text-xs font-medium text-gray-800 dark:text-gray-200 truncate">
          {tool.name}
        </span>

        {/* Duration chip */}
        {tool.durationMs !== undefined && (
          <span className="flex-shrink-0 px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700/60 text-[10px] font-mono font-medium text-gray-500 dark:text-gray-400 tabular-nums">
            {formatDuration(tool.durationMs)}
          </span>
        )}

        {/* Chevron */}
        <CaretDown
          size={12}
          weight="bold"
          className={[
            'flex-shrink-0 text-gray-300 dark:text-gray-600',
            'transition-transform duration-200',
            expanded ? 'rotate-180' : '',
          ].join(' ')}
        />
      </button>

      {/* Expandable content — CSS grid animation for smooth height */}
      <div
        className={[
          'grid transition-[grid-template-rows] duration-200 ease-out',
          expanded ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]',
        ].join(' ')}
      >
        <div className="overflow-hidden min-h-0">
          <div className="px-3 pb-3 pt-1 space-y-2">
            {/* Arguments — terminal-style dark code block */}
            {tool.arguments && (
              <div className="rounded-lg bg-[#1a1b26] dark:bg-gray-950 overflow-hidden ring-1 ring-gray-800/10 dark:ring-white/5">
                <div className="flex items-center justify-between px-3 py-1.5 bg-[#1e1f2e] dark:bg-gray-900/80 border-b border-white/5">
                  <span className="text-[10px] uppercase tracking-wider font-medium text-gray-500">
                    {t('slash.debugArguments')}
                  </span>
                  <CopyPill
                    text={tool.arguments}
                    label={t('slash.debugCopyTrace')}
                    copiedLabel={t('slash.debugCopied')}
                  />
                </div>
                <pre className="p-3 text-[11px] font-mono text-gray-300 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all leading-relaxed">
                  {tool.arguments}
                </pre>
              </div>
            )}

            {/* Result — terminal-style dark code block */}
            {tool.result && (
              <div className="rounded-lg bg-[#1a1b26] dark:bg-gray-950 overflow-hidden ring-1 ring-gray-800/10 dark:ring-white/5">
                <div className="flex items-center justify-between px-3 py-1.5 bg-[#1e1f2e] dark:bg-gray-900/80 border-b border-white/5">
                  <span className="text-[10px] uppercase tracking-wider font-medium text-gray-500">
                    {t('slash.debugResult')}
                  </span>
                  <CopyPill
                    text={tool.result}
                    label={t('slash.debugCopyTrace')}
                    copiedLabel={t('slash.debugCopied')}
                  />
                </div>
                <pre className="p-3 text-[11px] font-mono text-gray-300 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all leading-relaxed">
                  {tool.result}
                </pre>
              </div>
            )}

            {/* Error — red-tinted dark block */}
            {tool.error && (
              <div className="rounded-lg bg-red-950/80 dark:bg-red-950/40 overflow-hidden ring-1 ring-red-800/20">
                <div className="px-3 py-1.5 border-b border-red-800/20">
                  <span className="text-[10px] uppercase tracking-wider font-medium text-red-400">
                    {t('slash.debugError')}
                  </span>
                </div>
                <pre className="p-3 text-[11px] font-mono text-red-300 overflow-x-auto whitespace-pre-wrap break-all leading-relaxed">
                  {tool.error}
                </pre>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── DebugPanel ─────────────────────────────────────────────

export function DebugPanel({ trace }: DebugPanelProps) {
  const { t } = useTranslation()
  const hasData = !!trace && hasDebugTrace(trace)

  const tokensPerSecond = calculateCompletionTokensPerSecond(trace?.usage, trace?.generationMs)

  // Build metric list from available data
  const metrics: MetricCardProps[] = []

  if (hasData && trace) {
    if (trace.generationMs !== undefined) {
        metrics.push({
          icon: <Timer size={14} weight="duotone" className="text-amber-600 dark:text-amber-400" />,
          value: formatGenerationDuration(trace.generationMs),
          label: t('slash.debugGeneration'),
          accentBorder: 'border-l-amber-400 dark:border-l-amber-500',
          accentBg: 'bg-amber-50 dark:bg-amber-950/30',
      })
    }
    if (tokensPerSecond !== null) {
      metrics.push({
        icon: <Lightning size={14} weight="fill" className="text-orange-500 dark:text-orange-400" />,
        value: `${tokensPerSecond.toFixed(1)}/s`,
        label: t('slash.debugTokensPerSecond'),
        accentBorder: 'border-l-orange-400 dark:border-l-orange-500',
        accentBg: 'bg-orange-50 dark:bg-orange-950/30',
      })
    }
    if (hasMeaningfulUsage(trace.usage) && trace.usage) {
      metrics.push(
        {
          icon: <Stack size={14} weight="duotone" className="text-violet-600 dark:text-violet-400" />,
          value: trace.usage.totalTokens.toLocaleString(),
          label: t('slash.debugTotalTokens'),
          accentBorder: 'border-l-violet-400 dark:border-l-violet-500',
          accentBg: 'bg-violet-50 dark:bg-violet-950/30',
        },
        {
          icon: <ArrowUp size={14} weight="bold" className="text-blue-600 dark:text-blue-400" />,
          value: trace.usage.promptTokens.toLocaleString(),
          label: t('slash.debugPromptTokens'),
          accentBorder: 'border-l-blue-400 dark:border-l-blue-500',
          accentBg: 'bg-blue-50 dark:bg-blue-950/30',
        },
        {
          icon: <ArrowDown size={14} weight="bold" className="text-indigo-600 dark:text-indigo-400" />,
          value: trace.usage.completionTokens.toLocaleString(),
          label: t('slash.debugCompletionTokens'),
          accentBorder: 'border-l-indigo-400 dark:border-l-indigo-500',
          accentBg: 'bg-indigo-50 dark:bg-indigo-950/30',
        },
      )
      if (trace.usage.cachedTokens !== undefined && trace.usage.cachedTokens > 0) {
        metrics.push({
          icon: <Database size={14} weight="duotone" className="text-pink-600 dark:text-pink-400" />,
          value: trace.usage.cachedTokens.toLocaleString(),
          label: t('slash.debugCachedTokens'),
          accentBorder: 'border-l-pink-400 dark:border-l-pink-500',
          accentBg: 'bg-pink-50 dark:bg-pink-950/30',
        })
      }
    }
  }

  return (
    <div className="mt-4 pt-4 border-t border-gray-100 dark:border-gray-700/50">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2.5">
          <div className="flex items-center justify-center w-6 h-6 rounded-lg bg-gray-100 dark:bg-gray-800">
            <Bug size={14} weight="duotone" className="text-gray-500 dark:text-gray-400" />
          </div>
          <h3 className="text-[11px] uppercase tracking-widest font-semibold text-gray-400 dark:text-gray-500">
            {t('slash.debugPanelTitle')}
          </h3>
        </div>
        {hasData && trace && (
          <CopyPill
            text={JSON.stringify(trace, null, 2)}
            label={t('slash.debugCopyTrace')}
            copiedLabel={t('slash.debugCopied')}
          />
        )}
      </div>

      {hasData && trace ? (
        <div className="space-y-4">
          {/* Metric cards */}
          {metrics.length > 0 && (
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {metrics.map((m, i) => (
                <MetricCard key={i} {...m} />
              ))}
            </div>
          )}

          {/* Tool calls */}
          {trace.tools.length > 0 && (
            <div>
              <div className="flex items-center gap-2 mb-2.5">
                <Wrench size={13} weight="duotone" className="text-gray-400 dark:text-gray-500" />
                <span className="text-[11px] font-medium text-gray-500 dark:text-gray-400">
                  {t('slash.debugToolCalls')}
                </span>
                <span className="px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-gray-800 text-[10px] font-mono font-medium text-gray-500 dark:text-gray-400 tabular-nums">
                  {trace.tools.length}
                </span>
              </div>
              <div className="space-y-1.5">
                {trace.tools.map((tool, idx) => (
                  <ToolCard key={`${tool.callId || tool.name}-${idx}`} tool={tool} />
                ))}
              </div>
            </div>
          )}
        </div>
      ) : (
        <p className="text-xs text-gray-400 dark:text-gray-500 italic">
          {t('slash.debugUnavailable')}
        </p>
      )}
    </div>
  )
}

export default DebugPanel
