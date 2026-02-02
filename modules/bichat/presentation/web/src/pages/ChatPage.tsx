import { useParams } from 'react-router-dom'
import { useMemo } from 'react'
import { ChatSession } from '@iota-uz/bichat-ui'
import type { ChatDataSource, Session, Message, StreamChunk, Attachment, QuestionAnswers } from '@iota-uz/bichat-ui'
import { useClient } from 'urql'
import { useIotaContext } from '../contexts/IotaContext'

const SessionQuery = `
  query Session($id: UUID!) {
    session(id: $id) {
      id
      title
      status
      pinned
      createdAt
      updatedAt
      messages {
        id
        sessionID
        role
        content
        createdAt
        toolCalls {
          id
          name
          arguments
        }
        citations {
          source
          title
          url
          excerpt
        }
      }
    }
  }
`

export default function ChatPage() {
  const { id } = useParams<{ id: string }>()
  const client = useClient()
  const context = useIotaContext()

  const dataSource = useMemo<ChatDataSource>(() => ({
    async createSession(): Promise<Session> {
      const mutation = `
        mutation CreateSession($title: String) {
          createSession(title: $title) {
            id
            title
            status
            pinned
            createdAt
            updatedAt
          }
        }
      `
      const result = await client.mutation(mutation, { title: 'New Chat' }).toPromise()
      if (result.error) throw new Error(result.error.message)
      return result.data.createSession
    },

    async fetchSession(sessionId: string) {
      const result = await client.query(SessionQuery, { id: sessionId }).toPromise()
      if (result.error) throw new Error(result.error.message)
      if (!result.data?.session) return null

      const session = result.data.session
      return {
        session: {
          id: session.id,
          title: session.title,
          status: session.status.toLowerCase() as 'active' | 'archived',
          pinned: session.pinned,
          createdAt: session.createdAt,
          updatedAt: session.updatedAt,
        },
        messages: session.messages.map((msg: any) => ({
          id: msg.id,
          sessionId: msg.sessionID,
          role: msg.role.toLowerCase(),
          content: msg.content,
          createdAt: msg.createdAt,
          toolCalls: msg.toolCalls,
          citations: msg.citations?.map((c: any, idx: number) => ({
            id: `${msg.id}-citation-${idx}`,
            source: c.source,
            url: c.url,
            excerpt: c.excerpt,
          })),
        })) as Message[],
        pendingQuestion: null,
      }
    },

    async *sendMessage(
      sessionId: string,
      content: string,
      attachments?: Attachment[]
    ): AsyncGenerator<StreamChunk> {
      const response = await fetch(context.config.streamEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          sessionId,
          content,
          attachments: attachments || [],
        }),
      })

      if (!response.ok) {
        yield { type: 'error', error: `HTTP ${response.status}: ${response.statusText}` }
        return
      }

      if (!response.body) {
        yield { type: 'error', error: 'Response body is null' }
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      try {
        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop() || ''

          for (const line of lines) {
            if (!line.trim() || line.startsWith(':')) continue

            if (line.startsWith('data: ')) {
              const data = line.slice(6)
              try {
                const chunk = JSON.parse(data)
                yield {
                  type: chunk.type === 'CONTENT' ? 'chunk' : chunk.type.toLowerCase(),
                  content: chunk.content,
                  error: chunk.error,
                }
                if (chunk.type === 'DONE' || chunk.type === 'ERROR') return
              } catch (parseErr) {
                console.error('Failed to parse SSE data:', parseErr)
              }
            }
          }
        }
      } finally {
        reader.releaseLock()
      }
    },

    async submitQuestionAnswers(
      sessionId: string,
      questionId: string,
      answers: QuestionAnswers
    ): Promise<{ success: boolean; error?: string }> {
      const mutation = `
        mutation ResumeWithAnswer($sessionId: UUID!, $checkpointId: String!, $answers: JSON!) {
          resumeWithAnswer(sessionId: $sessionId, checkpointId: $checkpointId, answers: $answers) {
            userMessage { id }
          }
        }
      `
      const result = await client.mutation(mutation, { sessionId, checkpointId: questionId, answers }).toPromise()
      if (result.error) {
        return { success: false, error: result.error.message }
      }
      return { success: true }
    },

    async cancelPendingQuestion(): Promise<{ success: boolean; error?: string }> {
      return { success: true }
    },
  }), [client, context.config.streamEndpoint])

  if (!id) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-red-500">Session ID is required</div>
      </div>
    )
  }

  return <ChatSession dataSource={dataSource} sessionId={id} />
}
