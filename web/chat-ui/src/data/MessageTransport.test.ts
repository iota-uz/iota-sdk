import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  resumeStream,
  RunEventsConnectError,
  sendMessage,
  subscribeRunEvents,
  submitQuestionAnswers,
  type MessageTransportDeps,
} from './MessageTransport';
import type { StreamChunk } from '../types';

const encoder = new TextEncoder();

function createSSEStream(events: Array<Record<string, unknown>>): ReadableStream<Uint8Array> {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const event of events) {
        controller.enqueue(encoder.encode(`data: ${JSON.stringify(event)}\n\n`));
      }
      controller.close();
    },
  });
}

function createDeps(overrides: Partial<MessageTransportDeps> = {}): MessageTransportDeps {
  return {
    callRPC: vi.fn(async () => {
      throw new Error('callRPC should not be used in this test');
    }),
    baseUrl: '',
    streamEndpoint: '/stream',
    rpcTimeoutMs: 120_000,
    createHeaders: () => new Headers(),
    uploadFileFn: vi.fn(async () => {
      throw new Error('uploadFileFn should not be used in this test');
    }),
    logAttachmentLifecycle: vi.fn(),
    ...overrides,
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('MessageTransport stream timeout defaults', () => {
  it('does not schedule a send timeout when streamConnectTimeoutMs is omitted', async () => {
    const fetchMock = vi.fn(async () => new Response(
      createSSEStream([{ type: 'done' }]),
      { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
    ));
    vi.stubGlobal('fetch', fetchMock);
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');

    const chunks = [];
    for await (const chunk of sendMessage(
      createDeps(),
      'session-1',
      'hello world',
      [],
    )) {
      chunks.push(chunk);
    }

    expect(chunks).toEqual([{ type: 'done' }]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(setTimeoutSpy).not.toHaveBeenCalled();
  });

  it('does not schedule a resume timeout when connectTimeoutMs is omitted', async () => {
    const fetchMock = vi.fn(async () => new Response(
      createSSEStream([{ type: 'done' }]),
      { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
    ));
    vi.stubGlobal('fetch', fetchMock);
    const setTimeoutSpy = vi.spyOn(globalThis, 'setTimeout');

    const chunks: Array<{ type?: string }> = [];
    await resumeStream(
      {
        baseUrl: '',
        streamEndpoint: '/stream',
        createHeaders: () => new Headers(),
      },
      'session-1',
      'run-1',
      (chunk) => {
        chunks.push({ type: chunk.type });
      },
    );

    expect(chunks).toEqual([{ type: 'done' }]);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(setTimeoutSpy).not.toHaveBeenCalled();
  });
});

describe('sendMessage request_id idempotency', () => {
  it('includes an auto-generated requestId in the POST body', async () => {
    const fetchMock = vi.fn(async () => new Response(
      createSSEStream([{ type: 'done' }]),
      { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
    ));
    vi.stubGlobal('fetch', fetchMock);

    for await (const _ of sendMessage(createDeps(), 'session-1', 'hello', [])) {
      // drain stream
    }

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const firstCall = fetchMock.mock.calls[0] as unknown as [unknown, RequestInit];
    const body = JSON.parse(firstCall[1].body as string);
    expect(typeof body.requestId).toBe('string');
    expect(body.requestId.length).toBeGreaterThan(0);
  });

  it('preserves a caller-supplied requestId verbatim', async () => {
    const fetchMock = vi.fn(async () => new Response(
      createSSEStream([{ type: 'done' }]),
      { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
    ));
    vi.stubGlobal('fetch', fetchMock);

    const explicit = '11111111-2222-4333-8444-555566667777';
    for await (const _ of sendMessage(
      createDeps(),
      'session-1',
      'hello',
      [],
      undefined,
      { requestId: explicit },
    )) {
      // drain
    }

    const firstCall = fetchMock.mock.calls[0] as unknown as [unknown, RequestInit];
    const body = JSON.parse(firstCall[1].body as string);
    expect(body.requestId).toBe(explicit);
  });
});

// ---------------------------------------------------------------------------
// subscribeRunEvents — EventSource listener regression coverage
// ---------------------------------------------------------------------------

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners = new Map<string, Set<(e: MessageEvent) => void>>();
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  readyState = 0;
  closed = false;
  constructor(public url: string, public init?: EventSourceInit) {
    FakeEventSource.instances.push(this);
  }
  addEventListener(name: string, cb: (e: MessageEvent) => void) {
    if (!this.listeners.has(name)) {this.listeners.set(name, new Set());}
    this.listeners.get(name)!.add(cb);
  }
  removeEventListener(name: string, cb: (e: MessageEvent) => void) {
    this.listeners.get(name)?.delete(cb);
  }
  close() {
    this.readyState = 2;
    this.closed = true;
  }
  emit(name: string, data: unknown) {
    const payload = typeof data === 'string' ? data : JSON.stringify(data);
    const evt = new MessageEvent(name, { data: payload });
    for (const cb of this.listeners.get(name) ?? []) {cb(evt);}
  }
  emitError() {
    this.onerror?.(new Event('error'));
  }
}

function installFakeEventSource(): typeof FakeEventSource {
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
  return FakeEventSource;
}

describe('subscribeRunEvents — terminal event handling', () => {
  it('settles when backend emits a `cancelled` event (regression)', async () => {
    installFakeEventSource();
    const chunks: StreamChunk[] = [];
    const promise = subscribeRunEvents(
      { baseUrl: '', streamEndpoint: '/stream' },
      'session-1',
      'run-1',
      { onChunk: (c) => chunks.push(c) },
    );
    // Flush microtasks so the constructor + addEventListener calls
    // complete before we emit.
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    expect(es).toBeDefined();
    es.emit('cancelled', { type: 'cancelled', reason: 'user_stop' });
    await expect(promise).resolves.toBeUndefined();
    expect(es.closed).toBe(true);
    expect(chunks).toEqual([{ type: 'cancelled', reason: 'user_stop' }]);
  });

  it('settles when backend emits a `failed` event (regression)', async () => {
    installFakeEventSource();
    const chunks: StreamChunk[] = [];
    const promise = subscribeRunEvents(
      { baseUrl: '', streamEndpoint: '/stream' },
      'session-1',
      'run-1',
      { onChunk: (c) => chunks.push(c) },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    es.emit('failed', { type: 'failed', error: 'reaper_stale' });
    await expect(promise).resolves.toBeUndefined();
    expect(es.closed).toBe(true);
    expect(chunks).toHaveLength(1);
    expect(chunks[0].type).toBe('failed');
  });

  it('surfaces malformed JSON as an error chunk without tearing down the subscription', async () => {
    installFakeEventSource();
    const chunks: StreamChunk[] = [];
    const promise = subscribeRunEvents(
      { baseUrl: '', streamEndpoint: '/stream' },
      'session-1',
      'run-1',
      { onChunk: (c) => chunks.push(c) },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    es.emit('content', '{bad');
    // Subscription remains open — the first chunk was an injected error
    // chunk. A subsequent valid `done` settles the promise.
    expect(es.closed).toBe(false);
    es.emit('done', { type: 'done' });
    await expect(promise).resolves.toBeUndefined();
    expect(es.closed).toBe(true);
    expect(chunks).toHaveLength(2);
    expect(chunks[0].type).toBe('error');
    expect(chunks[1]).toEqual({ type: 'done' });
  });

  it('resolves and closes EventSource when the AbortSignal fires', async () => {
    installFakeEventSource();
    const ctrl = new AbortController();
    const promise = subscribeRunEvents(
      { baseUrl: '', streamEndpoint: '/stream' },
      'session-1',
      'run-1',
      { onChunk: () => {}, signal: ctrl.signal },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    expect(es.closed).toBe(false);
    ctrl.abort();
    await expect(promise).resolves.toBeUndefined();
    expect(es.closed).toBe(true);
  });

  it('rejects with RunEventsConnectError when onerror fires within the initial-connect grace', async () => {
    installFakeEventSource();
    const promise = subscribeRunEvents(
      { baseUrl: '', streamEndpoint: '/stream' },
      'session-1',
      'run-1',
      { onChunk: () => {} },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    es.emitError();
    await expect(promise).rejects.toBeInstanceOf(RunEventsConnectError);
    expect(es.closed).toBe(true);
  });
});

describe('sendMessage request_id fallback', () => {
  it('produces a valid UUID v4 when crypto.randomUUID is unavailable', async () => {
    const fetchMock = vi.fn(async () => new Response(
      createSSEStream([{ type: 'done' }]),
      { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
    ));
    vi.stubGlobal('fetch', fetchMock);
    // Stub crypto with a shape that is defined but lacks randomUUID —
    // forces the Math.random fallback path in generateRequestId.
    vi.stubGlobal('crypto', { randomUUID: undefined } as unknown as Crypto);

    for await (const _ of sendMessage(createDeps(), 'session-1', 'hello', [])) {
      // drain
    }

    const firstCall = fetchMock.mock.calls[0] as unknown as [unknown, RequestInit];
    const body = JSON.parse(firstCall[1].body as string);
    expect(body.requestId).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
  });
});

describe('submitQuestionAnswers', () => {
  it('flattens custom text answers for RPC submission', async () => {
    const callRPC = vi.fn(async () => ({
      accepted: true,
      operation: 'question_submit',
      sessionId: 'session-1',
      runId: 'run-1',
      startedAt: 123,
    }));

    const result = await submitQuestionAnswers(
      callRPC,
      'session-1',
      'checkpoint-1',
      {
        period: { options: [], customText: 'Show quarters for last year' },
        slice: { options: ['all'] },
      },
    );

    expect(callRPC).toHaveBeenCalledWith('bichat.question.submit', {
      sessionId: 'session-1',
      checkpointId: 'checkpoint-1',
      answers: {
        period: 'Show quarters for last year',
        slice: 'all',
      },
    });
    expect(result).toEqual({
      success: true,
      data: {
        accepted: true,
        operation: 'question_submit',
        sessionId: 'session-1',
        runId: 'run-1',
        startedAt: 123,
      },
    });
  });
});

describe('MessageTransport abort vs timeout classification', () => {
  it('re-throws an AbortError (user/unmount cancel) instead of yielding an error chunk', async () => {
    // A user stop / provider unmount aborts the fetch. The generator must let
    // the AbortError propagate so the machine takes its soft-cancel path; a
    // yielded error chunk would be re-thrown downstream as a generic Error and
    // discard the turn (the bug fixed here).
    const abortError = new Error('The operation was aborted');
    abortError.name = 'AbortError';
    const fetchMock = vi.fn(async () => {
      throw abortError;
    });
    vi.stubGlobal('fetch', fetchMock);

    const chunks: unknown[] = [];
    const iterate = (async () => {
      for await (const chunk of sendMessage(createDeps(), 'session-1', 'hi', [])) {
        chunks.push(chunk);
      }
    })();

    await expect(iterate).rejects.toMatchObject({ name: 'AbortError' });
    expect(chunks).toEqual([]);
  });

  it('yields a timeout error chunk when the connection times out', async () => {
    vi.useFakeTimers();
    try {
      const fetchMock = vi.fn(
        (_url: string, init: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init.signal?.addEventListener('abort', () => {
              const e = new Error('aborted');
              e.name = 'AbortError';
              reject(e);
            });
          }),
      );
      vi.stubGlobal('fetch', fetchMock);

      const chunks: Array<{ type?: string; error?: string }> = [];
      const iterate = (async () => {
        for await (const chunk of sendMessage(
          createDeps({ streamConnectTimeoutMs: 50 }),
          'session-1',
          'hi',
          [],
        )) {
          chunks.push(chunk as { type?: string; error?: string });
        }
      })();

      await vi.advanceTimersByTimeAsync(60);
      await iterate;

      expect(chunks).toHaveLength(1);
      expect(chunks[0]?.type).toBe('error');
      expect(chunks[0]?.error).toContain('timed out');
    } finally {
      vi.useRealTimers();
    }
  });
});
