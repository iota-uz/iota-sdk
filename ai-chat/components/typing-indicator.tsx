'use client';

import React from 'react';
import type { Translations } from '@/lib/translations';
import { MessageLoading } from './message-loading';

interface TypingIndicatorProps {
  translations: Translations
  botTitle?: string
}

export function TypingIndicator({ translations, botTitle }: TypingIndicatorProps) {
  return (
    <div className="max-w-[80%] bg-white rounded-lg p-4 shadow-sm">
      <div className="text-[#2e67b4] font-medium mb-2">{botTitle || translations.chatbotTitle}</div>
      <div className="flex items-center">
        <MessageLoading />
      </div>
    </div>
  );
}
