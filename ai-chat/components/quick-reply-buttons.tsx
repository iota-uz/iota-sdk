"use client"

import type { Translations } from "@/lib/translations"

interface QuickReplyButtonsProps {
  translations: Translations
  isTyping: boolean
  onQuickReply: (question: string) => void
}

export function QuickReplyButtons({ translations, isTyping, onQuickReply }: QuickReplyButtonsProps) {
  const buttonClasses =
    "flex-1 px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center"

  return (
    <div className="flex flex-wrap gap-2 mb-4">
      <button
        className={buttonClasses}
        onClick={() => onQuickReply(translations.extendPolicyQuestion)}
        disabled={isTyping}
      >
        {translations.extendPolicyQuestion}
      </button>
      <button
        className={buttonClasses}
        onClick={() => onQuickReply(translations.findContractNumberQuestion)}
        disabled={isTyping}
      >
        {translations.findContractNumberQuestion}
      </button>
      <button
        className={`w-full px-4 py-3 text-[#0a223e] bg-white border border-gray-300 rounded-full text-sm whitespace-normal text-center min-h-[40px] flex items-center justify-center mt-2`}
        onClick={() => onQuickReply(translations.submitClaimQuestion)}
        disabled={isTyping}
      >
        {translations.submitClaimQuestion}
      </button>
    </div>
  )
}
