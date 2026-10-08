import { afterEach, describe, expect, it, vi } from 'vitest';
import { subscribeActiveRuns } from './MessageTransport';
import type { ActiveRunDelivery } from '../types';

// Standalone fake EventSource duplicated from MessageTransport.test.ts
// so these two files can be moved independently; both exercise the
// native listener contract and both are purely local.
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners = new Map<string, Set<(e: MessageEvent) => void>>();
  onerror: ((e: Event) => void) | null = null;
  readyState = 0;
  closed = false;
  constructor(public url: string, public init?: EventSourceInit) {
    FakeEventSource.instances.push(this);
  }
  addEventListener(name: string, cb: (e: MessageEvent) => void) {
    if (!this.listeners.has(name)) {this.listeners.set(name, new Set());}
    this.listeners.get(name)!.add(cb);
  }
  removeEventListener() {}
  close() {
    this.readyState = 2;
    this.closed = true;
  }
  emit(name: string, data: unknown) {
    const payload = typeof data === 'string' ? data : JSON.stringify(data);
    const evt = new MessageEvent(name, { data: payload });
    for (const cb of this.listeners.get(name) ?? []) {cb(evt);}
  }
}

afterEach(() => {
  vi.restoreAllMocks();
});

function install(): typeof FakeEventSource {
  FakeEventSource.instances = [];
  vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
  return FakeEventSource;
}

describe('subscribeActiveRuns', () => {
  it('surfaces snapshot then update events through onEvent', async () => {
    install();
    const events: ActiveRunDelivery[] = [];
    const ctrl = new AbortController();
    const promise = subscribeActiveRuns(
      { baseUrl: '', streamEndpoint: '/stream' },
      {
        onEvent: (e) => events.push(e),
        signal: ctrl.signal,
      },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    es.emit('snapshot', {
      sessionId: 'sess-1', runId: 'run-1', status: 'streaming', updatedAt: 1,
    });
    es.emit('snapshot', {
      sessionId: 'sess-2', runId: 'run-2', status: 'queued', updatedAt: 2,
    });
    es.emit('update', {
      sessionId: 'sess-1', runId: 'run-1', status: 'completed', updatedAt: 3,
    });
    ctrl.abort();
    await promise;
    expect(events).toEqual([
      { event: 'snapshot', sessionId: 'sess-1', runId: 'run-1', status: 'streaming', updatedAt: 1 },
      { event: 'snapshot', sessionId: 'sess-2', runId: 'run-2', status: 'queued', updatedAt: 2 },
      { event: 'update', sessionId: 'sess-1', runId: 'run-1', status: 'completed', updatedAt: 3 },
    ]);
  });

  it('swallows malformed payloads silently', async () => {
    install();
    const events: ActiveRunDelivery[] = [];
    const ctrl = new AbortController();
    const promise = subscribeActiveRuns(
      { baseUrl: '', streamEndpoint: '/stream' },
      {
        onEvent: (e) => events.push(e),
        signal: ctrl.signal,
      },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    es.emit('snapshot', '{not-json');
    es.emit('update', {
      sessionId: 'sess-1', runId: 'run-1', status: 'streaming', updatedAt: 42,
    });
    ctrl.abort();
    await promise;
    expect(events).toEqual([
      { event: 'update', sessionId: 'sess-1', runId: 'run-1', status: 'streaming', updatedAt: 42 },
    ]);
  });

  it('resolves and closes EventSource when the AbortSignal fires', async () => {
    install();
    const ctrl = new AbortController();
    const promise = subscribeActiveRuns(
      { baseUrl: '', streamEndpoint: '/stream' },
      { onEvent: () => {}, signal: ctrl.signal },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    expect(es.closed).toBe(false);
    ctrl.abort();
    await expect(promise).resolves.toBeUndefined();
    expect(es.closed).toBe(true);
  });

  it('forwards errors through options.onError', async () => {
    install();
    const errors: Event[] = [];
    const ctrl = new AbortController();
    const promise = subscribeActiveRuns(
      { baseUrl: '', streamEndpoint: '/stream' },
      {
        onEvent: () => {},
        onError: (evt) => errors.push(evt),
        signal: ctrl.signal,
      },
    );
    await Promise.resolve();
    const es = FakeEventSource.instances[0];
    // Emit one event first so the onerror we trigger below is classed
    // as a transient flap (post-grace) rather than an initial-connect
    // boundary failure.
    es.emit('snapshot', {
      sessionId: 'sess-1', runId: 'run-1', status: 'streaming', updatedAt: 1,
    });
    es.onerror?.(new Event('error'));
    expect(errors).toHaveLength(1);
    ctrl.abort();
    await promise;
  });
});
