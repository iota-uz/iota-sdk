/**
 * Session lifecycle management: create, list, get, delete, archive, pin, rename.
 *
 * @internal — Not part of the public API. Consumed by HttpDataSource.
 */
import { AppletRPCException } from './applet-rpc';
import type { BichatRPC } from './rpc.generated';
import type { Session, SessionMember, SessionUser, SessionListResult, SessionArtifact, ConversationTurn, PendingQuestion, AsyncRunAccepted, } from '../types';
import { toSession, sanitizeConversationTurns, sanitizePendingQuestion, normalizeTurns, attachArtifactsToTurns, warnMalformedSessionPayload, } from './mappers';
// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
export interface SessionState {
    session: Session;
    turns: ConversationTurn[];
    pendingQuestion?: PendingQuestion | null;
}
type RPCCaller = <TMethod extends keyof BichatRPC & string>(method: TMethod, params: BichatRPC[TMethod]['params']) => Promise<BichatRPC[TMethod]['result']>;
type FetchSessionArtifactsFn = (sessionId: string, options?: {
    limit?: number;
    offset?: number;
}) => Promise<{
    artifacts: SessionArtifact[];
    hasMore?: boolean;
    nextOffset?: number;
}>;
// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function isSessionNotFoundError(err: unknown): boolean {
    if (!(err instanceof AppletRPCException)) {
        return false;
    }
    return err.code === 'not_found' || err.code === 'session_not_found';
}
// ---------------------------------------------------------------------------
// Session CRUD functions
// ---------------------------------------------------------------------------
export async function createSession(callRPC: RPCCaller): Promise<Session> {
    const data = await callRPC('bichat.session.create', { title: '' });
    return toSession(data.session);
}
export async function fetchSession(id: string, callRPC: RPCCaller, fetchArtifacts: FetchSessionArtifactsFn): Promise<SessionState | null> {
    try {
        const [data, artifactsData] = await Promise.all([
            callRPC('bichat.session.get', { id }),
            fetchArtifacts(id, { limit: 200, offset: 0 }).catch((err) => {
                console.warn('Failed to fetch session artifacts:', err);
                return { artifacts: [] as SessionArtifact[], hasMore: false, nextOffset: 0 };
            }),
        ]);
        const sanitizedTurns = sanitizeConversationTurns(data.turns, id);
        const turns = attachArtifactsToTurns(normalizeTurns(sanitizedTurns), artifactsData.artifacts || []);
        const pendingQuestion = sanitizePendingQuestion(data.pendingQuestion, id);
        if (data.pendingQuestion && pendingQuestion && pendingQuestion.questions.length === 0) {
            warnMalformedSessionPayload('Pending question normalized to zero renderable questions', {
                sessionID: id,
                checkpointID: pendingQuestion.id,
            });
        }
        return {
            session: toSession(data.session),
            turns,
            pendingQuestion,
        };
    }
    catch (err) {
        if (isSessionNotFoundError(err)) {
            return null;
        }
        console.error('Failed to fetch session:', err);
        throw err instanceof Error ? err : new Error('Failed to fetch session');
    }
}
export async function listSessions(callRPC: RPCCaller, options?: {
    limit?: number;
    offset?: number;
    includeArchived?: boolean;
}): Promise<SessionListResult> {
    const data = await callRPC('bichat.session.list', {
        limit: options?.limit ?? 200,
        offset: options?.offset ?? 0,
        includeArchived: options?.includeArchived ?? false,
    });
    return {
        sessions: data.sessions.map(toSession),
        total: typeof data.total === 'number' ? data.total : data.sessions.length,
        hasMore: typeof data.hasMore === 'boolean' ? data.hasMore : false,
    };
}
export async function archiveSession(callRPC: RPCCaller, sessionId: string): Promise<Session> {
    const data = await callRPC('bichat.session.archive', { id: sessionId });
    return toSession(data.session);
}
export async function unarchiveSession(callRPC: RPCCaller, sessionId: string): Promise<Session> {
    const data = await callRPC('bichat.session.unarchive', { id: sessionId });
    return toSession(data.session);
}
export async function pinSession(callRPC: RPCCaller, sessionId: string): Promise<Session> {
    const data = await callRPC('bichat.session.pin', { id: sessionId });
    return toSession(data.session);
}
export async function unpinSession(callRPC: RPCCaller, sessionId: string): Promise<Session> {
    const data = await callRPC('bichat.session.unpin', { id: sessionId });
    return toSession(data.session);
}
export async function deleteSession(callRPC: RPCCaller, sessionId: string): Promise<void> {
    await callRPC('bichat.session.delete', { id: sessionId });
}
export async function renameSession(callRPC: RPCCaller, sessionId: string, title: string): Promise<Session> {
    const data = await callRPC('bichat.session.updateTitle', { id: sessionId, title });
    return toSession(data.session);
}
export async function regenerateSessionTitle(callRPC: RPCCaller, sessionId: string): Promise<Session> {
    const data = await callRPC('bichat.session.regenerateTitle', { id: sessionId });
    return toSession(data.session);
}
export async function clearSessionHistory(callRPC: RPCCaller, sessionId: string): Promise<{
    success: boolean;
    deletedMessages: number;
    deletedArtifacts: number;
}> {
    return callRPC('bichat.session.clear', { id: sessionId });
}
export async function compactSessionHistory(callRPC: RPCCaller, sessionId: string): Promise<AsyncRunAccepted> {
    const result = await callRPC('bichat.session.compact', { id: sessionId });
    if (!result.accepted) {
        throw new Error('Session compact request was not accepted');
    }
    if (result.operation !== 'session_compact') {
        throw new Error(`Unexpected async operation: ${result.operation}`);
    }
    if (!result.sessionId || !result.runId) {
        throw new Error('Missing async run metadata');
    }
    return {
        accepted: true,
        operation: result.operation,
        sessionId: result.sessionId,
        runId: result.runId,
        startedAt: result.startedAt,
    };
}
export async function listUsers(callRPC: RPCCaller): Promise<SessionUser[]> {
    const data = await callRPC('bichat.user.list', {});
    return data.users.map((user) => ({
        id: String(user.id),
        firstName: user.firstName || '',
        lastName: user.lastName || '',
        initials: user.initials || '',
    }));
}
export async function listAllSessions(callRPC: RPCCaller, options?: {
    limit?: number;
    offset?: number;
    includeArchived?: boolean;
    userId?: string | null;
}): Promise<{
    sessions: Session[];
    total: number;
    hasMore: boolean;
}> {
    const data = await callRPC('bichat.session.listAll', {
        limit: options?.limit ?? 50,
        offset: options?.offset ?? 0,
        includeArchived: options?.includeArchived ?? false,
        userId: options?.userId ?? null,
    });
    return {
        sessions: data.sessions.map(toSession),
        total: typeof data.total === 'number' ? data.total : data.sessions.length,
        hasMore: Boolean(data.hasMore),
    };
}
export async function listSessionMembers(callRPC: RPCCaller, sessionId: string): Promise<SessionMember[]> {
    const data = await callRPC('bichat.session.members.list', { sessionId });
    return data.members.map((member) => ({
        user: {
            id: String(member.user.id),
            firstName: member.user.firstName,
            lastName: member.user.lastName,
            initials: member.user.initials,
        },
        role: (() => {
            const normalizedRole = (member.role || '').toLowerCase();
            if (normalizedRole === 'owner') {
                return 'owner';
            }
            if (normalizedRole === 'editor') {
                return 'editor';
            }
            return 'viewer';
        })(),
        createdAt: member.createdAt,
        updatedAt: member.updatedAt,
    }));
}
export async function addSessionMember(callRPC: RPCCaller, sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void> {
    await callRPC('bichat.session.members.add', {
        sessionId,
        userId,
        role: role.toUpperCase(),
    });
}
export async function updateSessionMemberRole(callRPC: RPCCaller, sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void> {
    await callRPC('bichat.session.members.updateRole', {
        sessionId,
        userId,
        role: role.toUpperCase(),
    });
}
export async function removeSessionMember(callRPC: RPCCaller, sessionId: string, userId: string): Promise<void> {
    await callRPC('bichat.session.members.remove', { sessionId, userId });
}
