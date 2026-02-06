/**
 * MessageList component
 * Displays conversation turns with auto-scroll and grouping
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 */

import { useCallback, useEffect, useRef, useMemo, ReactNode, useState, lazy, Suspense } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useChat } from '../context/ChatContext'
import { ConversationTurn } from '../types'
import { TurnBubble } from './TurnBubble'
import { TypingIndicator } from './TypingIndicator'
import StreamingCursor from './StreamingCursor'
import ScrollToBottomButton from './ScrollToBottomButton'
import CompactionDoodle from './CompactionDoodle'
import { useTranslation } from '../hooks/useTranslation'
import { normalizeStreamingMarkdown } from '../utils/markdownStream'

const MarkdownRenderer = lazy(() =>
  import('./MarkdownRenderer').then((m) => ({ default: m.MarkdownRenderer }))
)

interface MessageListProps {
  /** Custom render function for user turns */
  renderUserTurn?: (turn: ConversationTurn) => ReactNode
  /** Custom render function for assistant turns */
  renderAssistantTurn?: (turn: ConversationTurn) => ReactNode
  /** Custom verbs for the typing indicator (e.g. ['Thinking', 'Analyzing', ...]) */
  thinkingVerbs?: string[]
}

export function MessageList({ renderUserTurn, renderAssistantTurn, thinkingVerbs }: MessageListProps) {
  const { t } = useTranslation()
  const {
    turns,
    streamingContent,
    isStreaming,
    loading,
    isCompacting,
    compactionSummary,
    currentSessionId,
    fetching,
  } = useChat()
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const initialScrollSessionRef = useRef<string | undefined>(undefined)
  const [showScrollButton, setShowScrollButton] = useState(false)

  const scrollToBottom = useCallback((behavior: ScrollBehavior = 'smooth') => {
    const container = containerRef.current
    if (container) {
      container.scrollTo({
        top: container.scrollHeight,
        behavior,
      })
      return
    }
    messagesEndRef.current?.scrollIntoView({ behavior })
  }, [])

  // Auto-scroll to bottom on new turns or streaming content
  useEffect(() => {
    scrollToBottom('smooth')
  }, [turns.length, streamingContent, scrollToBottom])

  // On first open of a session, jump to latest message immediately.
  useEffect(() => {
    if (fetching || !currentSessionId || currentSessionId === 'new') return
    if (initialScrollSessionRef.current === currentSessionId) return

    const runInitialScroll = () => {
      scrollToBottom('auto')
      setShowScrollButton(false)
    }

    requestAnimationFrame(() => {
      requestAnimationFrame(runInitialScroll)
    })
    const timeoutOne = setTimeout(runInitialScroll, 80)
    const timeoutTwo = setTimeout(runInitialScroll, 200)
    const timeoutThree = setTimeout(runInitialScroll, 400)

    initialScrollSessionRef.current = currentSessionId
    return () => {
      clearTimeout(timeoutOne)
      clearTimeout(timeoutTwo)
      clearTimeout(timeoutThree)
    }
  }, [currentSessionId, fetching, turns.length, scrollToBottom])

  // Scroll detection for ScrollToBottomButton
  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 100
      setShowScrollButton(!isNearBottom)
    }

    container.addEventListener('scroll', handleScroll)
    return () => container.removeEventListener('scroll', handleScroll)
  }, [])

  const normalizedStreaming = useMemo(
    () => (streamingContent ? normalizeStreamingMarkdown(streamingContent) : ''),
    [streamingContent]
  )

  return (
    <div className="relative flex-1 min-h-0">
      <div ref={containerRef} className="h-full overflow-y-auto px-4 py-6">
        <div className="mx-auto space-y-6">
          {isCompacting && (
            <CompactionDoodle
              title={t('slash.compactingTitle')}
              subtitle={t('slash.compactingSubtitle')}
            />
          )}
          {!isCompacting && compactionSummary && (
            <div className="rounded-2xl border border-primary-200 dark:border-primary-800 bg-primary-50/70 dark:bg-primary-900/20 p-4">
              <p className="text-xs uppercase tracking-wide text-primary-700 dark:text-primary-300 mb-1">
                {t('slash.compactedSummaryLabel')}
              </p>
              <p className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">
                {compactionSummary}
              </p>
            </div>
          )}
          {/* Loading skeleton when no turns yet */}
          {fetching && turns.length === 0 && (
            <div className="space-y-6" aria-hidden="true">
              {/* User message skeleton */}
              <div className="flex justify-end">
                <div className="w-3/5 max-w-md rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
                  <div className="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                  <div className="h-3 w-4/5 rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                </div>
              </div>
              {/* Assistant message skeleton */}
              <div className="flex gap-3">
                <div className="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700 animate-pulse shrink-0" />
                <div className="w-4/5 max-w-lg rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
                  <div className="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                  <div className="h-3 w-5/6 rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                  <div className="h-3 w-3/5 rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                </div>
              </div>
              {/* Second user message skeleton */}
              <div className="flex justify-end">
                <div className="w-2/5 max-w-xs rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
                  <div className="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse" />
                </div>
              </div>
            </div>
          )}
          {turns.map((turn) => (
            <TurnBubble
              key={turn.id}
              turn={turn}
              renderUserTurn={renderUserTurn}
              renderAssistantTurn={renderAssistantTurn}
            />
          ))}
          {/* Typing Indicator — shown while waiting for first token */}
          <AnimatePresence>
            {loading && !streamingContent && !isCompacting && (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.2 }}
              >
                <TypingIndicator verbs={thinkingVerbs} />
              </motion.div>
            )}
          </AnimatePresence>
          {/* Streaming content — shown once tokens arrive */}
          {isStreaming && streamingContent && (
            <div className="flex gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-xs">
                AI
              </div>
              <div className="flex-1 max-w-[85%] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl rounded-bl-sm px-4 py-3 text-gray-900 dark:text-gray-100">
                <Suspense
                  fallback={
                    <div className="prose prose-sm max-w-none dark:prose-invert whitespace-pre-wrap">
                      {streamingContent}
                    </div>
                  }
                >
                  <MarkdownRenderer content={normalizedStreaming} sendDisabled />
                </Suspense>
                <StreamingCursor />
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <ScrollToBottomButton show={showScrollButton} onClick={() => scrollToBottom('smooth')} />
    </div>
  )
}
