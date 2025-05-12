import React from 'react';
import { Send } from 'lucide-react';
import type { Translations } from '@/lib/translations';

interface PhoneInputProps {
  phoneNumber: string
  setPhoneNumber: (_value: string) => void
  handleSubmit: () => void
  handleKeyPress: (_e: React.KeyboardEvent) => void
  isTyping: boolean
  translations: Translations
  isMobile: boolean
}

export const PhoneInput = ({ 
  phoneNumber, 
  setPhoneNumber, 
  handleSubmit, 
  handleKeyPress, 
  isTyping, 
  translations, 
  isMobile 
}: PhoneInputProps) => {
  return (
    <div className="flex items-center p-3 mb-4 bg-[#f2f5f8] rounded-lg">
      <input
        type="text"
        className="bg-transparent focus:outline-none text-[#0a223e] flex-1"
        placeholder={translations.phoneInputPlaceholder}
        value={phoneNumber}
        onChange={(e) => setPhoneNumber(e.target.value)}
        onKeyDown={handleKeyPress}
        style={{ fontSize: isMobile ? '16px' : 'inherit' }}
      />
      <button onClick={handleSubmit} disabled={isTyping}>
        <Send 
          className={`ml-auto ${isTyping ? 'text-[#8b98a5]' : 'text-[#0a223e]'}`} 
          size={isMobile ? 24 : 20} 
        />
      </button>
    </div>
  );
};