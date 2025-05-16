import React from 'react';
import { Send } from 'lucide-react';
import type { Translations } from '@/lib/translations';
import { InputMask, unformat } from '@react-input/mask';

interface PhoneInputProps {
  phoneNumber: string
  setPhoneNumber: (_value: string) => void
  handleSubmit: () => void
  handleKeyPress: (_e: React.KeyboardEvent) => void
  isTyping: boolean
  translations: Translations
  isMobile: boolean
  error?: string
}

export const PhoneInput = ({ 
  phoneNumber, 
  setPhoneNumber, 
  handleSubmit, 
  handleKeyPress, 
  isTyping, 
  translations, 
  isMobile,
  error
}: PhoneInputProps) => {
  // Check if the number is valid - should have 12 digits (including country code)
  const isValid = phoneNumber && phoneNumber.replace(/[^0-9]/g, '').length === 12;
  
  // Options for the mask
  const maskOptions = {
    mask: '+998 __ ___ __ __',
    replacement: { _: /\d/ }
  };
  
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const maskedValue = e.target.value;
    // Get the actual digits from the masked value
    const digitsOnly = maskedValue.replace(/[^0-9]/g, '').length;
    
    // Only update the actual value when user has entered something
    if (digitsOnly > 3) { // +998 has 4 digits
      // Save the actual phone number with proper formatting for API
      setPhoneNumber(maskedValue);
    } else {
      // Reset to just the country code
      setPhoneNumber('+998');
    }
  };
  
  const handleClick = (e: React.MouseEvent<HTMLInputElement>) => {
    const input = e.currentTarget;
    
    // If the value is empty or just the placeholder, place cursor after country code
    if (input.value === '+998 __ ___ __ __' || input.value === '+998') {
      // Place cursor after the country code
      setTimeout(() => {
        input.setSelectionRange(6, 6);
      }, 0);
    }
  };
  
  // Determine if we should show the initial mask
  // We always want to show the full mask for better UX
  const displayValue = phoneNumber && phoneNumber.replace(/[^0-9]/g, '').length > 3 
    ? phoneNumber 
    : '+998 __ ___ __ __';
  
  return (
    <div className="mb-4">
      <label htmlFor="phone-input" className="block mb-2 text-sm font-medium text-[#0a223e]">
        {translations.phoneInputLabel}
      </label>
      <div className={`flex items-center p-3 ${error ? 'bg-red-50 border border-red-300' : 'bg-[#f2f5f8]'} rounded-lg`}>
        <InputMask
          id="phone-input"
          mask={maskOptions.mask}
          replacement={maskOptions.replacement}
          showMask
          className="bg-transparent focus:outline-none text-[#0a223e] flex-1"
          value={displayValue}
          onChange={handleChange}
          onKeyDown={handleKeyPress}
          onClick={handleClick}
          style={{ fontSize: isMobile ? '16px' : 'inherit' }}
          aria-invalid={error ? 'true' : 'false'}
        />
        <button 
          onClick={handleSubmit} 
          disabled={isTyping || !isValid || phoneNumber === '+998 __ ___ __ __' || phoneNumber === '+998'}
        >
          <Send 
            className={`ml-auto ${isTyping || !isValid || phoneNumber === '+998 __ ___ __ __' || phoneNumber === '+998' ? 'text-[#8b98a5]' : 'text-[#0a223e]'}`} 
            size={isMobile ? 24 : 20} 
          />
        </button>
      </div>
      {error && (
        <div className="mt-1 text-sm text-red-600" role="alert">
          {error}
        </div>
      )}
    </div>
  );
};