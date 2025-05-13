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
}

export default function ChatbotInterface({
  locale = 'ru',
  apiEndpoint, // Direct API endpoint (required)
  faqItems,
  title,
  subtitle,
  chatIcon,
  soundOptions,
}: ChatbotInterfaceProps) {
  // Get translations for the specified locale
  const translations = getTranslations(locale);
  const isMobile = useIsMobile();
  const { playSubmitSound, playOperatorSound } = useSoundEffects(soundOptions);

  // Set API endpoint
  useEffect(() => {
    if (apiEndpoint) {
      chatApi.setApiEndpoint(apiEndpoint);
    }
  }, [apiEndpoint]);

  const [phoneSubmitted, setPhoneSubmitted] = useState(false);
  const [phoneNumber, setPhoneNumber] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [currentMessage, setCurrentMessage] = useState('');
  const [showDateHeader, setShowDateHeader] = useState(false);
  const [isCallbackModalOpen, setIsCallbackModalOpen] = useState(false);
  const [isTyping, setIsTyping] = useState(false);
  const [threadId, setThreadId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [messageCount, setMessageCount] = useState<number>(0);
  const [windowHeight, setWindowHeight] = useState(0);

  const chatbotTitle = title || translations.chatbotTitle;
  const chatbotSubtitle = subtitle || translations.chatbotSubtitle;

  const handleResetChat = useCallback(() => {
    localStorage.removeItem('chatThreadId');
    localStorage.removeItem('chatPhoneNumber');
    setThreadId(null);
    setPhoneSubmitted(false);
    setPhoneNumber('');
    setError(null);
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
      setError(null);

      const response = await chatApi.getMessages(threadId);

      const chatMessages: ChatMessage[] = response.messages.map((msg, index) => ({
        id: `${msg.role}-${index}`,
        content: msg.message,
        sender: msg.role === 'user' ? 'user' : 'bot',
        timestamp: new Date(msg.timestamp), // Parse ISO timestamp from API
      }));

      setMessages(chatMessages);
    } catch (error) {
      if (error instanceof Error && error.message.includes('404')) {
        handle404Error();
        return;
      }

      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorLoadingMessages}: ${errorMessage}`);

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
  }, [handle404Error, translations]);

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

    // For mobile, add specific event listeners for input focus to handle keyboard properly
    if (isMobile) {
      const handleFocus = () => {
        // Prevent scrolling issues when keyboard appears
        window.scrollTo(0, 0);
      };

      document.addEventListener('focus', handleFocus, true);

      return () => {
        window.removeEventListener('resize', updateWindowHeight);
        if (typeof window !== 'undefined') {
          window.removeEventListener('orientationchange', updateWindowHeight);
        }
        document.removeEventListener('focus', handleFocus, true);
      };
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
    const storedPhone = localStorage.getItem('chatPhoneNumber');

    if (storedThreadId && storedPhone) {
      setThreadId(storedThreadId);
      setPhoneNumber(storedPhone);
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
    if (phoneNumber.trim().length === 0) { return; }

    try {
      setIsTyping(true);
      setError(null);

      // Create thread without initial message
      const response = await chatApi.createThread({
        message: '', // Empty message instead of hardcoded text
        phone: phoneNumber,
      });

      setThreadId(response.thread_id);
      localStorage.setItem('chatThreadId', response.thread_id);
      localStorage.setItem('chatPhoneNumber', phoneNumber);

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
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorCreatingChat}: ${errorMessage}`);

      const now = new Date();
      setMessages([
        {
          id: 'error',
          content: translations.errorCreatingChat,
          sender: 'bot',
          timestamp: now,
        },
      ]);
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
    setError(null);

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
      // Check if it's a 404 error (thread not found)
      if (error instanceof Error && error.message.includes('404')) {
        handle404Error();
        return;
      }

      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorSendingMessage}: ${errorMessage}`);

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
    setCurrentMessage(question);
    setTimeout(() => {
      handleSendMessage();
    }, 100);
  };

  // Handle Enter key press
  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      phoneSubmitted ? handleSendMessage() : handlePhoneSubmit();
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
    <div className={`fixed ${isMobile ? 'inset-0' : 'bottom-4 right-4'} z-50 flex flex-col items-end`}>
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

                  {error && (
                    <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative">
                      <span className="block sm:inline">{error}</span>
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
                  setPhoneNumber={setPhoneNumber}
                  handleSubmit={handlePhoneSubmit}
                  handleKeyPress={handleKeyPress}
                  isTyping={isTyping}
                  translations={translations}
                  isMobile={isMobile}
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

              {/* Request Callback Button */}
              <button
                className={`w-full ${isMobile ? 'py-4 text-base' : 'py-3'} border border-[#2e67b4] text-[#2e67b4] rounded-lg`}
                onClick={() => setIsCallbackModalOpen(true)}
                disabled={isTyping}
              >
                {translations.callbackRequestButton}
              </button>
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
