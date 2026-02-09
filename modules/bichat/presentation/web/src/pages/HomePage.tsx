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
  useTranslation,
  RateLimiter,
} from '@iota-uz/sdk/bichat'
import { useBiChatDataSource } from '../data/bichatDataSource'
import { toRPCErrorDisplay } from '../utils/rpcErrors'

type LocationState = {
  prompt?: string
}

function LandingChat({ initialPrompt }: { initialPrompt: string }) {
  const { t } = useTranslation()
  const {
    session,
    turns,
    fetching,
    error,
    message,
    setMessage,
    inputError,
    setInputError,
    debugMode,
    sessionDebugUsage,
    debugLimits,
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
    const display = toRPCErrorDisplay(error, 'Failed to load BiChat')
    return (
      <div className="flex items-center justify-center h-full">
        <div
          className={
            display.isPermissionDenied
              ? 'max-w-md mx-4 p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl'
              : 'max-w-md mx-4 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl'
          }
        >
          <p
            className={
              display.isPermissionDenied
                ? 'text-sm text-amber-700 dark:text-amber-300 font-medium'
                : 'text-sm text-red-700 dark:text-red-300 font-medium'
            }
          >
            {display.title}
          </p>
          <p
            className={
              display.isPermissionDenied
                ? 'mt-1 text-sm text-amber-600 dark:text-amber-400'
                : 'mt-1 text-sm text-red-600 dark:text-red-400'
            }
          >
            {display.description}
          </p>
        </div>
      </div>
    )
  }

  const showWelcome = !session && turns.length === 0

  return (
    <main className="flex-1 flex flex-col overflow-hidden min-h-0 bg-gray-50 dark:bg-gray-900">
      <ChatHeader session={session} />

      {showWelcome ? (
        <div className="flex-1 overflow-auto flex flex-col">
          <div className="flex-1 flex items-center justify-center px-4 py-8">
            <div className="w-full max-w-5xl">
              <WelcomeContent
                onPromptSelect={(prompt: string) => {
                  void sendMessage(prompt)
                }}
                disabled={loading}
              />
              <MessageInput
                message={message}
                loading={loading}
                fetching={fetching}
                commandError={inputError}
                onClearCommandError={() => setInputError(null)}
                debugMode={debugMode}
                debugSessionUsage={sessionDebugUsage}
                debugLimits={debugLimits}
                onMessageChange={setMessage}
                onSubmit={handleSubmit}
                messageQueue={messageQueue}
                onUnqueue={handleUnqueue}
                placeholder="Ask BiChat about your business data..."
                containerClassName="pt-8 px-6"
                formClassName="mx-auto"
              />
              <p className="mt-4 text-center text-xs text-gray-500 dark:text-gray-400 pb-1">
                {t('welcome.disclaimer')}
              </p>
            </div>
          </div>
        </div>
      ) : (
        <>
          <MessageList />
          <MessageInput
            message={message}
            loading={loading}
            fetching={fetching}
            commandError={inputError}
            onClearCommandError={() => setInputError(null)}
            debugMode={debugMode}
            debugSessionUsage={sessionDebugUsage}
            debugLimits={debugLimits}
            onMessageChange={setMessage}
            onSubmit={handleSubmit}
            messageQueue={messageQueue}
            onUnqueue={handleUnqueue}
            placeholder="Ask BiChat about your business data..."
          />
        </>
      )}
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
  const rateLimiter = useMemo(
    () => new RateLimiter({ maxRequests: 20, windowMs: 60_000 }),
    []
  )

  return (
    <ChatSessionProvider dataSource={dataSource} sessionId="new" rateLimiter={rateLimiter}>
      <LandingChat initialPrompt={initialPrompt} />
    </ChatSessionProvider>
  )
}
