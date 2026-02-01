import { useState, useRef, useCallback } from 'react'
import type { StreamingHook } from '../types'

/**
 * useStreaming provides SSE (Server-Sent Events) streaming utilities with cancellation support.
 *
 * Usage:
 * const { isStreaming, processStream, cancel, reset } = useStreaming()
 *
 * // Process async generator stream
 * await processStream(messageStream, (chunk) => {
 *   console.log('Received:', chunk)
 * })
 *
 * // Cancel ongoing stream
 * cancel()
 *
 * // Reset state after stream completion
 * reset()
 */
export function useStreaming(): StreamingHook {
  const [isStreaming, setIsStreaming] = useState(false)
  const abortControllerRef = useRef<AbortController | null>(null)

  const processStream = useCallback(
    async <T,>(
      generator: AsyncGenerator<T>,
      onChunk: (chunk: T) => void,
      signal?: AbortSignal
    ): Promise<void> => {
      setIsStreaming(true)

      // Create abort controller if not provided
      const controller = new AbortController()
      abortControllerRef.current = controller

      // Listen to external signal if provided
      if (signal) {
        signal.addEventListener('abort', () => {
          controller.abort()
        })
      }

      try {
        for await (const chunk of generator) {
          // Check if stream was cancelled
          if (controller.signal.aborted) {
            break
          }

          onChunk(chunk)
        }
      } catch (error) {
        // Stream was cancelled or errored
        if (controller.signal.aborted) {
          // Cancellation is expected, don't throw
          return
        }
        throw error
      } finally {
        setIsStreaming(false)
        abortControllerRef.current = null
      }
    },
    []
  )

  const cancel = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }
    setIsStreaming(false)
  }, [])

  const reset = useCallback(() => {
    abortControllerRef.current = null
    setIsStreaming(false)
  }, [])

  return {
    isStreaming,
    processStream,
    cancel,
    reset
  }
}
