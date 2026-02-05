/**
 * MessageList component
 * Displays conversation turns with auto-scroll and grouping
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 */

import { useEffect, useRef, ReactNode, useState } from 'react'
import { useChat } from '../context/ChatContext'
import { ConversationTurn } from '../types'
import { TurnBubble } from './TurnBubble'
import ScrollToBottomButton from './ScrollToBottomButton'

interface MessageListProps {
  /** Custom render function for user turns */
  renderUserTurn?: (turn: ConversationTurn) => ReactNode
  /** Custom render function for assistant turns */
  renderAssistantTurn?: (turn: ConversationTurn) => ReactNode
}

export function MessageList({ renderUserTurn, renderAssistantTurn }: MessageListProps) {
  const { turns, streamingContent, isStreaming } = useChat()
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [showScrollButton, setShowScrollButton] = useState(false)

  // Auto-scroll to bottom on new turns or streaming content
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [turns.length, streamingContent])

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

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  return (
    <div className="relative flex-1 min-h-0">
      <div ref={containerRef} className="h-full overflow-y-auto px-4 py-6">
        <div className="max-w-4xl mx-auto space-y-6">
          {turns.map((turn) => (
            <TurnBubble
              key={turn.id}
              turn={turn}
              renderUserTurn={renderUserTurn}
              renderAssistantTurn={renderAssistantTurn}
            />
          ))}
          {isStreaming && streamingContent && (
            <div className="flex gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white">
                AI
              </div>
              <div className="flex-1 max-w-[80%] rounded-2xl px-4 py-3 bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-sm">
                <div className="prose prose-sm max-w-none dark:prose-invert">
                  {streamingContent}
                  <span className="inline-block w-2 h-4 ml-1 bg-primary-600 animate-pulse" />
                </div>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <ScrollToBottomButton show={showScrollButton} onClick={scrollToBottom} />
    </div>
  )
}
