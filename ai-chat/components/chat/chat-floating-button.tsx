import React from 'react';
import { MessageCircle } from 'lucide-react';

interface ChatFloatingButtonProps {
  onClick: () => void
  isMobile: boolean
  messageCount: number
  chatIcon?: React.ReactNode
}

export const ChatFloatingButton = ({ onClick, isMobile, messageCount, chatIcon }: ChatFloatingButtonProps) => {
  return (
    <div
      className="w-16 h-16 bg-[#0a223e] rounded-full shadow-lg flex items-center justify-center cursor-pointer hover:bg-[#1e3b66] transition-all duration-300 hover:scale-110 ml-auto"
      onClick={onClick}
      style={{
        position: isMobile ? 'fixed' : 'relative',
        bottom: isMobile ? '20px' : 'auto',
        right: isMobile ? '20px' : 'auto',
      }}
    >
      <div className="relative">
        {chatIcon ? (
          chatIcon
        ) : (
          <MessageCircle className="text-white" size={28} />
        )}
        <span className="absolute -top-1 -right-1 flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs text-white">{messageCount}</span>
      </div>
    </div>
  );
};