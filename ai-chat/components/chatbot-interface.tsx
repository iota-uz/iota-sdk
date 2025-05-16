'use client';

import * as React from 'react';

import { useState, useRef, useEffect, useCallback } from 'react';
import { formatDate } from '@/lib/utils';
import { CallbackModal } from '@/components/callback-modal';
import { TypingIndicator } from '@/components/typing-indicator';
import { QuickReplyButtons } from '@/components/quick-reply-buttons';
import { chatApi } from '@/lib/api-service';
import { getTranslations } from '@/lib/translations';
import { useIsMobile } from '@/hooks/use-mobile';
import { useSoundEffects } from '@/hooks/use-sound-effects';
import { useToast } from '@/hooks/use-toast';
import { ApiError } from '@/lib/errors';

// Import components
import {
  ChatHeader,
  ChatFloatingButton,
  MessageBubble,
  MessageInput,
  PhoneInput,
  WelcomeMessage
} from '@/components/chat';

// Message type definition for UI
export type ChatMessage = {
  id: string
  content: string
  sender: 'user' | 'bot'
  timestamp: Date
}

// FAQ item type
export interface FAQItem {
  id: string
  question: string
}

interface ChatbotInterfaceProps {
  locale?: string
  apiEndpoint: string // Required prop - direct API endpoint
  faqItems?: FAQItem[]
  title?: string
  subtitle?: string
  chatIcon?: React.ReactNode // Custom chat icon for the chat button
  soundOptions?: {
    enabled?: boolean
    volume?: number
    submitSoundPath?: string
    operatorSoundPath?: string
  }
  onMessageSubmit?: (message: string) => void
}

export default function ChatbotInterface({
  locale = 'ru',
  apiEndpoint, // Direct API endpoint (required)
  faqItems,
  title,
  subtitle,
  chatIcon,
  soundOptions,
  onMessageSubmit,
}: ChatbotInterfaceProps) {
  // Get translations for the specified locale
  const translations = getTranslations(locale);
  const isMobile = useIsMobile();
  const { playSubmitSound, playOperatorSound } = useSoundEffects(soundOptions);
  const { toast } = useToast();

  // Set API endpoint
  useEffect(() => {
    if (apiEndpoint) {
      chatApi.setApiEndpoint(apiEndpoint);
    }
  }, [apiEndpoint]);

  const [phoneSubmitted, setPhoneSubmitted] = useState(false);
  const [phoneNumber, setPhoneNumber] = useState('');
  const [phoneError, setPhoneError] = useState<string | undefined>(undefined);

  // Create a wrapper function to clear errors when phone number is changed
  const handlePhoneNumberChange = (value: string) => {
    setPhoneNumber(value);
    if (phoneError) {
      setPhoneError(undefined);
    }
  };
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [currentMessage, setCurrentMessage] = useState('');
  const [showDateHeader, setShowDateHeader] = useState(false);
  const [isCallbackModalOpen, setIsCallbackModalOpen] = useState(false);
  const [isTyping, setIsTyping] = useState(false);
  const [threadId, setThreadId] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [messageCount, setMessageCount] = useState<number>(1);
  const [windowHeight, setWindowHeight] = useState(0);

  const chatbotTitle = title || translations.chatbotTitle;
  const chatbotSubtitle = subtitle || translations.chatbotSubtitle;

  const handleResetChat = useCallback(() => {
    localStorage.removeItem('chatThreadId');
    setThreadId(null);
    setPhoneSubmitted(false);
    setPhoneNumber('');
    const now = new Date();
    setMessages([
      {
        id: 'welcome',
        content: `${translations.welcomeGreeting}\n\n${translations.welcomeMessage}`,
        sender: 'bot',
        timestamp: now,
      },
    ]);
  }, [translations]);

  const handle404Error = useCallback(() => {
    const now = new Date();
    const errorMsg: ChatMessage = {
      id: `bot-error-${Date.now()}`,
      content: translations.threadNotFoundMessage,
      sender: 'bot',
      timestamp: now,
    };

    setMessages([errorMsg]);
    handleResetChat();
  }, [translations, handleResetChat]);

  const fetchMessages = useCallback(async (threadId: string) => {
    try {
      setIsTyping(true);

      const response = await chatApi.getMessages(threadId);

      const chatMessages: ChatMessage[] = response.messages.map((msg, index) => ({
        id: `${msg.role}-${index}`,
        content: msg.message,
        sender: msg.role === 'user' ? 'user' : 'bot',
        timestamp: new Date(msg.timestamp), // Parse ISO timestamp from API
      }));

      setMessages(chatMessages);
    } catch (error) {
      // Handle ApiError directly
      if (error instanceof ApiError) {
        // Check for 404 errors
        if (error.hasOneOfCodes(['NOT_FOUND', 'THREAD_NOT_FOUND'])) {
          handle404Error();
          return;
        }

        toast({
          variant: 'destructive',
          title: translations.errorLoadingMessages,
          description: error.message,
        });
      } else {
        // Fallback for non-ApiError cases
        const errorMessage = error instanceof Error ? error.message : String(error);

        // Check for legacy 404 errors
        if (error instanceof Error && errorMessage.includes('404')) {
          handle404Error();
          return;
        }

        toast({
          variant: 'destructive',
          title: translations.errorLoadingMessages,
          description: errorMessage,
        });
      }

      const now = new Date();
      setMessages([
        {
          id: 'error',
          content: translations.errorLoadingMessages,
          sender: 'bot',
          timestamp: now,
        },
      ]);
    } finally {
      setIsTyping(false);
    }
  }, [handle404Error, toast, translations]);

  useEffect(() => {
    const updateWindowHeight = () => {
      // On mobile, use a slight delay to account for software keyboard and orientation changes
      setTimeout(() => {
        setWindowHeight(window.innerHeight);
      }, 100);
    };

    updateWindowHeight();

    window.addEventListener('resize', updateWindowHeight);

    // For mobile, also listen to orientation changes
    if (typeof window !== 'undefined') {
      window.addEventListener('orientationchange', () => {
        // Add a longer delay for orientation changes to ensure UI is fully updated
        setTimeout(updateWindowHeight, 300);
      });
    }

    return () => {
      window.removeEventListener('resize', updateWindowHeight);
      if (typeof window !== 'undefined') {
        window.removeEventListener('orientationchange', updateWindowHeight);
      }
    };
  }, [isMobile]);

  useEffect(() => {
    const storedThreadId = localStorage.getItem('chatThreadId');

    if (storedThreadId) {
      setThreadId(storedThreadId);
      setPhoneSubmitted(true);
      setShowDateHeader(true);
      fetchMessages(storedThreadId);
    } else {
      const now = new Date();
      setMessages([
        {
          id: 'welcome',
          content: `${translations.welcomeGreeting}\n\n${translations.welcomeMessage}`,
          sender: 'bot',
          timestamp: now,
        },
      ]);
    }

  }, [translations, fetchMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isTyping]);

  const handlePhoneSubmit = async () => {
    // Don't submit if the phone number is empty or is just the placeholder
    if (phoneNumber.trim().length === 0 || phoneNumber === '+998 __ ___ __ __' || phoneNumber === '+998') { return; }

    // Clear previous errors
    setPhoneError(undefined);

    try {
      setIsTyping(true);

      // Create thread without initial message
      const response = await chatApi.createThread({
        message: '', // Empty message instead of hardcoded text
        phone: phoneNumber,
      });

      setThreadId(response.thread_id);
      localStorage.setItem('chatThreadId', response.thread_id);

      setPhoneSubmitted(true);
      setShowDateHeader(true);

      // Don't add any initial user message
      setMessages([]);

      // Fetch messages and check if there are any existing user messages already
      const messagesResponse = await chatApi.getMessages(response.thread_id);

      const chatMessages: ChatMessage[] = messagesResponse.messages.map((msg, index) => ({
        id: `${msg.role}-${index}`,
        content: msg.message,
        sender: msg.role === 'user' ? 'user' : 'bot',
        timestamp: new Date(msg.timestamp),
      }));

      setMessages(chatMessages);

      // No need to call fetchMessages again since we already got the messages above
    } catch (error) {
      // Handle ApiError directly
      if (error instanceof ApiError) {
        // Check for phone validation errors based on error codes
        if (error.hasOneOfCodes(['INVALID_PHONE_FORMAT', 'UNKNOWN_COUNTRY_CODE'])) {
          // Display error message under the phone input field
          setPhoneError(error.message);
        } else {
          // For other errors, use toast
          toast({
            variant: 'destructive',
            title: translations.errorCreatingChat,
            description: error.message,
          });

          const now = new Date();
          setMessages([
            {
              id: 'error',
              content: translations.errorCreatingChat,
              sender: 'bot',
              timestamp: now,
            },
          ]);
        }
      } else {
        // Fallback for non-ApiError cases
        const errorMessage = error instanceof Error ? error.message : String(error);

        toast({
          variant: 'destructive',
          title: translations.errorCreatingChat,
          description: errorMessage,
        });

        const now = new Date();
        setMessages([
          {
            id: 'error',
            content: translations.errorCreatingChat,
            sender: 'bot',
            timestamp: now,
          },
        ]);
      }
    } finally {
      setIsTyping(false);
    }
  };

  const handleSendMessage = async () => {
    if (currentMessage.trim().length === 0 || !threadId) { return; }

    playSubmitSound();

    const now = new Date();
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      content: currentMessage,
      sender: 'user',
      timestamp: now,
    };

    setMessages((prev) => [...prev, userMessage]);
    const messageToSend = currentMessage;
    setCurrentMessage('');

    // Note: don't call onMessageSubmit here as it's already called from the input component
    // to avoid double-firing of the callback

    setIsTyping(true);

    try {
      await chatApi.addMessage(threadId, {
        message: messageToSend,
      });

      const response = await chatApi.getMessages(threadId);

      const assistantMessages = response.messages.filter((msg) => msg.role === 'assistant');
      if (assistantMessages.length > 0) {
        const latestAssistantMessage = assistantMessages[assistantMessages.length - 1];

        const botMessage: ChatMessage = {
          id: `bot-${Date.now()}`,
          content: latestAssistantMessage.message,
          sender: 'bot',
          timestamp: new Date(latestAssistantMessage.timestamp),
        };

        setMessages((prev) => {
          // Check if this message is already in the list to avoid duplicates
          const isDuplicate = prev.some((msg) => msg.sender === 'bot' && msg.content === latestAssistantMessage.message);

          if (isDuplicate) { return prev; }
          playOperatorSound();
          return [...prev, botMessage];
        });
      }
    } catch (error) {
      // Handle ApiError directly
      if (error instanceof ApiError) {
        // Check for 404 errors
        if (error.hasOneOfCodes(['NOT_FOUND', 'THREAD_NOT_FOUND'])) {
          handle404Error();
          return;
        }

        toast({
          variant: 'destructive',
          title: translations.errorSendingMessage,
          description: error.message,
        });
      } else {
        // Fallback for non-ApiError cases
        const errorMessage = error instanceof Error ? error.message : String(error);

        // Check for legacy 404 errors
        if (error instanceof Error && errorMessage.includes('404')) {
          handle404Error();
          return;
        }

        toast({
          variant: 'destructive',
          title: translations.errorSendingMessage,
          description: errorMessage,
        });
      }

      // Add error message to UI
      const now = new Date();
      const errorMsg: ChatMessage = {
        id: `bot-error-${Date.now()}`,
        content: translations.errorSendingMessage,
        sender: 'bot',
        timestamp: now,
      };

      setMessages((prev) => [...prev, errorMsg]);
    } finally {
      setIsTyping(false);
    }
  };

  // Handle quick reply button click
  const handleQuickReply = (question: string) => {
    // Directly process and send the question without setting input field
    const now = new Date();
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      content: question,
      sender: 'user',
      timestamp: now,
    };

    setMessages((prev) => [...prev, userMessage]);
    playSubmitSound();

    // Call the onMessageSubmit callback if it exists
    if (onMessageSubmit && question.trim().length > 0) {
      onMessageSubmit(question.trim());
    }

    // Process the message with the API
    setIsTyping(true);

    if (threadId) {
      (async () => {
        try {
          await chatApi.addMessage(threadId, {
            message: question,
          });

          const response = await chatApi.getMessages(threadId);

          const assistantMessages = response.messages.filter((msg) => msg.role === 'assistant');
          if (assistantMessages.length > 0) {
            const latestAssistantMessage = assistantMessages[assistantMessages.length - 1];

            const botMessage: ChatMessage = {
              id: `bot-${Date.now()}`,
              content: latestAssistantMessage.message,
              sender: 'bot',
              timestamp: new Date(latestAssistantMessage.timestamp),
            };

            setMessages((prev) => {
              // Check if this message is already in the list to avoid duplicates
              const isDuplicate = prev.some((msg) => msg.sender === 'bot' && msg.content === latestAssistantMessage.message);

              if (isDuplicate) { return prev; }
              playOperatorSound();
              return [...prev, botMessage];
            });
          }
        } catch (error) {
          // Handle ApiError directly
          if (error instanceof ApiError) {
            // Check for 404 errors
            if (error.hasOneOfCodes(['NOT_FOUND', 'THREAD_NOT_FOUND'])) {
              handle404Error();
              return;
            }

            toast({
              variant: 'destructive',
              title: translations.errorSendingMessage,
              description: error.message,
            });
          } else {
            // Fallback for non-ApiError cases
            const errorMessage = error instanceof Error ? error.message : String(error);

            // Check for legacy 404 errors
            if (error instanceof Error && errorMessage.includes('404')) {
              handle404Error();
              return;
            }

            toast({
              variant: 'destructive',
              title: translations.errorSendingMessage,
              description: errorMessage,
            });
          }

          // Add error message to UI
          const now = new Date();
          const errorMsg: ChatMessage = {
            id: `bot-error-${Date.now()}`,
            content: translations.errorSendingMessage,
            sender: 'bot',
            timestamp: now,
          };

          setMessages((prev) => [...prev, errorMsg]);
        } finally {
          setIsTyping(false);
        }
      })();
    }
  };

  // Handle Enter key press
  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      if (phoneSubmitted) {
        handleSendMessage();
        if (onMessageSubmit && currentMessage.trim().length > 0) {
          onMessageSubmit(currentMessage.trim());
        }
      } else {
        // Clear any previous phone errors before attempting to submit
        setPhoneError(undefined);
        handlePhoneSubmit();
      }
    }
  };

  // Handle callback request submission
  const handleCallbackSubmit = (callbackPhone: string) => {
    // Add bot confirmation message
    const now = new Date();
    const botMessage: ChatMessage = {
      id: `bot-${Date.now()}`,
      content: translations.callbackConfirmation.replace('{phone}', callbackPhone),
      sender: 'bot',
      timestamp: now,
    };

    playOperatorSound();
    setMessages((prev) => [...prev, botMessage]);

    // In a real application, you would send this request to your backend
  };

  // Calculate chat dimensions
  const chatWidth = isMobile ? '100%' : 450;
  const headerHeight = 60;
  const maxChatHeight = isMobile ? '100%' : (windowHeight ? Math.min(windowHeight * 0.8, 700) : 600);
  const contentHeight = isMobile ? `calc(100vh - ${headerHeight}px)` : (maxChatHeight as number) - headerHeight;

  // Reset message count when chat is opened
  useEffect(() => {
    if (isOpen) {
      setMessageCount(0);
    }
  }, [isOpen]);

  return (
    <div className={`fixed ${isMobile && isOpen ? 'inset-0' : 'bottom-4 right-4'} z-50 flex flex-col items-end ${isMobile && !isOpen ? 'pointer-events-none' : ''}`}>
      {!isOpen && (
        <ChatFloatingButton
          onClick={() => setIsOpen(true)}
          isMobile={isMobile}
          messageCount={messageCount}
          chatIcon={chatIcon}
        />
      )}
      <div
        className={`overflow-hidden bg-white transition-all duration-300 ${isMobile ? '' : 'rounded-lg shadow-lg'} ${isOpen ? 'opacity-100 scale-100' : 'opacity-0 scale-0'
          }`}
        style={{
          width: typeof chatWidth === 'number' ? `${chatWidth}px` : chatWidth,
          height: isOpen ? (typeof maxChatHeight === 'number' ? `${maxChatHeight}px` : maxChatHeight) : '0px',
          position: isMobile && isOpen ? 'fixed' : 'relative',
          top: isMobile && isOpen ? 0 : 'auto',
          left: isMobile && isOpen ? 0 : 'auto',
          right: isMobile && isOpen ? 0 : 'auto',
          bottom: isMobile && isOpen ? 0 : 'auto',
          visibility: isOpen ? 'visible' : 'hidden',
          zIndex: 40,
          marginLeft: isMobile ? 0 : 'auto',
          display: isMobile && isOpen ? 'flex' : 'block',
          flexDirection: isMobile && isOpen ? 'column' : 'initial',
          minHeight: isMobile ? '100vh' : 'auto',
          overflowY: isMobile ? 'hidden' : 'visible'
        }}
      >
        {/* Header */}
        <ChatHeader
          title={chatbotTitle}
          subtitle={chatbotSubtitle}
          chatIcon={chatIcon}
          onClose={() => setIsOpen(false)}
          isMobile={isMobile}
        />

        {isOpen && (
          <div
            className={`flex flex-col ${isMobile ? 'flex-1' : ''}`}
            style={{ height: isMobile ? contentHeight : `${contentHeight}px`, maxHeight: isMobile ? contentHeight : undefined }}>
            {/* Chat Area */}
            <div className={`bg-[#f2f5f8] ${isMobile ? 'p-3 pb-4' : 'p-4'} flex-grow overflow-y-auto`} style={{ minHeight: isMobile ? '50vh' : undefined }}>
              {!phoneSubmitted ? (
                <WelcomeMessage chatbotTitle={chatbotTitle} translations={translations} />
              ) : (
                <div className="space-y-4">
                  {showDateHeader && (
                    <div className="text-center text-[#8b98a5] text-sm py-2">
                      {formatDate(new Date(), translations)}
                    </div>
                  )}

                  {messages.map((message) => (
                    <MessageBubble
                      key={message.id}
                      message={message}
                      translations={translations}
                      botTitle={chatbotTitle}
                    />
                  ))}

                  {/* Typing indicator */}
                  {isTyping && <TypingIndicator translations={translations} botTitle={chatbotTitle} />}

                  <div ref={messagesEndRef} style={{ height: 10 }} />
                </div>
              )}
            </div>

            {/* Input Area */}
            <div className={`${isMobile ? 'p-3' : 'p-4'} bg-white shrink-0`} style={{ position: isMobile ? 'sticky' : 'relative', bottom: 0 }}>
              {!phoneSubmitted ? (
                /* Phone Input */
                <PhoneInput
                  phoneNumber={phoneNumber}
                  setPhoneNumber={handlePhoneNumberChange}
                  handleSubmit={handlePhoneSubmit}
                  handleKeyPress={handleKeyPress}
                  isTyping={isTyping}
                  translations={translations}
                  isMobile={isMobile}
                  error={phoneError}
                />
              ) : (
                /* Message Input */
                <MessageInput
                  currentMessage={currentMessage}
                  setCurrentMessage={setCurrentMessage}
                  handleSubmit={handleSendMessage}
                  handleKeyPress={handleKeyPress}
                  isTyping={isTyping}
                  translations={translations}
                  isMobile={isMobile}
                  onMessageSubmit={onMessageSubmit}
                />
              )}

              {/* Quick Reply Buttons - only show when phone is submitted and no user messages exist yet */}
              {phoneSubmitted &&
                // Only show for threads with no user messages
                messages.filter(msg => msg.sender === 'user').length === 0 &&
                !isTyping && (
                  <QuickReplyButtons
                    translations={translations}
                    isTyping={isTyping}
                    onQuickReply={handleQuickReply}
                    faqItems={faqItems}
                  />
                )}

              {/* Send Button */}
              <button
                className={`w-full py-3 ${isMobile ? 'py-4 text-base' : 'py-3'} rounded-lg mb-4 ${phoneSubmitted && currentMessage.trim() && !isTyping
                  ? 'bg-[#2e67b4] text-white'
                  : 'bg-[#e4e9ee] text-[#bdc8d2]'
                  }`}
                onClick={phoneSubmitted ? handleSendMessage : handlePhoneSubmit}
                disabled={
                  (phoneSubmitted ? currentMessage.trim().length === 0 : phoneNumber.trim().length === 0) || isTyping
                }
              >
                {translations.sendButton}
              </button>

              {/* Request Callback Button - temporarily hidden
              <button
                className={`w-full ${isMobile ? 'py-4 text-base' : 'py-3'} border border-[#2e67b4] text-[#2e67b4] rounded-lg`}
                onClick={() => setIsCallbackModalOpen(true)}
                disabled={isTyping}
              >
                {translations.callbackRequestButton}
              </button> */}
            </div>
          </div>
        )}
      </div>

      {/* Callback Request Modal */}
      {isCallbackModalOpen && (
        <CallbackModal
          isOpen={isCallbackModalOpen}
          onClose={() => setIsCallbackModalOpen(false)}
          onSubmit={handleCallbackSubmit}
          translations={translations}
        />
      )}
    </div>
  );
}
