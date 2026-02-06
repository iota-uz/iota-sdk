/**
 * Built-in HTTP data source with SSE streaming and AbortController
 * Implements ChatDataSource interface with real HTTP/RPC calls
 *
 * Uses turn-based architecture - fetches ConversationTurns instead of flat messages.
 */

import { createAppletRPCClient, type AppletRPCSchema } from '../../applet-host'
import type {
  ChatDataSource,
  Session,
  SessionListResult,
  ConversationTurn,
  PendingQuestion,
  Attachment,
  StreamChunk,
  QuestionAnswers,
  SendMessageOptions,
} from '../types'

export interface HttpDataSourceConfig {
  baseUrl: string
  rpcEndpoint: string
  streamEndpoint?: string
  csrfToken?: string | (() => string)
  headers?: Record<string, string>
  timeout?: number
}

interface SessionState {
  session: Session
  turns: ConversationTurn[]
  pendingQuestion?: PendingQuestion | null
}

interface Result<T> {
  success: boolean
  data?: T
  error?: string
}

export class HttpDataSource implements ChatDataSource {
  private config: HttpDataSourceConfig
  private abortController: AbortController | null = null
  private rpc: ReturnType<typeof createAppletRPCClient>

  constructor(config: HttpDataSourceConfig) {
    this.config = {
      streamEndpoint: '/stream',
      timeout: 30000,
      ...config,
    }
    this.rpc = createAppletRPCClient({
      endpoint: `${this.config.baseUrl}${this.config.rpcEndpoint}`,
    })
  }

  /**
   * Get CSRF token from config
   */
  private getCSRFToken(): string {
    if (!this.config.csrfToken) {
      return ''
    }
    return typeof this.config.csrfToken === 'function'
      ? this.config.csrfToken()
      : this.config.csrfToken
  }

  /**
   * Create headers for HTTP requests
   */
  private createHeaders(additionalHeaders?: Record<string, string>): Headers {
    const headers = new Headers({
      'Content-Type': 'application/json',
      ...this.config.headers,
      ...additionalHeaders,
    })

    const csrfToken = this.getCSRFToken()
    if (csrfToken) {
      headers.set('X-CSRF-Token', csrfToken)
    }

    return headers
  }

  private async callRPC<TMethod extends keyof BiChatRPC & string>(
    method: TMethod,
    params: BiChatRPC[TMethod]['params']
  ): Promise<BiChatRPC[TMethod]['result']> {
    return this.rpc.callTyped<BiChatRPC, TMethod>(method, params)
  }

  /**
   * Create a new chat session
   */
  async createSession(): Promise<Session> {
    const data = await this.callRPC('bichat.session.create', { title: '' })
    return data.session
  }

  /**
   * Fetch an existing session with turns (turn-based architecture)
   */
  async fetchSession(id: string): Promise<SessionState | null> {
    try {
      const data = await this.callRPC('bichat.session.get', { id })
      return {
        session: data.session,
        turns: data.turns as ConversationTurn[],
        pendingQuestion: (data.pendingQuestion as PendingQuestion | null) ?? null,
      }
    } catch (err) {
      console.error('Failed to fetch session:', err)
      return null
    }
  }

  /**
   * Send a message and stream the response using SSE
   */
  async *sendMessage(
    sessionId: string,
    content: string,
    attachments: Attachment[] = [],
    signal?: AbortSignal,
    options?: SendMessageOptions
  ): AsyncGenerator<StreamChunk> {
    // Create new abort controller for this stream
    this.abortController = new AbortController()

    // Link external signal if provided
    if (signal) {
      signal.addEventListener('abort', () => {
        this.abortController?.abort()
      })
    }

    const url = `${this.config.baseUrl}${this.config.streamEndpoint}`

    const payload = {
      sessionId,
      content,
      debugMode: options?.debugMode ?? false,
      attachments: attachments.map(a => ({
        id: a.id,
        filename: a.filename,
        mimeType: a.mimeType,
        sizeBytes: a.sizeBytes,
        base64Data: a.base64Data,
      })),
    }

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: this.createHeaders(),
        body: JSON.stringify(payload),
        signal: this.abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`Stream request failed: ${response.statusText}`)
      }

      if (!response.body) {
        throw new Error('Response body is null')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      try {
        while (true) {
          const { done, value } = await reader.read()

          if (done) {
            break
          }

          buffer += decoder.decode(value, { stream: true })

          // Process SSE events in buffer
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''

          for (const line of lines) {
            if (!line.trim() || line.startsWith(':')) {
              continue
            }

            if (line.startsWith('data: ')) {
              const data = line.slice(6)

              try {
                const parsed = JSON.parse(data) as StreamChunk & { chunk?: string }
                const inferredType =
                  parsed.type || (parsed.content || parsed.chunk ? 'content' : 'error')
                const normalized: StreamChunk = {
                  ...parsed,
                  type: inferredType,
                  content: parsed.content ?? parsed.chunk,
                }
                yield normalized

                // Stop if done or error
                if (normalized.type === 'done' || normalized.type === 'error') {
                  return
                }
              } catch (parseErr) {
                console.error('Failed to parse SSE data:', parseErr)
                yield {
                  type: 'error',
                  error: 'Failed to parse stream data',
                }
                return
              }
            }
          }
        }
      } finally {
        reader.releaseLock()
      }
    } catch (err) {
      if (err instanceof Error) {
        if (err.name === 'AbortError') {
          yield {
            type: 'error',
            error: 'Stream cancelled',
          }
        } else {
          yield {
            type: 'error',
            error: err.message,
          }
        }
      } else {
        yield {
          type: 'error',
          error: 'Unknown error',
        }
      }
    } finally {
      this.abortController = null
    }
  }

  /**
   * Cancel ongoing stream
   */
  cancelStream(): void {
    if (this.abortController) {
      this.abortController.abort()
      this.abortController = null
    }
  }

  /**
   * Clear session history in-place.
   */
  async clearSessionHistory(sessionId: string): Promise<{
    success: boolean
    deletedMessages: number
    deletedArtifacts: number
  }> {
    return this.callRPC('bichat.session.clear', { id: sessionId })
  }

  /**
   * Compact session history into summarized turn.
   */
  async compactSessionHistory(sessionId: string): Promise<{
    success: boolean
    summary: string
    deletedMessages: number
    deletedArtifacts: number
  }> {
    return this.callRPC('bichat.session.compact', { id: sessionId })
  }

  /**
   * Submit answers to a pending question
   */
  async submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<Result<void>> {
    try {
      // Convert QuestionAnswers to flat map[string]string for RPC
      const flatAnswers: Record<string, string> = {}
      for (const [qId, answerData] of Object.entries(answers)) {
        if (answerData.customText) {
          flatAnswers[qId] = answerData.customText
        } else if (answerData.options.length > 0) {
          flatAnswers[qId] = answerData.options.join(', ')
        }
      }
      await this.callRPC('bichat.question.submit', {
        sessionId,
        checkpointId: questionId,
        answers: flatAnswers,
      })
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
  }

  /**
   * Cancel a pending question
   */
  async cancelPendingQuestion(questionId: string): Promise<Result<void>> {
    try {
      await this.callRPC('bichat.question.cancel', { sessionId: questionId })
      return { success: true }
    } catch (err) {
      return { success: false, error: err instanceof Error ? err.message : 'Unknown error' }
    }
  }

  /**
   * Navigate to a session (optional, for SPA routing)
   */
  navigateToSession?(sessionId: string): void {
    // Default implementation - can be overridden
    if (typeof window !== 'undefined') {
      window.location.href = `/chat/${sessionId}`
    }
  }

  // Session management
  async listSessions(options?: {
    limit?: number
    offset?: number
    includeArchived?: boolean
  }): Promise<SessionListResult> {
    const data = await this.callRPC('bichat.session.list', {
      limit: options?.limit ?? 200,
      offset: options?.offset ?? 0,
      includeArchived: options?.includeArchived ?? false,
    })
    return {
      sessions: data.sessions,
      total: data.sessions.length,
      hasMore: false,
    }
  }
  async archiveSession(sessionId: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.archive', { id: sessionId })
    return data.session
  }
  async unarchiveSession(sessionId: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.unarchive', { id: sessionId })
    return data.session
  }
  async pinSession(sessionId: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.pin', { id: sessionId })
    return data.session
  }
  async unpinSession(sessionId: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.unpin', { id: sessionId })
    return data.session
  }
  async deleteSession(sessionId: string): Promise<void> {
    await this.callRPC('bichat.session.delete', { id: sessionId })
  }
  async renameSession(sessionId: string, title: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.updateTitle', { id: sessionId, title })
    return data.session
  }
  async regenerateSessionTitle(sessionId: string): Promise<Session> {
    const data = await this.callRPC('bichat.session.regenerateTitle', { id: sessionId })
    return data.session
  }
}

/**
 * Factory function to create HttpDataSource
 */
export function createHttpDataSource(config: HttpDataSourceConfig): ChatDataSource {
  return new HttpDataSource(config)
}

type BiChatRPC = AppletRPCSchema & {
  'bichat.session.create': { params: { title: string }; result: { session: Session } }
  'bichat.session.list': {
    params: { limit: number; offset: number; includeArchived: boolean }
    result: { sessions: Session[] }
  }
  'bichat.session.get': {
    params: { id: string }
    result: { session: Session; turns: ConversationTurn[]; pendingQuestion: PendingQuestion | null }
  }
  'bichat.session.updateTitle': {
    params: { id: string; title: string }
    result: { session: Session }
  }
  'bichat.session.clear': {
    params: { id: string }
    result: { success: boolean; deletedMessages: number; deletedArtifacts: number }
  }
  'bichat.session.compact': {
    params: { id: string }
    result: { success: boolean; summary: string; deletedMessages: number; deletedArtifacts: number }
  }
  'bichat.session.delete': { params: { id: string }; result: { ok: boolean } }
  'bichat.session.pin': { params: { id: string }; result: { session: Session } }
  'bichat.session.unpin': { params: { id: string }; result: { session: Session } }
  'bichat.session.archive': { params: { id: string }; result: { session: Session } }
  'bichat.session.unarchive': { params: { id: string }; result: { session: Session } }
  'bichat.session.regenerateTitle': { params: { id: string }; result: { session: Session } }
  'bichat.question.submit': {
    params: { sessionId: string; checkpointId: string; answers: Record<string, string> }
    result: { session: Session; turns: ConversationTurn[]; pendingQuestion: PendingQuestion | null }
  }
  'bichat.question.cancel': {
    params: { sessionId: string }
    result: { session: Session }
  }
}
