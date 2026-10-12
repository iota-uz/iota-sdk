/**
 * Split data source interfaces for BiChat.
 *
 * ChatDataSource was a single interface mixing CRUD, streaming, artifacts,
 * and admin. These focused interfaces let consumers implement only what
 * they need. ChatDataSource keeps the same shape (with optional artifact
 * and admin methods) for backwards compatibility — it is NOT a strict
 * intersection of these interfaces.
 */
import type { Session, SessionMember, SessionListResult, SessionUser, ConversationTurn, PendingQuestion, Attachment, StreamChunk, QuestionAnswers, SendMessageOptions, SessionArtifact, AsyncRunAccepted, } from './index';
// ---------------------------------------------------------------------------
// SessionStore — session lifecycle CRUD
// ---------------------------------------------------------------------------
export interface SessionStore {
    createSession(): Promise<Session>;
    fetchSession(id: string): Promise<{
        session: Session;
        turns: ConversationTurn[];
        pendingQuestion?: PendingQuestion | null;
    } | null>;
    listSessions(options?: {
        limit?: number;
        offset?: number;
        includeArchived?: boolean;
    }): Promise<SessionListResult>;
    archiveSession(sessionId: string): Promise<Session>;
    unarchiveSession(sessionId: string): Promise<Session>;
    pinSession(sessionId: string): Promise<Session>;
    unpinSession(sessionId: string): Promise<Session>;
    deleteSession(sessionId: string): Promise<void>;
    renameSession(sessionId: string, title: string): Promise<Session>;
    regenerateSessionTitle(sessionId: string): Promise<Session>;
    clearSessionHistory(sessionId: string): Promise<{
        success: boolean;
        deletedMessages: number;
        deletedArtifacts: number;
    }>;
    compactSessionHistory(sessionId: string): Promise<AsyncRunAccepted>;
}
// ---------------------------------------------------------------------------
// MessageTransport — sending messages and HITL
// ---------------------------------------------------------------------------
export interface MessageTransport {
    sendMessage(sessionId: string, content: string, attachments?: Attachment[], signal?: AbortSignal, options?: SendMessageOptions): AsyncGenerator<StreamChunk>;
    submitQuestionAnswers(sessionId: string, questionId: string, answers: QuestionAnswers): Promise<{
        success: boolean;
        data?: AsyncRunAccepted;
        error?: string;
    }>;
    rejectPendingQuestion(sessionId: string): Promise<{
        success: boolean;
        data?: AsyncRunAccepted;
        error?: string;
    }>;
}
// ---------------------------------------------------------------------------
// ArtifactStore — session artifact CRUD (optional)
// ---------------------------------------------------------------------------
export interface ArtifactStore {
    fetchSessionArtifacts(sessionId: string, options?: {
        limit?: number;
        offset?: number;
    }): Promise<{
        artifacts: SessionArtifact[];
        hasMore?: boolean;
        nextOffset?: number;
    }>;
    uploadSessionArtifacts(sessionId: string, files: File[]): Promise<{
        artifacts: SessionArtifact[];
    }>;
    renameSessionArtifact(artifactId: string, name: string, description?: string): Promise<SessionArtifact>;
    deleteSessionArtifact(artifactId: string): Promise<void>;
}
// ---------------------------------------------------------------------------
// AdminStore — org-wide features (optional)
// ---------------------------------------------------------------------------
export interface AdminStore {
    listUsers(): Promise<SessionUser[]>;
    listAllSessions(options?: {
        limit?: number;
        offset?: number;
        includeArchived?: boolean;
        userId?: string | null;
    }): Promise<{
        sessions: Session[];
        total: number;
        hasMore: boolean;
    }>;
    listSessionMembers(sessionId: string): Promise<SessionMember[]>;
    addSessionMember(sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void>;
    updateSessionMemberRole(sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void>;
    removeSessionMember(sessionId: string, userId: string): Promise<void>;
}
