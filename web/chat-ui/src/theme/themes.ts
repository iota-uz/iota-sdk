/**
 * Predefined theme configurations
 */
import { Theme } from './types';
export const lightTheme: Theme = {
    name: 'light',
    colors: {
        background: '#f9fafb', // --bichat-color-bg (gray-50)
        surface: '#ffffff', // --bichat-color-surface
        primary: '#2563eb', // --bichat-color-accent (primary-600)
        secondary: '#6b7280', // gray-500
        text: '#111827', // --bichat-color-text (gray-900)
        textMuted: '#6b7280', // --bichat-color-text-muted (gray-500)
        border: '#e5e7eb', // --bichat-color-border (gray-200)
        error: '#ef4444', // --bichat-color-error-500
        success: '#22c55e', // --bichat-color-success-500
        warning: '#f59e0b', // --bichat-color-warning-500
        userBubble: '#2563eb', // --bichat-color-user-bubble-bg (primary-600)
        assistantBubble: '#ffffff', // --bichat-color-assistant-bubble-bg (surface)
        userText: '#ffffff', // --bichat-color-user-bubble-text
        assistantText: '#111827', // --bichat-color-assistant-bubble-text (gray-900)
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
};
export const darkTheme: Theme = {
    name: 'dark',
    colors: {
        background: '#111827', // --bichat-color-bg dark (gray-900)
        surface: '#1f2937', // --bichat-color-surface dark (gray-800)
        primary: '#60a5fa', // primary-400 (lighter for dark mode contrast)
        secondary: '#9ca3af', // gray-400
        text: '#f3f4f6', // --bichat-color-text dark (gray-100)
        textMuted: '#9ca3af', // --bichat-color-text-muted dark (gray-400)
        border: '#374151', // --bichat-color-border dark (gray-700)
        error: '#f87171', // red-400
        success: '#34d399', // emerald-400
        warning: '#fbbf24', // amber-400
        userBubble: '#1d4ed8', // --bichat-color-user-bubble-bg dark (primary-700)
        assistantBubble: '#1f2937', // --bichat-color-assistant-bubble-bg dark (gray-800)
        userText: '#f3f4f6', // gray-100
        assistantText: '#f3f4f6', // gray-100
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
};
