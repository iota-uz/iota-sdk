import { useEffect } from 'react'

export interface ShortcutConfig {
  key: string
  ctrl?: boolean
  shift?: boolean
  alt?: boolean
  meta?: boolean
  callback: () => void
  preventDefault?: boolean
  description?: string
}

/**
 * Hook for managing global keyboard shortcuts
 * Automatically handles modifier keys and input field exclusion
 *
 * @param shortcuts - Array of keyboard shortcut configurations
 *
 * @example
 * useKeyboardShortcuts([
 *   { key: 'k', ctrl: true, callback: () => focusSearch(), description: 'Focus search' },
 *   { key: '?', callback: () => setShowHelp(true), description: 'Show keyboard shortcuts' },
 * ])
 */
export function useKeyboardShortcuts(shortcuts: ShortcutConfig[]) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ignore if user is typing in input/textarea/contenteditable
      const target = e.target as HTMLElement
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target.isContentEditable
      ) {
        // Allow some shortcuts even in inputs (like Escape)
        const allowInInput = shortcuts.find(s =>
          s.key.toLowerCase() === e.key.toLowerCase() &&
          s.key.toLowerCase() === 'escape'
        )
        if (!allowInInput) {
          return
        }
      }

      // Find matching shortcut
      const matchingShortcut = shortcuts.find((s) => {
        const keyMatches = e.key.toLowerCase() === s.key.toLowerCase()
        const modMatches = s.meta
          ? e.metaKey && !e.ctrlKey
          : s.ctrl
            ? e.ctrlKey || e.metaKey
            : !e.ctrlKey && !e.metaKey
        const shiftMatches = s.shift ? e.shiftKey : !e.shiftKey
        const altMatches = s.alt ? e.altKey : !e.altKey

        return keyMatches && modMatches && shiftMatches && altMatches
      })

      if (matchingShortcut) {
        if (matchingShortcut.preventDefault !== false) {
          e.preventDefault()
        }
        matchingShortcut.callback()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [shortcuts])
}
