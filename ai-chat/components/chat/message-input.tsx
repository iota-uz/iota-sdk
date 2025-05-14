import React from 'react';
import { Send } from 'lucide-react';
import type { Translations } from '@/lib/translations';

interface MessageInputProps {
  currentMessage: string
  setCurrentMessage: (_value: string) => void
  handleSubmit: () => void
  handleKeyPress: (_e: React.KeyboardEvent) => void
  isTyping: boolean
  translations: Translations
  isMobile: boolean
  onMessageSubmit?: (message: string) => void
}

export const MessageInput = ({ 
  currentMessage, 
  setCurrentMessage, 
  handleSubmit, 
  handleKeyPress, 
  isTyping, 
  translations, 
  isMobile,
  onMessageSubmit
}: MessageInputProps) => {
  return (
    <div className="flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg">
      <input
        type="text"
        className="bg-transparent focus:outline-none text-[#0a223e] flex-1"
        placeholder={translations.messageInputPlaceholder}
        value={currentMessage}
        onChange={(e) => setCurrentMessage(e.target.value)}
        onKeyDown={handleKeyPress}
        disabled={isTyping}
        style={{ fontSize: isMobile ? '16px' : 'inherit' }}
      />
      <button 
        onClick={() => {
          handleSubmit();
          if (onMessageSubmit && currentMessage.trim().length > 0) {
            onMessageSubmit(currentMessage.trim());
          }
        }} 
        disabled={isTyping || currentMessage.trim().length === 0}
      >
        <Send
          className={`ml-auto ${currentMessage.trim() && !isTyping ? 'text-[#0a223e]' : 'text-[#8b98a5]'}`}
          size={isMobile ? 24 : 20}
        />
      </button>
    </div>
  );
};