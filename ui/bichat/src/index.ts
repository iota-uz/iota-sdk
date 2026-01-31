/**
 * BI-Chat UI Components
 * Main export file
 */

// Components
export { ChatSession } from './components/ChatSession'
export { ChatHeader } from './components/ChatHeader'
export { MessageList } from './components/MessageList'
export { TurnBubble } from './components/TurnBubble'
export { UserTurnView } from './components/UserTurnView'
export { AssistantTurnView } from './components/AssistantTurnView'
export { MarkdownRenderer } from './components/MarkdownRenderer'
export { ChartCard } from './components/ChartCard'
export { SourcesPanel } from './components/SourcesPanel'
export { DownloadCard } from './components/DownloadCard'
export { InlineQuestionForm } from './components/InlineQuestionForm'
export { MessageInput } from './components/MessageInput'

// Context
export { ChatSessionProvider, useChat } from './context/ChatContext'
export { IotaContextProvider, useIotaContext, hasPermission } from './context/IotaContext'

// Hooks
export { useStreaming } from './hooks/useStreaming'
export { useTranslation } from './hooks/useTranslation'

// API utilities
export { getCSRFToken, addCSRFHeader, createHeadersWithCSRF } from './api/csrf'

// Types
export type {
  ChatSession as ChatSessionType,
  Message,
  MessageRole,
  ToolCall,
  Citation,
  Attachment,
  ChartData,
  Artifact,
  PendingQuestion,
  QuestionAnswers,
  StreamChunk,
  ChatDataSource,
  ChatSessionContextValue,
} from './types'

export type {
  UserContext,
  TenantContext,
  LocaleContext,
  AppConfig,
  IotaContext,
} from './types/iota'

export { MessageRole } from './types'

// Styles (import separately)
// import '@iota-uz/bichat-ui/styles.css'
