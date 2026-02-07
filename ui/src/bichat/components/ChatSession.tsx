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

import { ReactNode, useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { Sidebar } from '@phosphor-icons/react'
import { ChatSessionProvider, useChat } from '../context/ChatContext'
import { ChatDataSource, ConversationTurn } from '../types'
import { RateLimiter } from '../utils/RateLimiter'
import { ChatHeader } from './ChatHeader'
import { MessageList } from './MessageList'
import { MessageInput } from './MessageInput'
import WelcomeContent from './WelcomeContent'
import { useTranslation } from '../hooks/useTranslation'
import { SessionArtifactsPanel } from './SessionArtifactsPanel'

interface ChatSessionProps {
  dataSource: ChatDataSource
  sessionId?: string
  /** Optional rate limiter to throttle sendMessage */
  rateLimiter?: RateLimiter
  /** Alias for isReadOnly (preferred) */
  readOnly?: boolean
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
  /** Enables the built-in right-side artifacts panel for persisted session artifacts */
  showArtifactsPanel?: boolean
  /** Initial expanded state for artifacts panel when no persisted preference exists */
  artifactsPanelDefaultExpanded?: boolean
  /** localStorage key for artifacts panel expanded/collapsed state */
  artifactsPanelStorageKey?: string
}

function ChatSessionCore({
  dataSource,
  readOnly,
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
  showArtifactsPanel = false,
  artifactsPanelDefaultExpanded = false,
  artifactsPanelStorageKey = 'bichat.artifacts-panel.expanded',
}: Omit<ChatSessionProps, 'sessionId'>) {
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
    sessionDebugUsage,
    debugLimits,
    currentSessionId,
    isStreaming,
  } = useChat()

  const effectiveReadOnly = Boolean(readOnly ?? isReadOnly)

  const [artifactsPanelExpanded, setArtifactsPanelExpanded] = useState(
    artifactsPanelDefaultExpanded
  )

  useEffect(() => {
    if (!showArtifactsPanel) {
      return
    }

    let nextValue = artifactsPanelDefaultExpanded
    if (typeof window !== 'undefined') {
      const stored = window.localStorage.getItem(artifactsPanelStorageKey)
      if (stored !== null) {
        nextValue = stored === 'true'
      }
    }

    setArtifactsPanelExpanded(nextValue)
  }, [artifactsPanelDefaultExpanded, artifactsPanelStorageKey, showArtifactsPanel])

  if (fetching) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-gray-500 dark:text-gray-400">{t('input.processing')}</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-red-500 dark:text-red-400">
          {t('error.generic')}: {error}
        </div>
      </div>
    )
  }

  // Show welcome screen for new sessions with no turns
  const showWelcome = !session && turns.length === 0
  const activeSessionId =
    session?.id ||
    (currentSessionId && currentSessionId !== 'new'
      ? currentSessionId
      : undefined)

  const supportsArtifactsPanel = typeof dataSource.fetchSessionArtifacts === 'function'
  const showArtifactsControls = Boolean(showArtifactsPanel && supportsArtifactsPanel && activeSessionId)
  const shouldRenderArtifactsPanel = Boolean(
    showArtifactsControls && artifactsPanelExpanded && !showWelcome && activeSessionId
  )

  const handlePromptSelect = (prompt: string) => {
    setMessage(prompt)
  }

  const handleToggleArtifactsPanel = () => {
    const nextValue = !artifactsPanelExpanded
    setArtifactsPanelExpanded(nextValue)

    if (typeof window !== 'undefined') {
      window.localStorage.setItem(artifactsPanelStorageKey, nextValue ? 'true' : 'false')
    }
  }

  const headerActions = showArtifactsControls ? (
    <>
      <button
        type="button"
        onClick={handleToggleArtifactsPanel}
        className={[
          'inline-flex cursor-pointer items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-all duration-150',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
          artifactsPanelExpanded
            ? 'bg-primary-50 text-primary-700 hover:bg-primary-100 dark:bg-primary-950/30 dark:text-primary-300 dark:hover:bg-primary-900/40'
            : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200',
        ].join(' ')}
        aria-label={artifactsPanelExpanded ? t('artifacts.toggleHide') : t('artifacts.toggleShow')}
        title={artifactsPanelExpanded ? t('artifacts.toggleHide') : t('artifacts.toggleShow')}
      >
        <Sidebar className="h-4 w-4" weight={artifactsPanelExpanded ? 'duotone' : 'regular'} />
        {t('artifacts.title')}
      </button>
      {actionsSlot}
    </>
  ) : (
    actionsSlot
  )

  return (
    <main
      className={`flex min-h-0 flex-1 flex-col overflow-hidden bg-gray-50 dark:bg-gray-900 ${className}`}
    >
      {headerSlot || (
        <ChatHeader
          session={session}
          onBack={onBack}
          readOnly={effectiveReadOnly}
          logoSlot={logoSlot}
          actionsSlot={headerActions}
        />
      )}

      <div className="relative flex min-h-0 flex-1 overflow-hidden">
        <div className="flex min-h-0 flex-1 flex-col">
          {showWelcome ? (
            <div className="flex flex-1 flex-col overflow-auto">
              <div className="flex flex-1 items-center justify-center px-4 py-8">
                <div className="w-full max-w-5xl">
                  {welcomeSlot || (
                    <WelcomeContent onPromptSelect={handlePromptSelect} disabled={loading} />
                  )}
                  {!effectiveReadOnly && (
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
                      containerClassName="pt-6 px-6"
                      formClassName="mx-auto"
                    />
                  )}
                  <p className="mt-4 pb-1 text-center text-xs text-gray-500 dark:text-gray-400">
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
                readOnly={effectiveReadOnly}
              />
              {!effectiveReadOnly && (
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
                />
              )}
            </>
          )}
        </div>

        {/* Desktop: persistent slot with animated width so main content expands in sync */}
        <motion.div
          className="hidden lg:flex lg:min-h-0 shrink-0 overflow-hidden"
          animate={{
            width: shouldRenderArtifactsPanel && activeSessionId ? '22rem' : 0,
          }}
          transition={{ type: 'spring', stiffness: 320, damping: 32 }}
        >
          {shouldRenderArtifactsPanel && activeSessionId && (
            <motion.div
              className="flex min-h-0 w-[22rem]"
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              transition={{ type: 'spring', stiffness: 320, damping: 32 }}
            >
              <SessionArtifactsPanel
                dataSource={dataSource}
                sessionId={activeSessionId}
                isStreaming={isStreaming}
                className="min-h-0"
              />
            </motion.div>
          )}
        </motion.div>

        <AnimatePresence>
          {shouldRenderArtifactsPanel && activeSessionId && (
            <motion.div
              key="artifacts-mobile"
              className="fixed inset-0 z-40 flex lg:hidden"
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              exit={{ x: '100%' }}
              transition={{ type: 'spring', stiffness: 320, damping: 32 }}
              role="dialog"
              aria-modal="true"
            >
              <motion.button
                type="button"
                className="flex-1 bg-black/40"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                onClick={handleToggleArtifactsPanel}
                aria-label={t('common.close')}
              />
              <SessionArtifactsPanel
                dataSource={dataSource}
                sessionId={activeSessionId}
                isStreaming={isStreaming}
                className="flex h-full w-full max-w-sm min-h-0"
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </main>
  )
}

export function ChatSession(props: ChatSessionProps) {
  const { dataSource, sessionId, rateLimiter, ...coreProps } = props

  return (
    <ChatSessionProvider dataSource={dataSource} sessionId={sessionId} rateLimiter={rateLimiter}>
      <ChatSessionCore dataSource={dataSource} {...coreProps} />
    </ChatSessionProvider>
  )
}

export type { ChatSessionProps }
