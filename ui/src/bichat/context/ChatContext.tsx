/**
 * Chat session context provider and hook
 * Manages state for chat sessions including turns, loading, streaming, and HITL
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 *
 * Split into 3 focused contexts to minimize re-renders:
 * - ChatSessionContext: session lifecycle (session, fetching, error, debug)
 * - ChatMessagingContext: turns + streaming + tool interactions
 * - ChatInputContext: input form state (message, inputError, queue)
 *
 * Cross-context reads use refs (no subscription = no re-render).
 */

import { createContext, useContext, useState, useCallback, useEffect, ReactNode, useRef, useMemo } from 'react'
import {
  MessageRole,
  type ChatDataSource,
  type Session,
  type ConversationTurn,
  type PendingQuestion,
  type QuestionAnswers,
  type Attachment,
  type ImageAttachment,
  type QueuedMessage,
  type CodeOutput,
  type ChatSessionContextValue,
  type ChatSessionStateValue,
  type ChatMessagingStateValue,
  type ChatInputStateValue,
  type DebugTrace,
  type SendMessageOptions,
} from '../types'
import { RateLimiter } from '../utils/RateLimiter'
import {
  hasMeaningfulUsage,
  getSessionDebugUsage,
  hydrateDebugTraceFromToolCalls,
  attachDebugTraceToLatestTurn,
  mergeDebugTraceFromPreviousTurns,
} from '../utils/debugTrace'
import { hasPermission } from './IotaContext'

// ---------------------------------------------------------------------------
// Internal contexts
// ---------------------------------------------------------------------------

const SessionCtx = createContext<ChatSessionStateValue | null>(null)
const MessagingCtx = createContext<ChatMessagingStateValue | null>(null)
const InputCtx = createContext<ChatInputStateValue | null>(null)

// ---------------------------------------------------------------------------
// Helpers (unchanged)
// ---------------------------------------------------------------------------

interface ChatSessionProviderProps {
  dataSource: ChatDataSource
  sessionId?: string
  rateLimiter?: RateLimiter
  children: ReactNode
}

const DEFAULT_RATE_LIMIT_CONFIG = {
  maxRequests: 20,
  windowMs: 60000,
}

function generateTempId(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
}

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
    createdAt: now,
  }
}

function createCompactedSystemTurn(sessionId: string, summary: string): ConversationTurn {
  const now = new Date().toISOString()
  return {
    id: generateTempId('turn'),
    sessionId,
    userTurn: {
      id: generateTempId('user'),
      content: '',
      attachments: [],
      createdAt: now,
    },
    assistantTurn: {
      id: generateTempId('assistant'),
      role: MessageRole.System,
      content: summary,
      citations: [],
      artifacts: [],
      codeOutputs: [],
      createdAt: now,
    },
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

function readModelContextWindowFromGlobalContext(): number | null {
  if (typeof window === 'undefined') {
    return null
  }

  const raw = window.__BICHAT_CONTEXT__?.extensions?.debug?.contextWindow
  if (typeof raw === 'number' && Number.isFinite(raw) && raw > 0) {
    return raw
  }
  if (typeof raw === 'string') {
    const parsed = Number.parseInt(raw, 10)
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed
    }
  }

  return null
}

// ---------------------------------------------------------------------------
// Composed Provider
// ---------------------------------------------------------------------------

export function ChatSessionProvider({
  dataSource,
  sessionId,
  rateLimiter: externalRateLimiter,
  children
}: ChatSessionProviderProps) {
  // ── Session state ──────────────────────────────────────────────────────
  const [currentSessionId, setCurrentSessionId] = useState<string | undefined>(sessionId)
  const [session, setSession] = useState<Session | null>(null)
  const [fetching, setFetching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [debugModeBySession, setDebugModeBySession] = useState<Record<string, boolean>>({})

  const debugSessionKey = currentSessionId || 'new'
  const debugMode = debugModeBySession[debugSessionKey] ?? false
  const modelContextWindow = useMemo(() => readModelContextWindowFromGlobalContext(), [])

  // ── Messaging state ────────────────────────────────────────────────────
  const [turns, setTurns] = useState<ConversationTurn[]>([])
  const [loading, setLoading] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [pendingQuestion, setPendingQuestion] = useState<PendingQuestion | null>(null)
  const [codeOutputs, setCodeOutputs] = useState<CodeOutput[]>([])
  const [isCompacting, setIsCompacting] = useState(false)
  const [compactionSummary, setCompactionSummary] = useState<string | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // ── Input state ────────────────────────────────────────────────────────
  const [message, setMessage] = useState('')
  const [inputError, setInputError] = useState<string | null>(null)
  const [messageQueue, setMessageQueue] = useState<QueuedMessage[]>([])

  // ── Rate limiter ───────────────────────────────────────────────────────
  const rateLimiterRef = useRef<RateLimiter>(
    externalRateLimiter || new RateLimiter(DEFAULT_RATE_LIMIT_CONFIG)
  )

  // ── Refs for cross-context reads (no re-render subscription) ───────────
  const sessionRef = useRef({ currentSessionId, debugMode, debugSessionKey })
  sessionRef.current = { currentSessionId, debugMode, debugSessionKey }

  const messagingRef = useRef({ turns, pendingQuestion, loading })
  messagingRef.current = { turns, pendingQuestion, loading }

  // ── Derived ────────────────────────────────────────────────────────────
  const sessionDebugUsage = useMemo(() => getSessionDebugUsage(turns), [turns])

  // ── Sync sessionId prop ────────────────────────────────────────────────
  useEffect(() => {
    setCurrentSessionId(sessionId)
  }, [sessionId])

  // ── Fetch session ──────────────────────────────────────────────────────
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

  // ── Handlers ───────────────────────────────────────────────────────────

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

        const curDebugMode = sessionRef.current.debugMode
        const curDebugSessionKey = sessionRef.current.debugSessionKey
        const curSessionId = sessionRef.current.currentSessionId
        const nextDebugMode = !curDebugMode
        setDebugModeBySession((prev) => ({
          ...prev,
          [curDebugSessionKey]: nextDebugMode,
        }))

        if (nextDebugMode && curSessionId && curSessionId !== 'new') {
          try {
            const state = await dataSource.fetchSession(curSessionId)
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

      const curSessionId = sessionRef.current.currentSessionId
      if (!curSessionId || curSessionId === 'new') {
        setInputError('slash.error.sessionRequired')
        return true
      }

      if (command.name === '/clear') {
        setLoading(true)
        setStreamingContent('')

        try {
          await dataSource.clearSessionHistory(curSessionId)
          const state = await dataSource.fetchSession(curSessionId)
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
          const result = await dataSource.compactSessionHistory(curSessionId)
          const summary = result.summary || ''
          setTurns([createCompactedSystemTurn(curSessionId, summary)])
          setCompactionSummary(null)

          const state = await dataSource.fetchSession(curSessionId)
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
    [dataSource]
  )

  const sendMessageDirect = useCallback(
    async (
      content: string,
      attachments: Attachment[] = [],
      options?: SendMessageOptions
    ): Promise<void> => {
      if (!content.trim() || messagingRef.current.loading) return

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

      abortControllerRef.current = new AbortController()

      const curSessionId = sessionRef.current.currentSessionId
      const curDebugMode = sessionRef.current.debugMode
      const tempTurn = createPendingTurn(curSessionId || 'new', content, attachments)
      const replaceFromMessageID = options?.replaceFromMessageID
      setTurns((prev) => {
        if (!replaceFromMessageID) {
          return [...prev, tempTurn]
        }
        const replaceIndex = prev.findIndex((turn) => turn.userTurn.id === replaceFromMessageID)
        if (replaceIndex === -1) {
          return [...prev, tempTurn]
        }
        return [...prev.slice(0, replaceIndex), tempTurn]
      })

      try {
        let activeSessionId = curSessionId
        let shouldNavigateAfter = false

        if (!activeSessionId || activeSessionId === 'new') {
          const result = await dataSource.createSession()
          if (result) {
            const createdSessionID = result.id
            activeSessionId = createdSessionID
            setCurrentSessionId(createdSessionID)
            setDebugModeBySession((prev) => {
              if (!curDebugMode) return prev
              return { ...prev, [createdSessionID]: true }
            })
            shouldNavigateAfter = true
          }
        }

        let accumulatedContent = ''
        let createdSessionId: string | undefined
        const debugTrace: DebugTrace = { tools: [] }
        setIsStreaming(true)

        for await (const chunk of dataSource.sendMessage(
          activeSessionId || 'new',
          content,
          attachments,
          abortControllerRef.current?.signal,
          {
            debugMode: curDebugMode,
            replaceFromMessageID,
          }
        )) {
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

        const targetSessionId = createdSessionId || activeSessionId
        if (shouldNavigateAfter && targetSessionId && targetSessionId !== 'new') {
          dataSource.navigateToSession?.(targetSessionId)
        }
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
          setMessage(content)
          return
        }

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
    [dataSource, executeSlashCommand]
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

      const convertedAttachments: Attachment[] = attachments.map(att => ({
        id: '',
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
      const curSessionId = sessionRef.current.currentSessionId
      if (!curSessionId || curSessionId === 'new') return

      const turn = messagingRef.current.turns.find((t) => t.id === turnId)
      if (!turn) return

      setError(null)

      try {
        await sendMessageDirect(turn.userTurn.content, turn.userTurn.attachments, {
          replaceFromMessageID: turn.userTurn.id,
        })
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to regenerate response'
        setError(errorMessage)
        console.error('Regenerate error:', err)
      }
    },
    [sendMessageDirect]
  )

  const handleEdit = useCallback(
    async (turnId: string, newContent: string) => {
      const curSessionId = sessionRef.current.currentSessionId
      if (!curSessionId || curSessionId === 'new') {
        setMessage(newContent)
        setTurns((prev) => prev.filter((t) => t.id !== turnId))
        return
      }

      const turn = messagingRef.current.turns.find((t) => t.id === turnId)
      if (!turn) {
        setError('Failed to edit message')
        return
      }

      setError(null)

      try {
        await sendMessageDirect(newContent, turn.userTurn.attachments, {
          replaceFromMessageID: turn.userTurn.id,
        })
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to edit message'
        setError(errorMessage)
        console.error('Edit error:', err)
      }
    },
    [sendMessageDirect]
  )

  const handleSubmitQuestionAnswers = useCallback(
    (answers: QuestionAnswers) => {
      const curSessionId = sessionRef.current.currentSessionId
      const curPendingQuestion = messagingRef.current.pendingQuestion
      if (!curSessionId || !curPendingQuestion) return

      setLoading(true)
      setError(null)
      const previousPendingQuestion = curPendingQuestion
      setPendingQuestion(null)

      ;(async () => {
        try {
          const result = await dataSource.submitQuestionAnswers(
            curSessionId,
            previousPendingQuestion.id,
            answers
          )

          if (result.success) {
            if (curSessionId !== 'new') {
              try {
                const state = await dataSource.fetchSession(curSessionId)
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
    [dataSource]
  )

  const handleCancelPendingQuestion = useCallback(async () => {
    const curSessionId = sessionRef.current.currentSessionId
    const curPendingQuestion = messagingRef.current.pendingQuestion
    if (!curSessionId || !curPendingQuestion) return

    try {
      const result = await dataSource.cancelPendingQuestion(curPendingQuestion.id)

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
  }, [dataSource])

  // ── Context values (memoized per-context) ──────────────────────────────

  const sessionValue: ChatSessionStateValue = useMemo(() => ({
    session,
    currentSessionId,
    fetching,
    error,
    debugMode,
    sessionDebugUsage,
    modelContextWindow,
    setError,
  }), [session, currentSessionId, fetching, error, debugMode, sessionDebugUsage, modelContextWindow])

  const messagingValue: ChatMessagingStateValue = useMemo(() => ({
    turns,
    streamingContent,
    isStreaming,
    loading,
    pendingQuestion,
    codeOutputs,
    isCompacting,
    compactionSummary,
    sendMessage: sendMessageDirect,
    handleRegenerate,
    handleEdit,
    handleCopy,
    handleSubmitQuestionAnswers,
    handleCancelPendingQuestion,
    cancel: cancelStream,
    setCodeOutputs,
  }), [
    turns, streamingContent, isStreaming, loading, pendingQuestion,
    codeOutputs, isCompacting, compactionSummary,
    sendMessageDirect, handleRegenerate, handleEdit, handleCopy,
    handleSubmitQuestionAnswers, handleCancelPendingQuestion, cancelStream,
  ])

  const inputValue: ChatInputStateValue = useMemo(() => ({
    message,
    inputError,
    messageQueue,
    setMessage,
    setInputError,
    handleSubmit,
    handleUnqueue,
  }), [message, inputError, messageQueue, handleSubmit, handleUnqueue])

  return (
    <SessionCtx.Provider value={sessionValue}>
      <MessagingCtx.Provider value={messagingValue}>
        <InputCtx.Provider value={inputValue}>
          {children}
        </InputCtx.Provider>
      </MessagingCtx.Provider>
    </SessionCtx.Provider>
  )
}

// ---------------------------------------------------------------------------
// Focused hooks
// ---------------------------------------------------------------------------

export function useChatSession(): ChatSessionStateValue {
  const context = useContext(SessionCtx)
  if (!context) {
    throw new Error('useChatSession must be used within ChatSessionProvider')
  }
  return context
}

export function useChatMessaging(): ChatMessagingStateValue {
  const context = useContext(MessagingCtx)
  if (!context) {
    throw new Error('useChatMessaging must be used within ChatSessionProvider')
  }
  return context
}

export function useChatInput(): ChatInputStateValue {
  const context = useContext(InputCtx)
  if (!context) {
    throw new Error('useChatInput must be used within ChatSessionProvider')
  }
  return context
}

// ---------------------------------------------------------------------------
// Backwards-compatible merged hook
// ---------------------------------------------------------------------------

export function useChat(): ChatSessionContextValue {
  const s = useChatSession()
  const m = useChatMessaging()
  const i = useChatInput()

  return useMemo(() => ({
    ...s,
    ...m,
    ...i,
  }), [s, m, i])
}
