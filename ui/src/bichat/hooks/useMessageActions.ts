/**
 * useMessageActions Hook
 * Provides copy, regenerate, and edit functionality for messages
 */

import { useState, useCallback, useRef, useEffect } from 'react'

export interface UseMessageActionsOptions {
  /** Callback when copy succeeds */
  onCopy?: (content: string) => void
  /** Callback when copy fails */
  onCopyError?: (error: Error) => void
  /** Callback when regenerate is triggered */
  onRegenerate?: () => void | Promise<void>
  /** Callback when edit is triggered */
  onEdit?: (content: string) => void | Promise<void>
  /** Duration to show "copied" state in ms (default: 2000) */
  copiedDuration?: number
}

export interface UseMessageActionsReturn {
  /** Whether content was recently copied */
  isCopied: boolean
  /** Whether regenerate is in progress */
  isRegenerating: boolean
  /** Whether edit is in progress */
  isEditing: boolean
  /** Copy content to clipboard */
  copy: (content: string) => Promise<void>
  /** Trigger regenerate action */
  regenerate: () => Promise<void>
  /** Trigger edit action */
  edit: (content: string) => Promise<void>
  /** Reset all states */
  reset: () => void
}

/**
 * Hook for managing message actions (copy, regenerate, edit)
 *
 * @example
 * ```tsx
 * const actions = useMessageActions({
 *   onRegenerate: () => chatContext.regenerateMessage(messageId),
 *   onEdit: (content) => chatContext.editMessage(messageId, content),
 *   onCopy: () => toast.success('Copied!'),
 * })
 *
 * <button onClick={() => actions.copy(message.content)}>
 *   {actions.isCopied ? 'Copied!' : 'Copy'}
 * </button>
 *
 * <button onClick={actions.regenerate} disabled={actions.isRegenerating}>
 *   {actions.isRegenerating ? 'Regenerating...' : 'Regenerate'}
 * </button>
 * ```
 */
export function useMessageActions(options: UseMessageActionsOptions = {}): UseMessageActionsReturn {
  const { onCopy, onCopyError, onRegenerate, onEdit, copiedDuration = 2000 } = options

  const [isCopied, setIsCopied] = useState(false)
  const [isRegenerating, setIsRegenerating] = useState(false)
  const [isEditing, setIsEditing] = useState(false)

  const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (copiedTimeoutRef.current) {
        clearTimeout(copiedTimeoutRef.current)
        copiedTimeoutRef.current = null
      }
    }
  }, [])

  const copy = useCallback(
    async (content: string) => {
      try {
        await navigator.clipboard.writeText(content)
        setIsCopied(true)
        onCopy?.(content)

        // Clear existing timeout
        if (copiedTimeoutRef.current) {
          clearTimeout(copiedTimeoutRef.current)
        }

        // Reset copied state after duration
        copiedTimeoutRef.current = setTimeout(() => {
          setIsCopied(false)
          copiedTimeoutRef.current = null
        }, copiedDuration)
      } catch (error) {
        const err = error instanceof Error ? error : new Error('Failed to copy')
        onCopyError?.(err)
        throw err
      }
    },
    [onCopy, onCopyError, copiedDuration]
  )

  const regenerate = useCallback(async () => {
    if (!onRegenerate) return
    if (isRegenerating) return

    setIsRegenerating(true)
    try {
      await onRegenerate()
    } finally {
      setIsRegenerating(false)
    }
  }, [onRegenerate, isRegenerating])

  const edit = useCallback(
    async (content: string) => {
      if (!onEdit) return
      if (isEditing) return

      setIsEditing(true)
      try {
        await onEdit(content)
      } finally {
        setIsEditing(false)
      }
    },
    [onEdit, isEditing]
  )

  const reset = useCallback(() => {
    setIsCopied(false)
    setIsRegenerating(false)
    setIsEditing(false)
    if (copiedTimeoutRef.current) {
      clearTimeout(copiedTimeoutRef.current)
      copiedTimeoutRef.current = null
    }
  }, [])

  return {
    isCopied,
    isRegenerating,
    isEditing,
    copy,
    regenerate,
    edit,
    reset,
  }
}
