/**
 * SSE stream parser for consuming Server-Sent Events.
 */

export interface SSEEvent {
  type: string
  content?: string
  error?: string
  sessionId?: string
  toolName?: string
  toolCallId?: string
  durationMs?: number
  success?: boolean
  [key: string]: unknown
}

/**
 * Helper function to process SSE data lines and parse JSON events.
 */
function* processDataLines(lines: string[]): Generator<SSEEvent, void, unknown> {
  for (const line of lines) {
    if (line.startsWith(':')) continue

    if (line.startsWith('data: ')) {
      const jsonStr = line.slice(6)
      if (jsonStr === '[DONE]') continue

      try {
        const parsed = JSON.parse(jsonStr) as SSEEvent
        yield parsed
      } catch (err) {
        console.error('SSE parse error:', err, 'Data:', jsonStr)
        // Yield error event so consumer can react appropriately
        yield {
          type: 'error',
          error: 'Failed to parse SSE event',
        }
      }
    }
  }
}

/**
 * Parses an SSE stream and yields parsed JSON events.
 */
export async function* parseSSEStream(
  reader: ReadableStreamDefaultReader<Uint8Array>
): AsyncGenerator<SSEEvent, void, unknown> {
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const events = buffer.split('\n\n')
      buffer = events.pop() || ''

      for (const event of events) {
        if (!event.trim()) continue
        yield* processDataLines(event.split('\n'))
      }
    }

    // Process any remaining data in buffer
    if (buffer.trim()) {
      yield* processDataLines(buffer.split('\n'))
    }
  } finally {
    reader.releaseLock()
  }
}
