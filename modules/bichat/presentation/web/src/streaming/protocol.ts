// Wire protocol of the BiChat SSE stream. Event names mirror
// pkg/httpdto/stream_events.go (StreamEventType) — the Go side is the source
// of truth; there is a drift-guard test in protocol.test.ts.

export const STREAM_EVENT_TYPES = [
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
] as const

export type StreamEventType = (typeof STREAM_EVENT_TYPES)[number]

const TERMINAL_EVENT_TYPES: ReadonlySet<string> = new Set(['done', 'cancelled', 'error', 'failed'])

export function isTerminalStreamEvent(type: string): boolean {
  return TERMINAL_EVENT_TYPES.has(type)
}

export interface CitationPayload {
  id?: string
  title?: string
  url?: string
  snippet?: string
  source?: string
}

export interface UsagePayload {
  promptTokens?: number
  completionTokens?: number
  totalTokens?: number
  generationMs?: number
}

export interface ToolEventPayload {
  callId?: string
  name?: string
  phase?: string
  status?: string
  detail?: string
  error?: string
}

export interface InterruptQuestionOption {
  value: string
  label: string
}

export interface InterruptQuestionPayload {
  id?: string
  name?: string
  label?: string
  type?: string
  required?: boolean
  placeholder?: string
  options?: InterruptQuestionOption[]
}

export interface InterruptEventPayload {
  checkpointId?: string
  runId?: string
  questions?: InterruptQuestionPayload[]
}

export interface StreamSnapshotPayload {
  conversationTurns?: number
  turn?: number
  partialContent?: string
  partialMetadata?: Record<string, unknown>
}

export interface StreamChunk {
  type: StreamEventType | string
  content?: string
  citation?: CitationPayload
  usage?: UsagePayload
  tool?: ToolEventPayload
  interrupt?: InterruptEventPayload
  generationMs?: number
  error?: string
  timestamp?: number
  snapshot?: StreamSnapshotPayload
  runId?: string
  textBlockSeq?: number
}

export function parseStreamChunk(eventName: string, rawData: string): StreamChunk | undefined {
  let payload: Record<string, unknown> = {}
  if (rawData.trim().length > 0) {
    try {
      payload = JSON.parse(rawData) as Record<string, unknown>
    } catch {
      payload = { content: rawData }
    }
  }
  const type = normalizeType(eventName, payload)
  if (type === undefined) return undefined
  return {
    type,
    content: typeof payload.content === 'string' ? payload.content : undefined,
    citation: (payload.citation as StreamChunk['citation']) ?? undefined,
    usage: (payload.usage as StreamChunk['usage']) ?? undefined,
    tool: (payload.tool as StreamChunk['tool']) ?? undefined,
    interrupt: (payload.interrupt as StreamChunk['interrupt']) ?? undefined,
    generationMs: typeof payload.generationMs === 'number' ? payload.generationMs : undefined,
    error: typeof payload.error === 'string' ? payload.error : undefined,
    timestamp: typeof payload.timestamp === 'number' ? payload.timestamp : undefined,
    snapshot: (payload.snapshot as StreamChunk['snapshot']) ?? undefined,
    runId: typeof payload.runId === 'string' ? payload.runId : undefined,
    textBlockSeq: typeof payload.textBlockSeq === 'number' ? payload.textBlockSeq : undefined,
  }
}

function normalizeType(eventName: string, payload: Record<string, unknown>): string | undefined {
  if ((STREAM_EVENT_TYPES as readonly string[]).includes(eventName)) return eventName
  const payloadType = payload.type
  if (typeof payloadType === 'string' && payloadType.length > 0) return payloadType
  return undefined
}

/**
 * Minimal SSE reader over a fetch Response body. Comment lines (heartbeats
 * like `: ping`) are ignored; `id:`/`event:`/`data:` triples are dispatched.
 */
export async function readSSEResponse(
  response: Response,
  onEvent: (eventName: string, rawData: string) => void,
): Promise<void> {
  const body = response.body
  if (!body) throw new Error('streaming unsupported: missing response body')
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const dispatch = (block: string) => {
    let eventName = 'message'
    const dataLines: string[] = []
    for (const line of block.split('\n')) {
      if (line.startsWith(':')) continue
      if (line.startsWith('event:')) eventName = line.slice(6).trim()
      else if (line.startsWith('data:')) dataLines.push(line.slice(5).replace(/^ /, ''))
    }
    if (dataLines.length === 0 && eventName === 'message') return
    onEvent(eventName, dataLines.join('\n'))
  }

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let separator = buffer.indexOf('\n\n')
    while (separator !== -1) {
      const block = buffer.slice(0, separator)
      buffer = buffer.slice(separator + 2)
      if (block.trim().length > 0) dispatch(block)
      separator = buffer.indexOf('\n\n')
    }
  }
  if (buffer.trim().length > 0) dispatch(buffer.trim())
}
