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
  DebugTrace,
  ToolCall,
} from '../types'
import { RateLimiter } from '../utils/RateLimiter'
import { hasPermission } from './IotaContext'

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

type SlashCommandName = '/clear' | '/debug' | '/compact'

interface ParsedSlashCommand {
  name: SlashCommandName
  hasArgs: boolean
}

function parseSlashCommand(input: string): ParsedSlashCommand | null {
  const trimmed = input.trim()
  if (!trimmed.startsWith('/')) return null

  const parts = trimmed.split(/\s+/).filter(Boolean)
  if (parts.length === 0) return null

  const candidate = parts[0].toLowerCase()
  if (candidate !== '/clear' && candidate !== '/debug' && candidate !== '/compact') {
    return null
  }

  return {
    name: candidate,
    hasArgs: parts.length > 1,
  }
}

function hasMeaningfulUsage(trace?: DebugTrace['usage']): boolean {
  if (!trace) return false
  return (
    trace.promptTokens > 0 ||
    trace.completionTokens > 0 ||
    trace.totalTokens > 0 ||
    (trace.cachedTokens ?? 0) > 0 ||
    (trace.cost ?? 0) > 0
  )
}

function hasDebugTrace(trace: DebugTrace): boolean {
  return trace.tools.length > 0 || hasMeaningfulUsage(trace.usage) || !!trace.generationMs
}

function inferDebugTraceFromToolCalls(toolCalls?: ToolCall[]): DebugTrace | null {
  if (!toolCalls || toolCalls.length === 0) {
    return null
  }

  const tools = toolCalls
    .filter((call) => !!call.name)
    .map((call) => ({
      callId: call.id,
      name: call.name,
      arguments: call.arguments,
      result: call.result,
      error: call.error,
      durationMs: call.durationMs,
    }))

  if (tools.length === 0) {
    return null
  }

  return { tools }
}

function hydrateDebugTraceFromToolCalls(turns: ConversationTurn[]): ConversationTurn[] {
  let changed = false
  const hydrated = turns.map((turn) => {
    const assistantTurn = turn.assistantTurn
    if (!assistantTurn) {
      return turn
    }

    const inferred = inferDebugTraceFromToolCalls(assistantTurn.toolCalls)
    if (!inferred) {
      return turn
    }

    if (!assistantTurn.debug) {
      changed = true
      return {
        ...turn,
        assistantTurn: {
          ...assistantTurn,
          debug: inferred,
        },
      }
    }

    if (assistantTurn.debug.tools.length > 0) {
      return turn
    }

    changed = true
    return {
      ...turn,
      assistantTurn: {
        ...assistantTurn,
        debug: {
          ...assistantTurn.debug,
          tools: inferred.tools,
        },
      },
    }
  })

  return changed ? hydrated : turns
}

function attachDebugTraceToLatestTurn(turns: ConversationTurn[], trace: DebugTrace): ConversationTurn[] {
  if (!hasDebugTrace(trace)) {
    return turns
  }

  for (let i = turns.length - 1; i >= 0; i--) {
    const turn = turns[i]
    if (!turn.assistantTurn) {
      continue
    }

    const next = [...turns]
    next[i] = {
      ...turn,
      assistantTurn: {
        ...turn.assistantTurn,
        debug: trace,
      },
    }
    return next
  }

  return turns
}

function mergeDebugTraceFromPreviousTurns(previousTurns: ConversationTurn[], nextTurns: ConversationTurn[]): ConversationTurn[] {
  if (previousTurns.length === 0 || nextTurns.length === 0) {
    return nextTurns
  }

  const debugByAssistantTurnID = new Map<string, DebugTrace>()
  for (const turn of previousTurns) {
    const assistantTurn = turn.assistantTurn
    if (assistantTurn?.debug) {
      debugByAssistantTurnID.set(assistantTurn.id, assistantTurn.debug)
    }
  }

  if (debugByAssistantTurnID.size === 0) {
    return nextTurns
  }

  let changed = false
  const merged = nextTurns.map((turn) => {
    const assistantTurn = turn.assistantTurn
    if (!assistantTurn) {
      return turn
    }

    if (assistantTurn.debug) {
      return turn
    }

    const debug = debugByAssistantTurnID.get(assistantTurn.id)
    if (!debug) {
      return turn
    }

    changed = true
    return {
      ...turn,
      assistantTurn: {
        ...assistantTurn,
        debug,
      },
    }
  })

  return changed ? merged : nextTurns
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
  const [inputError, setInputError] = useState<string | null>(null)

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
  const [debugModeBySession, setDebugModeBySession] = useState<Record<string, boolean>>({})
  const [isCompacting, setIsCompacting] = useState(false)
  const [compactionSummary, setCompactionSummary] = useState<string | null>(null)

  // Rate limiter (use provided or create default)
  const rateLimiterRef = useRef<RateLimiter>(
    externalRateLimiter || new RateLimiter(DEFAULT_RATE_LIMIT_CONFIG)
  )

  // Update sessionId when prop changes
  useEffect(() => {
    setCurrentSessionId(sessionId)
  }, [sessionId])

  const debugSessionKey = currentSessionId || 'new'
  const debugMode = debugModeBySession[debugSessionKey] ?? false

  // Fetch session on mount/sessionId change
  useEffect(() => {
    if (!currentSessionId || currentSessionId === 'new') {
      setSession(null)
      setTurns([])
      setPendingQuestion(null)
      setFetching(false)
      setInputError(null)
      return
    }

    let cancelled = false

    setFetching(true)
    setError(null)
    setInputError(null)

    dataSource
      .fetchSession(currentSessionId)
      .then((state) => {
        if (cancelled) return

        if (state) {
          setSession(state.session)
          setTurns((prevTurns) =>
            hydrateDebugTraceFromToolCalls(mergeDebugTraceFromPreviousTurns(prevTurns, state.turns))
          )
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

  const executeSlashCommand = useCallback(
    async (command: ParsedSlashCommand): Promise<boolean> => {
      if (command.hasArgs) {
        setInputError('slash.error.noArguments')
        return true
      }

      setError(null)
      setInputError(null)

      if (command.name === '/debug') {
        if (!hasPermission('bichat.export')) {
          setInputError('slash.error.debugUnauthorized')
          return true
        }

        const nextDebugMode = !debugMode
        setDebugModeBySession((prev) => ({
          ...prev,
          [debugSessionKey]: nextDebugMode,
        }))

        if (nextDebugMode && currentSessionId && currentSessionId !== 'new') {
          try {
            const state = await dataSource.fetchSession(currentSessionId)
            if (state) {
              setSession(state.session)
              setTurns((prevTurns) =>
                hydrateDebugTraceFromToolCalls(mergeDebugTraceFromPreviousTurns(prevTurns, state.turns))
              )
              setPendingQuestion(state.pendingQuestion || null)
            }
          } catch (err) {
            console.error('Failed to refresh session for debug mode:', err)
          }
        } else {
          setTurns((prevTurns) => hydrateDebugTraceFromToolCalls(prevTurns))
        }

        setMessage('')
        return true
      }

      if (!currentSessionId || currentSessionId === 'new') {
        setInputError('slash.error.sessionRequired')
        return true
      }

      if (command.name === '/clear') {
        setLoading(true)
        setStreamingContent('')

        try {
          await dataSource.clearSessionHistory(currentSessionId)
          const state = await dataSource.fetchSession(currentSessionId)
          if (state) {
            setSession(state.session)
            setTurns((prevTurns) =>
              hydrateDebugTraceFromToolCalls(mergeDebugTraceFromPreviousTurns(prevTurns, state.turns))
            )
            setPendingQuestion(state.pendingQuestion || null)
          } else {
            setTurns([])
          }
          setCompactionSummary(null)
          setCodeOutputs([])
          setMessage('')
        } catch (err) {
          setInputError(err instanceof Error ? err.message : 'slash.error.clearFailed')
        } finally {
          setLoading(false)
          setIsStreaming(false)
        }
        return true
      }

      if (command.name === '/compact') {
        setLoading(true)
        setIsCompacting(true)
        setCompactionSummary(null)
        setStreamingContent('')

        try {
          const result = await dataSource.compactSessionHistory(currentSessionId)
          setCompactionSummary(result.summary || '')

          const state = await dataSource.fetchSession(currentSessionId)
          if (state) {
            setSession(state.session)
            setTurns((prevTurns) =>
              hydrateDebugTraceFromToolCalls(mergeDebugTraceFromPreviousTurns(prevTurns, state.turns))
            )
            setPendingQuestion(state.pendingQuestion || null)
          } else {
            setTurns([])
          }

          setCodeOutputs([])
          setMessage('')
        } catch (err) {
          setInputError(err instanceof Error ? err.message : 'slash.error.compactFailed')
        } finally {
          setIsCompacting(false)
          setLoading(false)
          setIsStreaming(false)
        }
        return true
      }

      setInputError('slash.error.unknownCommand')
      return true
    },
    [currentSessionId, dataSource, debugMode, debugSessionKey]
  )

  const sendMessageDirect = useCallback(
    async (content: string, attachments: Attachment[] = []): Promise<void> => {
      if (!content.trim() || loading) return

      const trimmedContent = content.trim()
      if (trimmedContent.startsWith('/')) {
        const maybeCommand = parseSlashCommand(content)
        if (!maybeCommand) {
          setInputError('slash.error.unknownCommand')
          return
        }
        if (attachments.length > 0) {
          setInputError('slash.error.noAttachments')
          return
        }
        await executeSlashCommand(maybeCommand)
        return
      }

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
      setInputError(null)
      setStreamingContent('')
      setCompactionSummary(null)

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
            const createdSessionID = result.id
            activeSessionId = createdSessionID
            setCurrentSessionId(createdSessionID)
            setDebugModeBySession((prev) => {
              if (!debugMode) return prev
              return { ...prev, [createdSessionID]: true }
            })
            shouldNavigateAfter = true
          }
        }

        // Stream response
        let accumulatedContent = ''
        let createdSessionId: string | undefined
        const debugTrace: DebugTrace = { tools: [] }
        setIsStreaming(true)

        for await (const chunk of dataSource.sendMessage(
          activeSessionId || 'new',
          content,
          attachments,
          abortControllerRef.current?.signal,
          { debugMode }
        )) {
          // Check if cancelled
          if (abortControllerRef.current?.signal.aborted) {
            break
          }

          if ((chunk.type === 'chunk' || chunk.type === 'content') && chunk.content) {
            accumulatedContent += chunk.content
            setStreamingContent(accumulatedContent)
          } else if (chunk.type === 'tool_start' && chunk.tool) {
            debugTrace.tools.push({ ...chunk.tool })
          } else if (chunk.type === 'tool_end' && chunk.tool) {
            const idx = chunk.tool.callId
              ? debugTrace.tools.findIndex((tool) => tool.callId === chunk.tool?.callId)
              : -1
            if (idx >= 0) {
              debugTrace.tools[idx] = { ...debugTrace.tools[idx], ...chunk.tool }
            } else {
              debugTrace.tools.push({ ...chunk.tool })
            }
          } else if (chunk.type === 'usage' && chunk.usage && hasMeaningfulUsage(chunk.usage)) {
            debugTrace.usage = chunk.usage
          } else if (chunk.type === 'error') {
            throw new Error(chunk.error || 'Stream error')
          } else if (chunk.type === 'done') {
            if (chunk.generationMs) {
              debugTrace.generationMs = chunk.generationMs
            }
            if (chunk.sessionId) {
              createdSessionId = chunk.sessionId
            }
            // Refetch session to get final state with proper turns
            const finalSessionId = createdSessionId || activeSessionId
            if (finalSessionId && finalSessionId !== 'new') {
              const state = await dataSource.fetchSession(finalSessionId)
              if (state) {
                setSession(state.session)
                setTurns((prevTurns) => {
                  const mergedTurns = mergeDebugTraceFromPreviousTurns(prevTurns, state.turns)
                  return hydrateDebugTraceFromToolCalls(attachDebugTraceToLatestTurn(mergedTurns, debugTrace))
                })
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

        const errorMessage = err instanceof Error ? err.message : 'error.networkError'
        setInputError(errorMessage)
        console.error('Send message error:', err)
      } finally {
        setLoading(false)
        setStreamingContent('')
        setIsStreaming(false)
        abortControllerRef.current = null
      }
    },
    [currentSessionId, loading, dataSource, debugMode, executeSlashCommand]
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
      setInputError(null)

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
                  setTurns((prevTurns) =>
                    hydrateDebugTraceFromToolCalls(mergeDebugTraceFromPreviousTurns(prevTurns, state.turns))
                  )
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
    inputError,
    currentSessionId,
    pendingQuestion,
    session,
    fetching,
    streamingContent,
    isStreaming,
    messageQueue,
    codeOutputs,
    debugMode,
    isCompacting,
    compactionSummary,

    // Setters
    setMessage,
    setError,
    setInputError,
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
