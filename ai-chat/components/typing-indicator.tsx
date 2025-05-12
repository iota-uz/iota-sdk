'use client';

import { useEffect, useState } from 'react';
import type { Translations } from '@/lib/translations';

interface TypingIndicatorProps {
  translations: Translations
  botTitle?: string
}

export function TypingIndicator({ translations, botTitle }: TypingIndicatorProps) {
  const [dots, setDots] = useState('.');

  useEffect(() => {
    const interval = setInterval(() => {
      setDots((prev) => {
        if (prev === '...') {return '.';}
        return prev + '.';
      });
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <div className="max-w-[80%] bg-white rounded-lg p-4 shadow-sm">
      <div className="text-[#2e67b4] font-medium mb-2">{botTitle || translations.chatbotTitle}</div>
      <div className="flex items-center">
        <span className="text-[#8b98a5] font-medium">{dots}</span>
      </div>
    </div>
  );
}
