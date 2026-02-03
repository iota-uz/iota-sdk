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

  // Chat header
  'chat.newChat': 'New Chat',
  'chat.archived': 'Archived',
  'chat.pinned': 'Pinned',
  'chat.goBack': 'Go back',

  // Message input
  'input.placeholder': 'Type a message...',
  'input.attachFiles': 'Attach files',
  'input.attachImages': 'Attach images',
  'input.dropImages': 'Drop images here',
  'input.sendMessage': 'Send message',
  'input.aiThinking': 'AI is thinking...',
  'input.processing': 'Processing...',
  'input.messagesQueued': '{count} message(s) queued',
  'input.dismissError': 'Dismiss error',

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
  'chart.fullscreen': 'View fullscreen',
  'chart.noData': 'No data available',

  // Example prompt categories
  'category.analysis': 'Data Analysis',
  'category.reports': 'Reports',
  'category.insights': 'Insights',
}
