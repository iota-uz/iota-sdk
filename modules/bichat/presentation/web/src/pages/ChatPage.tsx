import { useParams } from 'react-router-dom'
import { useMemo } from 'react'
import { ChatSession } from '@iotauz/bichat-ui'
import type { ChatDataSource, Session, Message, StreamChunk, Attachment, QuestionAnswers } from '@iotauz/bichat-ui'
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
        codeOutputs {
          id
          name
          mimeType
          url
          size
          createdAt
        }
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
        mutation CreateSession {
          createSession {
            id
            title
            status
            pinned
            createdAt
            updatedAt
          }
        }
      `
      const result = await client.mutation(mutation, {}).toPromise()
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
            title: c.title,
            url: c.url,
            excerpt: c.excerpt,
          })),
          codeOutputs: (msg.codeOutputs || []).map((o: any) => {
            const mimeType = (o.mimeType || '').toString()
            if (mimeType.startsWith('image/')) {
              return {
                type: 'image',
                content: {
                  filename: o.name,
                  mimeType: o.mimeType,
                  sizeBytes: o.size,
                  base64Data: '',
                  preview: o.url,
                },
              }
            }

            return {
              type: 'file',
              content: {
                id: o.id,
                name: o.name,
                mimeType: o.mimeType,
                url: o.url,
                size: o.size,
                createdAt: o.createdAt,
              },
            }
          }),
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
        credentials: 'include',
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
                
                // Map backend chunk types to frontend types
                // Backend sends: "content", "citation", "done", "error" (lowercase)
                // Frontend expects: "chunk", "error", "done", "user_message"
                let chunkType: 'chunk' | 'error' | 'done' | 'user_message'
                if (chunk.type === 'content' || chunk.type === 'citation') {
                  chunkType = 'chunk'
                } else if (chunk.type === 'done') {
                  chunkType = 'done'
                } else if (chunk.type === 'error') {
                  chunkType = 'error'
                } else {
                  // Fallback: use lowercase version
                  chunkType = chunk.type.toLowerCase() as 'chunk' | 'error' | 'done' | 'user_message'
                }
                
                yield {
                  type: chunkType,
                  content: chunk.content,
                  error: chunk.error,
                }
                if (chunk.type === 'done' || chunk.type === 'error') return
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
