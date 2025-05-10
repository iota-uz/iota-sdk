import * as react from 'react';

type ChatMessage = {
    id: string;
    content: string;
    sender: 'user' | 'bot';
    timestamp: Date;
};
interface FAQItem {
    id: string;
    question: string;
}
interface ChatbotInterfaceProps {
    locale?: string;
    apiEndpoint: string;
    faqItems?: FAQItem[];
    title?: string;
    subtitle?: string;
}
declare function ChatbotInterface({ locale, apiEndpoint, // Direct API endpoint (required)
faqItems, title, subtitle, }: ChatbotInterfaceProps): react.JSX.Element;

interface CreateThreadRequest {
    message: string;
    phone: string;
}
interface ThreadResponse {
    thread_id: string;
}
interface Message {
    role: 'user' | 'assistant';
    message: string;
}
interface MessagesResponse {
    messages: Message[];
}
interface AddMessageRequest {
    message: string;
}
declare class ChatApiService {
    private apiEndpoint;
    setApiEndpoint(endpoint: string): void;
    createThread(data: CreateThreadRequest): Promise<ThreadResponse>;
    getMessages(threadId: string): Promise<MessagesResponse>;
    addMessage(threadId: string, data: AddMessageRequest): Promise<ThreadResponse>;
}
declare const chatApi: ChatApiService;

interface Translations {
    chatbotTitle: string;
    chatbotSubtitle: string;
    welcomeGreeting: string;
    welcomeMessage: string;
    phoneRequestMessage: string;
    phoneInputPlaceholder: string;
    messageInputPlaceholder: string;
    sendButton: string;
    callbackRequestButton: string;
    extendPolicyQuestion: string;
    findContractNumberQuestion: string;
    submitClaimQuestion: string;
    callbackModalTitle: string;
    callbackModalSubtitle: string;
    callbackPhoneInputLabel: string;
    dataPrivacyMessage: string;
    dataProcessingConsent: string;
    backButton: string;
    requestCallButton: string;
    callbackConfirmation: string;
    errorLoadingMessages: string;
    errorCreatingChat: string;
    errorSendingMessage: string;
    threadNotFoundMessage: string;
    months: string[];
}
declare function getTranslations(locale: string): Translations;

interface CallbackModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (_phoneNumber: string) => void;
    translations: Translations;
}
declare function CallbackModal({ isOpen, onClose, onSubmit, translations }: CallbackModalProps): react.JSX.Element | null;

interface QuickReplyButtonsProps {
    translations: Translations;
    isTyping: boolean;
    onQuickReply: (_question: string) => void;
    faqItems?: FAQItem[];
}
declare function QuickReplyButtons({ translations, isTyping, onQuickReply, faqItems }: QuickReplyButtonsProps): react.JSX.Element;

interface TypingIndicatorProps {
    translations: Translations;
    botTitle?: string;
}
declare function TypingIndicator({ translations, botTitle }: TypingIndicatorProps): react.JSX.Element;

export { type AddMessageRequest, CallbackModal, type ChatMessage, ChatbotInterface, type CreateThreadRequest, type FAQItem, type Message, type MessagesResponse, QuickReplyButtons, type ThreadResponse, type Translations, TypingIndicator, chatApi, getTranslations };
