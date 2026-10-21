/**
 * Type definitions for BI-Chat UI components
 */
// ============================================================================
// Session Types
// ============================================================================
export interface Session {
    id: string;
    title: string;
    status: "active" | "archived";
    pinned: boolean;
    createdAt: string;
    updatedAt: string;
    owner?: SessionUser;
    isGroup?: boolean;
    memberCount?: number;
    access?: SessionAccess;
}
export interface SessionAccess {
    role: "owner" | "editor" | "viewer" | "read_all" | "none";
    source: "owner" | "member" | "permission" | "none";
    canRead: boolean;
    canWrite: boolean;
    canManageMembers: boolean;
}
// ============================================================================
// Turn-Based Architecture Types
// ============================================================================
/**
 * A conversation turn groups a user message with its assistant response.
 * This provides a cleaner mental model than flat message lists.
 */
export interface ConversationTurn {
    id: string;
    sessionId: string;
    userTurn: UserTurn;
    assistantTurn?: AssistantTurn;
    createdAt: string;
}
/**
 * Content of a user's message in a conversation turn
 */
export interface UserTurn {
    id: string;
    content: string;
    attachments: Attachment[];
    author?: SessionUser;
    createdAt: string;
}
/**
 * Assistant turn lifecycle used by renderers to distinguish completed output
 * from HITL checkpoints that require user input.
 */
export type AssistantTurnLifecycle = "complete" | "waiting_for_human_input";
/**
 * Content of an assistant's response in a conversation turn
 */
export interface AssistantTurn {
    id: string;
    role: MessageRole;
    content: string;
    explanation?: string;
    citations: Citation[];
    toolCalls?: ToolCall[];
    charts?: ChartData[];
    renderTables?: RenderTableData[];
    artifacts: Artifact[];
    codeOutputs: CodeOutput[];
    lifecycle: AssistantTurnLifecycle;
    debug?: DebugTrace;
    createdAt: string;
}
// ============================================================================
// Message Role Enum
// ============================================================================
/**
 * Role of a message in a conversation
 */
export enum MessageRole {
    User = "user",
    Assistant = "assistant",
    System = "system",
    Tool = "tool"
}
// ============================================================================
// Tool Call Types
// ============================================================================
/**
 * A tool/function call made by the assistant
 */
export interface ToolCall {
    id: string;
    name: string;
    arguments: string;
    result?: string;
    error?: string;
    durationMs?: number;
}
// ============================================================================
// Citation Types
// ============================================================================
/**
 * Citation with position information for inline replacement
 */
export interface Citation {
    id: string;
    /** Type of citation (e.g., "url_citation") */
    type: string;
    /** Title of the cited source */
    title: string;
    /** URL of the cited source */
    url: string;
    /** Starting character index in the message content where this citation is referenced */
    startIndex: number;
    /** Ending character index in the message content where this citation is referenced */
    endIndex: number;
    /** Optional excerpt from the source */
    excerpt?: string;
}
export interface Attachment {
    id?: string;
    clientKey: string;
    uploadId?: number;
    filename: string;
    mimeType: string;
    sizeBytes: number;
    base64Data?: string;
    url?: string;
    preview?: string;
}
// Image attachment with preview for MessageInput
export type ImageAttachment = Attachment & {
    base64Data: string;
    preview: string;
};
// ============================================================================
// Code Interpreter Output Types
// ============================================================================
/**
 * Output from code interpreter tool
 */
export interface CodeOutput {
    type: "image" | "text" | "error";
    content: string;
    /** File metadata for downloadable outputs */
    filename?: string;
    mimeType?: string;
    sizeBytes?: number;
}
// ============================================================================
// Message Queue Types
// ============================================================================
/**
 * Queued message for offline/loading state
 */
export interface QueuedMessage {
    content: string;
    attachments: Attachment[];
}
// ============================================================================
// Chart Types (ApexCharts format)
// ============================================================================
/**
 * Chart visualization data for ApexCharts
 */
export interface ChartData {
    /** Type of chart: line, bar, pie, area, or donut */
    chartType: "line" | "bar" | "area" | "pie" | "donut";
    /** Chart title displayed above the chart */
    title: string;
    /** Data series (multiple allowed for line/bar/area, single for pie/donut) */
    series: ChartSeries[];
    /** X-axis category labels or segment labels for pie/donut */
    labels?: string[];
    /** Hex color codes for series (e.g., '#4CAF50') */
    colors?: string[];
    /** Chart height in pixels */
    height?: number;
    /** Optional original Apex options (used by richer renderers) */
    options?: Record<string, unknown>;
    /** Optional logarithmic Y-axis hint */
    logarithmic?: boolean;
}
/**
 * A single data series in a chart
 */
export interface ChartSeries {
    /** Display name for this series */
    name: string;
    /** Numeric data values */
    data: number[];
}
export interface RenderTableExport {
    url: string;
    filename: string;
    rowCount?: number;
    fileSizeKB?: number;
}
export interface RenderTableData {
    id: string;
    title?: string;
    query: string;
    columns: string[];
    columnTypes?: string[];
    headers: string[];
    rows: unknown[][];
    totalRows: number;
    pageSize: number;
    truncated: boolean;
    truncatedReason?: string;
    export?: RenderTableExport;
    exportPrompt?: string;
}
export interface Artifact {
    type: "excel" | "pdf";
    filename: string;
    url: string;
    sizeReadable?: string;
    rowCount?: number;
    description?: string;
}
export interface SessionArtifact {
    id: string;
    sessionId: string;
    messageId?: string;
    uploadId?: number;
    type: string;
    name: string;
    description?: string;
    mimeType?: string;
    url?: string;
    sizeBytes: number;
    metadata?: Record<string, unknown>;
    createdAt: string;
}
// ============================================================================
// HITL (Human-in-the-Loop) Question Types
// ============================================================================
export interface PendingQuestion {
    id: string;
    turnId: string;
    agentName?: string;
    questions: Question[];
    status: "PENDING" | "ANSWER_SUBMITTED" | "REJECT_SUBMITTED" | "ANSWER_RESUME_FAILED" | "REJECT_RESUME_FAILED" | "ANSWERED" | "REJECTED" | "CANCELLED";
}
export interface Question {
    id: string;
    text: string;
    type: "SINGLE_CHOICE" | "MULTIPLE_CHOICE";
    options?: QuestionOption[];
    required?: boolean;
}
export interface QuestionOption {
    id: string;
    label: string;
    value: string;
}
/**
 * Answer data for a single question, including predefined options and custom "Other" text.
 */
export interface QuestionAnswerData {
    /** Selected predefined options (option IDs) */
    options: string[];
    /** Custom text entered for an "Other" answer; mutually exclusive with options */
    customText?: string;
}
/**
 * Map of question IDs to answer data.
 * Supports both multi-select options and custom "Other" text input.
 */
export interface QuestionAnswers {
    [questionId: string]: QuestionAnswerData;
}
// ---------------------------------------------------------------------------
// Activity Trace — ephemeral state during streaming
// ---------------------------------------------------------------------------
/**
 * A single step in the ephemeral activity trace shown during streaming.
 * Steps represent thinking, tool calls, or sub-agent delegations.
 */
export interface ActivityStep {
    id: string;
    type: "thinking" | "tool" | "agent_delegation";
    toolName: string;
    /** Raw tool arguments JSON string (used for label interpolation, e.g., delegation agent name). */
    arguments?: string;
    agentName?: string;
    status: "active" | "completed" | "failed";
    startedAt: number;
    completedAt?: number;
    durationMs?: number;
    error?: string;
}
// ---------------------------------------------------------------------------
// StreamEvent — discriminated union for type-safe stream handling
// ---------------------------------------------------------------------------
export type StreamEvent = {
    type: "content";
    content: string;
} | {
    type: "thinking";
    content: string;
} | {
    type: "tool_start";
    tool: StreamToolPayload;
} | {
    type: "tool_end";
    tool: StreamToolPayload;
} | {
    type: "usage";
    usage: DebugUsage;
} | {
    type: "user_message";
    sessionId: string;
} | {
    type: "interrupt";
    interrupt: StreamInterruptPayload;
    sessionId?: string;
} | {
    type: "text_block_end";
    seq: number;
} | {
    type: "done";
    sessionId?: string;
    generationMs?: number;
} | {
    type: "error";
    error: string;
};
/** Partial state when resuming a stream after refresh */
export interface StreamSnapshotPayload {
    partialContent?: string;
    partialMetadata?: Record<string, unknown>;
}
/** Active stream status for a session (from GET /stream/status) */
export interface StreamStatus {
    active: boolean;
    runId?: string;
    snapshot?: StreamSnapshotPayload;
    startedAt?: number;
}
export interface AsyncRunAccepted {
    accepted: true;
    operation: "question_submit" | "question_reject" | "session_compact";
    sessionId: string;
    runId: string;
    startedAt: number;
}
/**
 * @deprecated Use `StreamEvent` instead. `StreamChunk` is kept for backwards
 * compatibility but the flat all-optional shape is unsound.
 */
export interface StreamChunk {
    type: "chunk" | "content" | "thinking" | "tool_start" | "tool_end" | "usage" | "done" | "error" | "user_message" | "interrupt" | "snapshot" | "stream_started" | "text_block_end";
    content?: string;
    error?: string;
    /** Client-side delivery failure, distinct from a failed generation. */
    errorSource?: "transport";
    sessionId?: string;
    usage?: DebugUsage;
    tool?: StreamToolPayload;
    interrupt?: StreamInterruptPayload;
    generationMs?: number;
    timestamp?: number;
    snapshot?: StreamSnapshotPayload;
    /** Set when type is 'stream_started'; client should store for refresh-safe resume */
    runId?: string;
    /**
     * Zero-based ordinal of the assistant text segment that just ended.
     * Populated only when type === "text_block_end". Used to split the
     * accumulated assistant content into distinct blocks interleaved with
     * tool_call UI (text → tool → text → tool → final_text).
     */
    textBlockSeq?: number;
}
/**
 * Per-tenant active-run status event. Delivered via the
 * GET /bi-chat/stream/active-runs SSE endpoint. The first batch after
 * connect carries `event === "snapshot"` for each currently-running
 * session; subsequent rows carry `event === "update"` as runs
 * transition (queued → streaming → completed / cancelled / failed).
 */
export interface ActiveRunDelivery {
    event: "snapshot" | "update";
    sessionId: string;
    runId: string;
    status: "queued" | "streaming" | "completed" | "cancelled" | "failed";
    updatedAt: number;
}
export interface StreamInterruptPayload {
    checkpointId: string;
    agentName?: string;
    questions: StreamInterruptQuestion[];
}
export interface StreamInterruptQuestion {
    id: string;
    text: string;
    type: string;
    options: Array<{
        id: string;
        label: string;
    }>;
}
export interface DebugUsage {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    cachedTokens?: number;
    cost?: number;
}
export interface StreamToolPayload {
    callId?: string;
    name: string;
    arguments?: string;
    result?: string;
    error?: string;
    durationMs?: number;
    agentName?: string;
}
export interface DebugTrace {
    schemaVersion?: string;
    startedAt?: string;
    completedAt?: string;
    generationMs?: number;
    usage?: DebugUsage;
    tools: StreamToolPayload[];
    attempts?: DebugGeneration[];
    spans?: DebugSpan[];
    events?: DebugEvent[];
    traceId?: string;
    traceUrl?: string;
    sessionId?: string;
    thinking?: string;
    observationReason?: string;
}
export interface DebugGeneration {
    id?: string;
    requestId?: string;
    model?: string;
    provider?: string;
    finishReason?: string;
    promptTokens?: number;
    completionTokens?: number;
    totalTokens?: number;
    cachedTokens?: number;
    cost?: number;
    latencyMs?: number;
    input?: string;
    output?: string;
    thinking?: string;
    observationReason?: string;
    startedAt?: string;
    completedAt?: string;
    toolCalls?: StreamToolPayload[];
}
export interface DebugSpan {
    id?: string;
    parentId?: string;
    generationId?: string;
    name?: string;
    type?: string;
    status?: string;
    level?: string;
    callId?: string;
    toolName?: string;
    input?: string;
    output?: string;
    error?: string;
    durationMs?: number;
    startedAt?: string;
    completedAt?: string;
    attributes?: Record<string, unknown>;
}
export interface DebugEvent {
    id?: string;
    name?: string;
    type?: string;
    level?: string;
    message?: string;
    reason?: string;
    spanId?: string;
    generationId?: string;
    timestamp?: string;
    attributes?: Record<string, unknown>;
}
export interface DebugLimits {
    policyMaxTokens: number;
    modelMaxTokens: number;
    effectiveMaxTokens: number;
    completionReserveTokens: number;
}
export interface SessionDebugUsage {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    turnsWithUsage: number;
    latestPromptTokens: number;
    latestCompletionTokens: number;
    latestTotalTokens: number;
}
export interface SendMessageOptions {
    debugMode?: boolean;
    replaceFromMessageID?: string;
    reasoningEffort?: string;
    model?: string;
    /**
     * Client-generated idempotency key. Duplicate sends sharing the same
     * requestId within the backend's dedupe window (~30 min) converge on
     * a single server-side run. Omit to disable dedupe for a particular
     * send; the data source auto-generates one per call otherwise.
     */
    requestId?: string;
}
// ============================================================================
// Data Source Interface
// ============================================================================
export interface SessionListResult {
    sessions: Session[];
    total: number;
    hasMore: boolean;
}
export interface SessionUser {
    id: string;
    firstName: string;
    lastName: string;
    initials: string;
}
export interface SessionMember {
    user: SessionUser;
    role: "owner" | "editor" | "viewer";
    createdAt: string;
    updatedAt: string;
}
export interface SessionGroup {
    name: string;
    sessions: Session[];
}
// Re-export split interfaces for consumers that only need a subset
export type { SessionStore, MessageTransport, ArtifactStore, AdminStore, } from "./data-source";
/**
 * Full data source interface for BiChat.
 *
 * Combines session CRUD, message transport, and optional artifact/admin
 * methods. Existing implementations satisfy this without changes.
 *
 * For new code, prefer the focused interfaces (`SessionStore`,
 * `MessageTransport`, `ArtifactStore`, `AdminStore`) when you only need a
 * subset of capabilities.
 */
export interface ChatDataSource {
    // Core operations
    createSession(): Promise<Session>;
    fetchSession(id: string): Promise<{
        session: Session;
        turns: ConversationTurn[];
        pendingQuestion?: PendingQuestion | null;
    } | null>;
    fetchSessionArtifacts?(sessionId: string, options?: {
        limit?: number;
        offset?: number;
    }): Promise<{
        artifacts: SessionArtifact[];
        hasMore?: boolean;
        nextOffset?: number;
    }>;
    uploadSessionArtifacts?(sessionId: string, files: File[]): Promise<{
        artifacts: SessionArtifact[];
    }>;
    renameSessionArtifact?(artifactId: string, name: string, description?: string): Promise<SessionArtifact>;
    deleteSessionArtifact?(artifactId: string): Promise<void>;
    sendMessage(sessionId: string, content: string, attachments?: Attachment[], signal?: AbortSignal, options?: SendMessageOptions): AsyncGenerator<StreamChunk>;
    clearSessionHistory(sessionId: string): Promise<{
        success: boolean;
        deletedMessages: number;
        deletedArtifacts: number;
    }>;
    compactSessionHistory(sessionId: string): Promise<AsyncRunAccepted>;
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
    /**
     * Stops the active stream for the given session. No partial assistant message is persisted.
     * Optional for backward compatibility with data sources that do not support stop.
     */
    stopGeneration?(sessionId: string): Promise<void>;
    /**
     * Returns active stream status for the session (for refresh-safe resume).
     * Optional; if absent, no resume/passive flow is used.
     */
    getStreamStatus?(sessionId: string): Promise<StreamStatus | null>;
    /**
     * Resumes an active stream: delivers snapshot then new chunks. Use when getStreamStatus
     * reported active and the client has the same-browser run marker.
     * Optional; if absent, resume is not supported.
     */
    resumeStream?(sessionId: string, runId: string, onChunk: (chunk: StreamChunk) => void, signal?: AbortSignal): Promise<void>;
    /**
     * Tail a run via native EventSource against GET /stream/events. Honours
     * Last-Event-ID for auto-reconnect on wifi drops / tab sleep. Prefer
     * over resumeStream when connecting to a run that was started by
     * another tab (same request_id) or that needs to survive a reload
     * via cursor-based replay. Optional; data sources that don't
     * implement it fall back to resumeStream.
     */
    subscribeRunEvents?(sessionId: string, runId: string, options: {
        lastEventId?: string;
        onChunk: (chunk: StreamChunk) => void;
        onError?: (event: Event) => void;
        signal?: AbortSignal;
    }): Promise<void>;
    /**
     * Subscribe to the per-tenant active-run fan-out
     * (GET /stream/active-runs). Emits one "snapshot" event per
     * currently-running session on connect, then live "update" deltas.
     * Optional; data sources that don't implement it fall back to
     * per-session polling via getStreamStatus.
     */
    subscribeActiveRuns?(options: {
        onEvent: (event: ActiveRunDelivery) => void;
        onError?: (event: Event) => void;
        signal?: AbortSignal;
    }): Promise<void>;
    // Session management
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
    // Organization-wide features (optional)
    listUsers?(): Promise<SessionUser[]>;
    listAllSessions?(options?: {
        limit?: number;
        offset?: number;
        includeArchived?: boolean;
        userId?: string | null;
    }): Promise<{
        sessions: Session[];
        total: number;
        hasMore: boolean;
    }>;
    listSessionMembers?(sessionId: string): Promise<SessionMember[]>;
    addSessionMember?(sessionId: string, userId: string, role: "editor" | "viewer"): Promise<void>;
    updateSessionMemberRole?(sessionId: string, userId: string, role: "editor" | "viewer"): Promise<void>;
    removeSessionMember?(sessionId: string, userId: string): Promise<void>;
}
// ============================================================================
// Context Value Types
// ============================================================================
// ============================================================================
// Split Context Value Types
// ============================================================================
export interface ChatSessionStateValue {
    session: Session | null;
    currentSessionId?: string;
    fetching: boolean;
    error: string | null;
    errorRetryable: boolean;
    debugMode: boolean;
    sessionDebugUsage: SessionDebugUsage;
    debugLimits: DebugLimits | null;
    reasoningEffort: string | undefined;
    reasoningEffortOptions: string[] | undefined;
    model: string | undefined;
    setError: (error: string | null) => void;
    retryFetchSession: () => void;
    setReasoningEffort: (effort: string) => void;
    setModel: (model: string | undefined) => void;
}
export interface ChatMessagingStateValue {
    turns: ConversationTurn[];
    streamingContent: string;
    isStreaming: boolean;
    streamError: string | null;
    streamErrorRetryable: boolean;
    loading: boolean;
    pendingQuestion: PendingQuestion | null;
    codeOutputs: CodeOutput[];
    isCompacting: boolean;
    compactionSummary: string | null;
    /** Bumped when artifacts should be refetched (e.g. tool_end for artifact-producing tools). */
    artifactsInvalidationTrigger: number;
    /** Ephemeral reasoning/thinking content, cleared when final answer arrives. */
    thinkingContent: string;
    /** Ephemeral activity steps (tools, thinking, delegations), cleared on done. */
    activeSteps: ActivityStep[];
    showActivityTrace: boolean;
    showTypingIndicator: boolean;
    sendMessage: (content: string, attachments?: Attachment[]) => Promise<void>;
    handleRegenerate?: (turnId: string, model?: string) => Promise<void>;
    handleEdit?: (turnId: string, newContent: string) => Promise<void>;
    handleCopy: (text: string) => Promise<void>;
    handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void;
    handleRejectPendingQuestion: () => Promise<void>;
    retryLastMessage: () => Promise<void>;
    clearStreamError: () => void;
    cancel: () => void;
    setCodeOutputs: (outputs: CodeOutput[]) => void;
}
export interface ChatInputStateValue {
    message: string;
    inputError: string | null;
    messageQueue: QueuedMessage[];
    setMessage: (message: string) => void;
    setInputError: (error: string | null) => void;
    handleSubmit: (e: {
        preventDefault: () => void;
    }, attachments?: Attachment[]) => void;
    handleUnqueue: () => {
        content: string;
        attachments: Attachment[];
    } | null;
    enqueueMessage: (content: string, attachments: Attachment[]) => boolean;
    removeQueueItem: (index: number) => void;
    updateQueueItem: (index: number, content: string) => void;
}
// ============================================================================
// Combined Context Value (backwards compatible)
// ============================================================================
export interface ChatSessionContextValue extends ChatSessionStateValue, ChatMessagingStateValue, ChatInputStateValue {
    /**
     * @deprecated Use `retryLastMessage` from `ChatMessagingStateValue` instead.
     * This field is not populated by the current ChatMachine-based provider and
     * will always be `undefined` at runtime.
     */
    handleRetry?: () => Promise<void>;
}
// Translations
export type Translations = Record<string, string>;
// Branding
export interface ExamplePrompt {
    category: string;
    text: string;
    icon: string;
}
export interface BrandingConfig {
    appName: string;
    logoUrl?: string;
    theme?: {
        primary?: string;
        secondary?: string;
        accent?: string;
    };
    welcome?: {
        title?: string;
        description?: string;
        examplePrompts?: ExamplePrompt[];
    };
    colors?: {
        primary?: string;
        secondary?: string;
        accent?: string;
    };
    logo?: {
        src?: string;
        alt?: string;
    };
}
