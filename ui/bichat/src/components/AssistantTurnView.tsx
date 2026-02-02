/**
 * AssistantTurnView Component
 * Displays assistant messages with markdown, charts, sources, downloads, code outputs, and streaming cursor
 * Clean, professional design
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
      {/* Avatar - solid primary color */}
      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center text-white font-medium text-xs">
        AI
      </div>

      <div className="flex-1 flex flex-col gap-3 max-w-[85%]">
        {/* Code outputs */}
        {message.codeOutputs && message.codeOutputs.length > 0 && (
          <CodeOutputsPanel outputs={message.codeOutputs} />
        )}

        {/* Chart visualization */}
        {message.chartData && (
          <div className="mb-1 w-full">
            <ChartCard chartData={message.chartData} />
          </div>
        )}

        {/* Artifact cards */}
        {message.artifacts && message.artifacts.length > 0 && (
          <div className="mb-1 flex flex-wrap gap-2">
            {message.artifacts.map((artifact, index) => (
              <DownloadCard key={`${artifact.filename}-${index}`} artifact={artifact} />
            ))}
          </div>
        )}

        {/* Message bubble - clean card style */}
        {hasContent && (
          <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl rounded-bl-sm px-4 py-3">
            <Suspense
              fallback={
                <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-gray-500">
                  <div className="w-4 h-4 border-2 border-gray-300 dark:border-gray-600 border-t-transparent rounded-full animate-spin" />
                  Loading...
                </div>
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
              <div className="mt-4 border-t border-gray-100 dark:border-gray-700 pt-4">
                <button
                  type="button"
                  onClick={() => setExplanationExpanded(!explanationExpanded)}
                  className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
                  aria-expanded={explanationExpanded}
                >
                  <svg
                    className={`w-4 h-4 transition-transform duration-150 ${explanationExpanded ? 'rotate-90' : ''}`}
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
                  <span className="font-medium">How I arrived at this</span>
                </button>
                {explanationExpanded && (
                  <div className="pt-3 text-sm text-gray-600 dark:text-gray-400">
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
          <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
            <span className="text-xs text-gray-400 dark:text-gray-500 mr-1">
              {formatDistanceToNow(new Date(message.createdAt), { addSuffix: true })}
            </span>

            <button
              onClick={handleCopyClick}
              className="p-1.5 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors duration-150"
              aria-label="Copy message"
              title="Copy"
            >
              <Copy size={14} weight="regular" />
            </button>

            {handleRegenerate && (
              <button
                onClick={handleRegenerateClick}
                className="p-1.5 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors duration-150"
                aria-label="Regenerate message"
                title="Regenerate"
              >
                <ArrowsClockwise size={14} weight="regular" />
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
