import type {
  Attachment,
  Citation,
  CodeOutput,
  ConversationTurn,
  PendingQuestion,
} from '../rpc.generated'
import type { RichChartData } from '../charts/sdk-types'
import type { InterruptEventPayload, UsagePayload } from '../streaming/protocol'

export type { ConversationTurn, Citation, CodeOutput, Attachment, PendingQuestion }
export type { InterruptEventPayload, UsagePayload }
export type { RichChartData } from '../charts/sdk-types'

export interface TextBlock {
  seq: number
  content: string
  sealed: boolean
}

export interface ToolCallView {
  callId: string
  name: string
  status: 'running' | 'finished' | 'failed'
  detail?: string
  error?: string
}

export type TurnStatus = 'complete' | 'streaming' | 'failed' | 'cancelled'

export interface ChatTurn {
  /** Server turn id once persisted; local id for the optimistic tail turn. */
  id: string
  userContent: string
  attachments: Attachment[]
  assistant?: {
    id?: string
    textBlocks: TextBlock[]
    thinking: string[]
    toolCalls: ToolCallView[]
    citations: Citation[]
    charts: RichChartData[]
    codeOutputs: CodeOutput[]
    usage?: UsagePayload
    createdAt?: string
  }
  status: TurnStatus
  pendingQuestion?: PendingQuestion | null
  createdAt: string
}

export function assistantText(turn: ChatTurn): string {
  return (turn.assistant?.textBlocks ?? []).map((block) => block.content).join('')
}

/** Converts persisted server turns into the view model. */
export function turnFromServer(turn: ConversationTurn): ChatTurn {
  const assistant = turn.assistantTurn
  const charts = ((assistant as unknown as { charts?: RichChartData[] } | null)?.charts) ?? []
  return {
    id: turn.id,
    userContent: turn.userTurn?.content ?? '',
    attachments: turn.userTurn?.attachments ?? [],
    assistant: assistant
      ? {
          id: assistant.id,
          textBlocks: [{ seq: 0, content: assistant.content ?? '', sealed: true }],
          thinking: [],
          toolCalls: (assistant.toolCalls ?? []).map((call) => ({
            callId: call.id,
            name: call.name,
            status: call.error ? 'failed' : 'finished',
            error: call.error,
          })),
          citations: assistant.citations ?? [],
          charts,
          codeOutputs: assistant.codeOutputs ?? [],
          createdAt: assistant.createdAt,
        }
      : undefined,
    status: 'complete',
    pendingQuestion: null,
    createdAt: turn.createdAt,
  }
}

export function turnsFromServer(turns: ConversationTurn[]): ChatTurn[] {
  return turns.map(turnFromServer)
}
