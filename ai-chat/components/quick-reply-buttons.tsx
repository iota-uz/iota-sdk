'use client';

import React from 'react';
import type { Translations } from '@/lib/translations';
import type { FAQItem } from '@/components/chatbot-interface';

interface QuickReplyButtonsProps {
  translations: Translations
  isTyping: boolean
  onQuickReply: (_question: string) => void
  faqItems?: FAQItem[]
}

export function QuickReplyButtons({ translations, isTyping, onQuickReply, faqItems }: QuickReplyButtonsProps) {
  const buttonClasses =
    'flex-1 px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center';

  // Use custom FAQ items if provided, otherwise use default translations
  const defaultFAQs = [
    { id: 'extend-policy', question: translations.extendPolicyQuestion },
    { id: 'find-contract', question: translations.findContractNumberQuestion },
    { id: 'submit-claim', question: translations.submitClaimQuestion },
  ];

  const faqs = faqItems || defaultFAQs;

  return (
    <div className="flex flex-wrap gap-2 mb-4">
      {faqs.length > 0 &&
        faqs.slice(0, 2).map((faq) => (
          <button key={faq.id} className={buttonClasses} onClick={() => onQuickReply(faq.question)} disabled={isTyping}>
            {faq.question}
          </button>
        ))}

      {faqs.length > 2 && (
        <button
          className={'w-full px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center mt-2'}
          onClick={() => onQuickReply(faqs[2].question)}
          disabled={isTyping}
        >
          {faqs[2].question}
        </button>
      )}
    </div>
  );
}
