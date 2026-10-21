import { describe, it, expect } from 'vitest';
import type { ActivityStep } from '../types';
import { groupSteps } from '../utils/activitySteps';
import { getToolLabel } from '../utils/toolLabels';

// ---------------------------------------------------------------------------
// groupSteps
// ---------------------------------------------------------------------------

describe('groupSteps', () => {
  const step = (overrides: Partial<ActivityStep> = {}): ActivityStep => ({
    id: 'step-1',
    type: 'tool',
    toolName: 'sql_execute',
    status: 'active',
    startedAt: Date.now(),
    ...overrides,
  });

  it('returns empty arrays for empty input', () => {
    const result = groupSteps([]);
    expect(result.topLevel).toEqual([]);
    expect(result.agentGroups).toEqual([]);
  });

  it('puts steps without agentName into topLevel', () => {
    const steps = [step({ id: 'a' }), step({ id: 'b' })];
    const result = groupSteps(steps);
    expect(result.topLevel).toHaveLength(2);
    expect(result.agentGroups).toHaveLength(0);
  });

  it('groups steps with agentName together', () => {
    const steps = [
      step({ id: 'a', agentName: 'SQL Analyst' }),
      step({ id: 'b', agentName: 'SQL Analyst' }),
      step({ id: 'c', agentName: 'Chart Builder' }),
    ];
    const result = groupSteps(steps);
    expect(result.topLevel).toHaveLength(0);
    expect(result.agentGroups).toHaveLength(2);
    expect(result.agentGroups[0][0]).toBe('SQL Analyst');
    expect(result.agentGroups[0][1]).toHaveLength(2);
    expect(result.agentGroups[1][0]).toBe('Chart Builder');
    expect(result.agentGroups[1][1]).toHaveLength(1);
  });

  it('separates top-level from agent-grouped steps', () => {
    const steps = [
      step({ id: 'top1' }),
      step({ id: 'agent1', agentName: 'Analyst' }),
      step({ id: 'top2' }),
    ];
    const result = groupSteps(steps);
    expect(result.topLevel).toHaveLength(2);
    expect(result.agentGroups).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// getToolLabel
// ---------------------------------------------------------------------------

describe('getToolLabel', () => {
  it('returns SDK translation when key exists', () => {
    const t = (key: string) => {
      if (key === 'BiChat.Tools.sql_execute') {return 'Querying database';}
      return key;
    };
    expect(getToolLabel(t, 'sql_execute')).toBe('Querying database');
  });

  it('returns consumer-prefix translation when available', () => {
    const t = (key: string) => {
      if (key === 'Ali.Tools.custom_tool') {return 'Custom analysis';}
      if (key === 'BiChat.Tools.custom_tool') {return 'BiChat.Tools.custom_tool';} // not found
      return key;
    };
    expect(getToolLabel(t, 'custom_tool', undefined, 'Ali.Tools')).toBe('Custom analysis');
  });

  it('prefers consumer prefix over SDK prefix', () => {
    const t = (key: string) => {
      if (key === 'Ali.Tools.sql_execute') {return 'Running Ali query';}
      if (key === 'BiChat.Tools.sql_execute') {return 'Querying database';}
      return key;
    };
    expect(getToolLabel(t, 'sql_execute', undefined, 'Ali.Tools')).toBe('Running Ali query');
  });

  it('falls back to humanized name when no translation', () => {
    const t = (key: string) => key; // return key = not found
    expect(getToolLabel(t, 'sql_execute')).toBe('SQL Execute');
  });

  it('humanizes with known acronyms uppercased', () => {
    const t = (key: string) => key;
    expect(getToolLabel(t, 'kb_search')).toBe('KB Search');
    expect(getToolLabel(t, 'export_to_pdf')).toBe('Export To PDF');
    expect(getToolLabel(t, 'api_call')).toBe('API Call');
  });

  it('interpolates {{agent}} for delegation tool', () => {
    const t = (key: string, params?: Record<string, string | number | boolean>) => {
      if (key === 'BiChat.Tools.task' && params?.agent) {
        return `Delegating to ${params.agent}`;
      }
      return key;
    };
    const args = JSON.stringify({ description: 'SQL Analysis', subagent_type: 'analyst' });
    expect(getToolLabel(t, 'task', args)).toBe('Delegating to SQL Analysis');
  });

  it('uses subagent_type when description is missing', () => {
    const t = (key: string, params?: Record<string, string | number | boolean>) => {
      if (key === 'BiChat.Tools.task' && params?.agent) {
        return `Delegating to ${params.agent}`;
      }
      return key;
    };
    const args = JSON.stringify({ subagent_type: 'debugger' });
    expect(getToolLabel(t, 'task', args)).toBe('Delegating to debugger');
  });

  it('uses "specialist" fallback when args are malformed', () => {
    const t = (key: string, params?: Record<string, string | number | boolean>) => {
      if (key === 'BiChat.Tools.task' && params?.agent) {
        return `Delegating to ${params.agent}`;
      }
      return key;
    };
    expect(getToolLabel(t, 'task', 'invalid-json')).toBe('Delegating to specialist');
  });
});

// ---------------------------------------------------------------------------
// Step matching logic (extracted to test the algorithm)
// ---------------------------------------------------------------------------

describe('step matching', () => {
  // Mirrors ChatMachine._matchStep: callId is authoritative when present.
  function matchStep(
    step: ActivityStep,
    tool: { callId?: string; name: string; agentName?: string }
  ): boolean {
    if (tool.callId) {return step.id === tool.callId;}
    return step.toolName === tool.name && step.agentName === tool.agentName;
  }

  function findFirstMatch(
    steps: ActivityStep[],
    tool: { callId?: string; name: string; agentName?: string }
  ): number {
    return steps.findIndex((s) => s.status === 'active' && matchStep(s, tool));
  }

  const mkStep = (overrides: Partial<ActivityStep> = {}): ActivityStep => ({
    id: 'step-1',
    type: 'tool',
    toolName: 'sql_execute',
    status: 'active',
    startedAt: Date.now(),
    ...overrides,
  });

  it('matches by callId when present', () => {
    const step = mkStep({ id: 'call-123' });
    expect(matchStep(step, { callId: 'call-123', name: 'sql_execute' })).toBe(true);
  });

  it('does not match different callId', () => {
    const step = mkStep({ id: 'call-123' });
    expect(matchStep(step, { callId: 'call-999', name: 'sql_execute' })).toBe(false);
  });

  it('falls back to name + agentName when callId absent', () => {
    const step = mkStep({ id: 'random-id', toolName: 'sql_execute', agentName: 'Analyst' });
    expect(matchStep(step, { name: 'sql_execute', agentName: 'Analyst' })).toBe(true);
  });

  it('does not match different agentName', () => {
    const step = mkStep({ id: 'random-id', toolName: 'sql_execute', agentName: 'Analyst' });
    expect(matchStep(step, { name: 'sql_execute', agentName: 'OtherAgent' })).toBe(false);
  });

  it('findFirstMatch picks earliest active step for concurrent duplicates', () => {
    const steps = [
      mkStep({ id: 'a', toolName: 'sql_execute', status: 'completed' }),
      mkStep({ id: 'b', toolName: 'sql_execute', status: 'active' }),
      mkStep({ id: 'c', toolName: 'sql_execute', status: 'active' }),
    ];
    const idx = findFirstMatch(steps, { name: 'sql_execute' });
    expect(idx).toBe(1); // first active, not the completed one
  });

  it('findFirstMatch returns -1 when no active steps match', () => {
    const steps = [
      mkStep({ id: 'a', toolName: 'sql_execute', status: 'completed' }),
    ];
    expect(findFirstMatch(steps, { name: 'sql_execute' })).toBe(-1);
  });
});

// ---------------------------------------------------------------------------
// Thinking buffer limit
// ---------------------------------------------------------------------------

describe('thinking buffer', () => {
  const THINKING_BUFFER_LIMIT = 500;

  function applyThinkingChunk(prev: string, content: string): string {
    let updated = prev + content;
    if (updated.length > THINKING_BUFFER_LIMIT) {
      updated = updated.slice(-THINKING_BUFFER_LIMIT);
    }
    return updated;
  }

  it('accumulates content normally under limit', () => {
    expect(applyThinkingChunk('hello ', 'world')).toBe('hello world');
  });

  it('truncates from front when exceeding limit', () => {
    const prev = 'x'.repeat(490);
    const chunk = 'y'.repeat(20);
    const result = applyThinkingChunk(prev, chunk);
    expect(result.length).toBe(THINKING_BUFFER_LIMIT);
    expect(result.endsWith('y'.repeat(20))).toBe(true);
    expect(result.startsWith('x')).toBe(true); // still starts with x (trimmed from front)
  });

  it('caps at exact limit for very long input', () => {
    const result = applyThinkingChunk('', 'z'.repeat(1000));
    expect(result.length).toBe(THINKING_BUFFER_LIMIT);
    expect(result).toBe('z'.repeat(THINKING_BUFFER_LIMIT));
  });

  it('preserves full content when exactly at limit', () => {
    const result = applyThinkingChunk('a'.repeat(250), 'b'.repeat(250));
    expect(result.length).toBe(THINKING_BUFFER_LIMIT);
    expect(result).toBe('a'.repeat(250) + 'b'.repeat(250));
  });
});
