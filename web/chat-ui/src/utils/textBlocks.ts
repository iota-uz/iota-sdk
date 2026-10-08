/**
 * Utilities for splitting accumulated assistant content into the distinct
 * text blocks the server emitted.
 *
 * Background: the executor interleaves text with tool calls within one
 * turn — `text → tool_call → text → tool_call → final_text`. Before the
 * backend emitted `text_block_end` markers (iota-uz/iota-sdk#732 M1),
 * all text segments collapsed into one paragraph on the client. Now the
 * server emits a `text_block_end` event before each tool_start and at
 * snapshot time carries byte offsets in `partialMetadata.text_block_offsets`.
 *
 * `splitIntoTextBlocks` is the client-side inverse: given the
 * accumulated content string and the ordered list of byte offsets that
 * mark segment ends, return one entry per segment. The trailing un-closed
 * segment (if any) is included as the last entry.
 */
export interface AssistantTextBlock {
    /** Zero-based index matching the seq carried on text_block_end events. */
    seq: number;
    /** Markdown source for this segment. */
    content: string;
}
/**
 * Split an accumulated assistant content string into blocks using byte
 * offsets emitted by the server. Offsets are EXCLUSIVE end markers —
 * i.e. `offsets[0]` is the length of block 0.
 *
 * - When `offsets` is empty or undefined, returns a single block with
 *   the full content.
 * - When offsets are provided but content is shorter than the last
 *   offset (e.g. stale metadata), offsets beyond the content length are
 *   clamped so no crash propagates up.
 * - Trailing content after the last offset is included as an extra
 *   block (the un-closed segment during streaming / the final segment
 *   after the last tool call).
 */
export function splitIntoTextBlocks(content: string, offsets?: ReadonlyArray<number> | null): AssistantTextBlock[] {
    if (!content) {
        return [];
    }
    if (!offsets || offsets.length === 0) {
        return [{ seq: 0, content }];
    }
    const sanitized = [...offsets]
        .map((n) => Math.max(0, Math.min(Math.floor(n), content.length)))
        .sort((a, b) => a - b);
    const blocks: AssistantTextBlock[] = [];
    let cursor = 0;
    for (let i = 0; i < sanitized.length; i++) {
        const end = sanitized[i];
        if (end <= cursor) {
            continue;
        }
        const slice = content.slice(cursor, end);
        if (slice) {
            blocks.push({ seq: blocks.length, content: slice });
        }
        cursor = end;
    }
    if (cursor < content.length) {
        blocks.push({ seq: blocks.length, content: content.slice(cursor) });
    }
    return blocks;
}
/**
 * Normalise partialMetadata.text_block_offsets from a StreamSnapshotPayload
 * into a number[]. Guards against malformed server data (non-array,
 * non-numeric entries) so UI code never has to think about shapes.
 */
export function readTextBlockOffsets(partialMetadata?: Record<string, unknown> | null): number[] {
    if (!partialMetadata) {
        return [];
    }
    const raw = partialMetadata["text_block_offsets"];
    if (!Array.isArray(raw)) {
        return [];
    }
    const out: number[] = [];
    for (const entry of raw) {
        if (typeof entry === "number" && Number.isFinite(entry) && entry >= 0) {
            out.push(Math.floor(entry));
        }
    }
    return out;
}
