/**
 * SSE stream parser for consuming Server-Sent Events.
 */
import type { StreamChunk, StreamEvent } from '../types';
export interface SSEEvent {
    type: string;
    content?: string;
    error?: string;
    sessionId?: string;
    toolName?: string;
    toolCallId?: string;
    durationMs?: number;
    success?: boolean;
    [key: string]: unknown;
}
/**
 * Helper function to process SSE data lines and parse JSON events.
 */
function* processDataLines(lines: string[]): Generator<SSEEvent, void, unknown> {
    for (const line of lines) {
        if (line.startsWith(':')) {
            continue;
        }
        if (line.startsWith('data: ')) {
            const jsonStr = line.slice(6);
            if (jsonStr === '[DONE]') {
                continue;
            }
            try {
                const parsed = JSON.parse(jsonStr) as SSEEvent;
                yield parsed;
            }
            catch (err) {
                console.error('SSE parse error:', err, 'Data:', jsonStr);
                // Yield error event so consumer can react appropriately
                yield {
                    type: 'error',
                    error: 'Failed to parse SSE event',
                };
            }
        }
    }
}
/**
 * Parses an SSE stream and yields parsed JSON events.
 */
export async function* parseSSEStream(reader: ReadableStreamDefaultReader<Uint8Array>): AsyncGenerator<SSEEvent, void, unknown> {
    const decoder = new TextDecoder();
    let buffer = '';
    try {
        while (true) {
            const { done, value } = await reader.read();
            if (done) {
                break;
            }
            buffer += decoder.decode(value, { stream: true });
            const events = buffer.split('\n\n');
            buffer = events.pop() || '';
            for (const event of events) {
                if (!event.trim()) {
                    continue;
                }
                yield* processDataLines(event.split('\n'));
            }
        }
        // Process any remaining data in buffer
        if (buffer.trim()) {
            yield* processDataLines(buffer.split('\n'));
        }
    }
    finally {
        reader.releaseLock();
    }
}
/**
 * Terminal event types that signal end of stream.
 */
const TERMINAL_TYPES = new Set(['done', 'error']);
/**
 * Parses BiChat SSE stream with normalization and terminal event guarantee.
 *
 * Guarantees that the generator always yields a terminal event (`done` or
 * `error`) as its last item — even when the underlying stream closes silently
 * without one.
 */
export async function* parseBichatStream(reader: ReadableStreamDefaultReader<Uint8Array>): AsyncGenerator<StreamChunk, void, unknown> {
    let yieldedTerminal = false;
    for await (const event of parseSSEStream(reader)) {
        if (event.type === 'ping') {
            continue;
        }
        const parsed = event as StreamChunk;
        // Infer type if missing
        const inferredType = parsed.type || (parsed.content ? 'content' : 'error');
        const normalized: StreamChunk = {
            ...parsed,
            type: inferredType,
        };
        if (TERMINAL_TYPES.has(inferredType)) {
            yieldedTerminal = true;
        }
        yield normalized;
    }
    // Guarantee: always emit a terminal event
    if (!yieldedTerminal) {
        yield { type: 'done' };
    }
}
/**
 * Type-safe version of `parseBichatStream` that yields `StreamEvent`
 * discriminated union members instead of the flat `StreamChunk`.
 *
 * Use this in new code for proper type narrowing on `event.type`.
 */
export async function* parseBichatStreamEvents(reader: ReadableStreamDefaultReader<Uint8Array>): AsyncGenerator<StreamEvent, void, unknown> {
    for await (const chunk of parseBichatStream(reader)) {
        const event = toStreamEvent(chunk);
        if (event) {
            yield event;
        }
    }
}
/**
 * Convert a flat StreamChunk into a discriminated StreamEvent.
 * Returns null for chunks that can't be meaningfully mapped.
 */
function toStreamEvent(chunk: StreamChunk): StreamEvent | null {
    switch (chunk.type) {
        case 'chunk':
        case 'content':
            return { type: 'content', content: chunk.content ?? '' };
        case 'thinking':
            return { type: 'thinking', content: chunk.content ?? '' };
        case 'tool_start':
            return chunk.tool ? { type: 'tool_start', tool: chunk.tool } : null;
        case 'tool_end':
            return chunk.tool ? { type: 'tool_end', tool: chunk.tool } : null;
        case 'usage':
            return chunk.usage ? { type: 'usage', usage: chunk.usage } : null;
        case 'user_message':
            return chunk.sessionId ? { type: 'user_message', sessionId: chunk.sessionId } : null;
        case 'interrupt':
            return chunk.interrupt
                ? { type: 'interrupt', interrupt: chunk.interrupt, sessionId: chunk.sessionId }
                : null;
        case 'text_block_end':
            // seq defaults to 0 so consumers can always key on it even when
            // the server mints the first block implicitly.
            return { type: 'text_block_end', seq: chunk.textBlockSeq ?? 0 };
        case 'done':
            return { type: 'done', sessionId: chunk.sessionId, generationMs: chunk.generationMs };
        case 'error':
            return { type: 'error', error: chunk.error ?? 'Unknown error' };
        default:
            return null;
    }
}
