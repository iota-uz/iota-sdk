/**
 * BI-Chat UI Components
 * Main export file
 */

// Import styles (will be bundled as style.css)
import './styles.css'

// =============================================================================
// Layer 4: Full Components (Backward Compatible API)
// =============================================================================

export { ChatSession } from './components/ChatSession'
export { ChatHeader } from './components/ChatHeader'
export { MessageList } from './components/MessageList'
export { TurnBubble, type TurnBubbleProps, type TurnBubbleClassNames } from './components/TurnBubble'
export { UserTurnView, type UserTurnViewProps } from './components/UserTurnView'
export { AssistantTurnView, type AssistantTurnViewProps } from './components/AssistantTurnView'
export { MarkdownRenderer } from './components/MarkdownRenderer'
export { ChartCard } from './components/ChartCard'
export { SourcesPanel } from './components/SourcesPanel'
export { DownloadCard } from './components/DownloadCard'
export { InlineQuestionForm } from './components/InlineQuestionForm'
export { MessageInput, type MessageInputRef, type MessageInputProps } from './components/MessageInput'
export { AttachmentGrid } from './components/AttachmentGrid'
export { ImageModal } from './components/ImageModal'
export { WelcomeContent } from './components/WelcomeContent'
export { CodeOutputsPanel } from './components/CodeOutputsPanel'
export { StreamingCursor } from './components/StreamingCursor'
export { ScrollToBottomButton } from './components/ScrollToBottomButton'
export { EmptyState, type EmptyStateProps } from './components/EmptyState'
export { EditableText, type EditableTextProps, type EditableTextRef } from './components/EditableText'
export { SearchInput, type SearchInputProps } from './components/SearchInput'
export {
  Skeleton,
  SkeletonGroup,
  SkeletonText,
  SkeletonAvatar,
  SkeletonCard,
  ListItemSkeleton,
  type SkeletonProps,
  type SkeletonGroupProps,
} from './components/Skeleton'

// Phase 2 components
export { CodeBlock } from './components/CodeBlock'
export { LoadingSpinner } from './components/LoadingSpinner'
export { TableExportButton } from './components/TableExportButton'
export { TableWithExport } from './components/TableWithExport'

// Phase 5 generic components
export { Toast, type ToastProps } from './components/Toast'
export { ToastContainer } from './components/ToastContainer'
export { ConfirmModal, type ConfirmModalProps } from './components/ConfirmModal'
export { UserAvatar, type UserAvatarProps } from './components/UserAvatar'
export { PermissionGuard, type PermissionGuardProps } from './components/PermissionGuard'
export { ErrorBoundary, DefaultErrorContent } from './components/ErrorBoundary'
export { TypingIndicator, type TypingIndicatorProps, type TypingIndicatorVariant } from './components/TypingIndicator'

// =============================================================================
// Layer 3: Composites (Styled with Slots)
// =============================================================================

export {
  UserMessage,
  type UserMessageProps,
  type UserMessageSlots,
  type UserMessageClassNames,
  type UserMessageAvatarSlotProps,
  type UserMessageContentSlotProps,
  type UserMessageAttachmentsSlotProps,
  type UserMessageActionsSlotProps,
} from './components/UserMessage'

export {
  AssistantMessage,
  type AssistantMessageProps,
  type AssistantMessageSlots,
  type AssistantMessageClassNames,
  type AssistantMessageAvatarSlotProps,
  type AssistantMessageContentSlotProps,
  type AssistantMessageSourcesSlotProps,
  type AssistantMessageChartsSlotProps,
  type AssistantMessageCodeOutputsSlotProps,
  type AssistantMessageArtifactsSlotProps,
  type AssistantMessageActionsSlotProps,
  type AssistantMessageExplanationSlotProps,
} from './components/AssistantMessage'

// =============================================================================
// Layer 2: Primitives (Unstyled Compound Components)
// =============================================================================

// Primitives are exported from a separate entry point for tree-shaking
// import { Turn, Avatar, Bubble, ActionButton } from '@iota-uz/sdk/bichat/primitives'
export * from './primitives'

// =============================================================================
// Layer 1: Headless Hooks
// =============================================================================

// Existing hooks
export { useStreaming } from './hooks/useStreaming'
export { useTranslation } from './hooks/useTranslation'
export { useModalLock } from './hooks/useModalLock'
export { useFocusTrap } from './hooks/useFocusTrap'
export { useToast, type ToastItem, type ToastType, type UseToastReturn } from './hooks/useToast'

// New composability hooks
export {
  useImageGallery,
  type UseImageGalleryOptions,
  type UseImageGalleryReturn,
} from './hooks/useImageGallery'

export {
  useAutoScroll,
  type UseAutoScrollOptions,
  type UseAutoScrollReturn,
} from './hooks/useAutoScroll'

export {
  useMessageActions,
  type UseMessageActionsOptions,
  type UseMessageActionsReturn,
} from './hooks/useMessageActions'

export {
  useAttachments,
  type UseAttachmentsOptions,
  type UseAttachmentsReturn,
  type FileValidationError,
} from './hooks/useAttachments'

export {
  useMarkdownCopy,
  type UseMarkdownCopyOptions,
  type UseMarkdownCopyReturn,
} from './hooks/useMarkdownCopy'

// =============================================================================
// Animations
// =============================================================================

export * from './animations'

// =============================================================================
// Context
// =============================================================================

export { ChatSessionProvider, useChat } from './context/ChatContext'
export { IotaContextProvider, useIotaContext, hasPermission } from './context/IotaContext'
export {
  ConfigProvider,
  useConfig,
  useRequiredConfig,
  hasPermission as hasConfigPermission,
} from './config/ConfigContext'

// =============================================================================
// Theme
// =============================================================================

export { ThemeProvider, useTheme } from './theme/ThemeProvider'
export { lightTheme, darkTheme } from './theme/themes'

// =============================================================================
// API Utilities
// =============================================================================

export { getCSRFToken, addCSRFHeader, createHeadersWithCSRF } from './api/csrf'

// =============================================================================
// Data Sources
// =============================================================================

export { HttpDataSource, createHttpDataSource } from './data/HttpDataSource'

// =============================================================================
// Utilities
// =============================================================================

export { RateLimiter } from './utils/RateLimiter'
export * from './utils/fileUtils'
export { processCitations, type ProcessedContent } from './utils/citationProcessor'

// =============================================================================
// Types
// =============================================================================

export type {
  // Core types
  Session,
  ToolCall,
  Citation,
  Attachment,
  ImageAttachment,
  QueuedMessage,
  CodeOutput,
  ChartData,
  ChartSeries,
  Artifact,
  // HITL question types
  PendingQuestion,
  Question,
  QuestionOption,
  QuestionAnswerData,
  QuestionAnswers,
  // Streaming types
  StreamChunk,
  // Data source interface
  ChatDataSource,
  ChatSessionContextValue,
  // Turn-based architecture types
  ConversationTurn,
  UserTurn,
  AssistantTurn,
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

// =============================================================================
// Enums
// =============================================================================

export { MessageRole } from './types'

// =============================================================================
// CSS Variables Reference
// =============================================================================
// The styles.css file provides comprehensive CSS variables for theming.
// Import styles: import '@iota-uz/sdk/bichat/styles.css'
//
// Key variable prefixes:
// - --bichat-spacing-*    : Spacing scale (0-16, xs/sm/md/lg/xl/2xl)
// - --bichat-color-*      : Colors (gray, primary, semantic, component-specific)
// - --bichat-font-*       : Typography (family, size, weight, line-height)
// - --bichat-radius-*     : Border radius (sm/md/lg/xl/2xl/full, semantic)
// - --bichat-shadow-*     : Box shadows (xs/sm/md/lg/xl/2xl)
// - --bichat-transition-* : Transition durations
// - --bichat-z-*          : Z-index scale
//
// Dark mode: Use .dark class or [data-theme="dark"] attribute
