/**
 * Type definitions for BI-Chat UI components
 */

// ============================================================================
// Session Types
// ============================================================================

export interface Session {
  id: string
  title: string
  status: 'active' | 'archived'
  pinned: boolean
  createdAt: string
  updatedAt: string
}

// ============================================================================
// Turn-Based Architecture Types
// ============================================================================

/**
 * A conversation turn groups a user message with its assistant response.
 * This provides a cleaner mental model than flat message lists.
 */
export interface ConversationTurn {
  id: string
  sessionId: string
  userTurn: UserTurn
  assistantTurn?: AssistantTurn
  createdAt: string
}

/**
 * Content of a user's message in a conversation turn
 */
export interface UserTurn {
  id: string
  content: string
  attachments: Attachment[]
  createdAt: string
}

/**
 * Content of an assistant's response in a conversation turn
 */
export interface AssistantTurn {
  id: string
  role?: MessageRole
  content: string
  explanation?: string
  citations: Citation[]
  toolCalls?: ToolCall[]
  chartData?: ChartData
  artifacts: Artifact[]
  codeOutputs: CodeOutput[]
  debug?: DebugTrace
  createdAt: string
}

// ============================================================================
// Message Role Enum
// ============================================================================

/**
 * Role of a message in a conversation
 */
export enum MessageRole {
  User = 'user',
  Assistant = 'assistant',
  System = 'system',
  Tool = 'tool',
}

// ============================================================================
// Tool Call Types
// ============================================================================

/**
 * A tool/function call made by the assistant
 */
export interface ToolCall {
  id: string
  name: string
  arguments: string
  result?: string
  error?: string
  durationMs?: number
}

// ============================================================================
// Citation Types
// ============================================================================

/**
 * Citation with position information for inline replacement
 */
export interface Citation {
  id: string
  /** Type of citation (e.g., "url_citation") */
  type: string
  /** Title of the cited source */
  title: string
  /** URL of the cited source */
  url: string
  /** Starting character index in the message content where this citation is referenced */
  startIndex: number
  /** Ending character index in the message content where this citation is referenced */
  endIndex: number
  /** Optional excerpt from the source */
  excerpt?: string
  /** Legacy: source name (for backward compatibility) */
  source?: string
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

// ============================================================================
// Code Interpreter Output Types
// ============================================================================

/**
 * Output from code interpreter tool
 */
export interface CodeOutput {
  type: 'image' | 'text' | 'error'
  content: string
  /** File metadata for downloadable outputs */
  filename?: string
  mimeType?: string
  sizeBytes?: number
}

// ============================================================================
// Message Queue Types
// ============================================================================

/**
 * Queued message for offline/loading state
 */
export interface QueuedMessage {
  content: string
  attachments: ImageAttachment[]
}

// ============================================================================
// Chart Types (ApexCharts format)
// ============================================================================

/**
 * Chart visualization data for ApexCharts
 */
export interface ChartData {
  /** Type of chart: line, bar, pie, area, or donut */
  chartType: 'line' | 'bar' | 'area' | 'pie' | 'donut'
  /** Chart title displayed above the chart */
  title: string
  /** Data series (multiple allowed for line/bar/area, single for pie/donut) */
  series: ChartSeries[]
  /** X-axis category labels or segment labels for pie/donut */
  labels?: string[]
  /** Hex color codes for series (e.g., '#4CAF50') */
  colors?: string[]
  /** Chart height in pixels */
  height?: number
}

/**
 * A single data series in a chart
 */
export interface ChartSeries {
  /** Display name for this series */
  name: string
  /** Numeric data values */
  data: number[]
}

export interface Artifact {
  type: 'excel' | 'pdf'
  filename: string
  url: string
  sizeReadable?: string
  rowCount?: number
  description?: string
}

// ============================================================================
// HITL (Human-in-the-Loop) Question Types
// ============================================================================

export interface PendingQuestion {
  id: string
  turnId: string
  questions: Question[]
  status: 'PENDING' | 'ANSWERED' | 'CANCELLED'
}

export interface Question {
  id: string
  text: string
  type: 'SINGLE_CHOICE' | 'MULTIPLE_CHOICE'
  options?: QuestionOption[]
  required?: boolean
}

export interface QuestionOption {
  id: string
  label: string
  value: string
}

/**
 * Answer data for a single question, including predefined options and custom "Other" text.
 */
export interface QuestionAnswerData {
  /** Selected predefined options (labels) */
  options: string[]
  /** Custom text entered when user selects "Other" option */
  customText?: string
}

/**
 * Map of question IDs to answer data.
 * Supports both multi-select options and custom "Other" text input.
 */
export interface QuestionAnswers {
  [questionId: string]: QuestionAnswerData
}

export interface StreamChunk {
  type: 'chunk' | 'content' | 'tool_start' | 'tool_end' | 'usage' | 'done' | 'error' | 'user_message'
  content?: string
  error?: string
  sessionId?: string
  usage?: TokenUsage
  tool?: StreamToolPayload
  generationMs?: number
  timestamp?: number
}

export interface TokenUsage {
  promptTokens: number
  completionTokens: number
  totalTokens: number
  cachedTokens?: number
  cost?: number
}

export interface StreamToolPayload {
  callId?: string
  name: string
  arguments?: string
  result?: string
  error?: string
  durationMs?: number
}

export interface DebugTrace {
  generationMs?: number
  usage?: TokenUsage
  tools: StreamToolPayload[]
}

export interface SendMessageOptions {
  debugMode?: boolean
  replaceFromMessageID?: string
}

// ============================================================================
// Data Source Interface
// ============================================================================

export interface SessionListResult {
  sessions: Session[]
  total: number
  hasMore: boolean
}

export interface SessionUser {
  id: string
  firstName: string
  lastName: string
  initials: string
}

export interface SessionGroup {
  name: string
  sessions: Session[]
}

export interface ChatDataSource {
  // Core operations
  createSession(): Promise<Session>
  fetchSession(id: string): Promise<{
    session: Session
    turns: ConversationTurn[]
    pendingQuestion?: PendingQuestion | null
  } | null>
  sendMessage(
    sessionId: string,
    content: string,
    attachments?: Attachment[],
    signal?: AbortSignal,
    options?: SendMessageOptions
  ): AsyncGenerator<StreamChunk>
  clearSessionHistory(sessionId: string): Promise<{
    success: boolean
    deletedMessages: number
    deletedArtifacts: number
  }>
  compactSessionHistory(sessionId: string): Promise<{
    success: boolean
    summary: string
    deletedMessages: number
    deletedArtifacts: number
  }>
  submitQuestionAnswers(
    sessionId: string,
    questionId: string,
    answers: QuestionAnswers
  ): Promise<{ success: boolean; error?: string }>
  cancelPendingQuestion(questionId: string): Promise<{ success: boolean; error?: string }>
  navigateToSession?(sessionId: string): void

  // Session management
  listSessions(options?: {
    limit?: number
    offset?: number
    includeArchived?: boolean
  }): Promise<SessionListResult>
  archiveSession(sessionId: string): Promise<Session>
  unarchiveSession(sessionId: string): Promise<Session>
  pinSession(sessionId: string): Promise<Session>
  unpinSession(sessionId: string): Promise<Session>
  deleteSession(sessionId: string): Promise<void>
  renameSession(sessionId: string, title: string): Promise<Session>
  regenerateSessionTitle(sessionId: string): Promise<Session>

  // Organization-wide features (optional)
  listUsers?(): Promise<SessionUser[]>
  listAllSessions?(options?: {
    limit?: number
    offset?: number
    includeArchived?: boolean
    userId?: string | null
  }): Promise<{
    sessions: Array<Session & { owner: SessionUser }>
    total: number
    hasMore: boolean
  }>
}

// ============================================================================
// Context Value Types
// ============================================================================

export interface ChatSessionContextValue {
  // State
  message: string
  turns: ConversationTurn[]
  loading: boolean
  error: string | null
  inputError: string | null
  currentSessionId?: string
  pendingQuestion: PendingQuestion | null
  session: Session | null
  fetching: boolean
  streamingContent: string
  isStreaming: boolean
  messageQueue: QueuedMessage[]
  codeOutputs: CodeOutput[]
  debugMode: boolean
  isCompacting: boolean
  compactionSummary: string | null

  // Setters
  setMessage: (message: string) => void
  setError: (error: string | null) => void
  setInputError: (error: string | null) => void
  setCodeOutputs: (outputs: CodeOutput[]) => void

  // Handlers
  handleSubmit: (e: React.FormEvent, attachments?: ImageAttachment[]) => void
  handleRegenerate?: (turnId: string) => Promise<void>
  handleEdit?: (turnId: string, newContent: string) => Promise<void>
  handleCopy: (text: string) => Promise<void>
  handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void
  handleCancelPendingQuestion: () => Promise<void>
  handleRetry?: () => Promise<void>
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
