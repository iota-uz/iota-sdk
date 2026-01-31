/**
 * MessageList component
 * Displays messages with auto-scroll and grouping
 */

import { useEffect, useRef, ReactNode } from 'react'
import { useChat } from '../context/ChatContext'
import { Message, MessageRole } from '../types'
import { TurnBubble } from './TurnBubble'

interface MessageListProps {
  renderUserMessage?: (message: Message) => ReactNode
  renderAssistantMessage?: (message: Message) => ReactNode
}

export function MessageList({ renderUserMessage, renderAssistantMessage }: MessageListProps) {
  const { messages, streamingContent, isStreaming, pendingQuestion } = useChat()
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length, streamingContent])

  if (messages.length === 0 && !streamingContent) {
    return (
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="text-center text-gray-500">
          <p className="text-lg mb-2">Start a conversation</p>
          <p className="text-sm">Send a message to begin</p>
        </div>
      </div>
    )
  }

  return (
    <div className="bichat-messages flex-1 overflow-y-auto px-4 py-6">
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
            <div className="flex-shrink-0 w-8 h-8 rounded-full bg-[var(--bichat-primary)] flex items-center justify-center text-white">
              AI
            </div>
            <div className="flex-1 rounded-2xl px-5 py-3 bg-[var(--bichat-bubble-assistant)] border border-[var(--bichat-border)]">
              <div className="prose prose-sm max-w-none">
                {streamingContent}
                <span className="inline-block w-2 h-4 ml-1 bg-[var(--bichat-primary)] animate-pulse" />
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </div>
  )
}
