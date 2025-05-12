// Main component export
export { default as ChatbotInterface } from '../components/chatbot-interface';
export type { ChatMessage, FAQItem } from '../components/chatbot-interface';

// Re-export useful utils and services
export { chatApi } from '../lib/api-service';
export type {
  CreateThreadRequest,
  ThreadResponse,
  Message,
  MessagesResponse,
  AddMessageRequest
} from '../lib/api-service';

// Export only the used utils
export { formatDate } from '../lib/utils';

export { getTranslations } from '../lib/translations';
export type { Translations } from '../lib/translations';

// Re-export UI components
export { CallbackModal } from '../components/callback-modal';
export { QuickReplyButtons } from '../components/quick-reply-buttons';
export { TypingIndicator } from '../components/typing-indicator';