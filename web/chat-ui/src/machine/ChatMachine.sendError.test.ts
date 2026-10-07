import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  Attachment,
  ChatDataSource,
  SendMessageOptions,
  Session,
  StreamChunk,
} from "../types";
import type { RateLimiter } from "../utils/RateLimiter";
import { ChatMachine } from "./ChatMachine";

const SESSION_ID = "session-1";

function makeSession(id: string): Session {
  const now = new Date().toISOString();
  return {
    id,
    title: "Test session",
    status: "active",
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
  Object.defineProperty(globalThis, "window", {
    value: {
      sessionStorage: createMemorySessionStorage(),
      dispatchEvent: () => true,
    },
    configurable: true,
    writable: true,
  });
}

const rateLimiter = {
  canMakeRequest: () => true,
  getTimeUntilNextRequest: () => 0,
} as unknown as RateLimiter;

function makeDataSource(
  sendImpl: (
    sessionId: string,
    content: string,
    attachments?: Attachment[],
    signal?: AbortSignal,
    options?: SendMessageOptions,
  ) => AsyncGenerator<StreamChunk>,
): ChatDataSource {
  return {
    createSession: async () => makeSession(SESSION_ID),
    fetchSession: async (id: string) => ({
      session: makeSession(id),
      turns: [],
      pendingQuestion: null,
    }),
    sendMessage: sendImpl,
  } as unknown as ChatDataSource;
}

describe("ChatMachine send error handling", () => {
  beforeEach(() => {
    installWindowWithSessionStorage();
  });

  it("keeps the resume marker when unmount closes the subscription without a terminal event", async () => {
    // False-green guard: the adapter resolves on abort; a rejecting mock would
    // miss the success-path marker deletion that breaks reload reconnection.
    let started = false;
    const dataSource = {
      fetchSession: async () => ({
        session: makeSession(SESSION_ID), turns: [], pendingQuestion: null,
      }),
      getStreamStatus: async () => ({ active: false }),
      resumeStream: async (
        _session: string,
        _run: string,
        _onChunk: (chunk: StreamChunk) => void,
        signal: AbortSignal,
      ) => new Promise<void>(resolve => {
        started = true;
        signal.addEventListener("abort", () => resolve(), { once: true });
      }),
    } as unknown as ChatDataSource;
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);
    const resumed = (
      machine as unknown as {
        _resumeAcceptedRunOrPoll(session: string, run: string): Promise<void>;
      }
    )._resumeAcceptedRunOrPoll(SESSION_ID, "run-1");
    await vi.waitFor(() => expect(started).toBe(true));
    machine.dispose();
    await resumed;
    expect(window.sessionStorage.getItem(`bichat.run.${SESSION_ID}`)).toBe("run-1");
  });

  it.each([
    { transport: true, newAnswer: true, clearsError: true, poll: false },
    { transport: true, newAnswer: false, clearsError: false, poll: false },
    { transport: false, newAnswer: true, clearsError: false, poll: false },
    { transport: true, newAnswer: true, clearsError: true, poll: true },
  ])(
    "clears only a recovered transport failure: %j",
    async ({ transport, newAnswer, clearsError, poll }) => {
      // False-green guard: an old answer or a server generation error must not
      // be mistaken for successful recovery of this run.
      let recovered = false;
      let statusReads = 0;
      let fetches = 0;
      const now = new Date().toISOString();
      const dataSource = {
        fetchSession: async () => {
          fetches++;
          return {
            session: makeSession(SESSION_ID),
            pendingQuestion: null,
            turns: [
              {
                id: "turn-1",
                sessionId: SESSION_ID,
                createdAt: now,
                userTurn: {
                  id: "user-1",
                  content: "Analyze",
                  attachments: [],
                  createdAt: now,
                },
                assistantTurn: {
                  id:
                    recovered && newAnswer
                      ? "resumed-answer"
                      : "previous-answer",
                  role: "assistant",
                  content: "Saved answer",
                  lifecycle: "complete",
                  citations: [],
                  artifacts: [],
                  codeOutputs: [],
                  createdAt: now,
                },
              },
            ],
          };
        },
        getStreamStatus: async () => ({
          active: recovered && poll && statusReads++ === 0,
        }),
        resumeStream: async (
          _session: string,
          _run: string,
          onChunk: (chunk: StreamChunk) => void,
        ) => {
          recovered = true;
          onChunk({
            type: "error",
            error: "delivery failed",
            ...(transport ? { errorSource: "transport" as const } : {}),
          });
        },
      } as unknown as ChatDataSource;
      const machine = new ChatMachine({ dataSource, rateLimiter });
      try {
        machine.setSessionId(SESSION_ID);
        await vi.waitFor(() =>
          expect(machine.getMessagingSnapshot().turns).toHaveLength(1),
        );
        await (
          machine as unknown as {
            _resumeAcceptedRunOrPoll(
              session: string,
              run: string,
            ): Promise<void>;
          }
        )._resumeAcceptedRunOrPoll(SESSION_ID, "run-1");
        await vi.waitFor(() => expect(fetches).toBeGreaterThan(1), {
          timeout: 4000,
        });
        expect(machine.getMessagingSnapshot().streamError === null).toBe(
          clearsError,
        );
        expect(
          machine.getMessagingSnapshot().turns.at(-1)?.assistantTurn?.content,
        ).toBe("Saved answer");
      } finally {
        machine.dispose();
      }
    },
  );

  it("keeps the user turn in history and shows a retryable banner on a genuine stream error", async () => {
    // The answer streams partially, then a terminal error arrives. The send
    // must fail non-destructively: the optimistic user turn STAYS in history
    // (so the conversation view persists and a new chat never collapses back to
    // the welcome screen), a retryable banner is shown, and the prompt lives in
    // the turn — so the input is cleared rather than re-populated.
    const dataSource = makeDataSource(async function* () {
      yield { type: "content", content: "Partial answer" } as StreamChunk;
      yield { type: "error", error: "boom" } as StreamChunk;
    });
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);

    await machine.sendMessage("Hello world");

    const input = machine.getInputSnapshot();
    const messaging = machine.getMessagingSnapshot();
    expect(input.message).toBe("");
    expect(messaging.streamError).toBeTruthy();
    expect(messaging.streamErrorRetryable).toBe(true);
    expect(messaging.turns).toHaveLength(1);
    expect(messaging.turns[0].userTurn.content).toBe("Hello world");
    expect(messaging.turns[0].assistantTurn?.content).toBe("Partial answer");
    expect(messaging.streamingContent).toBe("");
  });

  it("keeps streamed assistant content when the transport fails before a terminal chunk", async () => {
    const dataSource = makeDataSource(async function* () {
      yield { type: "content", content: "Nearly complete " } as StreamChunk;
      yield { type: "content", content: "answer" } as StreamChunk;
      throw new Error("connection lost");
    });
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);

    await machine.sendMessage("Analyze this");

    const messaging = machine.getMessagingSnapshot();
    expect(messaging.streamError).toBeTruthy();
    expect(messaging.turns[0].assistantTurn?.content).toBe(
      "Nearly complete answer",
    );
    expect(messaging.streamingContent).toBe("");
  });

  it("keeps a resumed snapshot when the resumed stream ends with an error", async () => {
    const now = new Date().toISOString();
    const dataSource = {
      fetchSession: async () => ({
        session: makeSession(SESSION_ID),
        turns: [
          {
            id: "turn-1",
            sessionId: SESSION_ID,
            userTurn: {
              id: "user-1",
              content: "Analyze this",
              attachments: [],
              createdAt: now,
            },
            createdAt: now,
          },
        ],
        pendingQuestion: null,
      }),
      resumeStream: async (
        _sessionId: string,
        _runId: string,
        onChunk: (chunk: StreamChunk) => void,
      ) => {
        onChunk({
          type: "snapshot",
          snapshot: { partialContent: "Recovered answer" },
        } as StreamChunk);
        onChunk({ type: "error", error: "persist failed" } as StreamChunk);
      },
    } as unknown as ChatDataSource;
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);
    await vi.waitFor(() => {
      expect(machine.getMessagingSnapshot().turns).toHaveLength(1);
    });

    const resumableMachine = machine as unknown as {
      _runResumeStream: (sessionId: string, runId: string) => Promise<void>;
    };
    await expect(
      resumableMachine._runResumeStream(SESSION_ID, "run-1"),
    ).rejects.toThrow("persist failed");

    const messaging = machine.getMessagingSnapshot();
    expect(messaging.streamError).toBeTruthy();
    expect(messaging.turns[0].assistantTurn?.content).toBe("Recovered answer");
    expect(messaging.streamingContent).toBe("");
  });

  it("keeps the local partial answer when the server only has the user turn", async () => {
    let fetches = 0;
    const now = new Date().toISOString();
    const dataSource = {
      createSession: async () => makeSession(SESSION_ID),
      fetchSession: async () => {
        fetches++;
        return {
          session: makeSession(SESSION_ID),
          turns:
            fetches === 1
              ? []
              : [
                  {
                    id: "server-turn",
                    sessionId: SESSION_ID,
                    userTurn: {
                      id: "server-user",
                      content: "Analyze this",
                      attachments: [],
                      createdAt: now,
                    },
                    createdAt: now,
                  },
                ],
          pendingQuestion: null,
        };
      },
      sendMessage: async function* () {
        yield { type: "content", content: "Partial answer" } as StreamChunk;
        yield { type: "error", error: "persist failed" } as StreamChunk;
      },
    } as unknown as ChatDataSource;
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);
    await vi.waitFor(() => {
      expect(fetches).toBe(1);
    });
    await machine.sendMessage("Analyze this");

    const syncableMachine = machine as unknown as {
      _syncSessionFromServer: (
        sessionId: string,
        allowEmptyTurns: boolean,
      ) => Promise<void>;
    };
    await syncableMachine._syncSessionFromServer(SESSION_ID, true);

    const messaging = machine.getMessagingSnapshot();
    expect(messaging.turns).toHaveLength(1);
    expect(messaging.turns[0].userTurn.id).toBe("server-user");
    expect(messaging.turns[0].assistantTurn?.content).toBe("Partial answer");
  });

  it("keeps the turn on a brand-new chat error and retries by replacing it (no duplicate)", async () => {
    // Regression: a brand-new chat (no session yet) whose first message errors
    // must not reset to the welcome screen. The turn is kept, and Retry replaces
    // it via replaceFromMessageID instead of appending a second copy.
    let sends = 0;
    const dataSource = makeDataSource(async function* () {
      sends++;
      yield { type: "content", content: "Partial" } as StreamChunk;
      yield { type: "error", error: "boom" } as StreamChunk;
    });
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId("new");

    await machine.sendMessage("Hello world");

    let messaging = machine.getMessagingSnapshot();
    expect(messaging.turns).toHaveLength(1);
    expect(messaging.turns[0].userTurn.content).toBe("Hello world");
    expect(messaging.turns[0].assistantTurn?.content).toBe("Partial");

    await machine.retryLastMessage();

    messaging = machine.getMessagingSnapshot();
    expect(sends).toBe(2);
    expect(messaging.turns).toHaveLength(1);
    expect(messaging.turns[0].userTurn.content).toBe("Hello world");
    expect(messaging.turns[0].assistantTurn?.content).toBe("Partial");
  });

  it("restores the prompt without an error banner when the stream is aborted", async () => {
    // A user stop / unmount surfaces as a real AbortError (see
    // MessageTransport): the machine takes the soft-cancel path — prompt
    // restored, no error banner.
    const abortError = new Error("aborted");
    abortError.name = "AbortError";
    const dataSource = makeDataSource(async function* () {
      yield { type: "content", content: "Partial" } as StreamChunk;
      throw abortError;
    });
    const machine = new ChatMachine({ dataSource, rateLimiter });
    machine.setSessionId(SESSION_ID);

    await machine.sendMessage("Hello again");

    const input = machine.getInputSnapshot();
    const messaging = machine.getMessagingSnapshot();
    expect(input.message).toBe("Hello again");
    expect(messaging.streamError).toBeNull();
  });
});
