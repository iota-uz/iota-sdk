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
  Artifact as DownloadArtifact,
  SessionArtifact,
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

interface RPCArtifact {
  id: string
  sessionId: string
  messageId?: string
  type: string
  name: string
  description?: string
  mimeType?: string
  url?: string
  sizeBytes: number
  metadata?: Record<string, unknown>
  createdAt: string
}

function toSessionArtifact(artifact: RPCArtifact): SessionArtifact {
  return {
    id: artifact.id,
    sessionId: artifact.sessionId,
    messageId: artifact.messageId,
    type: artifact.type,
    name: artifact.name,
    description: artifact.description,
    mimeType: artifact.mimeType,
    url: artifact.url,
    sizeBytes: artifact.sizeBytes,
    metadata: artifact.metadata,
    createdAt: artifact.createdAt,
  }
}

function formatSizeReadable(bytes: number): string | undefined {
  if (!Number.isFinite(bytes) || bytes <= 0) return undefined

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx++
  }
  const precision = idx === 0 ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(precision)} ${units[idx]}`
}

function parseRowCount(metadata?: Record<string, unknown>): number | undefined {
  if (!metadata) return undefined
  const raw = metadata.row_count ?? metadata.rowCount
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return raw
  }
  if (typeof raw === 'string') {
    const parsed = Number.parseInt(raw, 10)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return undefined
}

function inferDownloadType(artifact: SessionArtifact): DownloadArtifact['type'] | null {
  const mime = artifact.mimeType?.toLowerCase() || ''
  const name = artifact.name?.toLowerCase() || ''
  const cleanURL = artifact.url?.split('?')[0].toLowerCase() || ''

  const isPDF = mime.includes('pdf') || name.endsWith('.pdf') || cleanURL.endsWith('.pdf')
  if (isPDF) return 'pdf'

  const isExcel =
    mime.includes('spreadsheet') ||
    mime.includes('excel') ||
    name.endsWith('.xlsx') ||
    name.endsWith('.xls') ||
    cleanURL.endsWith('.xlsx') ||
    cleanURL.endsWith('.xls')
  if (isExcel) return 'excel'

  return null
}

function extractFilename(artifact: SessionArtifact): string {
  const name = artifact.name?.trim()
  if (name) return name

  const urlPath = artifact.url?.split('?')[0] || ''
  const fromURL = urlPath.split('/').filter(Boolean).pop()
  if (fromURL) return fromURL

  return 'download'
}

function toDownloadArtifact(artifact: SessionArtifact): DownloadArtifact | null {
  if (!artifact.url) return null
  const type = inferDownloadType(artifact)
  if (!type) return null

  return {
    type,
    filename: extractFilename(artifact),
    url: artifact.url,
    sizeReadable: formatSizeReadable(artifact.sizeBytes),
    rowCount: parseRowCount(artifact.metadata),
    description: artifact.description,
  }
}

function toMillis(value: string): number {
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : Number.NaN
}

function attachArtifactsToTurns(
  turns: ConversationTurn[],
  artifacts: SessionArtifact[]
): ConversationTurn[] {
  if (artifacts.length === 0) return turns

  const downloadArtifacts = artifacts
    .map((raw) => ({ raw, mapped: toDownloadArtifact(raw) }))
    .filter((entry): entry is { raw: SessionArtifact; mapped: DownloadArtifact } => entry.mapped !== null)
    .sort((a, b) => toMillis(a.raw.createdAt) - toMillis(b.raw.createdAt))

  if (downloadArtifacts.length === 0) return turns

  const nextTurns = turns.map((turn) => {
    if (!turn.assistantTurn) {
      return turn
    }
    return {
      ...turn,
      assistantTurn: {
        ...turn.assistantTurn,
        artifacts: [...(turn.assistantTurn.artifacts || [])],
      },
    }
  })

  const assistantPositions: Array<{ index: number; createdAtMs: number }> = []
  const turnIndexByMessageID = new Map<string, number>()

  nextTurns.forEach((turn, index) => {
    turnIndexByMessageID.set(turn.userTurn.id, index)

    const assistantTurn = turn.assistantTurn
    if (!assistantTurn) return
    turnIndexByMessageID.set(assistantTurn.id, index)
    assistantPositions.push({
      index,
      createdAtMs: toMillis(assistantTurn.createdAt || turn.createdAt),
    })
  })

  if (assistantPositions.length === 0) return turns

  const findFallbackAssistantIndex = (artifactCreatedAt: string): number => {
    const artifactMs = toMillis(artifactCreatedAt)
    if (!Number.isFinite(artifactMs)) {
      return assistantPositions[assistantPositions.length - 1].index
    }
    for (const pos of assistantPositions) {
      if (Number.isFinite(pos.createdAtMs) && pos.createdAtMs >= artifactMs) {
        return pos.index
      }
    }
    return assistantPositions[assistantPositions.length - 1].index
  }

  for (const entry of downloadArtifacts) {
    const messageID = entry.raw.messageId
    const targetIndex =
      (messageID ? turnIndexByMessageID.get(messageID) : undefined) ??
      findFallbackAssistantIndex(entry.raw.createdAt)

    const assistantTurn = nextTurns[targetIndex]?.assistantTurn
    if (!assistantTurn) continue

    const exists = assistantTurn.artifacts.some(
      (existing) =>
        existing.url === entry.mapped.url && existing.filename === entry.mapped.filename
    )
    if (!exists) {
      assistantTurn.artifacts.push(entry.mapped)
    }
  }

  return nextTurns
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
      const [data, artifactsData] = await Promise.all([
        this.callRPC('bichat.session.get', { id }),
        this.fetchSessionArtifacts(id, { limit: 200, offset: 0 }).catch((err) => {
          console.warn('Failed to fetch session artifacts:', err)
          return { artifacts: [] as SessionArtifact[], hasMore: false, nextOffset: 0 }
        }),
      ])

      return {
        session: data.session,
        turns: attachArtifactsToTurns(data.turns as ConversationTurn[], artifactsData.artifacts || []),
        pendingQuestion: (data.pendingQuestion as PendingQuestion | null) ?? null,
      }
    } catch (err) {
      console.error('Failed to fetch session:', err)
      return null
    }
  }

  async fetchSessionArtifacts(
    sessionId: string,
    options?: { limit?: number; offset?: number }
  ): Promise<{ artifacts: SessionArtifact[]; hasMore?: boolean; nextOffset?: number }> {
    const limit = options?.limit ?? 50
    const offset = options?.offset ?? 0
    const data = await this.callRPC('bichat.session.artifacts', {
      sessionId,
      limit,
      offset,
    })

    const artifacts = (data.artifacts || []).map((artifact) => toSessionArtifact(artifact))
    const hasMore =
      typeof data.hasMore === 'boolean'
        ? data.hasMore
        : artifacts.length >= limit
    const nextOffset =
      typeof data.nextOffset === 'number'
        ? data.nextOffset
        : offset + artifacts.length

    return {
      artifacts,
      hasMore,
      nextOffset,
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
      replaceFromMessageId: options?.replaceFromMessageID,
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
  'bichat.session.artifacts': {
    params: { sessionId: string; limit: number; offset: number }
    result: { artifacts: RPCArtifact[]; hasMore?: boolean; nextOffset?: number }
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
