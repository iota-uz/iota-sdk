/**
 * Landing page + "new chat" view.
 * Uses ChatSessionProvider with sessionId="new" so the first message streams via SSE.
 */

import { useEffect, useMemo, useRef } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  ChatSessionProvider,
  useChat,
  ChatHeader,
  WelcomeContent,
  MessageList,
  MessageInput,
} from '@iotauz/iota-sdk/bichat'
import { useBiChatDataSource } from '../data/bichatDataSource'

type LocationState = {
  prompt?: string
}

function LandingChat({ initialPrompt }: { initialPrompt: string }) {
  const {
    session,
    messages,
    fetching,
    error,
    message,
    setMessage,
    loading,
    handleSubmit,
    sendMessage,
    messageQueue,
    handleUnqueue,
  } = useChat()

  const seededRef = useRef(false)

  useEffect(() => {
    if (!initialPrompt || seededRef.current) return
    seededRef.current = true
    void sendMessage(initialPrompt)
  }, [initialPrompt, sendMessage])

  if (fetching) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-gray-500 dark:text-gray-400">Loading...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-red-500 dark:text-red-400">Error: {error}</div>
      </div>
    )
  }

  const showWelcome = !session && messages.length === 0

  return (
    <main className="flex-1 flex flex-col overflow-hidden min-h-0 bg-gray-50 dark:bg-gray-900">
      <ChatHeader session={session} />

      {showWelcome ? (
        <div className="flex-1 flex items-center justify-center overflow-auto">
          <WelcomeContent
            onPromptSelect={(prompt) => {
              void sendMessage(prompt)
            }}
            disabled={loading}
          />
        </div>
      ) : (
        <MessageList />
      )}

      <MessageInput
        message={message}
        loading={loading}
        fetching={fetching}
        onMessageChange={setMessage}
        onSubmit={handleSubmit}
        messageQueue={messageQueue}
        onUnqueue={handleUnqueue}
        placeholder="Ask BiChat about your business data..."
      />
    </main>
  )
}

export default function HomePage() {
  const navigate = useNavigate()
  const location = useLocation()

  const initialPrompt = useMemo(() => {
    const state = (location.state || {}) as LocationState
    return state.prompt?.trim() || ''
  }, [location.state])

  useEffect(() => {
    if (!initialPrompt) return
    navigate('.', { replace: true, state: {} })
  }, [initialPrompt, navigate])

  const dataSource = useBiChatDataSource((sessionId: string) => navigate(`/session/${sessionId}`))

  return (
    <ChatSessionProvider dataSource={dataSource} sessionId="new">
      <LandingChat initialPrompt={initialPrompt} />
    </ChatSessionProvider>
  )
}
