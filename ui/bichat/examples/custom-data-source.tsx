/**
 * Custom Data Source Example
 *
 * This example demonstrates how to implement a custom ChatDataSource
 * for use with the BiChat library.
 */

import {
  ChatDataSource,
  Session,
  Message,
  PendingQuestion,
  Attachment,
  StreamChunk,
  QuestionAnswers,
  MessageRole,
} from '@iotauz/iota-sdk/bichat'

/**
 * Mock Data Source using local storage and mock API
 */
export class MockDataSource implements ChatDataSource {
  private baseUrl: string
  private sessions: Map<string, Session> = new Map()
  private messages: Map<string, Message[]> = new Map()

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
        this.messages = new Map(data.messages)
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
      messages: Array.from(this.messages.entries()),
    }
    localStorage.setItem('bichat_sessions', JSON.stringify(data))
  }

  /**
   * Generate unique ID
   */
  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
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
    this.messages.set(session.id, [])
    this.saveToLocalStorage()

    return session
  }

  /**
   * Fetch an existing session
   */
  async fetchSession(id: string): Promise<{
    session: Session
    messages: Message[]
    pendingQuestion?: PendingQuestion | null
  } | null> {
    const session = this.sessions.get(id)
    if (!session) {
      return null
    }

    const messages = this.messages.get(id) || []

    return {
      session,
      messages,
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
    // Add user message
    const userMessage: Message = {
      id: this.generateId(),
      sessionId,
      role: MessageRole.User,
      content,
      createdAt: new Date().toISOString(),
    }

    const messages = this.messages.get(sessionId) || []
    messages.push(userMessage)
    this.messages.set(sessionId, messages)
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
                // Save assistant message
                const assistantMessage: Message = {
                  id: this.generateId(),
                  sessionId,
                  role: MessageRole.Assistant,
                  content: assistantContent,
                  createdAt: new Date().toISOString(),
                }

                messages.push(assistantMessage)
                this.messages.set(sessionId, messages)
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
   * Cancel a pending question
   */
  async cancelPendingQuestion(
    questionId: string
  ): Promise<{ success: boolean; error?: string }> {
    try {
      const response = await fetch(`${this.baseUrl}/api/questions/${questionId}/cancel`, {
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
      messages: [],
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

  async cancelPendingQuestion() {
    return { success: true }
  }
}
