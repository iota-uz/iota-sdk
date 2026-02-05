/**
 * Predefined theme configurations
 */

import { Theme } from './types'

export const lightTheme: Theme = {
  name: 'light',
  colors: {
    background: '#ffffff',
    surface: '#f9fafb',
    primary: '#3b82f6',
    secondary: '#6b7280',
    text: '#111827',
    textMuted: '#6b7280',
    border: '#e5e7eb',
    error: '#ef4444',
    success: '#10b981',
    warning: '#f59e0b',
    userBubble: '#3b82f6',
    assistantBubble: '#f3f4f6',
    userText: '#ffffff',
    assistantText: '#111827',
  },
  spacing: {
    xs: '0.25rem',
    sm: '0.5rem',
    md: '1rem',
    lg: '1.5rem',
    xl: '2rem',
  },
  borderRadius: {
    sm: '0.25rem',
    md: '0.5rem',
    lg: '0.75rem',
    full: '9999px',
  },
}

export const darkTheme: Theme = {
  name: 'dark',
  colors: {
    background: '#111827',
    surface: '#1f2937',
    primary: '#60a5fa',
    secondary: '#9ca3af',
    text: '#f9fafb',
    textMuted: '#9ca3af',
    border: '#374151',
    error: '#f87171',
    success: '#34d399',
    warning: '#fbbf24',
    userBubble: '#2563eb',
    assistantBubble: '#1f2937',
    userText: '#f9fafb',
    assistantText: '#f9fafb',
  },
  spacing: {
    xs: '0.25rem',
    sm: '0.5rem',
    md: '1rem',
    lg: '1.5rem',
    xl: '2rem',
  },
  borderRadius: {
    sm: '0.25rem',
    md: '0.5rem',
    lg: '0.75rem',
    full: '9999px',
  },
}
