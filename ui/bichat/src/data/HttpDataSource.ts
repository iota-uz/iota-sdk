/**
 * Built-in HTTP data source with SSE streaming and AbortController
 * Implements ChatDataSource interface with real HTTP/GraphQL calls
 */

import type {
  ChatDataSource,
  Session,
  Message,
  PendingQuestion,
  Attachment,
  StreamChunk,
  QuestionAnswers,
} from '../types'

export interface HttpDataSourceConfig {
  baseUrl: string
  graphQLEndpoint?: string
  streamEndpoint?: string
  csrfToken?: string | (() => string)
  headers?: Record<string, string>
  timeout?: number
}

interface SessionState {
  session: Session
  messages: Message[]
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

  constructor(config: HttpDataSourceConfig) {
    this.config = {
      graphQLEndpoint: '/graphql',
      streamEndpoint: '/stream',
      timeout: 30000,
      ...config,
    }
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

  /**
   * Execute GraphQL query
   */
  private async graphql<T>(query: string, variables?: Record<string, any>): Promise<T> {
    const url = `${this.config.baseUrl}${this.config.graphQLEndpoint}`

    const response = await fetch(url, {
      method: 'POST',
      headers: this.createHeaders(),
      body: JSON.stringify({ query, variables }),
      signal: this.abortController?.signal,
    })

    if (!response.ok) {
      throw new Error(`GraphQL request failed: ${response.statusText}`)
    }

    const result = await response.json()

    if (result.errors && result.errors.length > 0) {
      throw new Error(result.errors[0].message || 'GraphQL error')
    }

    return result.data
  }

  /**
   * Create a new chat session
   */
  async createSession(): Promise<Session> {
    const query = `
      mutation CreateChatSession {
        createChatSession {
          id
          title
          status
          pinned
          createdAt
          updatedAt
        }
      }
    `

    const data = await this.graphql<{ createChatSession: Session }>(query)
    return data.createChatSession
  }

  /**
   * Fetch an existing session with messages
   */
  async fetchSession(id: string): Promise<SessionState | null> {
    const query = `
      query GetChatSession($id: ID!) {
        chatSession(id: $id) {
          session {
            id
            title
            status
            pinned
            createdAt
            updatedAt
          }
          messages {
            id
            sessionId
            role
            content
            createdAt
            toolCalls {
              id
              name
              arguments
            }
            citations {
              id
              source
              url
              excerpt
            }
            chartData {
              type
              title
              data
              xAxisKey
              yAxisKey
            }
            artifacts {
              type
              filename
              url
              sizeReadable
              rowCount
              description
            }
            explanation
          }
          pendingQuestion {
            id
            turnId
            question
            type
            options
            status
          }
        }
      }
    `

    try {
      const data = await this.graphql<{ chatSession: SessionState | null }>(query, { id })
      return data.chatSession
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
    signal?: AbortSignal
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
                const chunk = JSON.parse(data) as StreamChunk
                yield chunk

                // Stop if done or error
                if (chunk.type === 'done' || chunk.type === 'error') {
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
   * Submit answers to a pending question
   */
  async submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<Result<void>> {
    const query = `
      mutation SubmitQuestionAnswers($sessionId: ID!, $questionId: ID!, $answers: JSON!) {
        submitQuestionAnswers(sessionId: $sessionId, questionId: $questionId, answers: $answers) {
          success
          error
        }
      }
    `

    try {
      const data = await this.graphql<{
        submitQuestionAnswers: Result<void>
      }>(query, { sessionId, questionId, answers })

      return data.submitQuestionAnswers
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : 'Failed to submit answers',
      }
    }
  }

  /**
   * Cancel a pending question
   */
  async cancelPendingQuestion(questionId: string): Promise<Result<void>> {
    const query = `
      mutation CancelPendingQuestion($questionId: ID!) {
        cancelPendingQuestion(questionId: $questionId) {
          success
          error
        }
      }
    `

    try {
      const data = await this.graphql<{
        cancelPendingQuestion: Result<void>
      }>(query, { questionId })

      return data.cancelPendingQuestion
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : 'Failed to cancel question',
      }
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
}

/**
 * Factory function to create HttpDataSource
 */
export function createHttpDataSource(config: HttpDataSourceConfig): ChatDataSource {
  return new HttpDataSource(config)
}
