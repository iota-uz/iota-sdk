/**
 * BI-Chat UI Components
 * Main export file
 */

// Import styles (will be bundled as style.css)
import './styles.css'

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
export { MessageInput, type MessageInputRef, type MessageInputProps } from './components/MessageInput'
export { default as AttachmentGrid } from './components/AttachmentGrid'
export { default as ImageModal } from './components/ImageModal'
export { default as WelcomeContent } from './components/WelcomeContent'
export { default as CodeOutputsPanel } from './components/CodeOutputsPanel'
export { default as StreamingCursor } from './components/StreamingCursor'
export { default as ScrollToBottomButton } from './components/ScrollToBottomButton'
export { default as EmptyState, type EmptyStateProps } from './components/EmptyState'
export { default as EditableText, type EditableTextProps, type EditableTextRef } from './components/EditableText'
export { default as SearchInput, type SearchInputProps } from './components/SearchInput'
export {
  default as Skeleton,
  SkeletonGroup,
  SkeletonText,
  SkeletonAvatar,
  SkeletonCard,
  ListItemSkeleton,
  type SkeletonProps,
  type SkeletonGroupProps,
} from './components/Skeleton'

// Animations
export * from './animations'

// Context
export { ChatSessionProvider, useChat } from './context/ChatContext'
export { IotaContextProvider, useIotaContext, hasPermission } from './context/IotaContext'
export {
  ConfigProvider,
  useConfig,
  useRequiredConfig,
  hasPermission as hasConfigPermission,
} from './config/ConfigContext'

// Hooks
export { useStreaming } from './hooks/useStreaming'
export { useTranslation } from './hooks/useTranslation'

// Theme
export { ThemeProvider, useTheme } from './theme/ThemeProvider'
export { lightTheme, darkTheme } from './theme/themes'

// API utilities
export { getCSRFToken, addCSRFHeader, createHeadersWithCSRF } from './api/csrf'

// Data sources
export { HttpDataSource, createHttpDataSource } from './data/HttpDataSource'

// Utilities
export { RateLimiter } from './utils/RateLimiter'
export * from './utils/fileUtils'

// Types
export type {
  Session,
  Message,
  ToolCall,
  Citation,
  Attachment,
  ImageAttachment,
  QueuedMessage,
  CodeOutput,
  ChartData,
  Artifact,
  PendingQuestion,
  QuestionAnswers,
  StreamChunk,
  ChatDataSource,
  ChatSessionContextValue,
} from './types'

export type { Theme, ThemeColors, ThemeSpacing, ThemeBorderRadius } from './theme/types'

export type {
  UserContext,
  TenantContext,
  LocaleContext,
  AppConfig,
  IotaContext,
} from './types/iota'

export type { BiChatConfig } from './config/ConfigContext'
export type { RateLimiterConfig } from './utils/RateLimiter'
export type { HttpDataSourceConfig } from './data/HttpDataSource'

// Enums
export { MessageRole } from './types'

// Styles (import separately)
// import '@iotauz/bichat-ui/styles.css'
