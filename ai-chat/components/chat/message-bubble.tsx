import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage } from '@/components/chatbot-interface';
import type { Translations } from '@/lib/translations';

interface MessageBubbleProps {
  message: ChatMessage
  translations: Translations
  botTitle?: string
}

export const MessageBubble = ({ message, translations, botTitle }: MessageBubbleProps) => {
  return (
    <div
      className={`max-w-[80%] w-fit ${message.sender === 'user'
        ? 'ml-auto bg-[#dce6f3] rounded-tl-2xl rounded-tr-2xl rounded-bl-2xl p-3'
        : 'bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm'
        }`}
    >
      {message.sender === 'bot' && (
        <div className="text-[#2e67b4] font-medium mb-2">{botTitle || translations.chatbotTitle}</div>
      )}

      {message.sender === 'bot' ? (
        <div className="markdown-content">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {message.content}
          </ReactMarkdown>
        </div>
      ) : (
        <p className="whitespace-pre-line">{message.content}</p>
      )}

      {/* Optional timestamp display - uncomment if needed
      <div className="text-xs text-gray-500 mt-1 text-right">
        {message.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
      </div>
      */}
    </div>
  );
};