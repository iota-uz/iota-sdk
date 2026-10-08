/**
 * Built-in HTTP data source with SSE streaming and AbortController
 * Implements ChatDataSource interface with real HTTP/RPC calls
 *
 * Uses turn-based architecture - fetches ConversationTurns instead of flat messages.
 *
 * This file is a thin facade that delegates to focused internal modules:
 *   - SessionManager.ts  — session CRUD (create, list, get, delete, archive, pin)
 *   - MessageTransport.ts — send messages, stream responses, HITL questions
 *   - ArtifactManager.ts  — artifact fetch, upload, rename, delete
 *   - AttachmentUploader.ts — file decode, normalize, upload
 *   - mappers.ts          — RPC-to-domain type mapping and sanitization
 */
import { createAppletRPCClient } from './applet-rpc';
import type { BichatRPC } from './rpc.generated';
import type { ChatDataSource, Session, SessionMember, SessionListResult, SessionUser, SessionArtifact, Attachment, StreamChunk, StreamStatus, QuestionAnswers, SendMessageOptions, AsyncRunAccepted, } from '../types';
import * as Sessions from './SessionManager';
import type { SessionState } from './SessionManager';
import * as Messages from './MessageTransport';
import * as Artifacts from './ArtifactManager';
import { uploadFile } from './AttachmentUploader';
export interface HttpDataSourceConfig {
    baseUrl: string;
    rpcEndpoint: string;
    streamEndpoint?: string;
    uploadEndpoint?: string;
    csrfToken?: string | (() => string);
    headers?: Record<string, string>;
    rpcTimeoutMs?: number;
    /** Optional transport owned by the mounted host. */
    fetcher?: typeof fetch;
    streamConnectTimeoutMs?: number;
}
export class HttpDataSource implements ChatDataSource {
    private config: HttpDataSourceConfig;
    private abortController: AbortController | null = null;
    private rpc: ReturnType<typeof createAppletRPCClient>;
    constructor(config: HttpDataSourceConfig) {
        this.config = {
            streamEndpoint: '/stream',
            uploadEndpoint: '/api/uploads',
            ...config,
            rpcTimeoutMs: typeof config.rpcTimeoutMs === 'number' ? config.rpcTimeoutMs : 120000,
            streamConnectTimeoutMs: typeof config.streamConnectTimeoutMs === 'number'
                ? config.streamConnectTimeoutMs
                : undefined,
        };
        this.rpc = createAppletRPCClient({
            endpoint: `${this.config.baseUrl}${this.config.rpcEndpoint}`,
            timeoutMs: this.config.rpcTimeoutMs,
            fetcher: (input, init) => (this.config.fetcher ?? fetch)(input, { ...init, credentials: 'same-origin', headers: this.createHeaders(init?.headers ? Object.fromEntries(new Headers(init.headers)) : undefined) }),
        });
    }
    // -------------------------------------------------------------------------
    // Internal helpers
    // -------------------------------------------------------------------------
    private getCSRFToken(): string {
        if (!this.config.csrfToken) {
            return '';
        }
        return typeof this.config.csrfToken === 'function'
            ? this.config.csrfToken()
            : this.config.csrfToken;
    }
    private createHeaders(additionalHeaders?: Record<string, string>): Headers {
        const headers = new Headers({
            'Content-Type': 'application/json',
            ...this.config.headers,
            ...additionalHeaders,
        });
        const csrfToken = this.getCSRFToken();
        if (csrfToken) {
            headers.set('X-CSRF-Token', csrfToken);
        }
        return headers;
    }
    private createUploadHeaders(additionalHeaders?: Record<string, string>): Headers {
        const headers = new Headers({
            ...this.config.headers,
            ...additionalHeaders,
        });
        const csrfToken = this.getCSRFToken();
        if (csrfToken) {
            headers.set('X-CSRF-Token', csrfToken);
        }
        headers.delete('Content-Type');
        return headers;
    }
    private callRPC<TMethod extends keyof BichatRPC & string>(method: TMethod, params: BichatRPC[TMethod]['params']): Promise<BichatRPC[TMethod]['result']> {
        return this.rpc.callTyped<BichatRPC, TMethod>(method, params);
    }
    private boundCallRPC = <TMethod extends keyof BichatRPC & string>(method: TMethod, params: BichatRPC[TMethod]['params']): Promise<BichatRPC[TMethod]['result']> => {
        return this.callRPC(method, params);
    };
    private boundUploadFile = (file: File) => {
        return uploadFile(file, this.config.baseUrl, this.config.uploadEndpoint!, () => this.createUploadHeaders(), this.config.fetcher);
    };
    // -------------------------------------------------------------------------
    // Session management (delegates to SessionManager)
    // -------------------------------------------------------------------------
    async createSession(): Promise<Session> {
        return Sessions.createSession(this.boundCallRPC);
    }
    async fetchSession(id: string): Promise<SessionState | null> {
        return Sessions.fetchSession(id, this.boundCallRPC, (sessionId, options) => this.fetchSessionArtifacts(sessionId, options));
    }
    async listSessions(options?: {
        limit?: number;
        offset?: number;
        includeArchived?: boolean;
    }): Promise<SessionListResult> {
        return Sessions.listSessions(this.boundCallRPC, options);
    }
    async archiveSession(sessionId: string): Promise<Session> {
        return Sessions.archiveSession(this.boundCallRPC, sessionId);
    }
    async unarchiveSession(sessionId: string): Promise<Session> {
        return Sessions.unarchiveSession(this.boundCallRPC, sessionId);
    }
    async pinSession(sessionId: string): Promise<Session> {
        return Sessions.pinSession(this.boundCallRPC, sessionId);
    }
    async unpinSession(sessionId: string): Promise<Session> {
        return Sessions.unpinSession(this.boundCallRPC, sessionId);
    }
    async deleteSession(sessionId: string): Promise<void> {
        return Sessions.deleteSession(this.boundCallRPC, sessionId);
    }
    async renameSession(sessionId: string, title: string): Promise<Session> {
        return Sessions.renameSession(this.boundCallRPC, sessionId, title);
    }
    async regenerateSessionTitle(sessionId: string): Promise<Session> {
        return Sessions.regenerateSessionTitle(this.boundCallRPC, sessionId);
    }
    async clearSessionHistory(sessionId: string): Promise<{
        success: boolean;
        deletedMessages: number;
        deletedArtifacts: number;
    }> {
        return Sessions.clearSessionHistory(this.boundCallRPC, sessionId);
    }
    async compactSessionHistory(sessionId: string): Promise<AsyncRunAccepted> {
        return Sessions.compactSessionHistory(this.boundCallRPC, sessionId);
    }
    async listUsers(): Promise<SessionUser[]> {
        return Sessions.listUsers(this.boundCallRPC);
    }
    async listAllSessions(options?: {
        limit?: number;
        offset?: number;
        includeArchived?: boolean;
        userId?: string | null;
    }): Promise<{
        sessions: Session[];
        total: number;
        hasMore: boolean;
    }> {
        return Sessions.listAllSessions(this.boundCallRPC, options);
    }
    async listSessionMembers(sessionId: string): Promise<SessionMember[]> {
        return Sessions.listSessionMembers(this.boundCallRPC, sessionId);
    }
    async addSessionMember(sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void> {
        return Sessions.addSessionMember(this.boundCallRPC, sessionId, userId, role);
    }
    async updateSessionMemberRole(sessionId: string, userId: string, role: 'editor' | 'viewer'): Promise<void> {
        return Sessions.updateSessionMemberRole(this.boundCallRPC, sessionId, userId, role);
    }
    async removeSessionMember(sessionId: string, userId: string): Promise<void> {
        return Sessions.removeSessionMember(this.boundCallRPC, sessionId, userId);
    }
    // -------------------------------------------------------------------------
    // Message transport (delegates to MessageTransport)
    // -------------------------------------------------------------------------
    async stopGeneration(sessionId: string): Promise<void> {
        this.cancelStream();
        await Messages.stopStream({
            baseUrl: this.config.baseUrl,
            streamEndpoint: this.config.streamEndpoint!,
            createHeaders: (h) => this.createHeaders(h),
        }, sessionId);
    }
    async getStreamStatus(sessionId: string): Promise<StreamStatus | null> {
        return Messages.getStreamStatus({
            baseUrl: this.config.baseUrl,
            streamEndpoint: this.config.streamEndpoint!,
            createHeaders: (h) => this.createHeaders(h),
            timeoutMs: this.config.rpcTimeoutMs,
        }, sessionId);
    }
    async resumeStream(sessionId: string, runId: string, onChunk: (chunk: StreamChunk) => void, signal?: AbortSignal): Promise<void> {
        await Messages.resumeStream({
            baseUrl: this.config.baseUrl,
            streamEndpoint: this.config.streamEndpoint!,
            createHeaders: (h) => this.createHeaders(h),
            connectTimeoutMs: this.config.streamConnectTimeoutMs,
        }, sessionId, runId, onChunk, signal);
    }
    /**
     * Open a native EventSource against GET /stream/events for the given
     * run. Used by components that want browser-native auto-reconnect
     * with Last-Event-ID (tab close, wifi drop, device switch). Prefer
     * over resumeStream when tailing an already-running generation that
     * another tab started.
     */
    subscribeRunEvents(sessionId: string, runId: string, options: Messages.SubscribeRunEventsOptions): Promise<void> {
        return Messages.subscribeRunEvents({
            baseUrl: this.config.baseUrl,
            streamEndpoint: this.config.streamEndpoint!,
        }, sessionId, runId, options);
    }
    /**
     * Subscribe to the per-tenant active-run fan-out
     * (GET /stream/active-runs). Never resolves until the caller aborts
     * via the signal; use from a top-level component (sidebar container)
     * that mounts for the lifetime of the chat app.
     */
    subscribeActiveRuns(options: Messages.SubscribeActiveRunsOptions): Promise<void> {
        return Messages.subscribeActiveRuns({
            baseUrl: this.config.baseUrl,
            streamEndpoint: this.config.streamEndpoint!,
        }, options);
    }
    async *sendMessage(sessionId: string, content: string, attachments: Attachment[] = [], signal?: AbortSignal, options?: SendMessageOptions): AsyncGenerator<StreamChunk> {
        this.abortController = new AbortController();
        // Link external signal to our internal controller
        let onExternalAbort: (() => void) | undefined;
        if (signal) {
            onExternalAbort = () => { this.abortController?.abort(); };
            signal.addEventListener('abort', onExternalAbort);
        }
        try {
            const innerSignal = this.abortController.signal;
            yield* Messages.sendMessage({
                callRPC: this.boundCallRPC,
                baseUrl: this.config.baseUrl,
                streamEndpoint: this.config.streamEndpoint!,
                rpcTimeoutMs: this.config.rpcTimeoutMs!,
                streamConnectTimeoutMs: this.config.streamConnectTimeoutMs,
                createHeaders: (additional) => this.createHeaders(additional),
                uploadFileFn: this.boundUploadFile,
                logAttachmentLifecycle: () => {
                    // lifecycle events handled inside AttachmentUploader
                },
            }, sessionId, content, attachments, innerSignal, options);
        }
        finally {
            if (signal && onExternalAbort) {
                signal.removeEventListener('abort', onExternalAbort);
            }
            this.abortController = null;
        }
    }
    cancelStream(): void {
        if (this.abortController) {
            this.abortController.abort();
            this.abortController = null;
        }
    }
    async submitQuestionAnswers(sessionId: string, questionId: string, answers: QuestionAnswers): Promise<{
        success: boolean;
        data?: AsyncRunAccepted;
        error?: string;
    }> {
        return Messages.submitQuestionAnswers(this.boundCallRPC, sessionId, questionId, answers);
    }
    async rejectPendingQuestion(sessionId: string): Promise<{
        success: boolean;
        data?: AsyncRunAccepted;
        error?: string;
    }> {
        return Messages.rejectPendingQuestion(this.boundCallRPC, sessionId);
    }
    // -------------------------------------------------------------------------
    // Artifact management (delegates to ArtifactManager)
    // -------------------------------------------------------------------------
    async fetchSessionArtifacts(sessionId: string, options?: {
        limit?: number;
        offset?: number;
    }): Promise<{
        artifacts: SessionArtifact[];
        hasMore?: boolean;
        nextOffset?: number;
    }> {
        return Artifacts.fetchSessionArtifacts(this.boundCallRPC, sessionId, options);
    }
    async uploadSessionArtifacts(sessionId: string, files: File[]): Promise<{
        artifacts: SessionArtifact[];
    }> {
        return Artifacts.uploadSessionArtifacts(this.boundCallRPC, sessionId, files, this.boundUploadFile);
    }
    async renameSessionArtifact(artifactId: string, name: string, description: string = ''): Promise<SessionArtifact> {
        return Artifacts.renameSessionArtifact(this.boundCallRPC, artifactId, name, description);
    }
    async deleteSessionArtifact(artifactId: string): Promise<void> {
        return Artifacts.deleteSessionArtifact(this.boundCallRPC, artifactId);
    }
}
/**
 * Factory function to create HttpDataSource
 */
export function createHttpDataSource(config: HttpDataSourceConfig): ChatDataSource {
    return new HttpDataSource(config);
}
