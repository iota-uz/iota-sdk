/**
 * MessageList component
 * Displays messages with auto-scroll and grouping
 */

import { useEffect, useRef, ReactNode, useState } from 'react'
import { useChat } from '../context/ChatContext'
import { Message } from '../types'
import { TurnBubble } from './TurnBubble'
import ScrollToBottomButton from './ScrollToBottomButton'

interface MessageListProps {
  renderUserMessage?: (message: Message) => ReactNode
  renderAssistantMessage?: (message: Message) => ReactNode
}

export function MessageList({ renderUserMessage, renderAssistantMessage }: MessageListProps) {
  const { messages, streamingContent, isStreaming } = useChat()
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const [showScrollButton, setShowScrollButton] = useState(false)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length, streamingContent])

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
          {messages.map((message) => (
            <TurnBubble
              key={message.id}
              message={message}
              renderUserMessage={renderUserMessage}
              renderAssistantMessage={renderAssistantMessage}
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
