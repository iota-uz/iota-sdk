/**
 * useStreaming hook
 * Handles AsyncGenerator streaming responses with cancellation support
 */

import { useState, useCallback, useRef } from 'react'
import { StreamChunk } from '../types'

interface UseStreamingOptions {
  onChunk?: (content: string) => void
  onError?: (error: string) => void
  onDone?: () => void
}

export function useStreaming(options: UseStreamingOptions = {}) {
  const [content, setContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  const processStream = useCallback(
    async (stream: AsyncGenerator<StreamChunk>, signal?: AbortSignal) => {
      setIsStreaming(true)
      setError(null)
      setContent('')

      // Create abort controller for this stream
      abortControllerRef.current = new AbortController()

      // Link external signal if provided
      if (signal) {
        signal.addEventListener('abort', () => {
          abortControllerRef.current?.abort()
        })
      }

      try {
        for await (const chunk of stream) {
          // Check if cancelled
          if (abortControllerRef.current?.signal.aborted) {
            break
          }

          if (chunk.type === 'chunk' && chunk.content) {
            setContent((prev) => {
              const newContent = prev + chunk.content
              options.onChunk?.(newContent)
              return newContent
            })
          } else if (chunk.type === 'error') {
            const errorMsg = chunk.error || 'Stream error'
            const err = new Error(errorMsg)
            setError(err)
            options.onError?.(errorMsg)
            break
          } else if (chunk.type === 'done') {
            options.onDone?.()
            break
          }
        }
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
          // Stream was cancelled - this is expected
          return
        }

        const errorObj = err instanceof Error ? err : new Error('Unknown error')
        setError(errorObj)
        options.onError?.(errorObj.message)
      } finally {
        setIsStreaming(false)
        abortControllerRef.current = null
      }
    },
    [options]
  )

  const cancel = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
      setIsStreaming(false)
    }
  }, [])

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
    cancel,
    reset,
  }
}
