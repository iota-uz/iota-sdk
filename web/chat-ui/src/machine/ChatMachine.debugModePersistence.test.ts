import { beforeEach, describe, expect, it } from 'vitest';
import type { Attachment, ChatDataSource, SendMessageOptions, Session, StreamChunk } from '../types';
import type { RateLimiter } from '../utils/RateLimiter';
import { ChatMachine } from './ChatMachine';

const SESSION_ID = 'session-1';

function makeSession(id: string): Session {
  const now = new Date().toISOString();
  return {
    id,
    title: 'Test session',
    status: 'active',
    pinned: false,
    createdAt: now,
    updatedAt: now,
  };
}

function createMemorySessionStorage(): Storage {
  const store = new Map<string, string>();
  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key: string) {
      store.delete(key);
    },
    setItem(key: string, value: string) {
      store.set(key, value);
    },
  };
}

function installWindowWithSessionStorage(): void {
  Object.defineProperty(globalThis, 'window', {
    value: {
      sessionStorage: createMemorySessionStorage(),
      dispatchEvent: () => true,
    },
    configurable: true,
    writable: true,
  });
}

function createDataSource(sendOptions: Array<SendMessageOptions | undefined>): ChatDataSource {
  const session = makeSession(SESSION_ID);

  return {
    createSession: async () => session,
    fetchSession: async (id: string) => ({
      session: makeSession(id),
      turns: [],
      pendingQuestion: null,
    }),
    sendMessage: async function* (
      _sessionId: string,
      _content: string,
      _attachments: Attachment[] = [],
      _signal?: AbortSignal,
      options?: SendMessageOptions
    ): AsyncGenerator<StreamChunk> {
      sendOptions.push(options);
      yield { type: 'done', sessionId: SESSION_ID };
    },
  } as unknown as ChatDataSource;
}

describe('ChatMachine debug mode persistence', () => {
  beforeEach(() => {
    installWindowWithSessionStorage();
  });

  it('preserves debug mode after creating a new session and remounting', async () => {
    const sendOptions: Array<SendMessageOptions | undefined> = [];
    const rateLimiter = {
      canMakeRequest: () => true,
      getTimeUntilNextRequest: () => 0,
    } as unknown as RateLimiter;
    const dataSource = createDataSource(sendOptions);

    const firstMachine = new ChatMachine({
      dataSource,
      rateLimiter,
      onSessionCreated: () => {},
    });

    firstMachine.setSessionId('new');
    await firstMachine.sendMessage('/debug');
    expect(firstMachine.getSessionSnapshot().debugMode).toBe(true);

    await firstMachine.sendMessage('Hello');
    expect(sendOptions).toHaveLength(1);
    expect(sendOptions[0]?.debugMode).toBe(true);

    const remountedMachine = new ChatMachine({ dataSource, rateLimiter });
    remountedMachine.setSessionId(SESSION_ID);

    expect(remountedMachine.getSessionSnapshot().debugMode).toBe(true);
  });

  it('persists explicit debug disable across remounts', async () => {
    const sendOptions: Array<SendMessageOptions | undefined> = [];
    const rateLimiter = {
      canMakeRequest: () => true,
      getTimeUntilNextRequest: () => 0,
    } as unknown as RateLimiter;
    const dataSource = createDataSource(sendOptions);

    const firstMachine = new ChatMachine({ dataSource, rateLimiter });
    firstMachine.setSessionId(SESSION_ID);

    await firstMachine.sendMessage('/debug');
    expect(firstMachine.getSessionSnapshot().debugMode).toBe(true);
    await firstMachine.sendMessage('/debug');
    expect(firstMachine.getSessionSnapshot().debugMode).toBe(false);

    const remountedMachine = new ChatMachine({ dataSource, rateLimiter });
    remountedMachine.setSessionId(SESSION_ID);
    expect(remountedMachine.getSessionSnapshot().debugMode).toBe(false);
  });
});
