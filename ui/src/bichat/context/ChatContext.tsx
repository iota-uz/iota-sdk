/**
 * Chat session context provider and hook
 * Manages state for chat sessions including turns, loading, streaming, and HITL
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 */

import { createContext, useContext, useState, useCallback, useEffect, ReactNode, useRef } from 'react'
import type {
  ChatDataSource,
  Session,
  ConversationTurn,
  PendingQuestion,
  QuestionAnswers,
  Attachment,
  ImageAttachment,
  QueuedMessage,
  CodeOutput,
  ChatSessionContextValue,
} from '../types'
import { RateLimiter } from '../utils/RateLimiter'

const ChatSessionContext = createContext<ChatSessionContextValue | null>(null)

interface ChatSessionProviderProps {
  dataSource: ChatDataSource
  sessionId?: string
  rateLimiter?: RateLimiter
  children: ReactNode
}

// Default rate limiter configuration
const DEFAULT_RATE_LIMIT_CONFIG = {
  maxRequests: 20,
  windowMs: 60000, // 1 minute
}

/**
 * Generate a temporary ID for optimistic updates
 */
function generateTempId(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
}

/**
 * Create a new conversation turn with user message (assistant turn pending)
 */
function createPendingTurn(
  sessionId: string,
  content: string,
  attachments: Attachment[] = []
): ConversationTurn {
  const now = new Date().toISOString()
  return {
    id: generateTempId('turn'),
    sessionId,
    userTurn: {
      id: generateTempId('user'),
      content,
      attachments,
      createdAt: now,
    },
    // No assistantTurn yet - it will be added when streaming completes
    createdAt: now,
  }
}

export function ChatSessionProvider({
  dataSource,
  sessionId,
  rateLimiter: externalRateLimiter,
  children
}: ChatSessionProviderProps) {
  // Form state
  const [message, setMessage] = useState('')

  // Turn-based state (replaces messages)
  const [turns, setTurns] = useState<ConversationTurn[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Session state
  const [currentSessionId, setCurrentSessionId] = useState<string | undefined>(sessionId)
  const [session, setSession] = useState<Session | null>(null)
  const [fetching, setFetching] = useState(false)

  // Question state
  const [pendingQuestion, setPendingQuestion] = useState<PendingQuestion | null>(null)

  // Streaming state
  const [streamingContent, setStreamingContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Queue and code outputs state
  const [messageQueue, setMessageQueue] = useState<QueuedMessage[]>([])
  const [codeOutputs, setCodeOutputs] = useState<CodeOutput[]>([])

  // Rate limiter (use provided or create default)
  const rateLimiterRef = useRef<RateLimiter>(
    externalRateLimiter || new RateLimiter(DEFAULT_RATE_LIMIT_CONFIG)
  )

  // Update sessionId when prop changes
  useEffect(() => {
    setCurrentSessionId(sessionId)
  }, [sessionId])

  // Fetch session on mount/sessionId change
  useEffect(() => {
    if (!currentSessionId || currentSessionId === 'new') {
      setSession(null)
      setTurns([])
      setPendingQuestion(null)
      setFetching(false)
      return
    }

    let cancelled = false

    setFetching(true)
    setError(null)

    dataSource
      .fetchSession(currentSessionId)
      .then((state) => {
        if (cancelled) return

        if (state) {
          setSession(state.session)
          setTurns(state.turns)
          setPendingQuestion(state.pendingQuestion || null)
        } else {
          setError('Session not found')
        }
        setFetching(false)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err.message || 'Failed to load session')
        setFetching(false)
      })

    return () => {
      cancelled = true
    }
  }, [dataSource, currentSessionId])

  const handleCopy = useCallback(async (text: string) => {
    await navigator.clipboard.writeText(text)
  }, [])

  const sendMessageDirect = useCallback(
    async (content: string, attachments: Attachment[] = []): Promise<void> => {
      if (!content.trim() || loading) return

      // Check rate limit
      if (!rateLimiterRef.current.canMakeRequest()) {
        const timeUntilNext = rateLimiterRef.current.getTimeUntilNextRequest()
        const seconds = Math.ceil(timeUntilNext / 1000)
        setError(`Rate limit exceeded. Please wait ${seconds} seconds before sending another message.`)
        setTimeout(() => setError(null), 5000)
        return
      }

      setMessage('')
      setLoading(true)
      setError(null)
      setStreamingContent('')

      // Create abort controller for this request
      abortControllerRef.current = new AbortController()

      // Add optimistic turn (user message only, no assistant response yet)
      const tempTurn = createPendingTurn(currentSessionId || 'new', content, attachments)
      setTurns((prev) => [...prev, tempTurn])

      try {
        // Create session if needed
        let activeSessionId = currentSessionId
        let shouldNavigateAfter = false

        if (!activeSessionId || activeSessionId === 'new') {
          const result = await dataSource.createSession()
          if (result) {
            activeSessionId = result.id
            setCurrentSessionId(activeSessionId)
            shouldNavigateAfter = true
          }
        }

        // Stream response
        let accumulatedContent = ''
        let createdSessionId: string | undefined
        setIsStreaming(true)

        for await (const chunk of dataSource.sendMessage(
          activeSessionId || 'new',
          content,
          attachments,
          abortControllerRef.current?.signal
        )) {
          // Check if cancelled
          if (abortControllerRef.current?.signal.aborted) {
            break
          }

          if (chunk.type === 'chunk' && chunk.content) {
            accumulatedContent += chunk.content
            setStreamingContent(accumulatedContent)
          } else if (chunk.type === 'error') {
            throw new Error(chunk.error || 'Stream error')
          } else if (chunk.type === 'done') {
            if (chunk.sessionId) {
              createdSessionId = chunk.sessionId
            }
            // Refetch session to get final state with proper turns
            const finalSessionId = createdSessionId || activeSessionId
            if (finalSessionId && finalSessionId !== 'new') {
              const state = await dataSource.fetchSession(finalSessionId)
              if (state) {
                setSession(state.session)
                setTurns(state.turns)
                setPendingQuestion(state.pendingQuestion || null)
              }
            }
          } else if (chunk.type === 'user_message' && chunk.sessionId) {
            createdSessionId = chunk.sessionId
          }
        }

        // Navigate to session page if a new session was created
        const targetSessionId = createdSessionId || activeSessionId
        if (shouldNavigateAfter && targetSessionId && targetSessionId !== 'new') {
          dataSource.navigateToSession?.(targetSessionId)
        }
      } catch (err) {
        // Check if error is due to cancellation
        if (err instanceof Error && err.name === 'AbortError') {
          // Stream was cancelled - restore input message
          setMessage(content)
          return
        }

        // Remove optimistic turn on error
        setTurns((prev) => prev.filter((t) => t.id !== tempTurn.id))

        const errorMessage = err instanceof Error ? err.message : 'Failed to send message'
        setError(errorMessage)
        console.error('Send message error:', err)
      } finally {
        setLoading(false)
        setStreamingContent('')
        setIsStreaming(false)
        abortControllerRef.current = null
      }
    },
    [currentSessionId, loading, dataSource]
  )

  const cancelStream = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
      setIsStreaming(false)
      setLoading(false)
    }
  }, [])

  const handleSubmit = useCallback(
    (e: React.FormEvent, attachments: ImageAttachment[] = []) => {
      e.preventDefault()
      if (!message.trim() && attachments.length === 0) return

      // Convert ImageAttachment to Attachment for the data source
      const convertedAttachments: Attachment[] = attachments.map(att => ({
        id: '', // Will be assigned by backend
        filename: att.filename,
        mimeType: att.mimeType,
        sizeBytes: att.sizeBytes,
        base64Data: att.base64Data
      }))

      sendMessageDirect(message, convertedAttachments)
    },
    [message, sendMessageDirect]
  )

  const handleUnqueue = useCallback(() => {
    if (messageQueue.length === 0) {
      return null
    }

    const lastQueued = messageQueue[messageQueue.length - 1]
    setMessageQueue(prev => prev.slice(0, -1))

    return {
      content: lastQueued.content,
      attachments: lastQueued.attachments
    }
  }, [messageQueue])

  const handleRegenerate = useCallback(
    async (turnId: string) => {
      if (!currentSessionId || currentSessionId === 'new') return

      const turn = turns.find((t) => t.id === turnId)
      if (!turn) return

      setLoading(true)
      setError(null)

      try {
        // Resend the user message from this turn
        await sendMessageDirect(turn.userTurn.content, turn.userTurn.attachments)
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to regenerate response'
        setError(errorMessage)
        console.error('Regenerate error:', err)
      } finally {
        setLoading(false)
      }
    },
    [turns, currentSessionId, sendMessageDirect]
  )

  const handleEdit = useCallback(
    async (turnId: string, newContent: string) => {
      if (!currentSessionId || currentSessionId === 'new') {
        setMessage(newContent)
        setTurns((prev) => prev.filter((t) => t.id !== turnId))
        return
      }

      setLoading(true)
      setError(null)

      try {
        // For edit, we resend with the edited content
        await sendMessageDirect(newContent, [])
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to edit message'
        setError(errorMessage)
        console.error('Edit error:', err)
      } finally {
        setLoading(false)
      }
    },
    [currentSessionId, sendMessageDirect]
  )

  const handleSubmitQuestionAnswers = useCallback(
    (answers: QuestionAnswers) => {
      if (!currentSessionId || !pendingQuestion) return

      setLoading(true)
      setError(null)
      const previousPendingQuestion = pendingQuestion
      setPendingQuestion(null)

      ;(async () => {
        try {
          const result = await dataSource.submitQuestionAnswers(
            currentSessionId,
            previousPendingQuestion.id,
            answers
          )

          if (result.success) {
            if (currentSessionId !== 'new') {
              try {
                const state = await dataSource.fetchSession(currentSessionId)
                if (state) {
                  setTurns(state.turns)
                  setPendingQuestion(state.pendingQuestion || null)
                } else {
                  setPendingQuestion(previousPendingQuestion)
                  setError('Failed to load updated session')
                }
              } catch (fetchErr) {
                setPendingQuestion(previousPendingQuestion)
                const errorMessage =
                  fetchErr instanceof Error
                    ? fetchErr.message
                    : 'Failed to load updated session'
                setError(errorMessage)
              }
            }
          } else {
            setPendingQuestion(previousPendingQuestion)
            setError(result.error || 'Failed to submit answers')
          }
        } catch (err) {
          setPendingQuestion(previousPendingQuestion)
          const errorMessage =
            err instanceof Error ? err.message : 'Failed to submit answers'
          setError(errorMessage)
        } finally {
          setLoading(false)
        }
      })()
    },
    [currentSessionId, pendingQuestion, dataSource]
  )

  const handleCancelPendingQuestion = useCallback(async () => {
    if (!currentSessionId || !pendingQuestion) return

    try {
      const result = await dataSource.cancelPendingQuestion(pendingQuestion.id)

      if (result.success) {
        setPendingQuestion(null)
      } else {
        setError(result.error || 'Failed to cancel question')
      }
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : 'Failed to cancel question'
      setError(errorMessage)
    }
  }, [currentSessionId, pendingQuestion, dataSource])

  const value: ChatSessionContextValue = {
    // State
    message,
    turns,
    loading,
    error,
    currentSessionId,
    pendingQuestion,
    session,
    fetching,
    streamingContent,
    isStreaming,
    messageQueue,
    codeOutputs,

    // Setters
    setMessage,
    setError,
    setCodeOutputs,

    // Handlers
    handleCopy,
    handleRegenerate,
    handleEdit,
    handleSubmit,
    handleSubmitQuestionAnswers,
    handleCancelPendingQuestion,
    handleUnqueue,
    sendMessage: sendMessageDirect,
    cancel: cancelStream,
  }

  return (
    <ChatSessionContext.Provider value={value}>
      {children}
    </ChatSessionContext.Provider>
  )
}

export function useChat(): ChatSessionContextValue {
  const context = useContext(ChatSessionContext)
  if (!context) {
    throw new Error('useChat must be used within ChatSessionProvider')
  }
  return context
}
