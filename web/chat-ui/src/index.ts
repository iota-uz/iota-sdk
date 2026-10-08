/**
 * BI-Chat UI Components
 * Main export file
 */
// Import styles (will be bundled as style.css)
import './styles.css';
// =============================================================================
// Layer 4: Full Components
// =============================================================================
export { ChatSession, type ChatSessionProps } from './components/ChatSession';
export { ModelSelector } from './components/ModelSelector';
export { SessionArtifactsPanel } from './components/SessionArtifactsPanel';
export { SessionArtifactList } from './components/SessionArtifactList';
export { SessionArtifactPreview } from './components/SessionArtifactPreview';
export { ChatHeader } from './components/ChatHeader';
export { MessageList } from './components/MessageList';
export { TurnBubble, type TurnBubbleProps, type TurnBubbleClassNames } from './components/TurnBubble';
export { UserTurnView, type UserTurnViewProps } from './components/UserTurnView';
export { AssistantTurnView, type AssistantTurnViewProps } from './components/AssistantTurnView';
export { MarkdownRenderer } from './components/MarkdownRenderer';
export { ChartCard, type ChartCardHost } from './components/ChartCard';
export { SourcesPanel } from './components/SourcesPanel';
export { DownloadCard } from './components/DownloadCard';
export { InlineQuestionForm } from './components/InlineQuestionForm';
export { MessageInput, type MessageInputRef, type MessageInputProps } from './components/MessageInput';
export { AttachmentGrid } from './components/AttachmentGrid';
export { ImageModal } from './components/ImageModal';
export { WelcomeContent } from './components/WelcomeContent';
export { CodeOutputsPanel } from './components/CodeOutputsPanel';
export { StreamingCursor } from './components/StreamingCursor';
export { ScrollToBottomButton } from './components/ScrollToBottomButton';
export { CompactionDoodle } from './components/CompactionDoodle';
export { EmptyState, type EmptyStateProps } from './components/EmptyState';
export { EditableText, type EditableTextProps, type EditableTextRef } from './components/EditableText';
export { SearchInput, type SearchInputProps } from './components/SearchInput';
export { Skeleton, SkeletonGroup, SkeletonText, SkeletonAvatar, SkeletonCard, ListItemSkeleton, type SkeletonProps, type SkeletonGroupProps, } from './components/Skeleton';
// Phase 2 components
export { CodeBlock } from './components/CodeBlock';
export { LoadingSpinner } from './components/LoadingSpinner';
export { TableExportButton } from './components/TableExportButton';
export { TableWithExport } from './components/TableWithExport';
export { InteractiveTableCard, type TableCardHost } from './components/InteractiveTableCard';
export { TabbedTableGroup, type TabbedTableGroupProps } from './components/TabbedTableGroup';
export { TabbedChartGroup, type TabbedChartGroupProps } from './components/TabbedChartGroup';
export { useDataTable, type UseDataTableReturn, type DataTableOptions, type ColumnMeta, type SortState, type ColumnStats, } from './hooks/useDataTable';
export { type ColumnType, type FormattedCell } from './utils/columnTypes';
// Phase 5 generic components
export { Toast, type ToastProps } from './components/Toast';
export { ToastContainer } from './components/ToastContainer';
export { ConfirmModal, type ConfirmModalProps } from './components/ConfirmModal';
export { UserAvatar, type UserAvatarProps } from './components/UserAvatar';
export { AvatarStack, type AvatarStackProps } from './components/AvatarStack';
export { SessionMembersModal, type SessionMembersModalProps } from './components/SessionMembersModal';
export { PermissionGuard, type PermissionGuardProps } from './components/PermissionGuard';
export { ErrorBoundary, DefaultErrorContent } from './components/ErrorBoundary';
export { TypingIndicator, type TypingIndicatorProps } from './components/TypingIndicator';
export { ActivityTrace, type ActivityTraceProps } from './components/ActivityTrace';
// Session management components
export { default as Sidebar } from './components/Sidebar';
export type { SidebarProps, ActiveTab } from './components/Sidebar';
export { default as SessionItem } from './components/SessionItem';
export { default as ArchivedChatList } from './components/ArchivedChatList';
export { default as AllChatsList } from './components/AllChatsList';
export { UserFilter } from './components/UserFilter';
export { default as DateGroupHeader } from './components/DateGroupHeader';
export { default as SessionSkeleton } from './components/SessionSkeleton';
// Layout component
export { BiChatLayout, type BiChatLayoutProps, type SidebarDrawerProps } from './components/BiChatLayout';
// Specialized message components
export { SystemMessage } from './components/SystemMessage';
export { DebugPanel, type DebugPanelProps } from './components/DebugPanel';
// Generic UI components
export { default as Alert } from './components/Alert';
export { default as ArchiveBanner } from './components/ArchiveBanner';
export { RetryActionArea } from './components/RetryActionArea';
export { StreamError } from './components/StreamError';
export { MessageActions } from './components/MessageActions';
export { default as AttachmentPreview } from './components/AttachmentPreview';
export { default as AttachmentUpload } from './components/AttachmentUpload';
export { default as ScreenReaderAnnouncer } from './components/ScreenReaderAnnouncer';
export { default as SkipLink } from './components/SkipLink';
export { TouchContextMenu } from './components/TouchContextMenu';
// Question form wizard
export { default as QuestionForm } from './components/QuestionForm';
export { default as QuestionStep } from './components/QuestionStep';
export { default as ConfirmationStep } from './components/ConfirmationStep';
// =============================================================================
// Layer 3: Composites (Styled with Slots)
// =============================================================================
export { UserMessage, type UserMessageProps, type UserMessageSlots, type UserMessageClassNames, type UserMessageAvatarSlotProps, type UserMessageContentSlotProps, type UserMessageAttachmentsSlotProps, type UserMessageActionsSlotProps, } from './components/UserMessage';
export { AssistantMessage, type AssistantMessageProps, type AssistantMessageSlots, type AssistantMessageClassNames, type AssistantMessageAvatarSlotProps, type AssistantMessageContentSlotProps, type AssistantMessageSourcesSlotProps, type AssistantMessageChartsSlotProps, type AssistantMessageCodeOutputsSlotProps, type AssistantMessageTablesSlotProps, type AssistantMessageArtifactsSlotProps, type AssistantMessageActionsSlotProps, type AssistantMessageExplanationSlotProps, type RegenerateModelOption, } from './components/AssistantMessage';
// =============================================================================
// Layer 2: Primitives (Unstyled Compound Components)
// =============================================================================
// Primitives are exported from a separate entry point for tree-shaking
// import { Turn, Avatar, Bubble, ActionButton } from '@iota-uz/sdk/chat-ui/primitives'
// =============================================================================
// Layer 1: Headless Hooks
// =============================================================================
// Existing hooks
export { useActiveRuns, type UseActiveRunsOptions, type UseActiveRunsResult, type ActiveRunSnapshot, } from './hooks/useActiveRuns';
export { useTranslation } from './hooks/useTranslation';
export { useModalLock } from './hooks/useModalLock';
export { useFocusTrap } from './hooks/useFocusTrap';
export { useToast, type ToastAction, type ToastItem, type ToastType, type UseToastReturn } from './hooks/useToast';
// New composability hooks
export { useImageGallery, type UseImageGalleryOptions, type UseImageGalleryReturn, } from './hooks/useImageGallery';
export { useAutoScroll, type UseAutoScrollOptions, type UseAutoScrollReturn, } from './hooks/useAutoScroll';
export { useMessageActions, type UseMessageActionsOptions, type UseMessageActionsReturn, } from './hooks/useMessageActions';
export { useAttachments, type UseAttachmentsOptions, type UseAttachmentsReturn, type FileValidationError, } from './hooks/useAttachments';
export { useMarkdownCopy, type UseMarkdownCopyOptions, type UseMarkdownCopyReturn, } from './hooks/useMarkdownCopy';
// Session & interaction hooks
export { useScrollToBottom } from './hooks/useScrollToBottom';
export { useKeyboardShortcuts, type ShortcutConfig } from './hooks/useKeyboardShortcuts';
export { useLongPress } from './hooks/useLongPress';
export { useSidebarState, type UseSidebarStateReturn } from './hooks/useSidebarState';
export { useHttpDataSourceConfigFromApplet } from './hooks/useHttpDataSourceConfigFromApplet';
export { useBichatRouter, type UseBichatRouterParams, type UseBichatRouterReturn, } from './hooks/useBichatRouter';
// =============================================================================
// Animations
// =============================================================================
export * from './animations';
// =============================================================================
// Context
// =============================================================================
export { ChatSessionProvider, useChatSession, useChatMessaging, useOptionalChatMessaging, useChatInput, type ChatSessionProviderProps } from './context/ChatContext';
export { IotaContextProvider, useIotaContext, hasPermission } from './context/IotaContext';
export { ConfigProvider, useConfig, useRequiredConfig, } from './config/ConfigContext';
// =============================================================================
// Theme
// =============================================================================
export { lightTheme, darkTheme } from './theme/themes';
// =============================================================================
// API Utilities
// =============================================================================
export { getCSRFToken, addCSRFHeader, createHeadersWithCSRF } from './api/csrf';
// =============================================================================
// Data Sources
// =============================================================================
export { HttpDataSource, createHttpDataSource } from './data/HttpDataSource';
export type { BichatRPC } from './data/rpc.generated';
export { RunEventsConnectError } from './data/MessageTransport';
// =============================================================================
// Machine (framework-agnostic state management)
// =============================================================================
export { ChatMachine, type ChatMachineConfig } from './machine';
// =============================================================================
// SSE Parsing
// =============================================================================
export { parseBichatStream, parseBichatStreamEvents, parseSSEStream } from './utils/sseParser';
// =============================================================================
// Utilities
// =============================================================================
export { RateLimiter } from './utils/RateLimiter';
export { getToolLabel } from './utils/toolLabels';
export { groupSteps } from './utils/activitySteps';
export * from './utils/fileUtils';
export { groupSessionsByDate } from './utils/sessionGrouping';
export { toErrorDisplay, isPermissionDeniedError, type RPCErrorDisplay } from './utils/errorDisplay';
export { splitIntoTextBlocks, readTextBlockOffsets, type AssistantTextBlock, } from './utils/textBlocks';
export { STREAM_EVENT_TYPES, TERMINAL_STREAM_EVENT_TYPES, isTerminalEvent, type StreamEventType, type TerminalStreamEventType, } from './utils/eventNames';
// =============================================================================
// Types
// =============================================================================
export type { 
// Core types
Session, ToolCall, Citation, Attachment, ImageAttachment, QueuedMessage, CodeOutput, ChartData, ChartSeries, RenderTableData, RenderTableExport, Artifact, SessionArtifact, 
// HITL question types
PendingQuestion, Question, QuestionOption, QuestionAnswerData, QuestionAnswers, 
// Streaming types
StreamEvent, StreamChunk, ActiveRunDelivery, 
// Split data source interfaces
SessionStore, MessageTransport, ArtifactStore, AdminStore, 
// Combined data source interface
ChatDataSource, ChatSessionContextValue, ChatSessionStateValue, ChatMessagingStateValue, ChatInputStateValue, 
// Session management types
SessionListResult, SessionUser, SessionMember, SessionAccess, SessionGroup, 
// Turn-based architecture types
ConversationTurn, UserTurn, AssistantTurn, 
// Activity trace types
ActivityStep, } from './types';
export type { Theme, ThemeColors, ThemeSpacing, ThemeBorderRadius } from './theme/types';
export type { UserContext, TenantContext, LocaleContext, AppConfig, IotaContext, } from './types/iota';
export type { BiChatConfig } from './config/ConfigContext';
export type { RateLimiterConfig } from './utils/RateLimiter';
export type { HttpDataSourceConfig } from './data/HttpDataSource';
// =============================================================================
// Enums
// =============================================================================
export { MessageRole } from './types';
// =============================================================================
// CSS Variables Reference
// =============================================================================
// The styles.css file provides comprehensive CSS variables for theming.
// Import styles: import '@iota-uz/sdk/chat-ui/styles.css'
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

export type { ErrorInfo } from './components/ErrorBoundary';
export { createReactiveSnapshot, createReactiveValue } from './context/externalSnapshot';
export { ManagedHttpDataSource } from './data/ManagedHttpDataSource';
