'use client';

import type React from 'react';

import { ChevronDown, Send } from 'lucide-react';
import { useState, useRef, useEffect } from 'react';
import { formatDate } from '@/lib/utils';
import { CallbackModal } from '@/components/callback-modal';
import { TypingIndicator } from '@/components/typing-indicator';
import { QuickReplyButtons } from '@/components/quick-reply-buttons';
import { chatApi } from '@/lib/api-service';
import { getTranslations, type Translations } from '@/lib/translations';

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

// Message component
interface MessageBubbleProps {
  message: ChatMessage
  translations: Translations
  botTitle?: string
}

const MessageBubble = ({ message, translations, botTitle }: MessageBubbleProps) => {
  return (
    <div
      className={`max-w-[80%] w-fit ${
        message.sender === 'user'
          ? 'ml-auto bg-[#dce6f3] rounded-tl-2xl rounded-tr-2xl rounded-bl-2xl p-3'
          : 'bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm'
      }`}
    >
      {message.sender === 'bot' && (
        <div className="text-[#2e67b4] font-medium mb-2">{botTitle || translations.chatbotTitle}</div>
      )}
      <p className="whitespace-pre-line">{message.content}</p>
    </div>
  );
};

interface ChatbotInterfaceProps {
  locale?: string
  apiEndpoint: string // Required prop - direct API endpoint
  faqItems?: FAQItem[]
  title?: string
  subtitle?: string
}

export default function ChatbotInterface({
  locale = 'ru',
  apiEndpoint, // Direct API endpoint (required)
  faqItems,
  title,
  subtitle,
}: ChatbotInterfaceProps) {
  // Get translations for the specified locale
  const translations = getTranslations(locale);

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
  const [isOpen, setIsOpen] = useState(true);
  const [windowHeight, setWindowHeight] = useState(0);

  // Use custom title/subtitle or fallback to translations
  const chatbotTitle = title || translations.chatbotTitle;
  const chatbotSubtitle = subtitle || translations.chatbotSubtitle;

  // Update window height on mount and resize
  useEffect(() => {
    const updateWindowHeight = () => {
      setWindowHeight(window.innerHeight);
    };

    // Set initial height
    updateWindowHeight();

    // Add event listener for resize
    window.addEventListener('resize', updateWindowHeight);

    // Clean up
    return () => window.removeEventListener('resize', updateWindowHeight);
  }, []);

  // Check for existing thread ID in localStorage on component mount
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
      // Show initial welcome message if no existing thread
      setMessages([
        {
          id: 'welcome',
          content: `${translations.welcomeGreeting}\n\n${translations.welcomeMessage}`,
          sender: 'bot',
          timestamp: new Date(),
        },
      ]);
    }

    // Clear the newThreadId flag when loading an existing thread
    if (storedThreadId) {
      localStorage.removeItem('newThreadId');
    }
  }, [translations]);

  // Auto-scroll to bottom of messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isTyping]);

  // Reset chat - clear thread ID and messages
  const handleResetChat = () => {
    localStorage.removeItem('chatThreadId');
    localStorage.removeItem('chatPhoneNumber');
    setThreadId(null);
    setPhoneSubmitted(false);
    setPhoneNumber('');
    setError(null);
    setMessages([
      {
        id: 'welcome',
        content: `${translations.welcomeGreeting}\n\n${translations.welcomeMessage}`,
        sender: 'bot',
        timestamp: new Date(),
      },
    ]);
  };

  // Handle 404 errors by resetting the chat
  const handle404Error = () => {
    // Add a message explaining what happened
    const errorMsg: ChatMessage = {
      id: `bot-error-${Date.now()}`,
      content: translations.threadNotFoundMessage,
      sender: 'bot',
      timestamp: new Date(),
    };

    setMessages([errorMsg]);

    // Reset the chat state
    handleResetChat();
  };

  // Fetch message history for an existing thread
  const fetchMessages = async (threadId: string) => {
    try {
      setIsTyping(true);
      setError(null);

      const response = await chatApi.getMessages(threadId);

      // Convert API messages to UI message format
      const chatMessages: ChatMessage[] = response.messages.map((msg, index) => ({
        id: `${msg.role}-${index}`,
        content: msg.message,
        sender: msg.role === 'user' ? 'user' : 'bot',
        timestamp: new Date(), // API doesn't provide timestamps, so we use current time
      }));

      setMessages(chatMessages);
    } catch (error) {
      console.error('Error fetching messages:', error);

      // Check if it's a 404 error (thread not found)
      if (error instanceof Error && error.message.includes('404')) {
        handle404Error();
        return;
      }

      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorLoadingMessages}: ${errorMessage}`);

      setMessages([
        {
          id: 'error',
          content: translations.errorLoadingMessages,
          sender: 'bot',
          timestamp: new Date(),
        },
      ]);
    } finally {
      setIsTyping(false);
    }
  };

  // Handle phone submission - create a new thread
  const handlePhoneSubmit = async () => {
    if (phoneNumber.trim().length === 0) {return;}

    try {
      setIsTyping(true);
      setError(null);

      // Create initial message - using the same message regardless of locale for simplicity
      // In a real app, you might want to translate this too
      const initialMessage = 'Здравствуйте, я хотел бы узнать больше о страховых услугах.';

      // Create a new thread with the phone number
      const response = await chatApi.createThread({
        message: initialMessage,
        phone: phoneNumber,
      });

      // Store thread ID and phone number
      setThreadId(response.thread_id);
      // Mark this as a new thread created in this session
      localStorage.setItem('newThreadId', response.thread_id);
      localStorage.setItem('chatThreadId', response.thread_id);
      localStorage.setItem('chatPhoneNumber', phoneNumber);

      // Update UI state
      setPhoneSubmitted(true);
      setShowDateHeader(true);

      // Add initial message to UI
      const userMessage: ChatMessage = {
        id: 'user-initial',
        content: initialMessage,
        sender: 'user',
        timestamp: new Date(),
      };

      setMessages([userMessage]);

      // Fetch messages to get the assistant's response
      await fetchMessages(response.thread_id);
    } catch (error) {
      console.error('Error creating thread:', error);
      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorCreatingChat}: ${errorMessage}`);

      setMessages([
        {
          id: 'error',
          content: translations.errorCreatingChat,
          sender: 'bot',
          timestamp: new Date(),
        },
      ]);
    } finally {
      setIsTyping(false);
    }
  };

  // Handle message submission - add message to existing thread
  const handleSendMessage = async () => {
    if (currentMessage.trim().length === 0 || !threadId) {return;}

    // Add user message to UI immediately
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      content: currentMessage,
      sender: 'user',
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);
    const messageToSend = currentMessage;
    setCurrentMessage('');
    setError(null);

    // Show typing indicator
    setIsTyping(true);

    try {
      // Send message to API
      await chatApi.addMessage(threadId, {
        message: messageToSend,
      });

      // Fetch updated messages to get the assistant's response
      const response = await chatApi.getMessages(threadId);

      // Find the latest assistant message
      const assistantMessages = response.messages.filter((msg) => msg.role === 'assistant');
      if (assistantMessages.length > 0) {
        const latestAssistantMessage = assistantMessages[assistantMessages.length - 1];

        // Add bot response to UI
        const botMessage: ChatMessage = {
          id: `bot-${Date.now()}`,
          content: latestAssistantMessage.message,
          sender: 'bot',
          timestamp: new Date(),
        };

        setMessages((prev) => {
          // Check if this message is already in the list to avoid duplicates
          const isDuplicate = prev.some((msg) => msg.sender === 'bot' && msg.content === latestAssistantMessage.message);

          if (isDuplicate) {return prev;}
          return [...prev, botMessage];
        });
      }
    } catch (error) {
      console.error('Error sending message:', error);

      // Check if it's a 404 error (thread not found)
      if (error instanceof Error && error.message.includes('404')) {
        handle404Error();
        return;
      }

      const errorMessage = error instanceof Error ? error.message : 'Unknown error';
      setError(`${translations.errorSendingMessage}: ${errorMessage}`);

      // Add error message to UI
      const errorMsg: ChatMessage = {
        id: `bot-error-${Date.now()}`,
        content: translations.errorSendingMessage,
        sender: 'bot',
        timestamp: new Date(),
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
    const botMessage: ChatMessage = {
      id: `bot-${Date.now()}`,
      content: translations.callbackConfirmation.replace('{phone}', callbackPhone),
      sender: 'bot',
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, botMessage]);

    // In a real application, you would send this request to your backend
    console.log('Callback requested for phone:', callbackPhone);
  };

  // Calculate chat dimensions
  const chatWidth = 450;
  const headerHeight = 60;
  const maxChatHeight = windowHeight ? Math.min(windowHeight * 0.8, 700) : 600;
  const contentHeight = maxChatHeight - headerHeight;

  return (
    <div className="fixed bottom-4 right-4 z-50">
      <div
        className={`overflow-hidden bg-white rounded-lg shadow-lg transition-all duration-300 ${
          isOpen ? 'opacity-100' : 'opacity-95'
        }`}
        style={{
          width: `${chatWidth}px`,
          height: isOpen ? `${maxChatHeight}px` : `${headerHeight}px`,
        }}
      >
        {/* Header */}
        <div
          className="relative bg-[#0a223e] text-white p-4 flex items-center cursor-pointer"
          style={{ height: `${headerHeight}px` }}
          onClick={() => setIsOpen(!isOpen)}
        >
          <div className="w-10 h-10 bg-[#8b98a5] rounded-full flex items-center justify-center mr-3">
            <span className="text-white">•••</span>
          </div>
          <div>
            <h1 className="text-xl font-medium">{chatbotTitle}</h1>
            <p className="text-sm opacity-90">{chatbotSubtitle}</p>
          </div>
          <ChevronDown
            className={`absolute right-4 top-1/2 transform -translate-y-1/2 transition-transform duration-300 ${
              isOpen ? '' : 'rotate-180'
            }`}
          />
        </div>

        {isOpen && (
          <div className="flex flex-col" style={{ height: `${contentHeight}px` }}>
            {/* Chat Area */}
            <div className="bg-[#f2f5f8] p-4 flex-grow overflow-y-auto">
              {!phoneSubmitted ? (
                <div className="bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm">
                  <div className="text-[#2e67b4] font-medium mb-2">{chatbotTitle}</div>
                  <p className="mb-2">{translations.welcomeGreeting}</p>
                  <p className="mb-4">{translations.welcomeMessage}</p>
                  <p className="flex items-start">
                    <span className="inline-block mr-2 mt-1">🔒</span>
                    <span>{translations.phoneRequestMessage}</span>
                  </p>
                </div>
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

                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>

            {/* Input Area */}
            <div className="p-4 bg-white shrink-0">
              {!phoneSubmitted ? (
                /* Phone Input */
                <div className="flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg">
                  <input
                    type="text"
                    className="bg-transparent focus:outline-none text-[#0a223e] flex-1"
                    placeholder={translations.phoneInputPlaceholder}
                    value={phoneNumber}
                    onChange={(e) => setPhoneNumber(e.target.value)}
                    onKeyDown={handleKeyPress}
                  />
                  <button onClick={handlePhoneSubmit} disabled={isTyping}>
                    <Send className={`ml-auto ${isTyping ? 'text-[#8b98a5]' : 'text-[#0a223e]'}`} size={20} />
                  </button>
                </div>
              ) : (
                /* Message Input */
                <div className="flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg">
                  <input
                    type="text"
                    className="bg-transparent focus:outline-none text-[#0a223e] flex-1"
                    placeholder={translations.messageInputPlaceholder}
                    value={currentMessage}
                    onChange={(e) => setCurrentMessage(e.target.value)}
                    onKeyDown={handleKeyPress}
                    disabled={isTyping}
                  />
                  <button onClick={handleSendMessage} disabled={isTyping || currentMessage.trim().length === 0}>
                    <Send
                      className={`ml-auto ${currentMessage.trim() && !isTyping ? 'text-[#0a223e]' : 'text-[#8b98a5]'}`}
                      size={20}
                    />
                  </button>
                </div>
              )}

              {/* Quick Reply Buttons - only show for new conversations */}
              {(!threadId || threadId === localStorage.getItem('newThreadId')) && (
                <QuickReplyButtons
                  translations={translations}
                  isTyping={isTyping}
                  onQuickReply={handleQuickReply}
                  faqItems={faqItems}
                />
              )}

              {/* Send Button */}
              <button
                className={`w-full py-3 rounded-lg mb-4 ${
                  phoneSubmitted && currentMessage.trim() && !isTyping
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
                className="w-full py-3 border border-[#2e67b4] text-[#2e67b4] rounded-lg"
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
