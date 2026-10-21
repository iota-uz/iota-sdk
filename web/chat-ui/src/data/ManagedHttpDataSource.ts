import { HttpDataSource, type HttpDataSourceConfig } from './HttpDataSource';
import type { Attachment, StreamChunk } from '../types';
import { subscribeManagedStream, type SessionService, type StreamContract, type StreamSubscription, } from '@iota-uz/sdk/client-host';
const TERMINAL = new Set(['done', 'error', 'cancelled', 'failed']);
interface SendMessageOptions {
    requestId?: string;
    debugMode?: boolean;
    replaceFromMessageID?: string;
    reasoningEffort?: string;
    model?: string;
}
class ChunkQueue {
    private readonly chunks: StreamChunk[] = [];
    private waiter: (() => void) | undefined;
    private ended = false;
    push(chunk: StreamChunk) {
        if (this.ended)
            return;
        this.chunks.push(chunk);
        if (TERMINAL.has(chunk.type))
            this.ended = true;
        this.waiter?.();
        this.waiter = undefined;
    }
    async *read(): AsyncGenerator<StreamChunk> {
        while (!this.ended || this.chunks.length > 0) {
            if (this.chunks.length === 0) {
                await new Promise<void>(resolve => { this.waiter = resolve; });
                continue;
            }
            yield this.chunks.shift()!;
        }
    }
}
export class ManagedHttpDataSource extends HttpDataSource {
    private readonly owner: AbortController;
    private readonly subscriptions = new Map<StreamSubscription, () => void>();
    constructor(private readonly managedConfig: HttpDataSourceConfig, private readonly session: SessionService) {
        const owner = new AbortController();
        const transport = managedConfig.fetcher ?? fetch;
        super({ ...managedConfig, fetcher: (input, init) => transport(input, { ...init, signal: init?.signal ? AbortSignal.any([owner.signal, init.signal]) : owner.signal }) });
        this.owner = owner;
    }
    override async *sendMessage(sessionId: string, content: string, attachments: Attachment[] = [], signal?: AbortSignal, options?: SendMessageOptions): AsyncGenerator<StreamChunk> {
        const uploads = await Promise.all(attachments.map(attachment => this.uploadReference(attachment, signal)));
        if (this.owner.signal.aborted) throw new DOMException('Disposed', 'AbortError');
        const payload = {
            sessionId,
            content,
            debugMode: options?.debugMode ?? false,
            replaceFromMessageId: options?.replaceFromMessageID,
            attachments: uploads,
            requestId: options?.requestId ?? crypto.randomUUID(),
            ...(options?.reasoningEffort ? { reasoningEffort: options.reasoningEffort } : {}),
            ...(options?.model ? { model: options.model } : {}),
        };
        const queue = new ChunkQueue();
        const subscription = this.openPOSTStream('', payload, queue);
        const abort = () => {
            subscription.close();
            queue.push({ type: 'error', error: 'Stream cancelled' });
        };
        this.subscriptions.set(subscription, abort);
        signal?.addEventListener('abort', abort, { once: true });
        if (signal?.aborted)
            abort();
        try {
            yield* queue.read();
        }
        finally {
            signal?.removeEventListener('abort', abort);
            subscription.close();
            this.subscriptions.delete(subscription);
        }
    }
    override async resumeStream(sessionId: string, runId: string, onChunk: (chunk: StreamChunk) => void, signal?: AbortSignal): Promise<void> {
        if (this.owner.signal.aborted) return;
        await new Promise<void>(resolve => {
            let settled = false;
            let subscription: StreamSubscription | undefined;
            const finish = () => {
                if (settled)
                    return;
                settled = true;
                subscription?.close();
                if (subscription) this.subscriptions.delete(subscription);
                signal?.removeEventListener('abort', finish);
                resolve();
            };
            subscription = this.openEventStream(sessionId, runId, {
                push: chunk => {
                    onChunk(chunk);
                    if (TERMINAL.has(chunk.type))
                        finish();
                },
            });
            if (settled) subscription.close(); else this.subscriptions.set(subscription, finish);
            signal?.addEventListener('abort', finish, { once: true });
            if (signal?.aborted)
                finish();
        });
    }
    dispose(): void { this.owner.abort(); this.cancelStream(); }
    override cancelStream(): void {
        for (const cancel of this.subscriptions.values())
            cancel();
        this.subscriptions.clear();
        super.cancelStream();
    }
    private openPOSTStream(suffix: string, payload: unknown, sink: {
        push(chunk: StreamChunk): void;
    }): StreamSubscription {
        return this.openManagedStream({
            id: `bichat:${String((payload as {
                sessionId?: unknown;
            }).sessionId ?? 'stream')}:${suffix || 'send'}`,
            url: `${this.managedConfig.streamEndpoint ?? '/admin/ali/chat/stream'}${suffix}`,
            method: 'POST',
            body: () => JSON.stringify(payload),
            headers: { 'content-type': 'application/json' },
            terminal: event => TERMINAL.has(event.type),
        }, sink);
    }
    private openEventStream(sessionId: string, runId: string, sink: {
        push(chunk: StreamChunk): void;
    }): StreamSubscription {
        const query = new URLSearchParams({ sessionId, runId });
        return this.openManagedStream({
            id: `bichat:${sessionId}:events:${runId}`,
            url: `${this.managedConfig.streamEndpoint ?? '/admin/ali/chat/stream'}/events?${query}`,
            method: 'GET',
            terminal: event => TERMINAL.has(event.type),
        }, sink);
    }
    private openManagedStream(contract: StreamContract<StreamChunk>, sink: {
        push(chunk: StreamChunk): void;
    }): StreamSubscription {
        let terminalDelivered = false;
        const deliver = (chunk: StreamChunk) => {
            const eventType = String(chunk.type);
            if (this.owner.signal.aborted || eventType === 'ping' || terminalDelivered)
                return;
            const terminal = TERMINAL.has(eventType);
            terminalDelivered = terminalDelivered || terminal;
            sink.push(eventType === 'cancelled' || eventType === 'failed'
                ? { ...chunk, type: 'error', error: chunk.error || `Stream ${eventType}` }
                : chunk);
        };
        return subscribeManagedStream<StreamChunk>(contract, deliver, {
            session: this.session,
            staleAfterMs: this.managedConfig.streamConnectTimeoutMs ?? 45000,
            parse: (data, event) => {
                const parsed = JSON.parse(data) as StreamChunk;
                return { ...parsed, type: parsed.type || event };
            },
            diagnostics: {
                event: event => {
                    if (event.state === 'error')
                        deliver({ type: 'error', error: event.error?.message || 'Stream failed', errorSource: event.error?.retryable ? 'transport' : undefined });
                },
            },
            errors: {
                unauthenticated: error => deliver({ type: 'error', error: error.message }),
                permissionDenied: error => deliver({ type: 'error', error: error.message }),
            },
        });
    }
    private async uploadReference(attachment: Attachment, signal?: AbortSignal): Promise<{
        uploadId: number;
    }> {
        if (typeof attachment.uploadId === 'number' && attachment.uploadId > 0) {
            return { uploadId: attachment.uploadId };
        }
        const source = attachment.base64Data
            ? (attachment.base64Data.startsWith('data:')
                ? attachment.base64Data
                : `data:${attachment.mimeType || 'application/octet-stream'};base64,${attachment.base64Data}`)
            : attachment.url;
        if (!source)
            throw new Error(`Attachment "${attachment.filename}" has no uploadable data`);
        const ownedSignal = signal ? AbortSignal.any([this.owner.signal, signal]) : this.owner.signal;
        const blobResponse = await fetch(source, { signal: ownedSignal });
        if (!blobResponse.ok)
            throw new Error(`Attachment decode failed: HTTP ${blobResponse.status}`);
        const blob = await blobResponse.blob();
        const form = new FormData();
        form.append('file', new File([blob], attachment.filename, { type: attachment.mimeType || blob.type }));
        const session = this.session.snapshot();
        const response = await fetch(this.managedConfig.uploadEndpoint ?? '/api/uploads', {
            method: 'POST', credentials: 'same-origin', signal: ownedSignal, body: form,
            headers: {
                ...(session.csrf ? { 'x-csrf-token': session.csrf } : {}),
                ...session.headers,
            },
        });
        const result = await response.json() as {
            id?: number;
            error?: string;
        };
        if (!response.ok || !result.id)
            throw new Error(result.error || `Upload failed: HTTP ${response.status}`);
        return { uploadId: result.id };
    }
}
