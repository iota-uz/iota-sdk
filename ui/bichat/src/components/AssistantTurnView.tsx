/**
 * AssistantTurnView component
 * Displays assistant messages with markdown, charts, sources, and downloads
 */

import { useState } from 'react'
import { Message } from '../types'
import { useChat } from '../context/ChatContext'
import { MarkdownRenderer } from './MarkdownRenderer'
import { ChartCard } from './ChartCard'
import { SourcesPanel } from './SourcesPanel'
import { DownloadCard } from './DownloadCard'
import { InlineQuestionForm } from './InlineQuestionForm'

interface AssistantTurnViewProps {
  message: Message
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

  return (
    <div className="flex gap-3 group">
      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-[var(--bichat-primary)] flex items-center justify-center text-white">
        AI
      </div>
      <div className="flex-1 flex flex-col gap-2 max-w-2xl">
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
          <div className="rounded-2xl px-5 py-3 bg-[var(--bichat-bubble-assistant)] border border-[var(--bichat-border)]">
            <MarkdownRenderer content={message.content} citations={message.citations} />

            {/* Sources panel */}
            {message.citations && message.citations.length > 0 && (
              <SourcesPanel citations={message.citations} />
            )}

            {/* Explanation section */}
            {hasExplanation && (
              <div className="mt-3 border-t border-[var(--bichat-border)] pt-3">
                <button
                  type="button"
                  onClick={() => setExplanationExpanded(!explanationExpanded)}
                  className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 transition-colors"
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
                  <div className="pt-2 text-sm text-gray-600">
                    <MarkdownRenderer content={message.explanation!} />
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Inline Question Form */}
        {hasPendingQuestion && <InlineQuestionForm pendingQuestion={pendingQuestion} />}

        {/* Timestamp and actions */}
        {hasContent && (
          <div className="flex items-center gap-2 px-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <span className="text-xs text-gray-500">
              {new Date(message.createdAt).toLocaleTimeString()}
            </span>
            <button
              onClick={() => handleCopy(message.content)}
              className="text-xs text-gray-500 hover:text-gray-700"
              aria-label="Copy message"
            >
              Copy
            </button>
            {handleRegenerate && (
              <button
                onClick={() => handleRegenerate(message.id)}
                className="text-xs text-gray-500 hover:text-gray-700"
                aria-label="Regenerate message"
              >
                Regenerate
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
