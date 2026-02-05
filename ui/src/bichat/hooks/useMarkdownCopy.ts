/**
 * useMarkdownCopy Hook
 * Manages copy-to-clipboard state for code blocks in markdown
 */

import { useState, useCallback, useRef } from 'react'

export interface UseMarkdownCopyOptions {
  /** Duration to show "copied" state in ms (default: 2000) */
  copiedDuration?: number
  /** Callback when copy succeeds */
  onCopy?: (content: string, language?: string) => void
  /** Callback when copy fails */
  onError?: (error: Error) => void
}

export interface UseMarkdownCopyReturn {
  /** Map of copied states by block ID */
  copiedStates: Map<string, boolean>
  /** Check if a specific block is in copied state */
  isCopied: (blockId: string) => boolean
  /** Copy content with block ID tracking */
  copy: (blockId: string, content: string, language?: string) => Promise<void>
  /** Reset copied state for a specific block */
  reset: (blockId: string) => void
  /** Reset all copied states */
  resetAll: () => void
}

/**
 * Hook for managing copy states for multiple code blocks
 *
 * @example
 * ```tsx
 * const markdownCopy = useMarkdownCopy({
 *   onCopy: (content, lang) => console.log(`Copied ${lang} code`),
 * })
 *
 * function CodeBlock({ id, code, language }) {
 *   return (
 *     <div>
 *       <pre>{code}</pre>
 *       <button onClick={() => markdownCopy.copy(id, code, language)}>
 *         {markdownCopy.isCopied(id) ? 'Copied!' : 'Copy'}
 *       </button>
 *     </div>
 *   )
 * }
 * ```
 */
export function useMarkdownCopy(options: UseMarkdownCopyOptions = {}): UseMarkdownCopyReturn {
  const { copiedDuration = 2000, onCopy, onError } = options

  const [copiedStates, setCopiedStates] = useState<Map<string, boolean>>(new Map())
  const timeoutsRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())

  const isCopied = useCallback(
    (blockId: string): boolean => {
      return copiedStates.get(blockId) ?? false
    },
    [copiedStates]
  )

  const copy = useCallback(
    async (blockId: string, content: string, language?: string) => {
      try {
        await navigator.clipboard.writeText(content)

        // Set copied state
        setCopiedStates((prev) => {
          const next = new Map(prev)
          next.set(blockId, true)
          return next
        })

        onCopy?.(content, language)

        // Clear existing timeout for this block
        const existingTimeout = timeoutsRef.current.get(blockId)
        if (existingTimeout) {
          clearTimeout(existingTimeout)
        }

        // Set timeout to reset copied state
        const timeout = setTimeout(() => {
          setCopiedStates((prev) => {
            const next = new Map(prev)
            next.set(blockId, false)
            return next
          })
          timeoutsRef.current.delete(blockId)
        }, copiedDuration)

        timeoutsRef.current.set(blockId, timeout)
      } catch (error) {
        const err = error instanceof Error ? error : new Error('Failed to copy')
        onError?.(err)
        throw err
      }
    },
    [copiedDuration, onCopy, onError]
  )

  const reset = useCallback((blockId: string) => {
    setCopiedStates((prev) => {
      const next = new Map(prev)
      next.set(blockId, false)
      return next
    })

    const timeout = timeoutsRef.current.get(blockId)
    if (timeout) {
      clearTimeout(timeout)
      timeoutsRef.current.delete(blockId)
    }
  }, [])

  const resetAll = useCallback(() => {
    setCopiedStates(new Map())

    // Clear all timeouts
    timeoutsRef.current.forEach((timeout) => clearTimeout(timeout))
    timeoutsRef.current.clear()
  }, [])

  return {
    copiedStates,
    isCopied,
    copy,
    reset,
    resetAll,
  }
}
