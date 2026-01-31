/**
 * useStreaming hook
 * Handles AsyncGenerator streaming responses
 */

import { useState, useEffect, useCallback } from 'react'
import { StreamChunk } from '../types'

interface UseStreamingOptions {
  onChunk?: (content: string) => void
  onError?: (error: string) => void
  onDone?: () => void
}

export function useStreaming(options: UseStreamingOptions = {}) {
  const [content, setContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const processStream = useCallback(
    async (stream: AsyncGenerator<StreamChunk>) => {
      setIsStreaming(true)
      setError(null)
      setContent('')

      try {
        for await (const chunk of stream) {
          if (chunk.type === 'chunk' && chunk.content) {
            setContent((prev) => {
              const newContent = prev + chunk.content
              options.onChunk?.(newContent)
              return newContent
            })
          } else if (chunk.type === 'error') {
            const errorMsg = chunk.error || 'Stream error'
            setError(errorMsg)
            options.onError?.(errorMsg)
            break
          } else if (chunk.type === 'done') {
            options.onDone?.()
            break
          }
        }
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : 'Unknown error'
        setError(errorMsg)
        options.onError?.(errorMsg)
      } finally {
        setIsStreaming(false)
      }
    },
    [options]
  )

  const reset = useCallback(() => {
    setContent('')
    setError(null)
    setIsStreaming(false)
  }, [])

  return {
    content,
    isStreaming,
    error,
    processStream,
    reset,
  }
}
