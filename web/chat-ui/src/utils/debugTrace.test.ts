import { describe, expect, it } from 'vitest';
import { hasDebugTrace } from './debugTrace';

describe('hasDebugTrace', () => {
  it('returns true when trace metadata exists without usage/tools/timing', () => {
    expect(
      hasDebugTrace({
        tools: [],
        traceId: 'session-trace-1',
      })
    ).toBe(true);
  });
});
