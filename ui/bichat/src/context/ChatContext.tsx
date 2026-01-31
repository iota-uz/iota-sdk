/**
 * Chat session context provider and hook
 * Manages state for chat sessions including messages, loading, streaming, and HITL
 */

import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import type {
  ChatDataSource,
  ChatSession,
  Message,
  PendingQuestion,
  QuestionAnswers,
  Attachment,
  StreamChunk,
  ChatSessionContextValue,
  MessageRole,
} from '../types'

const ChatSessionContext = createContext<ChatSessionContextValue | null>(null)

interface ChatSessionProviderProps {
  dataSource: ChatDataSource
  sessionId?: string
  children: ReactNode
}

const MAX_MESSAGES_PER_MINUTE = 20
const RATE_LIMIT_WINDOW_MS = 60000

class RateLimiter {
  private timestamps: number[] = []

  canMakeRequest(): boolean {
    const now = Date.now()
    this.timestamps = this.timestamps.filter(t => now - t < RATE_LIMIT_WINDOW_MS)

    if (this.timestamps.length >= MAX_MESSAGES_PER_MINUTE) {
      return false
    }

    this.timestamps.push(now)
    return true
  }
}

const rateLimiter = new RateLimiter()

export function ChatSessionProvider({ dataSource, sessionId, children }: ChatSessionProviderProps) {
  // Form state
  const [message, setMessage] = useState('')

  // Message state
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Session state
  const [currentSessionId, setCurrentSessionId] = useState<string | undefined>(sessionId)
  const [session, setSession] = useState<ChatSession | null>(null)
  const [fetching, setFetching] = useState(false)

  // Question state
  const [pendingQuestion, setPendingQuestion] = useState<PendingQuestion | null>(null)

  // Streaming state
  const [streamingContent, setStreamingContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)

  // Update sessionId when prop changes
  useEffect(() => {
    setCurrentSessionId(sessionId)
  }, [sessionId])

  // Fetch session on mount/sessionId change
  useEffect(() => {
    if (!currentSessionId || currentSessionId === 'new') {
      setSession(null)
      setMessages([])
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
          setMessages(state.messages)
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
      if (!rateLimiter.canMakeRequest()) {
        setError('Rate limit exceeded. Please wait before sending another message.')
        setTimeout(() => setError(null), 5000)
        return
      }

      setMessage('')
      setLoading(true)
      setError(null)
      setStreamingContent('')

      // Add optimistic user message
      const tempUserMessage: Message = {
        id: `temp-user-${Date.now()}`,
        sessionId: currentSessionId || 'new',
        role: MessageRole.User,
        content,
        createdAt: new Date().toISOString(),
      }
      setMessages((prev) => [...prev, tempUserMessage])

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
          attachments
        )) {
          if (chunk.type === 'chunk' && chunk.content) {
            accumulatedContent += chunk.content
            setStreamingContent(accumulatedContent)
          } else if (chunk.type === 'error') {
            throw new Error(chunk.error || 'Stream error')
          } else if (chunk.type === 'done') {
            if (chunk.sessionId) {
              createdSessionId = chunk.sessionId
            }
            // Refetch session to get final state
            const finalSessionId = createdSessionId || activeSessionId
            if (finalSessionId && finalSessionId !== 'new') {
              const state = await dataSource.fetchSession(finalSessionId)
              if (state) {
                setSession(state.session)
                setMessages(state.messages)
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
        // Remove optimistic message on error
        setMessages((prev) => prev.filter((m) => m.id !== tempUserMessage.id))

        const errorMessage = err instanceof Error ? err.message : 'Failed to send message'
        setError(errorMessage)
        console.error('Send message error:', err)
      } finally {
        setLoading(false)
        setStreamingContent('')
        setIsStreaming(false)
      }
    },
    [currentSessionId, loading, dataSource]
  )

  const handleSubmit = useCallback(
    (e: React.FormEvent, attachments: Attachment[] = []) => {
      e.preventDefault()
      if (!message.trim()) return
      sendMessageDirect(message, attachments)
    },
    [message, sendMessageDirect]
  )

  const handleRegenerate = useCallback(
    async (messageId: string) => {
      if (!currentSessionId || currentSessionId === 'new') return

      const messageIndex = messages.findIndex((m) => m.id === messageId)
      if (messageIndex <= 0) return

      setLoading(true)
      setError(null)

      try {
        // Find the user message before this assistant message
        const userMessage = messages[messageIndex - 1]
        if (userMessage && userMessage.role === MessageRole.User) {
          await sendMessageDirect(userMessage.content, [])
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to regenerate message'
        setError(errorMessage)
        console.error('Regenerate error:', err)
      } finally {
        setLoading(false)
      }
    },
    [messages, currentSessionId, sendMessageDirect]
  )

  const handleEdit = useCallback(
    async (messageId: string, newContent: string) => {
      if (!currentSessionId || currentSessionId === 'new') {
        setMessage(newContent)
        setMessages((prev) => prev.filter((m) => m.id !== messageId))
        return
      }

      setLoading(true)
      setError(null)

      try {
        // For edit, we resend the edited message
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
                  setMessages(state.messages)
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
    messages,
    loading,
    error,
    currentSessionId,
    pendingQuestion,
    session,
    fetching,
    streamingContent,
    isStreaming,

    // Setters
    setMessage,
    setError,

    // Handlers
    handleCopy,
    handleRegenerate,
    handleEdit,
    handleSubmit,
    handleSubmitQuestionAnswers,
    handleCancelPendingQuestion,
    sendMessage: sendMessageDirect,
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
