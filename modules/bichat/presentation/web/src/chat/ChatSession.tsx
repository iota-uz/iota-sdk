import { batch, createEffect, createContext, useContext } from 'solid-js'
import { createSignal, type Accessor, type JSX } from 'solid-js'
import { createBichatRPCClient, RPCError } from '../rpc/client'
import { createBichatStreamClient } from '../streaming/client'
import type { StreamChunk } from '../streaming/protocol'
import { appendOptimisticTurn, applyChunk, initialState, type MachineState } from './sessionMachine'
import type { ChatTurn, PendingQuestion } from './types'
import type { Artifact, ConversationTurn, Session } from '../rpc.generated'
import { useAppContext } from '../context/appContext'
import { fetchSessionWithArtifacts } from '../data/source'

export interface RateLimiter {
  allow: () => boolean
}

export function createRateLimiter(maxRequests: number, windowMs: number): RateLimiter {
  let timestamps: number[] = []
  return {
    allow() {
      const now = Date.now()
      timestamps = timestamps.filter((ts) => now - ts < windowMs)
      if (timestamps.length >= maxRequests) return false
      timestamps.push(now)
      return true
    },
  }
}

export interface ChatSessionProps {
  sessionId: string | undefined
  readOnly?: boolean
  rateLimiter?: RateLimiter
  onSessionCreated?: (sessionId: string) => void
}

export interface ChatSessionController {
  session: Accessor<Session | undefined>
  turns: Accessor<ChatTurn[]>
  loading: Accessor<boolean>
  streaming: Accessor<boolean>
  error: Accessor<string | undefined>
  pendingQuestion: Accessor<PendingQuestion | null>
  artifacts: Accessor<Artifact[]>
  send: (content: string) => Promise<void>
  cancel: () => Promise<void>
  retry: () => Promise<void>
  submitAnswers: (checkpointId: string, answers: Record<string, string>) => Promise<void>
  rejectQuestion: () => Promise<void>
}

const ChatSessionContext = createContext<ChatSessionController>()

export function useChatSession(): ChatSessionController {
  const controller = useContext(ChatSessionContext)
  if (!controller) throw new Error('useChatSession must be used within ChatSession')
  return controller
}

let localTurnCounter = 0

export function ChatSession(props: ChatSessionProps & { children?: JSX.Element }): JSX.Element {
  const ctx = useAppContext()
  const rpc = createBichatRPCClient(ctx)
  const stream = createBichatStreamClient({
    streamEndpoint: ctx.config.streamEndpoint ?? '/bi-chat/stream',
    csrfToken: () => ctx.session.csrfToken ?? '',
  })

  const [session, setSession] = createSignal<Session | undefined>(undefined)
  const [state, setState] = createSignal<MachineState>(initialState([]))
  const [loading, setLoading] = createSignal(false)
  const [error, setError] = createSignal<string | undefined>(undefined)
  const [artifacts, setArtifacts] = createSignal<Artifact[]>([])

  let activeController: AbortController | undefined

  const pendingQuestion = () => {
    for (let i = state().turns.length - 1; i >= 0; i -= 1) {
      const question = state().turns[i].pendingQuestion
      if (question) return question
    }
    return null
  }

  const refreshSession = async (sessionId: string): Promise<void> => {
    try {
      const result = await fetchSessionWithArtifacts(rpc, sessionId)
      batch(() => {
        setSession(result.session)
        setState(initialState(result.turns as ConversationTurn[]))
        setArtifacts(result.artifacts)
        setError(undefined)
      })
    } catch (cause) {
      setError(cause instanceof RPCError ? cause.message : String(cause))
    }
  }

  const load = async (sessionId: string): Promise<void> => {
    setLoading(true)
    await refreshSession(sessionId)
    setLoading(false)
  }

  createEffect(() => {
    const sessionId = props.sessionId
    if (sessionId && sessionId !== 'new') void load(sessionId)
  })

  const streaming = () => {
    const turns = state().turns
    return turns.length > 0 && turns[turns.length - 1].status === 'streaming'
  }

  const handleChunk = (chunk: StreamChunk): void => {
    setState((current) => applyChunk(current, chunk))
  }

  const send = async (content: string): Promise<void> => {
    const trimmed = content.trim()
    if (!trimmed || streaming() || props.readOnly) return
    if (props.rateLimiter && !props.rateLimiter.allow()) {
      setError('rate_limited')
      return
    }
    const localId = `local-${Date.now()}-${localTurnCounter++}`
    const knownSession = session()?.id

    activeController?.abort()
    activeController = new AbortController()
    const controller = activeController
    setError(undefined)
    setState((current) => appendOptimisticTurn(current, trimmed, localId))

    try {
      let sessionId: string
      if (knownSession) {
        sessionId = knownSession
      } else {
        const created = await rpc.call('bichat.session.create', { title: trimmed.slice(0, 80) })
        sessionId = created.session.id
        setSession(created.session)
        props.onSessionCreated?.(sessionId)
      }
      await stream.sendMessage(
        { sessionId, content: trimmed, requestId: localId },
        { onEvent: handleChunk },
        controller.signal,
      )
      await refreshSession(sessionId)
    } catch (cause) {
      if (controller.signal.aborted) {
        setState((current) => ({ ...current, turns: current.turns.map((turn, index) => (
          index === current.turns.length - 1 && turn.status === 'streaming'
            ? { ...turn, status: 'cancelled' }
            : turn
        )) }))
        return
      }
      setError(cause instanceof Error ? cause.message : String(cause))
      setState((current) => ({ ...current, turns: current.turns.map((turn, index) => (
        index === current.turns.length - 1 && turn.status === 'streaming'
          ? { ...turn, status: 'failed' }
          : turn
      )) }))
    }
  }

  const cancel = async (): Promise<void> => {
    const current = session()?.id
    activeController?.abort()
    if (current) await stream.stop(current, state().runId)
  }

  const retry = async (): Promise<void> => {
    const turns = state().turns
    const last = turns[turns.length - 1]
    if (!last || last.status === 'streaming') return
    await send(last.userContent)
  }

  const submitAnswers = async (checkpointId: string, answers: Record<string, string>): Promise<void> => {
    const sessionId = session()?.id
    if (!sessionId) return
    try {
      const result = await rpc.call('bichat.question.submit', { sessionId, checkpointId, answers })
      if ('runId' in result && result.runId) {
        setState((current) => ({ ...current, runId: result.runId }))
        await stream.subscribeRunEvents({ sessionId, runId: result.runId }, { onEvent: handleChunk }).settled
        await refreshSession(sessionId)
      } else {
        await refreshSession(sessionId)
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    }
  }

  const rejectQuestion = async (): Promise<void> => {
    const sessionId = session()?.id
    if (!sessionId) return
    try {
      await rpc.call('bichat.question.reject', { sessionId })
      await refreshSession(sessionId)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause))
    }
  }

  const controller: ChatSessionController = {
    session,
    turns: () => state().turns,
    loading,
    streaming,
    error,
    pendingQuestion,
    artifacts,
    send,
    cancel,
    retry,
    submitAnswers,
    rejectQuestion,
  }

  return (
    <ChatSessionContext.Provider value={controller}>
      {props.children}
    </ChatSessionContext.Provider>
  )
}
