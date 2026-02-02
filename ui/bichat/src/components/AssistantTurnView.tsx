/**
 * AssistantTurnView Component
 * Displays assistant messages with markdown, charts, sources, downloads, code outputs, and streaming cursor
 */

import { useState, lazy, Suspense } from 'react'
import { Copy, ArrowsClockwise } from '@phosphor-icons/react'
import { formatDistanceToNow } from 'date-fns'
import CodeOutputsPanel from './CodeOutputsPanel'
import StreamingCursor from './StreamingCursor'
import { ChartCard } from './ChartCard'
import { SourcesPanel } from './SourcesPanel'
import { DownloadCard } from './DownloadCard'
import { InlineQuestionForm } from './InlineQuestionForm'
import { useChat } from '../context/ChatContext'
import type { Message, CodeOutput } from '../types'

// Lazy load MarkdownRenderer for performance
const MarkdownRenderer = lazy(() =>
  import('./MarkdownRenderer').then((module) => ({ default: module.MarkdownRenderer }))
)

interface AssistantTurnViewProps {
  message: Message & {
    codeOutputs?: CodeOutput[]
    isStreaming?: boolean
  }
}

export function AssistantTurnView({ message }: AssistantTurnViewProps) {
  const { handleCopy, handleRegenerate, pendingQuestion } = useChat()
  const [explanationExpanded, setExplanationExpanded] = useState(false)

  const hasContent = message.content?.trim().length > 0
  const hasExplanation = !!message.explanation?.trim()
  const hasPendingQuestion =
    !!pendingQuestion &&
    pendingQuestion.status === 'PENDING' &&
    pendingQuestion.turnId === message.id

  const handleCopyClick = async () => {
    if (handleCopy) {
      await handleCopy(message.content)
    } else {
      // Fallback to clipboard API
      try {
        await navigator.clipboard.writeText(message.content)
      } catch (err) {
        console.error('Failed to copy:', err)
      }
    }
  }

  const handleRegenerateClick = async () => {
    if (handleRegenerate) {
      await handleRegenerate(message.id)
    }
  }

  return (
    <div className="flex gap-3 group">
      {/* Avatar */}
      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 dark:bg-primary-700 flex items-center justify-center text-white font-semibold text-sm shadow-sm">
        AI
      </div>

      <div className="flex-1 flex flex-col gap-2 max-w-[80%]">
        {/* Code outputs */}
        {message.codeOutputs && message.codeOutputs.length > 0 && (
          <CodeOutputsPanel outputs={message.codeOutputs} />
        )}

        {/* Chart visualization */}
        {message.chartData && (
          <div className="mb-2 w-full">
            <ChartCard chartData={message.chartData} />
          </div>
        )}

        {/* Artifact cards - for Excel and PDF exports */}
        {message.artifacts && message.artifacts.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-2">
            {message.artifacts.map((artifact, index) => (
              <DownloadCard key={`${artifact.filename}-${index}`} artifact={artifact} />
            ))}
          </div>
        )}

        {/* Message bubble */}
        {hasContent && (
          <div className="rounded-2xl px-5 py-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-sm">
            <Suspense
              fallback={
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading...</div>
              }
            >
              <MarkdownRenderer content={message.content} citations={message.citations} />
            </Suspense>

            {/* Streaming cursor */}
            {message.isStreaming && <StreamingCursor />}

            {/* Sources panel */}
            {message.citations && message.citations.length > 0 && (
              <SourcesPanel citations={message.citations} />
            )}

            {/* Explanation section */}
            {hasExplanation && (
              <div className="mt-3 border-t border-gray-200 dark:border-gray-700 pt-3">
                <button
                  type="button"
                  onClick={() => setExplanationExpanded(!explanationExpanded)}
                  className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                  aria-expanded={explanationExpanded}
                >
                  <svg
                    className={`w-4 h-4 transition-transform ${explanationExpanded ? 'rotate-90' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5l7 7-7 7"
                    />
                  </svg>
                  <span>How I arrived at this</span>
                </button>
                {explanationExpanded && (
                  <div className="pt-2 text-sm text-gray-600 dark:text-gray-400">
                    <Suspense fallback={<div>Loading...</div>}>
                      <MarkdownRenderer content={message.explanation!} />
                    </Suspense>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Inline Question Form */}
        {hasPendingQuestion && <InlineQuestionForm pendingQuestion={pendingQuestion} />}

        {/* Actions */}
        {hasContent && (
          <div className="flex items-center gap-2 px-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {formatDistanceToNow(new Date(message.createdAt), { addSuffix: true })}
            </span>

            {/* Copy button */}
            <button
              onClick={handleCopyClick}
              className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
              aria-label="Copy message"
              title="Copy"
            >
              <Copy size={14} />
            </button>

            {/* Regenerate button */}
            {handleRegenerate && (
              <button
                onClick={handleRegenerateClick}
                className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
                aria-label="Regenerate message"
                title="Regenerate"
              >
                <ArrowsClockwise size={14} />
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
