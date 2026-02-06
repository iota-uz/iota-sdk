/**
 * Main ChatSession component
 * Composes ChatHeader, MessageList, and MessageInput
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 *
 * Supports customization via slots:
 * - headerSlot: Custom content above the message list
 * - welcomeSlot: Replace the default welcome screen for new chats
 * - logoSlot: Custom logo in the header
 * - actionsSlot: Custom action buttons in the header
 */

import { ReactNode } from 'react'
import { ChatSessionProvider, useChat } from '../context/ChatContext'
import { ChatDataSource, ConversationTurn } from '../types'
import { ChatHeader } from './ChatHeader'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'
import WelcomeContent from './WelcomeContent'
import { useTranslation } from '../hooks/useTranslation'

interface ChatSessionProps {
  dataSource: ChatDataSource
  sessionId?: string
  isReadOnly?: boolean
  /** Custom render function for user turns */
  renderUserTurn?: (turn: ConversationTurn) => ReactNode
  /** Custom render function for assistant turns */
  renderAssistantTurn?: (turn: ConversationTurn) => ReactNode
  className?: string
  /** Custom content to display as header */
  headerSlot?: ReactNode
  /** Custom welcome screen component (replaces default WelcomeContent) */
  welcomeSlot?: ReactNode
  /** Custom logo for the header */
  logoSlot?: ReactNode
  /** Custom action buttons for the header */
  actionsSlot?: ReactNode
  /** Callback when user navigates back */
  onBack?: () => void
  /** Custom verbs for the typing indicator (e.g. ['Thinking', 'Analyzing', ...]) */
  thinkingVerbs?: string[]
}

function ChatSessionCore({
  isReadOnly,
  renderUserTurn,
  renderAssistantTurn,
  className = '',
  headerSlot,
  welcomeSlot,
  logoSlot,
  actionsSlot,
  onBack,
  thinkingVerbs,
}: Omit<ChatSessionProps, 'dataSource' | 'sessionId'>) {
  const { t } = useTranslation()
  const {
    session,
    turns,
    fetching,
    error,
    inputError,
    message,
    setMessage,
    setInputError,
    loading,
    handleSubmit,
    messageQueue,
    handleUnqueue,
    debugMode,
  } = useChat()

  if (fetching) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-gray-500 dark:text-gray-400">{t('input.processing')}</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-red-500 dark:text-red-400">{t('error.generic')}: {error}</div>
      </div>
    )
  }

  // Show welcome screen for new sessions with no turns
  const showWelcome = !session && turns.length === 0

  const handlePromptSelect = (prompt: string) => {
    setMessage(prompt)
  }

  return (
    <main
      className={`flex-1 flex flex-col overflow-hidden min-h-0 bg-gray-50 dark:bg-gray-900 ${className}`}
    >
      {/* Header slot or default header */}
      {headerSlot || (
        <ChatHeader session={session} onBack={onBack} logoSlot={logoSlot} actionsSlot={actionsSlot} />
      )}

      {/* Welcome: single centered unit (content + input + disclaimer) */}
      {showWelcome ? (
        <div className="flex-1 overflow-auto flex flex-col">
          <div className="flex-1 flex items-center justify-center px-4 py-8">
            <div className="w-full max-w-5xl">
              {welcomeSlot || <WelcomeContent onPromptSelect={handlePromptSelect} disabled={loading} />}
              {!isReadOnly && (
                <MessageInput
                  message={message}
                  loading={loading}
                  fetching={fetching}
                  commandError={inputError}
                  onClearCommandError={() => setInputError(null)}
                  debugMode={debugMode}
                  onMessageChange={setMessage}
                  onSubmit={handleSubmit}
                  messageQueue={messageQueue}
                  onUnqueue={handleUnqueue}
                  containerClassName="pt-6 px-6"
                  formClassName="mx-auto"
                />
              )}
              <p className="mt-4 text-center text-xs text-gray-500 dark:text-gray-400 pb-1">
                {t('welcome.disclaimer')}
              </p>
            </div>
          </div>
        </div>
      ) : (
        <>
          <MessageList
            renderUserTurn={renderUserTurn}
            renderAssistantTurn={renderAssistantTurn}
            thinkingVerbs={thinkingVerbs}
          />
          {!isReadOnly && (
            <MessageInput
              message={message}
              loading={loading}
              fetching={fetching}
              commandError={inputError}
              onClearCommandError={() => setInputError(null)}
              debugMode={debugMode}
              onMessageChange={setMessage}
              onSubmit={handleSubmit}
              messageQueue={messageQueue}
              onUnqueue={handleUnqueue}
            />
          )}
        </>
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

export type { ChatSessionProps }
