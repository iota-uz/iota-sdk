/**
 * Main ChatSession component
 * Composes ChatHeader, MessageList, and MessageInput
 */

import { ReactNode } from 'react'
import { ChatSessionProvider, useChat } from '../context/ChatContext'
import { ChatDataSource, Message } from '../types'
import { ChatHeader } from './ChatHeader'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'

interface ChatSessionProps {
  dataSource: ChatDataSource
  sessionId?: string
  isReadOnly?: boolean
  renderUserMessage?: (message: Message) => ReactNode
  renderAssistantMessage?: (message: Message) => ReactNode
  className?: string
}

function ChatSessionCore({
  isReadOnly,
  renderUserMessage,
  renderAssistantMessage,
  className = '',
}: Omit<ChatSessionProps, 'dataSource' | 'sessionId'>) {
  const {
    session,
    fetching,
    error,
    message,
    setMessage,
    loading,
    handleSubmit,
    messageQueue,
    handleUnqueue
  } = useChat()

  if (fetching) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-gray-500">Loading session...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-red-500">Error: {error}</div>
      </div>
    )
  }

  return (
    <main className={`flex-1 flex flex-col overflow-hidden min-h-0 bg-gray-50 dark:bg-gray-900 ${className}`}>
      <ChatHeader session={session} />
      <MessageList
        renderUserMessage={renderUserMessage}
        renderAssistantMessage={renderAssistantMessage}
      />
      {!isReadOnly && (
        <MessageInput
          message={message}
          loading={loading}
          fetching={fetching}
          onMessageChange={setMessage}
          onSubmit={handleSubmit}
          messageQueue={messageQueue}
          onUnqueue={handleUnqueue}
        />
      )}
    </main>
  )
}

export function ChatSession(props: ChatSessionProps) {
  const { dataSource, sessionId, ...coreProps } = props

  return (
    <ChatSessionProvider dataSource={dataSource} sessionId={sessionId}>
      <ChatSessionCore {...coreProps} />
    </ChatSessionProvider>
  )
}
