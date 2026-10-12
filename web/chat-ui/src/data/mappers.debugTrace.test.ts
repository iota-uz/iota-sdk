import { describe, expect, it } from 'vitest';
import { sanitizeConversationTurns } from './mappers';

describe('sanitizeConversationTurns debug trace metadata', () => {
  it('maps traceId and traceUrl into assistant debug trace', () => {
    const sessionID = 'session-1';
    const turns = sanitizeConversationTurns([
      {
        id: 'turn-1',
        sessionId: sessionID,
        createdAt: '2026-02-22T00:00:00.000Z',
        userTurn: {
          id: 'user-1',
          content: 'hello',
          attachments: [],
          createdAt: '2026-02-22T00:00:00.000Z',
        },
        assistantTurn: {
          id: 'assistant-1',
          role: 'assistant',
          content: 'world',
          createdAt: '2026-02-22T00:00:01.000Z',
          debug: {
            traceId: 'trace-123',
            traceUrl: 'https://langfuse.local/trace/trace-123',
            tools: [],
          },
        },
      },
    ], sessionID);

    expect(turns).toHaveLength(1);
    expect(turns[0]?.assistantTurn?.debug?.traceId).toBe('trace-123');
    expect(turns[0]?.assistantTurn?.debug?.traceUrl).toBe('https://langfuse.local/trace/trace-123');
  });
});
