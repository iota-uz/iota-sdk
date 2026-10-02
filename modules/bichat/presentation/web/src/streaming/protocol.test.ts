import { describe, expect, it } from 'vitest'
import { parseStreamChunk, readSSEResponse, STREAM_EVENT_TYPES } from './protocol'

// Mirror of pkg/httpdto/stream_events.go allStreamEventTypes. If this drifts
// from the Go source of truth, this test fails.
const GO_STREAM_EVENT_TYPES = [
  'chunk',
  'content',
  'thinking',
  'tool_start',
  'tool_end',
  'text_block_end',
  'snapshot',
  'interrupt',
  'citation',
  'usage',
  'ping',
  'stream_started',
  'done',
  'cancelled',
  'error',
  'failed',
]

describe('stream protocol', () => {
  it('event names stay in sync with the Go wire contract', () => {
    expect([...STREAM_EVENT_TYPES].sort()).toEqual([...GO_STREAM_EVENT_TYPES].sort())
  })

  it('parses an event triple into a chunk', () => {
    const chunk = parseStreamChunk('chunk', JSON.stringify({ content: 'hello' }))
    expect(chunk).toEqual({ type: 'chunk', content: 'hello' })
  })

  it('falls back to payload type when the event line is generic', () => {
    const chunk = parseStreamChunk('message', JSON.stringify({ type: 'stream_started', runId: 'r-1' }))
    expect(chunk?.type).toBe('stream_started')
    expect(chunk?.runId).toBe('r-1')
  })

  it('returns undefined for unknown event names without a payload type', () => {
    expect(parseStreamChunk('mystery', '{}')).toBeUndefined()
  })

  it('treats non-JSON data as raw content', () => {
    expect(parseStreamChunk('content', 'plain text')).toEqual({ type: 'content', content: 'plain text' })
  })
})

describe('readSSEResponse', () => {
  function sseResponse(frames: string[]): Response {
    const encoder = new TextEncoder()
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const frame of frames) controller.enqueue(encoder.encode(frame))
        controller.close()
      },
    })
    return new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } })
  }

  it('dispatches id-less triples and skips comments', async () => {
    const events: Array<[string, string]> = []
    await readSSEResponse(
      sseResponse([
        ': stream-open\n\n',
        'event: chunk\ndata: {"content":"he"}\n\n',
        'event: chunk\ndata: {"content":"llo"}\n\n',
        'event: done\ndata: {}\n\n',
      ]),
      (name, data) => events.push([name, data]),
    )
    expect(events).toEqual([
      ['chunk', '{"content":"he"}'],
      ['chunk', '{"content":"llo"}'],
      ['done', '{}'],
    ])
  })

  it('handles frames split across chunk boundaries', async () => {
    const events: Array<[string, string]> = []
    await readSSEResponse(
      sseResponse(['event: chun', 'k\ndata: {"con', 'tent":"abc"}\n\nevent: done\ndata: {}\n\n']),
      (name, data) => events.push([name, data]),
    )
    expect(events.map(([name]) => name)).toEqual(['chunk', 'done'])
  })

  it('joins multi-line data fields', async () => {
    const events: Array<[string, string]> = []
    await readSSEResponse(
      sseResponse(['event: content\ndata: line1\ndata: line2\n\n']),
      (name, data) => events.push([name, data]),
    )
    expect(events).toEqual([['content', 'line1\nline2']])
  })
})
