import type { BichatRPCClient } from '../rpc/client'
import type { Artifact, ConversationTurn, Session, SessionArtifactsResult } from '../rpc.generated'
import {
  attachRichChartDataToTurns,
  normalizeChartArtifactsForSdk,
} from '../charts/chartData'
import type { SessionArtifact } from '../charts/sdk-types'

export interface SessionLoadResult {
  session: Session
  turns: ConversationTurn[]
  pendingQuestion?: import('../rpc.generated').PendingQuestion | null
  artifacts: SessionArtifact[]
}

/**
 * Loads the session and its full artifact list. Chart artifacts are
 * normalized and attached to their assistant turns so renderers can draw
 * rich charts without a second lookup.
 */
export async function fetchSessionWithArtifacts(
  rpc: BichatRPCClient,
  sessionId: string,
): Promise<SessionLoadResult> {
  const result = await rpc.call('bichat.session.get', { id: sessionId })
  const artifacts = await collectSessionArtifacts(rpc, sessionId)
  const normalized = normalizeChartArtifactsForSdk(artifacts)
  const turns =
    normalized.length > 0 ? attachRichChartDataToTurns(result.turns, normalized) : result.turns
  return {
    session: result.session,
    turns,
    pendingQuestion: result.pendingQuestion ?? null,
    artifacts: normalized,
  }
}

export async function collectSessionArtifacts(
  rpc: BichatRPCClient,
  sessionId: string,
): Promise<Artifact[]> {
  const collected: Artifact[] = []
  const pageSize = 200
  let offset = 0
  for (;;) {
    const page: SessionArtifactsResult = await rpc.call('bichat.session.artifacts', {
      sessionId,
      limit: pageSize,
      offset,
    })
    collected.push(...(page.artifacts ?? []))
    if (!page.hasMore || (page.artifacts?.length ?? 0) === 0) break
    offset = page.nextOffset ?? offset + (page.artifacts?.length ?? 0)
  }
  return collected
}
