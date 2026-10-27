import { beforeEach, describe, expect, it } from 'vitest';
import type {
  Attachment,
  ChatDataSource,
  PendingQuestion,
  SendMessageOptions,
  Session,
  StreamChunk,
} from '../types';
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

function makePendingQuestion(status: PendingQuestion['status']): PendingQuestion {
  return {
    id: 'checkpoint-1',
    turnId: 'turn-1',
    status,
    questions: [
      {
        id: 'scope',
        text: 'Scope?',
        type: 'SINGLE_CHOICE',
        options: [
          { id: 'all', label: 'All', value: 'all' },
        ],
      },
    ],
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

function createDataSource(
  pendingQuestion: PendingQuestion,
  sendOptions: Array<SendMessageOptions | undefined>,
): ChatDataSource {
  return {
    createSession: async () => makeSession(SESSION_ID),
    fetchSession: async (id: string) => ({
      session: makeSession(id),
      turns: [],
      pendingQuestion,
    }),
    sendMessage: async function* (
      _sessionId: string,
      _content: string,
      _attachments: Attachment[] = [],
      _signal?: AbortSignal,
      options?: SendMessageOptions,
    ): AsyncGenerator<StreamChunk> {
      sendOptions.push(options);
      yield { type: 'done', sessionId: SESSION_ID };
    },
  } as unknown as ChatDataSource;
}

async function flushAsyncWork(): Promise<void> {
  await Promise.resolve();
  await new Promise((resolve) => setTimeout(resolve, 0));
}

describe('ChatMachine pending question guard', () => {
  beforeEach(() => {
    installWindowWithSessionStorage();
  });

  it('does not send a new message while a clarification is still open', async () => {
    const sendOptions: Array<SendMessageOptions | undefined> = [];
    const rateLimiter = {
      canMakeRequest: () => true,
      getTimeUntilNextRequest: () => 0,
    } as unknown as RateLimiter;
    const dataSource = createDataSource(makePendingQuestion('PENDING'), sendOptions);

    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);
    await flushAsyncWork();

    await machine.sendMessage('continue');

    expect(sendOptions).toHaveLength(0);
  });

  it('keeps the guard active while answer submission is still resuming', async () => {
    const sendOptions: Array<SendMessageOptions | undefined> = [];
    const rateLimiter = {
      canMakeRequest: () => true,
      getTimeUntilNextRequest: () => 0,
    } as unknown as RateLimiter;
    const dataSource = createDataSource(makePendingQuestion('ANSWER_SUBMITTED'), sendOptions);

    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);
    await flushAsyncWork();

    await machine.sendMessage('continue');

    expect(sendOptions).toHaveLength(0);
  });
});
