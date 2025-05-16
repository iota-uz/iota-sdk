'use client';

import React from 'react';
import { X } from 'lucide-react';
import { useState } from 'react';
import type { Translations } from '@/lib/translations';

interface CallbackModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (_phoneNumber: string) => void
  translations: Translations
}

export function CallbackModal({ isOpen, onClose, onSubmit, translations }: CallbackModalProps) {
  const [phoneNumber, setPhoneNumber] = useState('');
  const [consentChecked, setConsentChecked] = useState(false);

  if (!isOpen) {return null;}

  const handleSubmit = () => {
    if (phoneNumber.trim() && consentChecked) {
      onSubmit(phoneNumber);
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-3xl w-full max-w-md mx-4 overflow-hidden">
        <div className="p-4 flex justify-between items-center">
          <span className="text-gray-500 text-sm">Modal</span>
          <button
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded-full border border-gray-300"
          >
            <X size={16} />
          </button>
        </div>

        <div className="p-6 pt-0">
          <h2 className="text-2xl font-medium text-[#0a223e] mb-2">{translations.callbackModalTitle}</h2>
          <p className="text-[#8b98a5] mb-6">{translations.callbackModalSubtitle}</p>

          <div className="mb-2 text-[#8b98a5] text-sm">{translations.callbackPhoneInputLabel}</div>

          <div className="relative mb-4">
            <input
              type="text"
              className="w-full p-3 border border-gray-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-[#2e67b4]"
              placeholder={translations.phoneInputPlaceholder}
              value={phoneNumber}
              onChange={(e) => setPhoneNumber(e.target.value)}
            />
            <div className="absolute right-3 top-1/2 transform -translate-y-1/2 bg-[#2e67b4] text-white w-6 h-6 rounded-full flex items-center justify-center">
              <span className="text-xs">B</span>
            </div>
          </div>

          <p className="text-[#2e67b4] text-sm mb-4">{translations.dataPrivacyMessage}</p>

          <div className="flex items-start mb-6">
            <div
              className={`w-6 h-6 flex-shrink-0 rounded border ${
                consentChecked ? 'bg-[#2e67b4] border-[#2e67b4] flex items-center justify-center' : 'border-gray-300'
              } mr-2 cursor-pointer`}
              onClick={() => setConsentChecked(!consentChecked)}
            >
              {consentChecked && <span className="text-white text-xs">✓</span>}
            </div>
            <span className="text-[#0a223e]">{translations.dataProcessingConsent}</span>
          </div>

          <div className="flex gap-4">
            <button onClick={onClose} className="flex-1 py-3 bg-[#e4e9ee] text-[#0a223e] rounded-lg">
              {translations.backButton}
            </button>
            <button
              onClick={handleSubmit}
              disabled={!phoneNumber.trim() || !consentChecked}
              className={`flex-1 py-3 rounded-lg ${
                phoneNumber.trim() && consentChecked ? 'bg-[#2e67b4] text-white' : 'bg-[#e4e9ee] text-[#bdc8d2]'
              }`}
            >
              {translations.requestCallButton}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
