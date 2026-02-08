/**
 * Default English translations for BiChat UI.
 * These serve as fallback when no custom translations are provided.
 *
 * Key naming convention:
 * - Use dot notation for namespacing: "section.key"
 * - Use {param} for interpolation: "You have {count} messages"
 */

import type { Translations } from '../types'

export const defaultTranslations: Translations = {
  // Welcome screen
  'welcome.title': 'Welcome to BiChat',
  'welcome.description':
    'Your intelligent business analytics assistant. Ask questions about your data, generate reports, or explore insights.',
  'welcome.tryAsking': 'Try asking',
  'welcome.quickStart': 'Quick Start',
  'welcome.disclaimer': 'BiChat is powered by AI. Mistakes are possible.',

  // Chat header
  'chat.newChat': 'New Chat',
  'chat.archived': 'Archived',
  'chat.readOnly': 'Read-only',
  'chat.pinned': 'Pinned',
  'chat.goBack': 'Go back',

  // Artifacts panel
  'artifacts.title': 'Artifacts',
  'artifacts.loading': 'Loading artifacts...',
  'artifacts.empty': 'No artifacts yet',
  'artifacts.back': 'Back',
  'artifacts.refresh': 'Refresh',
  'artifacts.loadMore': 'Load more',
  'artifacts.loadingMore': 'Loading more...',
  'artifacts.toggleShow': 'Show artifacts',
  'artifacts.toggleHide': 'Hide artifacts',
  'artifacts.resize': 'Resize panel',
  'artifacts.failedToLoad': 'Failed to load artifacts',
  'artifacts.unsupported': 'Artifacts panel is not available for this data source.',
  'artifacts.download': 'Download',
  'artifacts.downloadUnavailable': 'Download URL is not available for this artifact.',
  'artifacts.imageUnavailable': 'Image URL is not available for this artifact.',
  'artifacts.chartUnavailable': 'Chart metadata is not in a renderable format.',
  'artifacts.metadata': 'Metadata',
  'artifacts.typeLabel': 'Type',
  'artifacts.mimeTypeLabel': 'MIME type',
  'artifacts.sizeLabel': 'Size',
  'artifacts.createdLabel': 'Created',
  'artifacts.emptySubtitle': 'Artifacts will appear here as you chat',
  'artifacts.groupCharts': 'Charts',
  'artifacts.groupCodeOutputs': 'Code Outputs',
  'artifacts.groupExports': 'Exports',
  'artifacts.groupAttachments': 'Attachments',
  'artifacts.groupOther': 'Other',

  // Message input
  'input.placeholder': 'Type a message...',
  'input.attachFiles': 'Attach files',
  'input.attachImages': 'Attach files',
  'input.dropImages': 'Drop files here',
  'input.dropFiles': 'Drop files here',
  'input.sendMessage': 'Send message',
  'input.aiThinking': 'AI is thinking...',
  'input.processing': 'Processing...',
  'input.messagesQueued': '{count} message(s) queued',
  'input.filesAdded': 'Files added successfully',
  'input.dismissError': 'Dismiss error',
  'input.shiftEnterHint': 'Shift + Enter for new line',
  'input.fileInput': 'File input',
  'input.messageInput': 'Message input',
  'slash.commandsList': 'Slash commands',
  'slash.clearDescription': 'Clear current chat history in place',
  'slash.debugDescription': 'Toggle developer debug mode for this session',
  'slash.compactDescription': 'Compact history into a concise summary',
  'slash.noMatches': 'No matching slash command',
  'slash.debugBadge': 'Debug mode ON',
  'slash.compactingTitle': 'Compacting history',
  'slash.compactingSubtitle': 'Summarizing previous turns and pruning tool traces...',
  'slash.compactedSummaryLabel': 'Compacted Summary',
  'slash.debugPanelTitle': 'Debug Trace',
  'slash.debugGeneration': 'Generation time',
  'slash.debugTokensPerSecond': 'Tokens/sec',
  'slash.debugUnavailable': 'No persisted debug trace is available for this turn.',
  'slash.debugContextUsage': 'Current context usage',
  'slash.debugContextWindow': 'Model context window',
  'slash.debugPolicyMaxContextWindow': 'Policy Max',
  'slash.debugModelMaxContextWindow': 'Model Max',
  'slash.debugEffectiveContextWindow': 'Effective Max',
  'slash.debugSessionUsageUnavailable': 'Session usage will appear after the first debug response.',
  'slash.debugCopyTrace': 'Copy trace',
  'slash.debugCopied': 'Copied!',
  'slash.debugToolCalls': 'Tool calls',
  'slash.debugArguments': 'Arguments',
  'slash.debugResult': 'Result',
  'slash.debugCachedTokens': 'Cached',
  'slash.debugPromptTokens': 'Prompt',
  'slash.debugCompletionTokens': 'Completion',
  'slash.debugTotalTokens': 'Total',
  'slash.debugError': 'Error',
  'slash.error.noArguments': 'This slash command does not accept arguments yet.',
  'slash.error.sessionRequired': 'Create or open a chat session before using this command.',
  'slash.error.debugUnauthorized': 'You do not have permission to enable debug mode.',
  'slash.error.unknownCommand': 'Unknown slash command.',
  'slash.error.noAttachments': 'Slash commands cannot be used with attachments.',
  'slash.error.clearFailed': 'Failed to clear session history.',
  'slash.error.compactFailed': 'Failed to compact session history.',

  // Message actions
  'message.copy': 'Copy',
  'message.copied': 'Copied!',
  'message.regenerate': 'Regenerate',
  'message.edit': 'Edit',
  'message.save': 'Save',
  'message.cancel': 'Cancel',

  // Assistant turn
  'assistant.thinking': 'Thinking...',
  'assistant.toolCall': 'Using tool: {name}',
  'assistant.generating': 'Generating response...',
  'assistant.explanation': 'How I arrived at this',

  // System message
  'system.conversationSummary': 'Conversation summary',
  'system.loadingSummary': 'Loading summary...',
  'system.showMore': 'Show more',
  'system.showLess': 'Show less',

  // Question form
  'question.submit': 'Submit',
  'question.selectOne': 'Select one option',
  'question.selectMulti': 'Select one or more options',
  'question.required': 'This field is required',
  'question.other': 'Other',
  'question.specifyOther': 'Please specify',

  // Errors
  'error.generic': 'Something went wrong',
  'error.networkError': 'Network error. Please try again.',
  'error.sessionExpired': 'Session expired. Please refresh.',
  'error.fileTooLarge': 'File is too large',
  'error.invalidFile': 'Invalid file type',
  'error.maxFiles': 'Maximum {max} files allowed',

  // Empty states
  'empty.noMessages': 'No messages yet',
  'empty.noSessions': 'No chat sessions',
  'empty.startChat': 'Start a new chat to begin',

  // Sources panel
  'sources.title': 'Sources',
  'sources.viewMore': 'View more',
  'sources.citations': '{count} citation(s)',

  // Code outputs
  'codeOutputs.title': 'Code Outputs',
  'codeOutputs.download': 'Download',
  'codeOutputs.expand': 'Expand',
  'codeOutputs.collapse': 'Collapse',

  // Charts
  'chart.download': 'Download chart',
  'chart.downloadPNG': 'Download PNG',
  'chart.exporting': 'Exporting...',
  'chart.fullscreen': 'View fullscreen',
  'chart.noData': 'No data available for chart visualization.',

  // Example prompt categories
  'category.analysis': 'Data Analysis',
  'category.reports': 'Reports',
  'category.insights': 'Insights',

  // Sidebar
  'sidebar.chatSessions': 'Chat Sessions',
  'sidebar.closeSidebar': 'Close sidebar',
  'sidebar.searchChats': 'Search chats...',
  'sidebar.createNewChat': 'Create new chat',
  'sidebar.archivedChats': 'Archived Chats',
  'sidebar.pinnedChats': 'Pinned Chats',
  'sidebar.chatOptions': 'Chat options',
  'sidebar.myChats': 'My Chats',
  'sidebar.allChats': 'All Chats',
  'sidebar.chatViewSelector': 'Chat view selector',
  'sidebar.pinChat': 'Pin chat',
  'sidebar.unpinChat': 'Unpin chat',
  'sidebar.pin': 'Pin',
  'sidebar.unpin': 'Unpin',
  'sidebar.renameChat': 'Rename chat',
  'sidebar.archiveChat': 'Archive chat',
  'sidebar.deleteChat': 'Delete chat',
  'sidebar.regenerateTitle': 'Regenerate title',
  'sidebar.noChatsFound': 'No chats found for "{query}"',
  'sidebar.noChatsYet': 'No chats yet',
  'sidebar.createOneToGetStarted': 'Create one to get started',
  'sidebar.failedToLoadSessions': 'Failed to load sessions',
  'sidebar.chatRenamedSuccessfully': 'Chat renamed successfully',
  'sidebar.failedToRenameChat': 'Failed to rename chat',
  'sidebar.titleRegenerated': 'Title regenerated',
  'sidebar.failedToRegenerateTitle': 'Failed to regenerate title',
  'sidebar.chatPinned': 'Chat pinned',
  'sidebar.chatUnpinned': 'Chat unpinned',
  'sidebar.failedToTogglePin': 'Failed to toggle pin',
  'sidebar.archiveChatSession': 'Archive Chat Session',
  'sidebar.archiveChatMessage':
    'Are you sure you want to archive this chat? You can restore it later from Archived Chats.',
  'sidebar.archiveButton': 'Archive',
  'sidebar.deleteChatSession': 'Delete Chat Session',
  'sidebar.deleteChatMessage': 'Are you sure you want to delete this chat? This action cannot be undone.',
  'sidebar.deleteButton': 'Delete',
  'sidebar.chatArchived': 'Chat archived',
  'sidebar.failedToArchiveChat': 'Failed to archive chat',
  'sidebar.chatDeleted': 'Chat deleted',
  'sidebar.failedToDeleteChat': 'Failed to delete chat',

  // Archived chats
  'archived.title': 'Archived Chats',
  'archived.backToChats': 'Back to chats',
  'archived.searchArchivedChats': 'Search archived chats...',
  'archived.noArchivedChats': 'No archived chats',
  'archived.noArchivedChatsDescription': 'Archived chats will appear here',
  'archived.noResults': 'No results found',
  'archived.noResultsDescription': 'No chats match "{query}"',
  'archived.restoreChat': 'Restore Chat',
  'archived.restoreChatMessage': 'Are you sure you want to restore this chat?',
  'archived.restoreButton': 'Restore',
  'archived.chatRestoredSuccessfully': 'Chat restored successfully',
  'archived.failedToRestoreChat': 'Failed to restore chat',

  // All chats (organization-wide)
  'allChats.includeArchived': 'Include archived',
  'allChats.chatFound': '{count} chat found',
  'allChats.chatsFound': '{count} chats found',
  'allChats.organizationChats': 'Organization chats',
  'allChats.loadMore': 'Load more',
  'allChats.noChatsFound': 'No chats found',
  'allChats.noChatsFromUser': 'No chats from {firstName} {lastName}',
  'allChats.noChatsInOrg': 'No chats in organization',
  'allChats.noActiveChatsInOrg': 'No active chats in organization',
  'allChats.failedToLoad': 'Failed to load chats',
  'allChats.allUsers': 'All users',
  'allChats.loadingUsers': 'Loading users...',

  // Alert component
  'alert.retry': 'Retry',
  'alert.dismiss': 'Dismiss',

  // Retry / stream errors
  'retry.title': 'No response received',
  'retry.description': 'The assistant did not respond. Try sending again.',
  'retry.button': 'Retry',
  'streamError.retry': 'Retry',
  'streamError.regenerate': 'Regenerate',

  // Attachment upload
  'attachment.selectFiles': 'Select files',
  'attachment.uploading': 'Uploading...',
  'attachment.maxReached': 'Maximum {max} attachments',
  'attachment.fileAdded': '{count} file(s) added',
  'attachment.invalidFile': 'Invalid file: {filename}',

  // Question form wizard
  'questionForm.title': 'Questions',
  'questionForm.step': 'Step {current} of {total}',
  'questionForm.back': 'Back',
  'questionForm.next': 'Next',
  'questionForm.confirm': 'Confirm & Submit',
  'questionForm.submitting': 'Submitting...',
  'questionForm.reviewTitle': 'Review Your Answers',
  'questionForm.reviewDescription': 'Please review your answers before submitting.',
  'questionForm.skip': 'Skip',

  // Date grouping
  'dateGroup.today': 'Today',
  'dateGroup.yesterday': 'Yesterday',
  'dateGroup.last7Days': 'Last 7 days',
  'dateGroup.last30Days': 'Last 30 days',
  'dateGroup.older': 'Older',

  // Relative time
  'relativeTime.justNow': 'Just now',
  'relativeTime.minutesAgo': '{count}m ago',
  'relativeTime.hoursAgo': '{count}h ago',
  'relativeTime.daysAgo': '{count}d ago',

  // Common
  'common.pinned': 'Pinned',
  'common.back': 'Back',
  'common.clear': 'Clear',
  'common.generating': 'Generating...',
  'common.dismiss': 'Dismiss',
  'common.close': 'Close',
  'common.loading': 'Loading...',
  'common.cancel': 'Cancel',
  'common.untitled': 'Untitled',
  'common.skipToContent': 'Skip to content',

  // Skip link
  'skipLink.label': 'Skip to content',
}
