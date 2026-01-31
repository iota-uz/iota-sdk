/**
 * Type definitions for BI-Chat UI components
 */

export interface ChatSession {
  id: string
  title: string
  status: 'active' | 'archived'
  pinned: boolean
  createdAt: string
  updatedAt: string
}

export enum MessageRole {
  User = 'user',
  Assistant = 'assistant',
  System = 'system',
  Tool = 'tool',
}

export interface Message {
  id: string
  sessionId: string
  role: MessageRole
  content: string
  createdAt: string
  toolCalls?: ToolCall[]
  citations?: Citation[]
  chartData?: ChartData
  artifacts?: Artifact[]
  explanation?: string
}

export interface ToolCall {
  id: string
  name: string
  arguments: string
}

export interface Citation {
  id: string
  source: string
  url?: string
  excerpt?: string
}

export interface Attachment {
  id: string
  filename: string
  mimeType: string
  sizeBytes: number
  base64Data?: string
}

export interface ChartData {
  type: 'bar' | 'line' | 'pie' | 'area'
  title?: string
  data: any[]
  xAxisKey?: string
  yAxisKey?: string
}

export interface Artifact {
  type: 'excel' | 'pdf'
  filename: string
  url: string
  sizeReadable?: string
  rowCount?: number
  description?: string
}

export interface PendingQuestion {
  id: string
  turnId: string
  question: string
  type: 'MULTIPLE_CHOICE' | 'FREE_TEXT'
  options?: string[]
  status: 'PENDING' | 'ANSWERED' | 'CANCELLED'
}

export type QuestionAnswers = Record<string, string>

export interface StreamChunk {
  type: 'chunk' | 'error' | 'done' | 'user_message'
  content?: string
  error?: string
  sessionId?: string
}

export interface ChatDataSource {
  createSession(): Promise<ChatSession>
  fetchSession(id: string): Promise<{
    session: ChatSession
    messages: Message[]
    pendingQuestion?: PendingQuestion | null
  } | null>
  sendMessage(
    sessionId: string,
    content: string,
    attachments?: Attachment[]
  ): AsyncGenerator<StreamChunk>
  submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }>
  cancelPendingQuestion(questionId: string): Promise<{ success: boolean; error?: string }>
  navigateToSession?(sessionId: string): void
}

export interface ChatSessionContextValue {
  // State
  message: string
  messages: Message[]
  loading: boolean
  error: string | null
  currentSessionId?: string
  pendingQuestion: PendingQuestion | null
  session: ChatSession | null
  fetching: boolean
  streamingContent: string
  isStreaming: boolean

  // Setters
  setMessage: (message: string) => void
  setError: (error: string | null) => void

  // Handlers
  handleSubmit: (e: React.FormEvent, attachments?: Attachment[]) => void
  handleRegenerate?: (messageId: string) => Promise<void>
  handleEdit?: (messageId: string, newContent: string) => Promise<void>
  handleCopy: (text: string) => Promise<void>
  handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void
  handleCancelPendingQuestion: () => Promise<void>
  sendMessage: (content: string, attachments?: Attachment[]) => Promise<void>
}
