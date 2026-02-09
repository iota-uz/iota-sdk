/**
 * Custom Data Source Example
 *
 * This example demonstrates how to implement a custom ChatDataSource
 * for use with the BiChat library.
 */

import {
  ChatDataSource,
  Session,
  ConversationTurn,
  UserTurn,
  AssistantTurn,
  PendingQuestion,
  Attachment,
  StreamChunk,
  QuestionAnswers,
  MessageRole,
  SessionListResult,
} from '@iota-uz/sdk/bichat'

/**
 * Mock Data Source using local storage and mock API
 */
export class MockDataSource implements ChatDataSource {
  private baseUrl: string
  private sessions: Map<string, Session> = new Map()
  private turns: Map<string, ConversationTurn[]> = new Map()

  constructor(baseUrl: string = 'http://localhost:3000') {
    this.baseUrl = baseUrl
    this.loadFromLocalStorage()
  }

  /**
   * Load sessions from localStorage
   */
  private loadFromLocalStorage() {
    const stored = localStorage.getItem('bichat_sessions')
    if (stored) {
      try {
        const data = JSON.parse(stored)
        this.sessions = new Map(data.sessions)
        this.turns = new Map(data.turns)
      } catch (err) {
        console.error('Failed to load sessions:', err)
      }
    }
  }

  /**
   * Save sessions to localStorage
   */
  private saveToLocalStorage() {
    const data = {
      sessions: Array.from(this.sessions.entries()),
      turns: Array.from(this.turns.entries()),
    }
    localStorage.setItem('bichat_sessions', JSON.stringify(data))
  }

  /**
   * Generate unique ID
   */
  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`
  }

  /**
   * Create a new chat session
   */
  async createSession(): Promise<Session> {
    const session: Session = {
      id: this.generateId(),
      title: 'New Chat',
      status: 'active',
      pinned: false,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }

    this.sessions.set(session.id, session)
    this.turns.set(session.id, [])
    this.saveToLocalStorage()

    return session
  }

  /**
   * Fetch an existing session
   */
  async fetchSession(id: string): Promise<{
    session: Session
    turns: ConversationTurn[]
    pendingQuestion?: PendingQuestion | null
  } | null> {
    const session = this.sessions.get(id)
    if (!session) {
      return null
    }

    const turns = this.turns.get(id) || []

    return {
      session,
      turns,
      pendingQuestion: null,
    }
  }

  /**
   * Send a message and stream the response
   */
  async *sendMessage(
    sessionId: string,
    content: string,
    attachments: Attachment[] = [],
    signal?: AbortSignal
  ): AsyncGenerator<StreamChunk> {
    // Create conversation turn
    const turnId = this.generateId()
    const userTurn: UserTurn = {
      id: this.generateId(),
      content,
      attachments,
      createdAt: new Date().toISOString(),
    }

    const turn: ConversationTurn = {
      id: turnId,
      sessionId,
      userTurn,
      createdAt: new Date().toISOString(),
    }

    const turns = this.turns.get(sessionId) || []
    turns.push(turn)
    this.turns.set(sessionId, turns)
    this.saveToLocalStorage()

    // Yield user message event
    yield {
      type: 'user_message',
      sessionId,
    }

    // Check for cancellation
    if (signal?.aborted) {
      yield {
        type: 'error',
        error: 'Request cancelled',
      }
      return
    }

    try {
      // Simulate API call to get streaming response
      const response = await fetch(`${this.baseUrl}/api/chat`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          sessionId,
          message: content,
          attachments,
        }),
        signal,
      })

      if (!response.ok) {
        throw new Error(`API error: ${response.statusText}`)
      }

      if (!response.body) {
        throw new Error('No response body')
      }

      // Read SSE stream
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let assistantContent = ''

      while (true) {
        const { done, value } = await reader.read()

        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })

        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (!line.trim() || line.startsWith(':')) {
            continue
          }

          if (line.startsWith('data: ')) {
            const data = line.slice(6)

            try {
              const chunk = JSON.parse(data) as StreamChunk

              if (chunk.type === 'chunk' && chunk.content) {
                assistantContent += chunk.content
                yield chunk
              } else if (chunk.type === 'error') {
                yield chunk
                return
              } else if (chunk.type === 'done') {
                // Save assistant turn
                const assistantTurn: AssistantTurn = {
                  id: this.generateId(),
                  content: assistantContent,
                  citations: [],
                  artifacts: [],
                  codeOutputs: [],
                  createdAt: new Date().toISOString(),
                }

                turn.assistantTurn = assistantTurn
                this.turns.set(sessionId, turns)
                this.saveToLocalStorage()

                yield chunk
                return
              }
            } catch (parseErr) {
              console.error('Failed to parse chunk:', parseErr)
            }
          }
        }
      }
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        yield {
          type: 'error',
          error: 'Request cancelled',
        }
      } else {
        yield {
          type: 'error',
          error: err instanceof Error ? err.message : 'Unknown error',
        }
      }
    }
  }

  /**
   * Submit answers to a pending question
   */
  async submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`${this.baseUrl}/api/questions/${questionId}/answers`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ sessionId, answers }),
      })

      if (!response.ok) {
        return {
          success: false,
          error: `API error: ${response.statusText}`,
        }
      }

      return { success: true }
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : 'Unknown error',
      }
    }
  }

  /**
   * Reject a pending question
   */
  async rejectPendingQuestion(sessionId: string): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`${this.baseUrl}/api/sessions/${sessionId}/reject-question`, {
        method: 'POST',
      })

      if (!response.ok) {
        return {
          success: false,
          error: `API error: ${response.statusText}`,
        }
      }

      return { success: true }
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : 'Unknown error',
      }
    }
  }

  /**
   * Navigate to a session (SPA routing example)
   */
  navigateToSession(sessionId: string): void {
    // Example: Use React Router
    // navigate(`/chat/${sessionId}`)

    // Or vanilla JS
    window.history.pushState({}, '', `/chat/${sessionId}`)
  }

  /**
   * List all sessions with pagination
   */
  async listSessions(options?: {
    limit?: number
    offset?: number
    includeArchived?: boolean
  }): Promise<SessionListResult> {
    const allSessions = Array.from(this.sessions.values())
    const filtered = options?.includeArchived
      ? allSessions
      : allSessions.filter((s) => s.status === 'active')

    const limit = options?.limit || 20
    const offset = options?.offset || 0
    const sessions = filtered.slice(offset, offset + limit)

    return {
      sessions,
      total: filtered.length,
      hasMore: offset + limit < filtered.length,
    }
  }

  /**
   * Archive a session
   */
  async archiveSession(sessionId: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    const updated = { ...session, status: 'archived' as const, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Unarchive a session
   */
  async unarchiveSession(sessionId: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    const updated = { ...session, status: 'active' as const, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Pin a session
   */
  async pinSession(sessionId: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    const updated = { ...session, pinned: true, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Unpin a session
   */
  async unpinSession(sessionId: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    const updated = { ...session, pinned: false, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Delete a session
   */
  async deleteSession(sessionId: string): Promise<void> {
    this.sessions.delete(sessionId)
    this.turns.delete(sessionId)
    this.saveToLocalStorage()
  }

  /**
   * Rename a session
   */
  async renameSession(sessionId: string, title: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    const updated = { ...session, title, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Regenerate session title
   */
  async regenerateSessionTitle(sessionId: string): Promise<Session> {
    const session = this.sessions.get(sessionId)
    if (!session) {
      throw new Error('Session not found')
    }

    // Mock title generation
    const title = `Chat ${new Date().toLocaleString()}`
    const updated = { ...session, title, updatedAt: new Date().toISOString() }
    this.sessions.set(sessionId, updated)
    this.saveToLocalStorage()
    return updated
  }

  /**
   * Clear session history
   */
  async clearSessionHistory(sessionId: string): Promise<{
    success: boolean
    deletedMessages: number
    deletedArtifacts: number
  }> {
    const turns = this.turns.get(sessionId) || []
    const count = turns.length
    this.turns.set(sessionId, [])
    this.saveToLocalStorage()

    return {
      success: true,
      deletedMessages: count,
      deletedArtifacts: 0,
    }
  }

  /**
   * Compact session history
   */
  async compactSessionHistory(sessionId: string): Promise<{
    success: boolean
    summary: string
    deletedMessages: number
    deletedArtifacts: number
  }> {
    const turns = this.turns.get(sessionId) || []
    const count = turns.length

    // Mock compaction - keep only last 3 turns
    const compacted = turns.slice(-3)
    this.turns.set(sessionId, compacted)
    this.saveToLocalStorage()

    return {
      success: true,
      summary: `Compacted ${count - compacted.length} turns`,
      deletedMessages: count - compacted.length,
      deletedArtifacts: 0,
    }
  }
}

/**
 * Simple mock data source for testing (no API calls)
 */
export class SimpleMockDataSource implements ChatDataSource {
  async createSession(): Promise<Session> {
    return {
      id: `session-${Date.now()}`,
      title: 'Mock Chat',
      status: 'active',
      pinned: false,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
  }

  async fetchSession(id: string) {
    return {
      session: {
        id,
        title: 'Mock Chat',
        status: 'active' as const,
        pinned: false,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      turns: [],
      pendingQuestion: null,
    }
  }

  async *sendMessage(
    sessionId: string,
    content: string,
    attachments: Attachment[] = [],
    signal?: AbortSignal
  ): AsyncGenerator<StreamChunk> {
    yield { type: 'user_message', sessionId }

    // Simulate streaming response
    const response = `You said: ${content}`
    for (let i = 0; i < response.length; i++) {
      if (signal?.aborted) {
        yield { type: 'error', error: 'Cancelled' }
        return
      }

      await new Promise((resolve) => setTimeout(resolve, 50))
      yield { type: 'chunk', content: response[i] }
    }

    yield { type: 'done', sessionId }
  }

  async submitQuestionAnswers() {
    return { success: true }
  }

  async rejectPendingQuestion() {
    return { success: true }
  }

  async listSessions(): Promise<SessionListResult> {
    return { sessions: [], total: 0, hasMore: false }
  }

  async archiveSession(sessionId: string): Promise<Session> {
    return this.createSession()
  }

  async unarchiveSession(sessionId: string): Promise<Session> {
    return this.createSession()
  }

  async pinSession(sessionId: string): Promise<Session> {
    return this.createSession()
  }

  async unpinSession(sessionId: string): Promise<Session> {
    return this.createSession()
  }

  async deleteSession(): Promise<void> {}

  async renameSession(sessionId: string, title: string): Promise<Session> {
    return this.createSession()
  }

  async regenerateSessionTitle(sessionId: string): Promise<Session> {
    return this.createSession()
  }

  async clearSessionHistory(): Promise<{
    success: boolean
    deletedMessages: number
    deletedArtifacts: number
  }> {
    return { success: true, deletedMessages: 0, deletedArtifacts: 0 }
  }

  async compactSessionHistory(): Promise<{
    success: boolean
    summary: string
    deletedMessages: number
    deletedArtifacts: number
  }> {
    return {
      success: true,
      summary: 'No turns to compact',
      deletedMessages: 0,
      deletedArtifacts: 0,
    }
  }
}
