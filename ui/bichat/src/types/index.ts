/**
 * Type definitions for BI-Chat UI components
 */

export interface Session {
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

// Image attachment with preview for MessageInput
export interface ImageAttachment {
  filename: string
  mimeType: string
  sizeBytes: number
  base64Data: string
  preview: string  // data URL for img src
}

// Code interpreter output
export interface CodeOutput {
  type: 'image' | 'text' | 'error'
  content: string | ImageAttachment
}

// Queued message for offline/loading state
export interface QueuedMessage {
  content: string
  attachments: ImageAttachment[]
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
  createSession(): Promise<Session>
  fetchSession(id: string): Promise<{
    session: Session
    messages: Message[]
    pendingQuestion?: PendingQuestion | null
  } | null>
  sendMessage(
    sessionId: string,
    content: string,
    attachments?: Attachment[],
    signal?: AbortSignal
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
  session: Session | null
  fetching: boolean
  streamingContent: string
  isStreaming: boolean
  messageQueue: QueuedMessage[]
  codeOutputs: CodeOutput[]

  // Setters
  setMessage: (message: string) => void
  setError: (error: string | null) => void
  setCodeOutputs: (outputs: CodeOutput[]) => void

  // Handlers
  handleSubmit: (e: React.FormEvent, attachments?: ImageAttachment[]) => void
  handleRegenerate?: (messageId: string) => Promise<void>
  handleEdit?: (messageId: string, newContent: string) => Promise<void>
  handleCopy: (text: string) => Promise<void>
  handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void
  handleCancelPendingQuestion: () => Promise<void>
  handleUnqueue: () => { content: string; attachments: ImageAttachment[] } | null
  sendMessage: (content: string, attachments?: Attachment[]) => Promise<void>
  cancel: () => void
}

// Translations
export type Translations = Record<string, string>

// Branding
export interface ExamplePrompt {
  category: string
  text: string
  icon: string
}

export interface BrandingConfig {
  appName: string
  logoUrl?: string
  theme?: {
    primary?: string
    secondary?: string
    accent?: string
  }
  welcome?: {
    title?: string
    description?: string
    examplePrompts?: ExamplePrompt[]
  }
  colors?: {
    primary?: string
    secondary?: string
    accent?: string
  }
  logo?: {
    src?: string
    alt?: string
  }
}
