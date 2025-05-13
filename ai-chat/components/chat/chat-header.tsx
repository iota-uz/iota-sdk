import React from 'react';
import { X } from 'lucide-react';

interface ChatHeaderProps {
  title: string
  subtitle: string
  chatIcon?: React.ReactNode
  onClose: () => void
  isMobile: boolean
}

export const ChatHeader = ({ title, subtitle, chatIcon, onClose, isMobile }: ChatHeaderProps) => {
  return (
    <div
      className="relative bg-[#0a223e] text-white p-4 flex items-center"
      style={{ height: '60px' }}
    >
      <div className="w-10 h-10 rounded-full flex items-center justify-center mr-3">
        {chatIcon ? (
          chatIcon
        ) : (
          <span className="text-white">•••</span>
        )}
      </div>
      <div>
        <h1 className="text-xl font-medium">{title}</h1>
        <p className="text-sm opacity-90">{subtitle}</p>
      </div>
      <div
        className="absolute right-4 top-1/2 transform -translate-y-1/2 cursor-pointer"
        onClick={onClose}
      >
        <X
          className="transition-transform duration-300"
          size={isMobile ? 28 : 24}
        />
      </div>
    </div>
  );
};
