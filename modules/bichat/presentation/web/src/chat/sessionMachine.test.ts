import { describe, expect, it } from 'vitest'
import { appendOptimisticTurn, applyChunk, initialState, isStreaming } from './sessionMachine'
import type { ConversationTurn } from '../rpc.generated'

function serverTurn(overrides: Partial<ConversationTurn> = {}): ConversationTurn {
  return {
    id: 'turn-1',
    sessionId: 's-1',
    userTurn: { id: 'u-1', content: 'Hello', attachments: [], createdAt: '2026-01-01T00:00:00Z' },
    assistantTurn: {
      id: 'a-1',
      content: 'Hi there',
      citations: [],
      artifacts: [],
      codeOutputs: [],
      createdAt: '2026-01-01T00:00:01Z',
    },
    createdAt: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('sessionMachine', () => {
  it('converts server turns and reports not streaming', () => {
    const state = initialState([serverTurn()])
    expect(state.turns).toHaveLength(1)
    expect(state.turns[0].userContent).toBe('Hello')
    expect(isStreaming(state)).toBe(false)
  })

  it('appends an optimistic streaming turn', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Question?', 'local-1')
    expect(state.turns[0].status).toBe('streaming')
    expect(isStreaming(state)).toBe(true)
  })

  it('accumulates content into one block and seals it on text_block_end', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Q', 'local-1')
    state = applyChunk(state, { type: 'stream_started', runId: 'run-1' })
    state = applyChunk(state, { type: 'chunk', content: 'Hel' })
    state = applyChunk(state, { type: 'chunk', content: 'lo' })
    state = applyChunk(state, { type: 'text_block_end', textBlockSeq: 0 })
    state = applyChunk(state, { type: 'chunk', content: 'second' })

    const blocks = state.turns[0].assistant?.textBlocks ?? []
    expect(state.runId).toBe('run-1')
    expect(blocks).toHaveLength(2)
    expect(blocks[0]).toEqual({ seq: 0, content: 'Hello', sealed: true })
    expect(blocks[1].content).toBe('second')
  })

  it('tracks tool lifecycle', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Q', 'l')
    state = applyChunk(state, { type: 'tool_start', tool: { callId: 'c1', name: 'sql.query' } })
    expect(state.turns[0].assistant?.toolCalls[0]).toMatchObject({ name: 'sql.query', status: 'running' })
    state = applyChunk(state, { type: 'tool_end', tool: { callId: 'c1' } })
    expect(state.turns[0].assistant?.toolCalls[0].status).toBe('finished')
    state = applyChunk(state, { type: 'tool_start', tool: { callId: 'c2', name: 'boom' } })
    state = applyChunk(state, { type: 'tool_end', tool: { callId: 'c2', error: 'exploded' } })
    expect(state.turns[0].assistant?.toolCalls[1]).toMatchObject({ status: 'failed', error: 'exploded' })
  })

  it('seals all blocks and finishes on done', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Q', 'l')
    state = applyChunk(state, { type: 'chunk', content: 'answer' })
    state = applyChunk(state, { type: 'done' })
    expect(state.turns[0].status).toBe('complete')
    expect(isStreaming(state)).toBe(false)
  })

  it('maps terminal failure and cancelled states', () => {
    let failed = initialState([])
    failed = appendOptimisticTurn(failed, 'Q', 'l')
    failed = applyChunk(failed, { type: 'failed', error: 'overloaded' })
    expect(failed.turns[0].status).toBe('failed')

    let cancelled = initialState([])
    cancelled = appendOptimisticTurn(cancelled, 'Q', 'l')
    cancelled = applyChunk(cancelled, { type: 'cancelled' })
    expect(cancelled.turns[0].status).toBe('cancelled')
  })

  it('captures HITL interrupt as a pending question', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Q', 'l')
    state = applyChunk(state, {
      type: 'interrupt',
      interrupt: {
        checkpointId: 'cp-1',
        questions: [{ id: 'q1', label: 'Which year?', type: 'select', options: [{ value: '2025', label: '2025' }] }],
      },
    })
    const pending = state.turns[0].pendingQuestion
    expect(pending?.checkpointId).toBe('cp-1')
    expect(pending?.questions[0]).toMatchObject({ id: 'q1', text: 'Which year?' })
    expect(pending?.questions[0].options[0]).toEqual({ id: '2025', label: '2025' })
  })

  it('snapshot replaces unsealed content', () => {
    let state = initialState([])
    state = appendOptimisticTurn(state, 'Q', 'l')
    state = applyChunk(state, { type: 'chunk', content: 'wro' })
    state = applyChunk(state, { type: 'snapshot', snapshot: { partialContent: 'wrong answer' } })
    expect(state.turns[0].assistant?.textBlocks[0].content).toBe('wrong answer')
  })

  it('never mutates the input state', () => {
    const base = appendOptimisticTurn(initialState([]), 'Q', 'l')
    const frozen = JSON.parse(JSON.stringify(base)) as typeof base
    applyChunk(base, { type: 'chunk', content: 'more' })
    expect(JSON.parse(JSON.stringify(base))).toEqual(frozen)
  })
})
