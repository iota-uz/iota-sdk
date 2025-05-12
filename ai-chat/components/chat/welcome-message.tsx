import React from 'react';
import type { Translations } from '@/lib/translations';

interface WelcomeMessageProps {
  chatbotTitle: string
  translations: Translations
}

export const WelcomeMessage = ({ chatbotTitle, translations }: WelcomeMessageProps) => {
  return (
    <div className="bg-white rounded-tr-2xl rounded-tl-2xl rounded-br-xl p-4 shadow-sm">
      <div className="text-[#2e67b4] font-medium mb-2">{chatbotTitle}</div>
      <p className="mb-2">{translations.welcomeGreeting}</p>
      <p className="mb-4">{translations.welcomeMessage}</p>
      <p className="flex items-start">
        <span className="inline-block mr-2 mt-1">🔒</span>
        <span>{translations.phoneRequestMessage}</span>
      </p>
    </div>
  );
};