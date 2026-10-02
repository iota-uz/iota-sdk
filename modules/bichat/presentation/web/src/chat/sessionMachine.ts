import type { Citation } from '../rpc.generated'
import type { StreamChunk } from '../streaming/protocol'
import { type ChatTurn, type TextBlock, type ToolCallView, turnsFromServer } from './types'

export interface MachineState {
  turns: ChatTurn[]
  runId?: string
}

export function initialState(serverTurns: Parameters<typeof turnsFromServer>[0]): MachineState {
  return { turns: turnsFromServer(serverTurns) }
}

function cloneTurn(turn: ChatTurn): ChatTurn {
  return {
    ...turn,
    attachments: [...turn.attachments],
    assistant: turn.assistant
      ? {
          ...turn.assistant,
          textBlocks: turn.assistant.textBlocks.map((block) => ({ ...block })),
          thinking: [...turn.assistant.thinking],
          toolCalls: turn.assistant.toolCalls.map((call) => ({ ...call })),
          citations: [...turn.assistant.citations],
          charts: [...turn.assistant.charts],
          codeOutputs: [...turn.assistant.codeOutputs],
        }
      : undefined,
  }
}

function nextSeq(blocks: TextBlock[]): number {
  return blocks.length === 0 ? 0 : blocks[blocks.length - 1].seq + 1
}

/**
 * Applies one stream chunk to the turn list. Chunks always target the last
 * turn: a streaming run continues the most recent exchange. Returns a new
 * state; never mutates the input.
 */
export function applyChunk(state: MachineState, chunk: StreamChunk): MachineState {
  switch (chunk.type) {
    case 'stream_started':
      return { ...state, runId: chunk.runId ?? state.runId }
    case 'ping':
      return state
    default:
      return applyTurnChunk(state, chunk)
  }
}

function applyTurnChunk(state: MachineState, chunk: StreamChunk): MachineState {
  if (state.turns.length === 0) return state
  const turns = state.turns.slice(0, -1)
  const tail = cloneTurn(state.turns[state.turns.length - 1])
  const assistant = tail.assistant ?? {
    textBlocks: [],
    thinking: [],
    toolCalls: [] as ToolCallView[],
    citations: [],
    charts: [],
    codeOutputs: [],
  }
  tail.assistant = assistant

  switch (chunk.type) {
    case 'chunk':
    case 'content': {
      const text = chunk.content ?? ''
      if (text) {
        const last = assistant.textBlocks[assistant.textBlocks.length - 1]
        if (last && !last.sealed) last.content += text
        else assistant.textBlocks.push({ seq: nextSeq(assistant.textBlocks), content: text, sealed: false })
      }
      break
    }
    case 'thinking': {
      if (chunk.content) assistant.thinking.push(chunk.content)
      break
    }
    case 'tool_start': {
      const callId = chunk.tool?.callId ?? `${assistant.toolCalls.length}`
      const existing = assistant.toolCalls.find((call) => call.callId === callId)
      if (existing) {
        existing.status = 'running'
        existing.name = chunk.tool?.name ?? existing.name
      } else {
        assistant.toolCalls.push({
          callId,
          name: chunk.tool?.name ?? 'tool',
          status: 'running',
          detail: chunk.tool?.detail,
        })
      }
      break
    }
    case 'tool_end': {
      const callId = chunk.tool?.callId
      const call = callId
        ? assistant.toolCalls.find((entry) => entry.callId === callId)
        : [...assistant.toolCalls].reverse().find((entry) => entry.status === 'running')
      if (call) {
        call.status = chunk.tool?.error ? 'failed' : 'finished'
        call.error = chunk.tool?.error
      }
      break
    }
    case 'text_block_end': {
      const last = assistant.textBlocks[assistant.textBlocks.length - 1]
      if (last && !last.sealed) {
        last.sealed = true
        if (typeof chunk.textBlockSeq === 'number') last.seq = chunk.textBlockSeq
      }
      break
    }
    case 'citation': {
      if (chunk.citation) {
        assistant.citations.push(chunk.citation as Citation)
      }
      break
    }
    case 'usage': {
      if (chunk.usage) assistant.usage = chunk.usage
      break
    }
    case 'snapshot': {
      const partial = chunk.snapshot?.partialContent
      if (typeof partial === 'string') {
        const last = assistant.textBlocks[assistant.textBlocks.length - 1]
        if (last && !last.sealed) last.content = partial
        else assistant.textBlocks.push({ seq: nextSeq(assistant.textBlocks), content: partial, sealed: false })
      }
      break
    }
    case 'interrupt': {
      if (chunk.interrupt) {
        tail.pendingQuestion = {
          checkpointId: chunk.interrupt.checkpointId ?? '',
          turnId: tail.id,
          status: 'pending',
          questions: (chunk.interrupt.questions ?? []).map((question, index) => ({
            id: question.id ?? `q-${index}`,
            text: question.label ?? question.name ?? '',
            type: question.type ?? 'text',
            options: (question.options ?? []).map((option) => ({ id: option.value, label: option.label })),
          })),
        }
      }
      break
    }
    case 'done': {
      tail.status = 'complete'
      for (const block of assistant.textBlocks) block.sealed = true
      for (const call of assistant.toolCalls) {
        if (call.status === 'running') call.status = 'finished'
      }
      break
    }
    case 'cancelled': {
      tail.status = 'cancelled'
      for (const block of assistant.textBlocks) block.sealed = true
      break
    }
    case 'error':
    case 'failed': {
      tail.status = 'failed'
      for (const block of assistant.textBlocks) block.sealed = true
      for (const call of assistant.toolCalls) {
        if (call.status === 'running') call.status = 'failed'
      }
      break
    }
    default:
      break
  }

  turns.push(tail)
  return { ...state, turns }
}

/** Appends the optimistic user turn that starts a streaming exchange. */
export function appendOptimisticTurn(state: MachineState, content: string, localId: string): MachineState {
  const turn: ChatTurn = {
    id: localId,
    userContent: content,
    attachments: [],
    assistant: {
      textBlocks: [],
      thinking: [],
      toolCalls: [],
      citations: [],
      charts: [],
      codeOutputs: [],
    },
    status: 'streaming',
    pendingQuestion: null,
    createdAt: new Date().toISOString(),
  }
  return { ...state, turns: [...state.turns, turn] }
}

export function isStreaming(state: MachineState): boolean {
  const tail = state.turns[state.turns.length - 1]
  return tail?.status === 'streaming'
}
